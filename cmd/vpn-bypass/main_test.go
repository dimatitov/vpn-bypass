package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dimatitov/vpn-bypass/internal/service"
)

type fakeServiceManager struct {
	state      service.State
	stopErr    error
	removeErr  error
	removal    service.Removal
	operations *[]string
}

func (f *fakeServiceManager) Install(context.Context) error {
	*f.operations = append(*f.operations, "install")
	return nil
}

func (f *fakeServiceManager) Stop(context.Context) error {
	*f.operations = append(*f.operations, "stop")
	return f.stopErr
}

func (f *fakeServiceManager) Remove(context.Context) (service.Removal, error) {
	*f.operations = append(*f.operations, "remove")
	return f.removal, f.removeErr
}

func (f *fakeServiceManager) Status(context.Context) (service.State, error) {
	*f.operations = append(*f.operations, "status")
	return f.state, nil
}

func TestVersionOutput(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	version, commit, date = "v1.2.3", "abc123", "2026-07-13T10:00:00Z"
	t.Cleanup(func() { version, commit, date = oldVersion, oldCommit, oldDate })

	var out bytes.Buffer
	err := run([]string{"version"}, &out, &bytes.Buffer{}, func() (service.Manager, error) {
		t.Fatal("version must not initialize service management")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "version: v1.2.3\ncommit: abc123\ndate: 2026-07-13T10:00:00Z\n"
	if out.String() != want {
		t.Fatalf("unexpected version output:\n%s", out.String())
	}
}

func TestServiceStatusOutput(t *testing.T) {
	for _, state := range []service.State{service.StateNotInstalled, service.StateRunning, service.StateStopped} {
		t.Run(string(state), func(t *testing.T) {
			operations := []string{}
			manager := &fakeServiceManager{state: state, operations: &operations}
			var out bytes.Buffer
			err := run([]string{"service", "status"}, &out, &bytes.Buffer{}, func() (service.Manager, error) {
				return manager, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != "status: "+string(state)+"\n" {
				t.Fatalf("unexpected status output: %q", out.String())
			}
		})
	}
}

func TestInvalidServiceCommand(t *testing.T) {
	err := run([]string{"service", "restart"}, &bytes.Buffer{}, &bytes.Buffer{}, func() (service.Manager, error) {
		t.Fatal("invalid command must not initialize service management")
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "unknown service command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUninstallContinuesAndJoinsErrors(t *testing.T) {
	operations := []string{}
	stopErr := errors.New("stop failed")
	clearErr := errors.New("clear failed")
	removeErr := errors.New("remove failed")
	manager := &fakeServiceManager{
		stopErr:    stopErr,
		removeErr:  removeErr,
		operations: &operations,
	}

	_, err := uninstallService(context.Background(), manager, func() error {
		operations = append(operations, "clear")
		return clearErr
	})
	if strings.Join(operations, ",") != "stop,clear,remove" {
		t.Fatalf("unexpected operation order: %v", operations)
	}
	for _, expected := range []error{stopErr, clearErr, removeErr} {
		if !errors.Is(err, expected) {
			t.Fatalf("combined error does not contain %v: %v", expected, err)
		}
	}
}

func TestHelpIsEnglish(t *testing.T) {
	var out bytes.Buffer
	if err := run(nil, &out, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.String(), "АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЫЭЮЯабвгдежзийклмнопрстуфхцчшщыэюя") {
		t.Fatalf("help contains Cyrillic text: %s", out.String())
	}
}
