package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dimatitov/vpn-bypass/internal/config"
	"github.com/dimatitov/vpn-bypass/internal/platform"
	"github.com/dimatitov/vpn-bypass/internal/state"
)

type App struct {
	configPath           string
	statePath            string
	router               platform.Router
	out                  io.Writer
	errOut               io.Writer
	requireAdministrator func() error
	lookupIP             func(context.Context, string) ([]net.IP, error)
}

type StatusInfo struct {
	Gateway    string
	Interface  string
	Routes     int
	LastSync   time.Time
	ConfigPath string
	StatePath  string
}

func New() (*App, error) {
	return NewWithWriters(os.Stdout, os.Stderr)
}

func NewWithWriters(out, errOut io.Writer) (*App, error) {
	paths, err := config.Paths()
	if err != nil {
		return nil, err
	}

	router, err := platform.NewRouter()
	if err != nil {
		return nil, err
	}

	return &App{
		configPath:           paths.Config,
		statePath:            paths.State,
		router:               router,
		out:                  out,
		errOut:               errOut,
		requireAdministrator: platform.RequireAdministrator,
		lookupIP: func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip4", host)
		},
	}, nil
}

func (a *App) Add(value string) error {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}

	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}

	if _, err := netip.ParsePrefix(value); err == nil {
		cfg.CIDRs = appendUnique(cfg.CIDRs, value)
	} else if _, err := netip.ParseAddr(value); err == nil {
		cfg.CIDRs = appendUnique(cfg.CIDRs, value+"/32")
	} else {
		cfg.Domains = appendUnique(cfg.Domains, value)
	}

	if err := config.Save(a.configPath, cfg); err != nil {
		return err
	}

	fmt.Fprintln(a.out, "Added:", value)
	return nil
}

func (a *App) Remove(value string) error {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}

	value = strings.TrimSpace(strings.ToLower(value))
	cfg.Domains = removeValue(cfg.Domains, value)
	cfg.CIDRs = removeValue(cfg.CIDRs, value)
	cfg.CIDRs = removeValue(cfg.CIDRs, value+"/32")

	if err := config.Save(a.configPath, cfg); err != nil {
		return err
	}

	fmt.Fprintln(a.out, "Removed:", value)
	return nil
}

func (a *App) List() error {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}

	fmt.Fprintln(a.out, "Domains:")
	if len(cfg.Domains) == 0 {
		fmt.Fprintln(a.out, "  —")
	}
	for _, value := range cfg.Domains {
		fmt.Fprintln(a.out, " ", value)
	}

	fmt.Fprintln(a.out, "CIDR/IP:")
	if len(cfg.CIDRs) == 0 {
		fmt.Fprintln(a.out, "  —")
	}
	for _, value := range cfg.CIDRs {
		fmt.Fprintln(a.out, " ", value)
	}

	return nil
}

func (a *App) Sync(ctx context.Context) error {
	if err := a.requireAdministrator(); err != nil {
		return err
	}

	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}

	gateway, err := a.router.DirectGateway(ctx)
	if err != nil {
		return err
	}

	desired, warnings := a.resolveDesired(ctx, cfg)
	var errs []error
	for _, warning := range warnings {
		fmt.Fprintln(a.errOut, "Warning:", warning)
		errs = append(errs, warning)
	}

	oldState, err := state.Load(a.statePath)
	if err != nil {
		return err
	}

	lastSuccessfulSync := oldState.UpdatedAt
	if oldState.Gateway != "" && (oldState.Gateway != gateway.Address || oldState.Interface != gateway.Interface) {
		remaining := make([]string, 0)
		for _, prefix := range oldState.Routes {
			if err := a.router.DeleteRoute(ctx, prefix, oldState.Interface); err != nil {
				remaining = append(remaining, prefix)
				errs = append(errs, fmt.Errorf("delete route %s from previous gateway: %w", prefix, err))
				continue
			}
			fmt.Fprintln(a.out, "DEL", prefix)
		}
		if len(remaining) != 0 {
			oldState.Routes = remaining
			if err := state.Save(a.statePath, oldState); err != nil {
				errs = append(errs, fmt.Errorf("save route state: %w", err))
			}
			return errors.Join(errs...)
		}
		oldState = state.State{}
	}

	oldSet := sliceSet(oldState.Routes)
	newSet := sliceSet(desired)
	managed := sliceSet(oldState.Routes)

	for prefix := range oldSet {
		if !newSet[prefix] {
			if err := a.router.DeleteRoute(ctx, prefix, oldState.Interface); err != nil {
				errs = append(errs, fmt.Errorf("delete route %s: %w", prefix, err))
				continue
			}
			delete(managed, prefix)
			fmt.Fprintln(a.out, "DEL", prefix)
		}
	}

	for prefix := range newSet {
		if oldSet[prefix] {
			continue
		}
		if err := a.router.AddRoute(ctx, prefix, gateway); err != nil {
			fmt.Fprintln(a.errOut, "Failed to add", prefix, ":", err)
			errs = append(errs, fmt.Errorf("add route %s: %w", prefix, err))
			continue
		}
		managed[prefix] = true
		fmt.Fprintln(a.out, "ADD", prefix, "via", gateway.Address)
	}
	managedRoutes := make([]string, 0, len(managed))
	for prefix := range managed {
		managedRoutes = append(managedRoutes, prefix)
	}
	sort.Strings(managedRoutes)

	next := state.State{
		Gateway:   gateway.Address,
		Interface: gateway.Interface,
		Routes:    managedRoutes,
		UpdatedAt: lastSuccessfulSync,
	}
	if len(errs) == 0 {
		next.UpdatedAt = time.Now()
	}

	if err := state.Save(a.statePath, next); err != nil {
		errs = append(errs, fmt.Errorf("save route state: %w", err))
	}

	fmt.Fprintf(a.out, "SYNC routes=%d gateway=%s interface=%s\n", len(managedRoutes), gateway.Address, gateway.Interface)
	return errors.Join(errs...)
}

func (a *App) Clear(ctx context.Context) error {
	if err := a.requireAdministrator(); err != nil {
		return err
	}

	st, err := state.Load(a.statePath)
	if err != nil {
		return err
	}

	remaining := make([]string, 0)
	var errs []error
	for _, prefix := range st.Routes {
		if err := a.router.DeleteRoute(ctx, prefix, st.Interface); err != nil {
			remaining = append(remaining, prefix)
			errs = append(errs, fmt.Errorf("delete route %s: %w", prefix, err))
			continue
		}
		fmt.Fprintln(a.out, "DEL", prefix)
	}

	next := st
	next.Routes = remaining
	if len(remaining) == 0 {
		next = state.State{}
	}
	if err := state.Save(a.statePath, next); err != nil {
		errs = append(errs, fmt.Errorf("save route state: %w", err))
	}

	if len(errs) == 0 {
		fmt.Fprintln(a.out, "Routes cleared")
	}
	return errors.Join(errs...)
}

func (a *App) Status(ctx context.Context) (StatusInfo, error) {
	st, err := state.Load(a.statePath)
	if err != nil {
		return StatusInfo{}, err
	}
	gateway, err := a.router.DirectGateway(ctx)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("detect direct gateway: %w", err)
	}
	return StatusInfo{
		Gateway:    gateway.Address,
		Interface:  gateway.Interface,
		Routes:     len(st.Routes),
		LastSync:   st.UpdatedAt,
		ConfigPath: a.configPath,
		StatePath:  a.statePath,
	}, nil
}

func (a *App) Doctor(ctx context.Context) error {
	var errs []error
	record := func(name string, err error, detail string) {
		if err != nil {
			fmt.Fprintf(a.out, "check: %s failed: %v\n", name, err)
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			return
		}
		fmt.Fprintf(a.out, "check: %s ok%s\n", name, detail)
	}

	adminErr := a.requireAdministrator()
	record("administrator", adminErr, "")
	cfg, configErr := config.Load(a.configPath)
	record("configuration", configErr, " path="+a.configPath)
	gateway, gatewayErr := a.router.DirectGateway(ctx)
	detail := ""
	if gatewayErr == nil {
		detail = " gateway=" + gateway.Address + " interface=" + gateway.Interface
	}
	record("direct-gateway", gatewayErr, detail)
	if configErr != nil || gatewayErr != nil {
		return errors.Join(errs...)
	}

	var host, ipv4 string
	var dnsErr error
	for _, candidate := range cfg.Domains {
		ips, err := a.lookupIP(ctx, candidate)
		if err != nil {
			dnsErr = err
			continue
		}
		for _, ip := range ips {
			if value := ip.To4(); value != nil {
				host, ipv4 = candidate, value.String()
				break
			}
		}
		if ipv4 != "" {
			break
		}
	}
	if ipv4 == "" {
		if dnsErr == nil {
			dnsErr = fmt.Errorf("no configured domain resolved to IPv4")
		}
		record("dns", dnsErr, "")
	} else {
		record("dns", nil, " domain="+host+" ip="+ipv4)
		route, err := a.router.RouteFor(ctx, ipv4)
		if err == nil && !sameGateway(route, gateway) {
			err = fmt.Errorf("%s is not routed through the direct gateway", host)
		}
		record("bypass-route", err, " domain="+host)
	}

	publicRoute, publicErr := a.router.RouteFor(ctx, "1.1.1.1")
	if publicErr == nil && sameGateway(publicRoute, gateway) {
		publicErr = fmt.Errorf("unrelated public IP uses the direct gateway; VPN route not detected")
	}
	record("vpn-route", publicErr, " target=1.1.1.1")
	return errors.Join(errs...)
}

func (a *App) Watch(ctx context.Context, interval time.Duration) error {
	if interval < 15*time.Second {
		interval = 15 * time.Second
	}

	fmt.Fprintln(a.out, "WATCH interval=", interval)
	for {
		if err := a.Sync(ctx); err != nil {
			fmt.Fprintln(a.errOut, "SYNC error:", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (a *App) resolveDesired(ctx context.Context, cfg config.Config) ([]string, []error) {
	set := map[string]bool{}
	var warnings []error

	for _, domain := range cfg.Domains {
		ips, err := a.lookupIP(ctx, domain)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("%s: %w", domain, err))
			continue
		}

		for _, ip := range ips {
			if v := ip.To4(); v != nil {
				set[v.String()+"/32"] = true
			}
		}
	}

	for _, value := range cfg.CIDRs {
		if addr, err := netip.ParseAddr(value); err == nil {
			set[addr.String()+"/32"] = true
			continue
		}

		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("invalid CIDR %s", value))
			continue
		}
		if prefix.Addr().Is4() {
			set[prefix.Masked().String()] = true
		}
	}

	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, warnings
}

func sameGateway(first, second platform.Gateway) bool {
	return first.Address == second.Address && first.Interface == second.Interface
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func removeValue(values []string, value string) []string {
	result := values[:0]
	for _, current := range values {
		if current != value {
			result = append(result, current)
		}
	}
	return result
}

func sliceSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
