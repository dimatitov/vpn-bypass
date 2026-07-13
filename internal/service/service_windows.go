//go:build windows

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const windowsTaskName = "io.github.dimatitov.vpn-bypass"

type taskScheduler interface {
	Register(context.Context, string) error
	Start(context.Context) error
	Stop(context.Context) error
	Delete(context.Context) error
	Status(context.Context) (State, error)
}

type windowsManager struct {
	scheduler         taskScheduler
	binaryPath        string
	requireAdmin      func() error
	resolveExecutable func() (string, error)
	copyExecutable    func(string, string, os.FileMode) error
	remove            func(string) error
	scheduleDelete    func(string) error
}

func newManager() (Manager, error) {
	programFiles := os.Getenv("ProgramFiles")
	if programFiles == "" {
		return nil, fmt.Errorf("ProgramFiles is not set")
	}
	return &windowsManager{
		scheduler:         commandTaskScheduler{},
		binaryPath:        filepath.Join(programFiles, "vpn-bypass", "vpn-bypass.exe"),
		requireAdmin:      requireWindowsAdministrator,
		resolveExecutable: resolveExecutable,
		copyExecutable:    copyFileAtomic,
		remove:            os.Remove,
		scheduleDelete:    scheduleDeleteOnReboot,
	}, nil
}

func (m *windowsManager) Install(ctx context.Context) error {
	if err := m.requireAdmin(); err != nil {
		return err
	}
	source, err := m.resolveExecutable()
	if err != nil {
		return err
	}
	state, err := m.scheduler.Status(ctx)
	if err != nil {
		return err
	}
	if state != StateNotInstalled {
		if err := m.scheduler.Stop(ctx); err != nil {
			return fmt.Errorf("stop existing scheduled task: %w", err)
		}
	}
	if err := m.copyExecutable(source, m.binaryPath, 0755); err != nil {
		return fmt.Errorf("install executable: %w", err)
	}
	if err := m.scheduler.Register(ctx, m.binaryPath); err != nil {
		return err
	}
	return m.scheduler.Start(ctx)
}

func (m *windowsManager) Stop(ctx context.Context) error {
	if err := m.requireAdmin(); err != nil {
		return err
	}
	state, err := m.scheduler.Status(ctx)
	if err != nil {
		return err
	}
	if state != StateRunning {
		return nil
	}
	return m.scheduler.Stop(ctx)
}

func (m *windowsManager) Remove(ctx context.Context) (Removal, error) {
	if err := m.requireAdmin(); err != nil {
		return Removal{}, err
	}
	var errs []error
	if err := m.scheduler.Delete(ctx); err != nil {
		errs = append(errs, fmt.Errorf("delete scheduled task: %w", err))
	}

	if err := m.remove(m.binaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		current, resolveErr := m.resolveExecutable()
		if resolveErr == nil && strings.EqualFold(filepath.Clean(current), filepath.Clean(m.binaryPath)) {
			if scheduleErr := m.scheduleDelete(m.binaryPath); scheduleErr == nil {
				return Removal{RebootRequired: true}, errors.Join(errs...)
			} else {
				errs = append(errs, fmt.Errorf("schedule installed executable removal after reboot: %w", scheduleErr))
			}
		} else {
			errs = append(errs, fmt.Errorf("remove installed executable: %w", err))
		}
	}
	return Removal{}, errors.Join(errs...)
}

func (m *windowsManager) Status(ctx context.Context) (State, error) {
	return m.scheduler.Status(ctx)
}

func requireWindowsAdministrator() error {
	script := `([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("check administrator privileges: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if !strings.EqualFold(strings.TrimSpace(string(out)), "true") {
		return fmt.Errorf("administrator privileges are required; run from an elevated terminal")
	}
	return nil
}

type commandTaskScheduler struct{}

func (commandTaskScheduler) Register(ctx context.Context, binaryPath string) error {
	return runPowerShell(ctx, "register scheduled task", windowsTaskRegistrationScript(binaryPath))
}

func windowsTaskRegistrationScript(binaryPath string) string {
	return fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$action = New-ScheduledTaskAction -Execute %s -Argument 'watch --interval 60s'
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -MultipleInstances IgnoreNew
Register-ScheduledTask -TaskName %s -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
`, quotePowerShell(binaryPath), quotePowerShell(windowsTaskName))
}

func (commandTaskScheduler) Start(ctx context.Context) error {
	script := fmt.Sprintf("Start-ScheduledTask -TaskName %s -ErrorAction Stop", quotePowerShell(windowsTaskName))
	return runPowerShell(ctx, "start scheduled task", script)
}

func (commandTaskScheduler) Stop(ctx context.Context) error {
	script := fmt.Sprintf("Stop-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue", quotePowerShell(windowsTaskName))
	return runPowerShell(ctx, "stop scheduled task", script)
}

func (commandTaskScheduler) Delete(ctx context.Context) error {
	script := fmt.Sprintf("Unregister-ScheduledTask -TaskName %s -Confirm:$false -ErrorAction SilentlyContinue", quotePowerShell(windowsTaskName))
	return runPowerShell(ctx, "delete scheduled task", script)
}

func (commandTaskScheduler) Status(ctx context.Context) (State, error) {
	script := fmt.Sprintf(`
$task = Get-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue
if ($null -eq $task) {
  @{ installed = $false; state = '' } | ConvertTo-Json -Compress
} else {
  @{ installed = $true; state = [string]$task.State } | ConvertTo-Json -Compress
}
`, quotePowerShell(windowsTaskName))
	out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query scheduled task: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var result struct {
		Installed bool   `json:"installed"`
		State     string `json:"state"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("parse scheduled task status: %w", err)
	}
	if !result.Installed {
		return StateNotInstalled, nil
	}
	if strings.EqualFold(result.State, "Running") {
		return StateRunning, nil
	}
	return StateStopped, nil
}

func runPowerShell(ctx context.Context, operation, script string) error {
	out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", operation, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func quotePowerShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func scheduleDeleteOnReboot(path string) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	moveFileEx := kernel32.NewProc("MoveFileExW")
	pathPointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	const moveFileDelayUntilReboot = 0x4
	result, _, callErr := moveFileEx.Call(uintptr(unsafe.Pointer(pathPointer)), 0, moveFileDelayUntilReboot)
	if result == 0 {
		return callErr
	}
	return nil
}
