//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"strings"
)

func RequireAdministrator() error {
	script := `([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("check administrator privileges: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(string(out))) != "true" {
		return fmt.Errorf("administrator privileges are required")
	}
	return nil
}
