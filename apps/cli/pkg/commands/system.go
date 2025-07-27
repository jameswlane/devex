package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewSystemCmd creates the system command for CLI.
func NewSystemCmd() *cobra.Command {
	var user string

	cmd := &cobra.Command{
		Use:   "system",
		Short: "Manage system settings",
		Long: `Configure and optimize your system settings for development.

The system command manages system-level configurations including:
  • Git global configuration (aliases, user settings, SSH keys)
  • Shell configuration (Zsh/Bash profiles, environment variables)
  • Desktop environment settings (GNOME, KDE themes and preferences)
  • Terminal configuration and color schemes
  • Font installation and management

This command requires elevated privileges for system-wide changes and
can be customized per user for user-specific configurations.

Note: This is a placeholder command. Full functionality will be implemented
in future versions based on the system.yaml configuration file.`,
		Example: `  # Configure system settings for current user
  devex system --user $USER

  # Configure with verbose output
  devex system --user $USER --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if user == "" {
				return fmt.Errorf("the --user flag is required")
			}
			fmt.Printf("Configuring system for user: %s\n", user)
			return nil
		},
	}

	cmd.Flags().StringVar(&user, "user", "", "Specify the target user (required)")
	err := cmd.MarkFlagRequired("user")
	if err != nil {
		return nil
	}

	return cmd
}
