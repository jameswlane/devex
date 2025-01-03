package commands

import (
	"github.com/spf13/cobra"
)

func RegisterRootCommand(homeDir string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "devex",
		Short: "DevEx CLI for setting up your development environment",
		Long:  "DevEx is a CLI tool that helps you install and configure your development environment easily.",
	}

	// Register subcommands
	rootCmd.AddCommand(CreateInstallCommand(homeDir))

	return rootCmd
}
