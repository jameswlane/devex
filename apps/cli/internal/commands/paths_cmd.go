package commands

import (
	"fmt"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	"github.com/spf13/cobra"
)

func newPathsCmd() *cobra.Command {
	return &cobra.Command{Use: "paths", Short: "Show resolved DevEx files and directories", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := devexpaths.Resolve()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\nData: %s\nState: %s\nCache: %s\nBinary directory: %s\nPlugins: %s\nBackups: %s\nLogs: %s\n", p.Config, p.Data, p.State, p.Cache, p.Bin, p.Plugins(), p.Backups(), p.Logs())
		return err
	}}
}
