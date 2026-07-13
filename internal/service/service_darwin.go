//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	macLabel      = "io.github.dimatitov.vpn-bypass"
	macBinaryPath = "/usr/local/bin/vpn-bypass"
	macPlistPath  = "/Library/LaunchDaemons/io.github.dimatitov.vpn-bypass.plist"
	macLogDir     = "/Library/Logs/vpn-bypass"
)

type darwinManager struct{}

func newManager() (Manager, error) { return darwinManager{}, nil }

func (darwinManager) Install() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator privileges are required; run with sudo")
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}

	if err := copyFile(exe, macBinaryPath, 0755); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	if err := os.MkdirAll(macLogDir, 0755); err != nil {
		return err
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>watch</string>
    <string>--interval</string>
    <string>60s</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s/stdout.log</string>
  <key>StandardErrorPath</key><string>%s/stderr.log</string>
</dict>
</plist>
`, macLabel, macBinaryPath, macLogDir, macLogDir)

	if err := os.WriteFile(macPlistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("write launchd plist: %w", err)
	}

	_ = exec.Command("launchctl", "bootout", "system/"+macLabel).Run()
	if out, err := exec.Command("launchctl", "bootstrap", "system", macPlistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("launchctl", "enable", "system/"+macLabel).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl enable: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("launchctl", "kickstart", "-k", "system/"+macLabel).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl kickstart: %w: %s", err, strings.TrimSpace(string(out)))
	}

	fmt.Println("Service installed and started.")
	return nil
}

func (darwinManager) Uninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator privileges are required; run with sudo")
	}
	_ = exec.Command("launchctl", "bootout", "system/"+macLabel).Run()
	_ = os.Remove(macPlistPath)
	_ = os.Remove(macBinaryPath)
	fmt.Println("Service uninstalled.")
	return nil
}

func (darwinManager) Status() error {
	out, err := exec.Command("launchctl", "print", "system/"+macLabel).CombinedOutput()
	if err != nil {
		fmt.Println("Service is not installed or not running.")
		return nil
	}
	fmt.Print(string(out))
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}
