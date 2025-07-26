package commands

import (
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
)

func NewRootCmd(version string, repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "devex",
		Short:   "DevEx CLI for setting up your development environment",
		Long:    `DevEx is a CLI tool that helps you configure your development environment with minimal effort.`,
		Version: version,
	}

	// Register other commands
	cmd.AddCommand(NewInstallCmd(repo, settings))
	cmd.AddCommand(NewSystemCmd())
	cmd.AddCommand(NewCompletionCmd())

	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().Bool("dry-run", false, "Run commands without making changes")

	return cmd
}
