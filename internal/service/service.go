package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type State string

const (
	StateNotInstalled State = "not-installed"
	StateRunning      State = "running"
	StateStopped      State = "stopped"
)

type Removal struct {
	RebootRequired bool
}

// Manager installs and manages the background watcher for the current OS.
type Manager interface {
	Install(context.Context) error
	Stop(context.Context) error
	Remove(context.Context) (Removal, error)
	Status(context.Context) (State, error)
}

// New returns the platform-specific service manager.
func New() (Manager, error) {
	return newManager()
}

func resolveExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	return resolveExecutablePath(executable)
}

func resolveExecutablePath(executable string) (string, error) {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlinks: %w", err)
	}
	return filepath.Abs(resolved)
}

func copyFileAtomic(source, destination string, mode os.FileMode) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source executable: %w", err)
	}
	if destinationInfo, statErr := os.Stat(destination); statErr == nil && os.SameFile(sourceInfo, destinationInfo) {
		return nil
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat installed executable: %w", statErr)
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("create executable directory: %w", err)
	}

	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source executable: %w", err)
	}
	defer sourceFile.Close()

	temporary, err := os.CreateTemp(filepath.Dir(destination), ".vpn-bypass-*")
	if err != nil {
		return fmt.Errorf("create temporary executable: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if _, err := io.Copy(temporary, sourceFile); err != nil {
		temporary.Close()
		return fmt.Errorf("copy executable: %w", err)
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("set executable permissions: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close executable: %w", err)
	}
	if err := replaceFile(temporaryName, destination); err != nil {
		return fmt.Errorf("replace installed executable: %w", err)
	}
	return nil
}

func writeFileAtomic(destination string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".vpn-bypass-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceFile(temporaryName, destination)
}
