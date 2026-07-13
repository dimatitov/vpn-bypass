//go:build darwin

package platform

import (
	"fmt"
	"os"
)

func RequireAdministrator() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("нужны права администратора: запусти через sudo")
	}
	return nil
}
