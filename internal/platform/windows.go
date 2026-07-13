//go:build windows

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type windowsRouter struct{}

func (windowsRouter) DirectGateway(ctx context.Context) (Gateway, error) {
	script := `
$routes = Get-NetRoute -AddressFamily IPv4 -DestinationPrefix "0.0.0.0/0" |
  Where-Object { $_.NextHop -ne "0.0.0.0" } |
  Sort-Object RouteMetric, InterfaceMetric
foreach ($r in $routes) {
  $a = Get-NetAdapter -InterfaceIndex $r.InterfaceIndex -ErrorAction SilentlyContinue
  if ($a -and $a.Status -eq "Up" -and $a.InterfaceDescription -notmatch "TAP|TUN|OpenVPN|WireGuard|Wintun") {
    [PSCustomObject]@{
      Address = $r.NextHop
      Interface = [string]$r.InterfaceIndex
    } | ConvertTo-Json -Compress
    break
  }
}`
	return runPowerShellGateway(ctx, script)
}

func (windowsRouter) RouteFor(ctx context.Context, ip string) (Gateway, error) {
	script := fmt.Sprintf(`
$r = Find-NetRoute -RemoteIPAddress "%s" |
  Sort-Object RouteMetric, InterfaceMetric |
  Select-Object -First 1
[PSCustomObject]@{
  Address = $r.NextHop
  Interface = [string]$r.InterfaceIndex
} | ConvertTo-Json -Compress`, ip)
	return runPowerShellGateway(ctx, script)
}

func runPowerShellGateway(ctx context.Context, script string) (Gateway, error) {
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil {
		return Gateway{}, fmt.Errorf("powershell: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var result Gateway
	if err := json.Unmarshal(out, &result); err != nil {
		return Gateway{}, fmt.Errorf("разбор PowerShell: %w", err)
	}
	if result.Address == "" || result.Interface == "" {
		return Gateway{}, fmt.Errorf("шлюз не найден")
	}
	return result, nil
}

func (windowsRouter) AddRoute(ctx context.Context, prefix string, gateway Gateway) error {
	iface, err := strconv.Atoi(gateway.Interface)
	if err != nil {
		return err
	}

	script := fmt.Sprintf(
		`New-NetRoute -DestinationPrefix "%s" -InterfaceIndex %d -NextHop "%s" -RouteMetric 1 -PolicyStore ActiveStore -ErrorAction Stop`,
		prefix,
		iface,
		gateway.Address,
	)

	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil && !strings.Contains(strings.ToLower(string(out)), "already exists") {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (windowsRouter) DeleteRoute(ctx context.Context, prefix string, interfaceName string) error {
	script := fmt.Sprintf(
		`Remove-NetRoute -DestinationPrefix "%s" -InterfaceIndex %s -Confirm:$false -ErrorAction SilentlyContinue`,
		prefix,
		interfaceName,
	)
	_, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script).CombinedOutput()
	return err
}
