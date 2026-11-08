package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/cobra"
)

func writeAPIJSON(cmd *cobra.Command, value any, err error) error {
	if err != nil {
		value = struct {
			Error *agentapi.Error `json:"error"`
		}{agentapi.APIError(err)}
	}
	if encodeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(value); encodeErr != nil {
		return encodeErr
	}
	return err
}

func NewPlatformCmd(_ types.Repository, _ config.CrossPlatformSettings) *cobra.Command {
	var format string
	cmd := &cobra.Command{SilenceUsage: true, Use: "platform", Short: "Inspect the current platform without initializing plugins", RunE: func(cmd *cobra.Command, _ []string) error {
		result, err := agentapi.DetectPlatform(cmd.Context())
		if format == "json" {
			return writeAPIJSON(cmd, result, err)
		}
		if format != "table" {
			return fmt.Errorf("unsupported format %q", format)
		}
		if err != nil {
			return err
		}
		cmd.Println(result.String())
		return nil
	}}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	return cmd
}
func newConfigEffectiveCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string
	cmd := &cobra.Command{SilenceUsage: true, Use: "effective", Short: "Show allowlisted effective application settings and precedence", RunE: func(cmd *cobra.Command, _ []string) error {
		if format != "json" {
			return fmt.Errorf("unsupported format %q: use json", format)
		}
		result, err := agentapi.Effective(cmd.Context(), settings)
		return writeAPIJSON(cmd, result, err)
	}}
	cmd.Flags().StringVar(&format, "format", "json", "Output format (json)")
	return cmd
}
func init() { Register(NewPlatformCmd) }
