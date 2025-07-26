package commands

import (
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

func init() {
	Register(NewInstallCmd)
}

func NewInstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
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
devex install`,
		Run: func(cmd *cobra.Command, args []string) {
			runInstall(repo, settings)
		},
	}

	return cmd
}

func runInstall(repo types.Repository, settings config.CrossPlatformSettings) {
	log.Info("Starting installation process", "dryRun", settings.DryRun)

	// Get default apps for installation
	defaultApps := settings.GetDefaultApps()

	// Install apps using the cross-platform installer
	if err := installers.InstallCrossPlatformApps(defaultApps, settings, repo); err != nil {
		log.Error("Failed to install apps", err)
		return
	}

	if settings.DryRun {
		log.Info("Dry run completed. No changes applied.")
		return
	}

	log.Info("Installation process completed successfully")
}
