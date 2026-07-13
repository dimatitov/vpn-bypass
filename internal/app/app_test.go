package app

import (
	"bytes"
	"context"
	"errors"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dimatitov/vpn-bypass/internal/config"
	"github.com/dimatitov/vpn-bypass/internal/platform"
	"github.com/dimatitov/vpn-bypass/internal/state"
)

type fakeRouter struct {
	gateway   platform.Gateway
	added     []string
	deleted   []string
	addErr    map[string]error
	deleteErr map[string]error
	routes    map[string]platform.Gateway
	routeErr  map[string]error
}

func (f *fakeRouter) DirectGateway(context.Context) (platform.Gateway, error) {
	return f.gateway, nil
}

func (f *fakeRouter) RouteFor(_ context.Context, target string) (platform.Gateway, error) {
	return f.routes[target], f.routeErr[target]
}

func (f *fakeRouter) AddRoute(_ context.Context, prefix string, _ platform.Gateway) error {
	f.added = append(f.added, prefix)
	return f.addErr[prefix]
}

func TestSyncDoesNotRecordFailedAdditions(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.json")
	statePath := filepath.Join(directory, "state.json")
	if err := config.Save(configPath, config.Config{CIDRs: []string{"198.51.100.1/32"}}); err != nil {
		t.Fatal(err)
	}
	addErr := errors.New("route rejected")
	router := &fakeRouter{
		gateway: platform.Gateway{Address: "192.0.2.1", Interface: "en0"},
		addErr:  map[string]error{"198.51.100.1/32": addErr},
	}
	application := &App{
		configPath:           configPath,
		statePath:            statePath,
		router:               router,
		out:                  &bytes.Buffer{},
		errOut:               &bytes.Buffer{},
		requireAdministrator: func() error { return nil },
	}

	err := application.Sync(context.Background())
	if !errors.Is(err, addErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(router.added, []string{"198.51.100.1/32"}) {
		t.Fatalf("unexpected add attempts: %v", router.added)
	}
	saved, err := state.Load(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Routes) != 0 {
		t.Fatalf("failed addition was recorded: %+v", saved)
	}
	if !saved.UpdatedAt.IsZero() {
		t.Fatalf("failed sync changed last successful sync: %+v", saved)
	}
}

func TestSuccessfulSyncUpdatesLastSuccessfulSync(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.json")
	statePath := filepath.Join(directory, "state.json")
	if err := config.Save(configPath, config.Config{CIDRs: []string{"198.51.100.1/32"}}); err != nil {
		t.Fatal(err)
	}
	router := &fakeRouter{
		gateway: platform.Gateway{Address: "192.0.2.1", Interface: "en0"},
		addErr:  map[string]error{},
	}
	application := &App{
		configPath:           configPath,
		statePath:            statePath,
		router:               router,
		out:                  &bytes.Buffer{},
		errOut:               &bytes.Buffer{},
		requireAdministrator: func() error { return nil },
	}
	if err := application.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved, err := state.Load(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.UpdatedAt.IsZero() {
		t.Fatalf("successful sync did not update timestamp: %+v", saved)
	}
}

func TestSyncRetainsFailedDeletions(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.json")
	statePath := filepath.Join(directory, "state.json")
	if err := config.Save(configPath, config.Config{}); err != nil {
		t.Fatal(err)
	}
	recorded := state.State{
		Gateway:   "192.0.2.1",
		Interface: "en0",
		Routes:    []string{"198.51.100.1/32"},
		UpdatedAt: time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC),
	}
	if err := state.Save(statePath, recorded); err != nil {
		t.Fatal(err)
	}
	deleteErr := errors.New("route is busy")
	router := &fakeRouter{
		gateway:   platform.Gateway{Address: "192.0.2.1", Interface: "en0"},
		addErr:    map[string]error{},
		deleteErr: map[string]error{"198.51.100.1/32": deleteErr},
	}
	application := &App{
		configPath:           configPath,
		statePath:            statePath,
		router:               router,
		out:                  &bytes.Buffer{},
		errOut:               &bytes.Buffer{},
		requireAdministrator: func() error { return nil },
	}

	err := application.Sync(context.Background())
	if !errors.Is(err, deleteErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(router.deleted, recorded.Routes) {
		t.Fatalf("unexpected delete attempts: %v", router.deleted)
	}
	saved, err := state.Load(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.Routes, recorded.Routes) {
		t.Fatalf("failed deletion was forgotten: %+v", saved)
	}
	if !saved.UpdatedAt.Equal(recorded.UpdatedAt) {
		t.Fatalf("failed sync changed last successful sync: %+v", saved)
	}
}

func TestStatusReturnsRuntimeAndPathInformation(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	lastSync := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	if err := state.Save(statePath, state.State{Routes: []string{"198.51.100.1/32"}, UpdatedAt: lastSync}); err != nil {
		t.Fatal(err)
	}
	application := &App{
		configPath: "/config.json",
		statePath:  statePath,
		router: &fakeRouter{gateway: platform.Gateway{
			Address: "192.0.2.1", Interface: "en0",
		}},
	}
	status, err := application.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Gateway != "192.0.2.1" || status.Interface != "en0" || status.Routes != 1 || !status.LastSync.Equal(lastSync) || status.ConfigPath != "/config.json" || status.StatePath != statePath {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestDoctorPassesCompleteRoutingChecks(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(configPath, config.Config{Domains: []string{"example.com"}}); err != nil {
		t.Fatal(err)
	}
	direct := platform.Gateway{Address: "192.0.2.1", Interface: "en0"}
	vpn := platform.Gateway{Address: "10.0.0.1", Interface: "utun0"}
	var out bytes.Buffer
	application := &App{
		configPath: configPath,
		router: &fakeRouter{
			gateway: direct,
			routes: map[string]platform.Gateway{
				"198.51.100.10": direct,
				"1.1.1.1":       vpn,
			},
			routeErr: map[string]error{},
		},
		out:                  &out,
		requireAdministrator: func() error { return nil },
		lookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("198.51.100.10")}, nil
		},
	}
	if err := application.Doctor(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, check := range []string{"administrator", "configuration", "direct-gateway", "dns", "bypass-route", "vpn-route"} {
		if !strings.Contains(out.String(), "check: "+check+" ok") {
			t.Fatalf("missing successful %s check:\n%s", check, out.String())
		}
	}
}

func TestDoctorFailsWhenVPNRouteIsNotDetected(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(configPath, config.Config{Domains: []string{"example.com"}}); err != nil {
		t.Fatal(err)
	}
	direct := platform.Gateway{Address: "192.0.2.1", Interface: "en0"}
	application := &App{
		configPath: configPath,
		router: &fakeRouter{
			gateway:  direct,
			routes:   map[string]platform.Gateway{"198.51.100.10": direct, "1.1.1.1": direct},
			routeErr: map[string]error{},
		},
		out:                  &bytes.Buffer{},
		requireAdministrator: func() error { return nil },
		lookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("198.51.100.10")}, nil
		},
	}
	if err := application.Doctor(context.Background()); err == nil {
		t.Fatal("expected doctor to fail without a VPN route")
	}
}

func (f *fakeRouter) DeleteRoute(_ context.Context, prefix, _ string) error {
	f.deleted = append(f.deleted, prefix)
	return f.deleteErr[prefix]
}

func TestClearDeletesOnlyRecordedRoutesAndRetainsFailures(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	recorded := state.State{
		Gateway:   "192.0.2.1",
		Interface: "en0",
		Routes:    []string{"198.51.100.1/32", "203.0.113.0/24"},
	}
	if err := state.Save(statePath, recorded); err != nil {
		t.Fatal(err)
	}
	deleteErr := errors.New("route is busy")
	router := &fakeRouter{deleteErr: map[string]error{"203.0.113.0/24": deleteErr}}
	application := &App{
		statePath:            statePath,
		router:               router,
		out:                  &bytes.Buffer{},
		errOut:               &bytes.Buffer{},
		requireAdministrator: func() error { return nil },
	}

	err := application.Clear(context.Background())
	if !errors.Is(err, deleteErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(router.deleted, recorded.Routes) {
		t.Fatalf("deleted routes differ from recorded routes: %v", router.deleted)
	}
	remaining, err := state.Load(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(remaining.Routes, []string{"203.0.113.0/24"}) {
		t.Fatalf("failed routes were not retained: %+v", remaining)
	}
	if remaining.Gateway != recorded.Gateway || remaining.Interface != recorded.Interface {
		t.Fatalf("route metadata was not retained: %+v", remaining)
	}
}

func TestClearResetsStateAfterSuccessfulCleanup(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := state.Save(statePath, state.State{Routes: []string{"198.51.100.1/32"}}); err != nil {
		t.Fatal(err)
	}
	router := &fakeRouter{deleteErr: map[string]error{}}
	application := &App{
		statePath:            statePath,
		router:               router,
		out:                  &bytes.Buffer{},
		errOut:               &bytes.Buffer{},
		requireAdministrator: func() error { return nil },
	}
	if err := application.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	cleared, err := state.Load(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared.Routes) != 0 || cleared.Gateway != "" || cleared.Interface != "" {
		t.Fatalf("state was not reset: %+v", cleared)
	}
}
