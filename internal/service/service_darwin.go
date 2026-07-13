//go:build darwin

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	macLabel      = "io.github.dimatitov.vpn-bypass"
	macBinaryPath = "/usr/local/bin/vpn-bypass"
	macPlistPath  = "/Library/LaunchDaemons/io.github.dimatitov.vpn-bypass.plist"
	macLogDir     = "/Library/Logs/vpn-bypass"
)

type launchd interface {
	Bootout(context.Context) error
	Bootstrap(context.Context) error
	Enable(context.Context) error
	Kickstart(context.Context) error
	Status(context.Context) (State, error)
}

type darwinManager struct {
	launchd           launchd
	requireAdmin      func() error
	resolveExecutable func() (string, error)
	copyExecutable    func(string, string, os.FileMode) error
	writePlist        func(string, []byte, os.FileMode) error
	chown             func(string, int, int) error
	remove            func(string) error
	mkdirAll          func(string, os.FileMode) error
}

func newManager() (Manager, error) {
	return &darwinManager{
		launchd:           commandLaunchd{},
		requireAdmin:      requireDarwinAdministrator,
		resolveExecutable: resolveExecutable,
		copyExecutable:    copyFileAtomic,
		writePlist:        writeFileAtomic,
		chown:             os.Chown,
		remove:            os.Remove,
		mkdirAll:          os.MkdirAll,
	}, nil
}

func (m *darwinManager) Install(ctx context.Context) error {
	if err := m.requireAdmin(); err != nil {
		return err
	}
	source, err := m.resolveExecutable()
	if err != nil {
		return err
	}
	if err := m.copyExecutable(source, macBinaryPath, 0755); err != nil {
		return fmt.Errorf("install executable: %w", err)
	}
	if err := m.mkdirAll(macLogDir, 0755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	if err := m.writePlist(macPlistPath, []byte(darwinPlist()), 0644); err != nil {
		return fmt.Errorf("write LaunchDaemon definition: %w", err)
	}
	if err := m.chown(macPlistPath, 0, 0); err != nil {
		return fmt.Errorf("set LaunchDaemon ownership: %w", err)
	}
	_ = m.launchd.Bootout(ctx)
	if err := m.launchd.Bootstrap(ctx); err != nil {
		return err
	}
	if err := m.launchd.Enable(ctx); err != nil {
		return err
	}
	return m.launchd.Kickstart(ctx)
}

func (m *darwinManager) Stop(ctx context.Context) error {
	if err := m.requireAdmin(); err != nil {
		return err
	}
	state, err := m.launchd.Status(ctx)
	if err != nil {
		return err
	}
	if state != StateRunning {
		return nil
	}
	return m.launchd.Bootout(ctx)
}

func (m *darwinManager) Remove(context.Context) (Removal, error) {
	if err := m.requireAdmin(); err != nil {
		return Removal{}, err
	}
	var errs []error
	for _, path := range []string{macPlistPath, macBinaryPath} {
		if err := m.remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s: %w", path, err))
		}
	}
	return Removal{}, errors.Join(errs...)
}

func (m *darwinManager) Status(ctx context.Context) (State, error) {
	return m.launchd.Status(ctx)
}

func requireDarwinAdministrator() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator privileges are required; run with sudo")
	}
	return nil
}

func darwinPlist() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>io.github.dimatitov.vpn-bypass</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/vpn-bypass</string>
    <string>watch</string>
    <string>--interval</string>
    <string>60s</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ProcessType</key><string>Background</string>
  <key>StandardOutPath</key><string>/Library/Logs/vpn-bypass/stdout.log</string>
  <key>StandardErrorPath</key><string>/Library/Logs/vpn-bypass/stderr.log</string>
</dict>
</plist>
`
}

type commandLaunchd struct{}

func (commandLaunchd) Bootout(ctx context.Context) error {
	return runLaunchctl(ctx, "bootout", "system/"+macLabel)
}

func (commandLaunchd) Bootstrap(ctx context.Context) error {
	return runLaunchctl(ctx, "bootstrap", "system", macPlistPath)
}

func (commandLaunchd) Enable(ctx context.Context) error {
	return runLaunchctl(ctx, "enable", "system/"+macLabel)
}

func (commandLaunchd) Kickstart(ctx context.Context) error {
	return runLaunchctl(ctx, "kickstart", "-k", "system/"+macLabel)
}

func (commandLaunchd) Status(ctx context.Context) (State, error) {
	out, err := exec.CommandContext(ctx, "launchctl", "print", "system/"+macLabel).CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == "state = running" {
				return StateRunning, nil
			}
		}
		return StateStopped, nil
	}

	if _, statErr := os.Stat(macPlistPath); errors.Is(statErr, os.ErrNotExist) {
		return StateNotInstalled, nil
	} else if statErr != nil {
		return "", fmt.Errorf("inspect LaunchDaemon definition: %w", statErr)
	}
	return StateStopped, nil
}

func runLaunchctl(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, "launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return nil
}
