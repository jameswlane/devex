package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var inventoryVersion = "dev"

// NewInventoryCmd creates the inventory command.
func NewInventoryCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		format          string
		includeHostname bool
		check           bool
	)
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Export the software DevEx manages, with installed versions",
		Long: `Export the applications DevEx manages on this machine, as recorded by DevEx,
with the manager, packages and versions the native package manager reports
(dpkg for apt and deb, snap, flatpak), and reconcile the two.

Drift lists applications DevEx tracks that are no longer installed, and
applications without a recorded definition. The machine is identified by a
salted hash of /etc/machine-id; the hostname is included only with
--include-hostname. The JSON and YAML schema is ` + inventory.SchemaVersion + `.

Nothing is changed. Use --check in automation to exit nonzero on drift.`,
		Example: `  devex inventory --format json > inventory.json
  devex inventory --check`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Annotations:  map[string]string{catalogAnnotation: catalogRequired},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "table" && format != "json" && format != "yaml" {
				return fmt.Errorf("unsupported format %q; use table, json or yaml", format)
			}
			if repo == nil {
				return fmt.Errorf("the DevEx datastore is unavailable")
			}
			resolve := func(name string) (types.CrossPlatformApp, bool) {
				for _, app := range settings.GetAllApps() {
					if strings.EqualFold(app.Name, name) {
						return app, true
					}
				}
				return types.CrossPlatformApp{}, false
			}
			result, err := inventory.Build(cmd.Context(), repo, inventory.Options{DevexVersion: inventoryVersion, IncludeHostname: includeHostname, Resolve: resolve})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch format {
			case "json":
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(result); err != nil {
					return err
				}
			case "yaml":
				data, err := yaml.Marshal(result)
				if err != nil {
					return err
				}
				if _, err := out.Write(data); err != nil {
					return err
				}
			default:
				if len(result.Items) == 0 {
					fmt.Fprintln(out, "DevEx does not manage any applications on this machine yet.")
				} else {
					w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "APPLICATION\tMANAGER\tVERSION\tSTATE")
					for _, item := range result.Items {
						version := item.Version
						if version == "" {
							version = "-"
						}
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.Name, item.Manager, version, item.State)
					}
					if err := w.Flush(); err != nil {
						return err
					}
				}
				for _, drift := range result.Drift {
					fmt.Fprintf(out, "Drift: %s %s: %s\n", drift.App, drift.Kind, drift.Detail)
				}
			}
			if check && len(result.Drift) > 0 {
				return fmt.Errorf("%d drift finding(s)", len(result.Drift))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	cmd.Flags().BoolVar(&includeHostname, "include-hostname", false, "Include the hostname in the export")
	cmd.Flags().BoolVar(&check, "check", false, "Exit nonzero when there is drift")
	return cmd
}
