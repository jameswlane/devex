package commands

import (
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const catalogAnnotation = "devex.catalog"
const catalogRequired = "required"

// requiresCatalog inherits opt-ins from command groups, but the root's opt-in
// applies only to its own setup flow, never to unrelated children.
func requiresCatalog(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current != cmd && current.Parent() == nil {
			break
		}
		if current.Annotations[catalogAnnotation] == catalogRequired {
			return true
		}
	}
	return false
}

func loadCommandCatalog(cmd *cobra.Command, settings config.CrossPlatformSettings) error {
	if settings.Catalog == nil {
		return nil
	}
	if err := config.RefreshCatalog(cmd.Context(), settings); err != nil {
		return err
	}
	result := settings.Catalog
	verbose := viper.GetBool("verbose")
	fallback := result.Source == "cache (stale)" || result.Source == "bundled"
	if verbose || fallback {
		cmd.PrintErrf("Application catalog: %s\n", result.Source)
		if result.Warning != nil {
			cmd.PrintErrf("Catalog warning: %v\n", result.Warning)
		}
	}
	if verbose {
		cmd.PrintErrf("Catalog URL: %s/api/v1/applications; cache age: %s\n", result.URL, result.Age.Round(time.Second))
	}
	return nil
}
