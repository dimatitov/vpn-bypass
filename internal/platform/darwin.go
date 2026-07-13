//go:build darwin

package platform

import (
	"bufio"
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

type darwinRouter struct{}

func (darwinRouter) DirectGateway(ctx context.Context) (Gateway, error) {
	return parseDarwinGateway(ctx, "default")
}

func (darwinRouter) RouteFor(ctx context.Context, ip string) (Gateway, error) {
	return parseDarwinGateway(ctx, ip)
}

func parseDarwinGateway(ctx context.Context, target string) (Gateway, error) {
	out, err := exec.CommandContext(ctx, "route", "-n", "get", target).CombinedOutput()
	if err != nil {
		return Gateway{}, fmt.Errorf("route get %s: %w: %s", target, err, strings.TrimSpace(string(out)))
	}

	var result Gateway
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), ":")
		if !ok {
			continue
		}
		switch key {
		case "gateway":
			result.Address = strings.TrimSpace(value)
		case "interface":
			result.Interface = strings.TrimSpace(value)
		}
	}

	if result.Address == "" || result.Interface == "" {
		return Gateway{}, fmt.Errorf("не удалось определить шлюз для %s", target)
	}
	return result, nil
}

func (darwinRouter) AddRoute(ctx context.Context, prefix string, gateway Gateway) error {
	network, err := netip.ParsePrefix(prefix)
	if err != nil {
		return err
	}

	var args []string
	if network.Bits() == 32 {
		args = []string{"-n", "add", "-host", network.Addr().String(), gateway.Address}
	} else {
		args = []string{"-n", "add", "-net", network.String(), gateway.Address}
	}

	out, err := exec.CommandContext(ctx, "route", args...).CombinedOutput()
	if err != nil && !strings.Contains(strings.ToLower(string(out)), "file exists") {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (darwinRouter) DeleteRoute(ctx context.Context, prefix string, _ string) error {
	network, err := netip.ParsePrefix(prefix)
	if err != nil {
		return err
	}

	var args []string
	if network.Bits() == 32 {
		args = []string{"-n", "delete", "-host", network.Addr().String()}
	} else {
		args = []string{"-n", "delete", "-net", network.String()}
	}

	_, err = exec.CommandContext(ctx, "route", args...).CombinedOutput()
	return err
}
