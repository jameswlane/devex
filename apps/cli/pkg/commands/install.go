package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/tui"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

func init() {
	Register(NewInstallCmd)
}

func NewInstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install your development environment",
		Long: `The install command automates the installation of your complete development environment.

It installs applications from four main categories:
  • Development tools (Git, Docker, VS Code, Neovim, etc.)
  • Programming languages (Node.js, Python, Go, Ruby, etc.)
  • Databases (PostgreSQL, MySQL, Redis, SQLite)
  • Optional applications (browsers, communication tools, etc.)

The installation respects platform-specific package managers and handles:
  • Package repository configuration and GPG keys
  • Dependency resolution and conflict handling
  • Post-installation configuration and shell setup
  • Theme application and desktop environment setup

Configuration files are located in ~/.local/share/devex/config/:
  • applications.yaml - Development tools and applications
  • environment.yaml - Programming languages and fonts
  • desktop.yaml - Themes, extensions, and desktop settings
  • system.yaml - Git configuration and system settings`,
		Example: `  # Install all default applications
  devex install

  # Install with verbose output
  devex install --verbose`,
		Run: func(cmd *cobra.Command, args []string) {
			runInstall(repo, settings)
		},
	}

	return cmd
}

func runInstall(repo types.Repository, settings config.CrossPlatformSettings) {
	// Update settings with runtime flags
	settings.Verbose = viper.GetBool("verbose")

	// Get default apps for installation
	defaultApps := settings.GetDefaultApps()

	// Convert cross-platform apps to legacy AppConfig format for TUI
	legacyApps := make([]types.AppConfig, 0, len(defaultApps))
	for _, app := range defaultApps {
		legacyApps = append(legacyApps, app.ToLegacyAppConfig())
	}

	// Start TUI installation
	if err := tui.StartInstallation(legacyApps, repo, settings); err != nil {
		log.Error("Installation failed", err)
		return
	}

	// Switch to zsh as the final step (after all installations are complete)
	log.Info("Switching to zsh shell as final step")
	if err := switchToZsh(); err != nil {
		log.Warn("Failed to switch to zsh shell", "error", err)
		log.Info("You can manually switch to zsh later with: chsh -s $(which zsh)")
	} else {
		log.Info("Successfully switched to zsh shell. Please restart your terminal or run 'exec zsh' to use the new shell.")
	}
}

// switchToZsh attempts to switch the user's shell to zsh
func switchToZsh() error {
	// Get the full path to zsh
	zshPath, err := utils.GetShellPath("zsh")
	if err != nil {
		return err
	}

	// Change the user's shell to zsh
	return utils.ChangeUserShell(zshPath)
}
