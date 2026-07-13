package service

import "fmt"

// Manager installs and manages the background watcher for the current OS.
type Manager interface {
	Install() error
	Uninstall() error
	Status() error
}

// New returns the platform-specific service manager.
func New() (Manager, error) {
	return newManager()
}

func unsupported(osName string) error {
	return fmt.Errorf("service management is not supported on %s", osName)
}
