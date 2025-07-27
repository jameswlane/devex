package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
)

func NewRootCmd(version string, repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devex",
		Short: "DevEx CLI for setting up your development environment",
		Long: `DevEx is a CLI tool that helps you configure your development environment with minimal effort.

It automates the installation and configuration of:
  • Programming languages (Node.js, Python, Go, Ruby, etc.)
  • Development tools (Docker, Git, VS Code, etc.)
  • Databases (PostgreSQL, MySQL, Redis, etc.)
  • Desktop themes and GNOME extensions
  • Shell configurations and dotfiles

DevEx supports multiple platforms and package managers:
  • Linux: APT (Ubuntu/Debian), DNF (Fedora/RHEL), Pacman (Arch), Flatpak
  • macOS: Homebrew, Mac App Store
  • Windows: winget, Chocolatey, Scoop

All installations are configurable via YAML files in ~/.local/share/devex/config/`,
		Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			// If no subcommand is provided, run the guided setup
			setupCmd := NewSetupCmd(repo, settings)
			setupCmd.SetArgs(args)
			_ = setupCmd.Execute()
		},
	}

	// Register other commands
	cmd.AddCommand(NewSetupCmd(repo, settings))
	cmd.AddCommand(NewInstallCmd(repo, settings))
	cmd.AddCommand(NewUninstallCmd(repo, settings))
	cmd.AddCommand(NewSystemCmd())
	cmd.AddCommand(NewCompletionCmd())

	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().Bool("dry-run", false, "Run commands without making changes")

	// Bind flags to viper for global access
	_ = viper.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("dry_run", cmd.PersistentFlags().Lookup("dry-run"))

	return cmd
}
