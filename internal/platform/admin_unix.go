//go:build darwin

package platform

import (
	"fmt"
	"os"
)

func RequireAdministrator() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator privileges are required; run with sudo")
	}
	return nil
}
