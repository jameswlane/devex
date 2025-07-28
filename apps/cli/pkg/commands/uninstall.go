package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

func NewUninstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var appName string

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall applications from your development environment",
		Long: `The uninstall command removes applications that were previously installed by DevEx.

It supports uninstalling:
  • Individual applications by name
  • Applications by category (future feature)
  • All applications (future feature)

The uninstall process includes:
  • Removing the application via the appropriate package manager
  • Cleaning up configuration files (optional)
  • Removing dependencies that are no longer needed (optional)
  • Updating the local installation database

Note: This is a basic implementation. Advanced features like dependency
cleanup and configuration removal will be implemented in future versions.`,
		Example: `  # Uninstall a specific application
  devex uninstall --app curl

  # Uninstall with verbose output
  devex uninstall --app git --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(appName, repo, settings)
		},
	}

	cmd.Flags().StringVar(&appName, "app", "", "Name of the application to uninstall (required)")
	_ = cmd.MarkFlagRequired("app")

	return cmd
}

func runUninstall(appName string, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Update settings with runtime flags
	settings.Verbose = viper.GetBool("verbose")

	log.Info("Starting uninstall process", "app", appName)

	// Uninstall functionality will be implemented in a future release
	log.Info("Uninstall functionality not yet implemented", "app", appName)
	fmt.Printf("Uninstall command received for app: %s\n", appName)
	fmt.Println("This is a placeholder implementation.")
	fmt.Println("Future implementation will:")
	fmt.Println("  1. Check if app is installed via DevEx")
	fmt.Println("  2. Determine the installation method used")
	fmt.Println("  3. Remove via appropriate package manager")
	fmt.Println("  4. Clean up configuration files (optional)")
	fmt.Println("  5. Remove unused dependencies (optional)")
	fmt.Println("  6. Update installation database")

	return nil
}
