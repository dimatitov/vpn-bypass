package app

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dimatitov/vpn-bypass/internal/platform"
	"github.com/dimatitov/vpn-bypass/internal/state"
)

type fakeRouter struct {
	deleted   []string
	deleteErr map[string]error
}

func (f *fakeRouter) DirectGateway(context.Context) (platform.Gateway, error) {
	return platform.Gateway{}, nil
}

func (f *fakeRouter) RouteFor(context.Context, string) (platform.Gateway, error) {
	return platform.Gateway{}, nil
}

func (f *fakeRouter) AddRoute(context.Context, string, platform.Gateway) error {
	return nil
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
