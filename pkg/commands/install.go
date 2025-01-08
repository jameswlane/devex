package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

func init() {
	Register(NewInstallCmd)
}

func NewInstallCmd(repo types.Repository, settings config.Settings) *cobra.Command {
	var apps string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install your development environment",
		Long: `The install command automates the installation of:
  - Programming languages
  - Tools
  - Databases
  You can configure the apps to be installed using the configuration file.`,
		Example: `
# Install all default applications
devex install

# Install specific applications
devex install --apps app1,app2`,
		Run: func(cmd *cobra.Command, args []string) {
			if apps != "" {
				settings.Apps = parseApps(apps)
			}
			runInstall(repo, settings)
		},
	}

	cmd.Flags().StringVar(&apps, "apps", "", "Comma-separated list of apps to install")
	return cmd
}

// Helper function to parse apps from a comma-separated string
func parseApps(apps string) []types.AppConfig {
	parsedApps := make([]types.AppConfig, 0, len(apps))
	for _, appName := range strings.Split(apps, ",") {
		parsedApps = append(parsedApps, types.AppConfig{Name: appName})
	}
	return parsedApps
}

func runInstall(repo types.Repository, settings config.Settings) {
	log.Info("Starting installation process", "dryRun", settings.DryRun)

	// Install apps
	if err := installApps(repo, settings, FilterDefaultApps(settings.Apps)); err != nil {
		log.Error("Failed to install apps", err)
		return
	}

	if settings.DryRun {
		log.Info("Dry run completed. No changes applied.")
		return
	}

	log.Info("Installation process completed successfully")
}

func installApps(repo types.Repository, settings config.Settings, apps []types.AppConfig) error {
	for _, app := range apps {
		log.Info("Installing app", "app", app.Name, "method", app.InstallMethod)

		// Validate app configuration
		if err := config.ValidateApp(app); err != nil {
			log.Error("Invalid app configuration", err, "app", app.Name)
			continue
		}

		// Perform installation
		if err := installers.InstallApp(app, settings, repo); err != nil {
			log.Error("Error installing app", err, "app", app.Name)
			return err
		}

		log.Info("App installed successfully", "app", app.Name)
	}
	return nil
}

func FilterDefaultApps(apps []types.AppConfig) []types.AppConfig {
	var defaultApps []types.AppConfig
	for _, app := range apps {
		if app.Default {
			defaultApps = append(defaultApps, app)
		}
	}
	return defaultApps
}
