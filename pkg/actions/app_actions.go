package actions

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/config"
	"time"
)

// InstallApp installs an application
func InstallApp(app config.App) error {
	fmt.Printf("Installing app: %s\n", app.Name)
	time.Sleep(1 * time.Second) // Simulate app installation
	return nil
}
