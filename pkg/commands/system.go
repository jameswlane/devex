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
		Long:  "Configure and optimize your system settings for development.",
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
