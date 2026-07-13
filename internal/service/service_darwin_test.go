//go:build darwin

package service

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

type fakeLaunchd struct {
	state      State
	operations []string
}

func (f *fakeLaunchd) Bootout(context.Context) error {
	f.operations = append(f.operations, "bootout")
	return nil
}

func (f *fakeLaunchd) Bootstrap(context.Context) error {
	f.operations = append(f.operations, "bootstrap")
	return nil
}

func (f *fakeLaunchd) Enable(context.Context) error {
	f.operations = append(f.operations, "enable")
	return nil
}

func (f *fakeLaunchd) Kickstart(context.Context) error {
	f.operations = append(f.operations, "kickstart")
	return nil
}

func (f *fakeLaunchd) Status(context.Context) (State, error) {
	f.operations = append(f.operations, "status")
	return f.state, nil
}

func TestDarwinInstallUsesStructuredLaunchdLifecycle(t *testing.T) {
	launchd := &fakeLaunchd{state: StateStopped}
	var copiedSource, copiedDestination, plist string
	manager := &darwinManager{
		launchd:           launchd,
		requireAdmin:      func() error { return nil },
		resolveExecutable: func() (string, error) { return "/Cellar/vpn-bypass/1/bin/vpn-bypass", nil },
		copyExecutable: func(source, destination string, _ os.FileMode) error {
			copiedSource, copiedDestination = source, destination
			return nil
		},
		writePlist: func(_ string, data []byte, _ os.FileMode) error {
			plist = string(data)
			return nil
		},
		chown:    func(string, int, int) error { return nil },
		mkdirAll: func(string, os.FileMode) error { return nil },
		remove:   func(string) error { return nil },
	}
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if copiedSource != "/Cellar/vpn-bypass/1/bin/vpn-bypass" || copiedDestination != macBinaryPath {
		t.Fatalf("unexpected copy: %s -> %s", copiedSource, copiedDestination)
	}
	wantOperations := []string{"bootout", "bootstrap", "enable", "kickstart"}
	if !reflect.DeepEqual(launchd.operations, wantOperations) {
		t.Fatalf("unexpected lifecycle: %v", launchd.operations)
	}
	for _, expected := range []string{macLabel, macBinaryPath, "<string>watch</string>", "<string>60s</string>", macLogDir + "/stdout.log", macLogDir + "/stderr.log"} {
		if !strings.Contains(plist, expected) {
			t.Fatalf("plist does not contain %q", expected)
		}
	}
}

func TestDarwinStopIsIdempotent(t *testing.T) {
	launchd := &fakeLaunchd{state: StateNotInstalled}
	manager := &darwinManager{launchd: launchd, requireAdmin: func() error { return nil }}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(launchd.operations, []string{"status"}) {
		t.Fatalf("unexpected lifecycle: %v", launchd.operations)
	}
}

func TestDarwinStopBootsOutStoppedJob(t *testing.T) {
	launchd := &fakeLaunchd{state: StateStopped}
	manager := &darwinManager{launchd: launchd, requireAdmin: func() error { return nil }}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(launchd.operations, []string{"status", "bootout"}) {
		t.Fatalf("unexpected lifecycle: %v", launchd.operations)
	}
}

func TestDarwinRemoveContinuesAfterErrors(t *testing.T) {
	removeErr := errors.New("permission denied")
	var paths []string
	manager := &darwinManager{
		requireAdmin: func() error { return nil },
		remove: func(path string) error {
			paths = append(paths, path)
			return removeErr
		},
	}
	_, err := manager.Remove(context.Background())
	if !reflect.DeepEqual(paths, []string{macPlistPath, macBinaryPath}) {
		t.Fatalf("unexpected removal paths: %v", paths)
	}
	if !errors.Is(err, removeErr) {
		t.Fatalf("unexpected error: %v", err)
	}
}
