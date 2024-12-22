package commands

import (
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/commands/install"
)

func RegisterRootCommand(homeDir string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "devex",
		Short: "DevEx CLI for setting up your development environment",
		Long:  "DevEx is a CLI tool that helps you install and configure your development environment easily.",
	}

	// Register subcommands
	rootCmd.AddCommand(install.CreateInstallCommand(homeDir))

	return rootCmd
}
