package app

import (
	"context"
	"fmt"
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
	configPath string
	statePath  string
	router     platform.Router
}

func New() (*App, error) {
	paths, err := config.Paths()
	if err != nil {
		return nil, err
	}

	router, err := platform.NewRouter()
	if err != nil {
		return nil, err
	}

	return &App{
		configPath: paths.Config,
		statePath:  paths.State,
		router:     router,
	}, nil
}

func (a *App) Add(value string) error {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}

	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return fmt.Errorf("пустое значение")
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

	fmt.Println("Добавлено:", value)
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

	fmt.Println("Удалено:", value)
	return nil
}

func (a *App) List() error {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}

	fmt.Println("Домены:")
	if len(cfg.Domains) == 0 {
		fmt.Println("  —")
	}
	for _, value := range cfg.Domains {
		fmt.Println(" ", value)
	}

	fmt.Println("CIDR/IP:")
	if len(cfg.CIDRs) == 0 {
		fmt.Println("  —")
	}
	for _, value := range cfg.CIDRs {
		fmt.Println(" ", value)
	}

	return nil
}

func (a *App) Sync(ctx context.Context) error {
	if err := platform.RequireAdministrator(); err != nil {
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

	desired, warnings := resolveDesired(cfg)
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "Предупреждение:", warning)
	}

	oldState, err := state.Load(a.statePath)
	if err != nil {
		return err
	}

	if oldState.Gateway != "" && oldState.Gateway != gateway.Address {
		for _, prefix := range oldState.Routes {
			_ = a.router.DeleteRoute(ctx, prefix, oldState.Interface)
		}
		oldState.Routes = nil
	}

	oldSet := sliceSet(oldState.Routes)
	newSet := sliceSet(desired)

	for prefix := range oldSet {
		if !newSet[prefix] {
			_ = a.router.DeleteRoute(ctx, prefix, gateway.Interface)
			fmt.Println("DEL", prefix)
		}
	}

	for prefix := range newSet {
		if oldSet[prefix] {
			continue
		}
		if err := a.router.AddRoute(ctx, prefix, gateway); err != nil {
			fmt.Fprintln(os.Stderr, "Ошибка добавления", prefix, ":", err)
			continue
		}
		fmt.Println("ADD", prefix, "via", gateway.Address)
	}

	next := state.State{
		Gateway:   gateway.Address,
		Interface: gateway.Interface,
		Routes:    desired,
		UpdatedAt: time.Now(),
	}

	if err := state.Save(a.statePath, next); err != nil {
		return err
	}

	fmt.Printf("SYNC routes=%d gateway=%s interface=%s\n", len(desired), gateway.Address, gateway.Interface)
	return nil
}

func (a *App) Clear(ctx context.Context) error {
	if err := platform.RequireAdministrator(); err != nil {
		return err
	}

	st, err := state.Load(a.statePath)
	if err != nil {
		return err
	}

	for _, prefix := range st.Routes {
		_ = a.router.DeleteRoute(ctx, prefix, st.Interface)
		fmt.Println("DEL", prefix)
	}

	if err := state.Save(a.statePath, state.State{}); err != nil {
		return err
	}

	fmt.Println("Маршруты удалены")
	return nil
}

func (a *App) Status() error {
	st, err := state.Load(a.statePath)
	if err != nil {
		return err
	}

	fmt.Println("Обычный шлюз:", emptyDash(st.Gateway))
	fmt.Println("Интерфейс:", emptyDash(st.Interface))
	fmt.Println("Маршрутов:", len(st.Routes))
	if !st.UpdatedAt.IsZero() {
		fmt.Println("Последнее обновление:", st.UpdatedAt.Format(time.RFC3339))
	}
	return nil
}

func (a *App) Doctor(ctx context.Context) error {
	gateway, err := a.router.DirectGateway(ctx)
	if err != nil {
		return err
	}

	fmt.Println("DIRECT gateway:", gateway.Address)
	fmt.Println("DIRECT interface:", gateway.Interface)

	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}

	if len(cfg.Domains) == 0 {
		fmt.Println("Домены: не настроены")
		return nil
	}

	host := cfg.Domains[0]
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		fmt.Println(host, "DNS: ошибка")
		return nil
	}

	var ipv4 string
	for _, ip := range ips {
		if v := ip.To4(); v != nil {
			ipv4 = v.String()
			break
		}
	}

	if ipv4 == "" {
		fmt.Println(host, "IPv4: не найден")
		return nil
	}

	route, err := a.router.RouteFor(ctx, ipv4)
	if err != nil {
		fmt.Println(host, ipv4, "route: ошибка:", err)
		return nil
	}

	fmt.Printf("%s %s -> gateway=%s interface=%s\n", host, ipv4, route.Address, route.Interface)
	if route.Address == gateway.Address {
		fmt.Println("DIRECT: OK")
	} else {
		fmt.Println("DIRECT: маршрут ещё не установлен; запусти sync")
	}

	return nil
}

func (a *App) Watch(ctx context.Context, interval time.Duration) error {
	if interval < 15*time.Second {
		interval = 15 * time.Second
	}

	fmt.Println("WATCH interval=", interval)
	for {
		if err := a.Sync(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "SYNC error:", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func resolveDesired(cfg config.Config) ([]string, []error) {
	set := map[string]bool{}
	var warnings []error

	for _, domain := range cfg.Domains {
		ips, err := net.LookupIP(domain)
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
			warnings = append(warnings, fmt.Errorf("неверный CIDR %s", value))
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

func emptyDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
