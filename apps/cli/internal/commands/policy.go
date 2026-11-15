package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/spf13/cobra"
)

// NewPolicyCmd creates the organization policy inspection command.
func NewPolicyCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "policy",
		Short:        "Inspect organization installation policy",
		SilenceUsage: true,
	}
	var format string
	show := &cobra.Command{
		Use:          "show",
		Short:        "Show the active organization policy",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("invalid format %q: use table or json", format)
			}
			policy, err := security.LoadOrgPolicy(settings.HomeDir)
			if err != nil {
				return err
			}
			if format == "json" {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(policy)
			}
			if policy == nil {
				cmd.Println("No organization policy is configured")
				return nil
			}
			cmd.Printf("Name: %s\nVersion: %s\nSource: %s\nSHA256: %s\n", policy.DisplayName(), policy.Version, policy.Source, policy.SHA256)
			cmd.Printf("Maximum privilege scope: %s\nMinimum security level: %s\nUser overrides allowed: %t\n", policy.Privilege.MaxScope, policy.Security.MinimumLevel, policy.Security.AllowUserOverrides)
			return nil
		},
	}
	show.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	cmd.AddCommand(show)
	return cmd
}
