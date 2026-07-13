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
		return fmt.Errorf("проверка прав администратора: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(string(out))) != "true" {
		return fmt.Errorf("нужны права администратора")
	}
	return nil
}
