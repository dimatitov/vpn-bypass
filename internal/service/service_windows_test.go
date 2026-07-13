//go:build windows

package service

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

type fakeTaskScheduler struct {
	state      State
	operations []string
}

func (f *fakeTaskScheduler) Register(_ context.Context, path string) error {
	f.operations = append(f.operations, "register:"+path)
	return nil
}

func (f *fakeTaskScheduler) Start(context.Context) error {
	f.operations = append(f.operations, "start")
	return nil
}

func (f *fakeTaskScheduler) Stop(context.Context) error {
	f.operations = append(f.operations, "stop")
	return nil
}

func (f *fakeTaskScheduler) Delete(context.Context) error {
	f.operations = append(f.operations, "delete")
	return nil
}

func (f *fakeTaskScheduler) Status(context.Context) (State, error) {
	f.operations = append(f.operations, "status")
	return f.state, nil
}

func TestWindowsInstallStopsExistingTaskAndStartsReplacement(t *testing.T) {
	scheduler := &fakeTaskScheduler{state: StateRunning}
	manager := &windowsManager{
		scheduler:         scheduler,
		binaryPath:        `C:\Program Files\vpn-bypass\vpn-bypass.exe`,
		requireAdmin:      func() error { return nil },
		resolveExecutable: func() (string, error) { return `C:\Downloads\vpn-bypass.exe`, nil },
		copyExecutable: func(source, destination string, _ os.FileMode) error {
			scheduler.operations = append(scheduler.operations, "copy:"+source+":"+destination)
			return nil
		},
	}
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"status",
		"stop",
		`copy:C:\Downloads\vpn-bypass.exe:C:\Program Files\vpn-bypass\vpn-bypass.exe`,
		`register:C:\Program Files\vpn-bypass\vpn-bypass.exe`,
		"start",
	}
	if !reflect.DeepEqual(scheduler.operations, want) {
		t.Fatalf("unexpected lifecycle: %v", scheduler.operations)
	}
}

func TestWindowsRemoveSchedulesSelfDeletion(t *testing.T) {
	scheduler := &fakeTaskScheduler{}
	sharingViolation := errors.New("sharing violation")
	var scheduled string
	manager := &windowsManager{
		scheduler:         scheduler,
		binaryPath:        `C:\Program Files\vpn-bypass\vpn-bypass.exe`,
		requireAdmin:      func() error { return nil },
		resolveExecutable: func() (string, error) { return `C:\Program Files\vpn-bypass\vpn-bypass.exe`, nil },
		remove:            func(string) error { return sharingViolation },
		scheduleDelete: func(path string) error {
			scheduled = path
			return nil
		},
	}
	removal, err := manager.Remove(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !removal.RebootRequired || scheduled != manager.binaryPath {
		t.Fatalf("unexpected removal result: %+v, %q", removal, scheduled)
	}
	if !reflect.DeepEqual(scheduler.operations, []string{"delete"}) {
		t.Fatalf("unexpected lifecycle: %v", scheduler.operations)
	}
}

func TestWindowsTaskDefinition(t *testing.T) {
	path := `C:\Program Files\vpn-bypass\vpn-bypass.exe`
	quoted := quotePowerShell(path)
	if quoted != `'C:\Program Files\vpn-bypass\vpn-bypass.exe'` {
		t.Fatalf("unexpected quoted path: %s", quoted)
	}
	for _, value := range []string{"SYSTEM", "Highest", "AtStartup", "ExecutionTimeLimit", "RestartCount", "MultipleInstances"} {
		if !strings.Contains(windowsTaskRegistrationScript(path), value) {
			t.Fatalf("task definition does not contain %q", value)
		}
	}
}
