package actions

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/config"
	"time"
)

// InstallGnomeExtension installs a GNOME extension
func InstallGnomeExtension(ext config.GnomeExtension) error {
	fmt.Printf("Installing GNOME extension: %s\n", ext.Name)
	time.Sleep(1 * time.Second) // Simulate GNOME extension installation
	return nil
}
