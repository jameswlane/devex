package commands

import (
	"fmt"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// configuredApplications resolves the config read by initializeConfig after Cobra
// parses flags. Inline definitions override bundled definitions by name.
func configuredApplications(settings config.CrossPlatformSettings) ([]types.CrossPlatformApp, config.CrossPlatformSettings, bool, error) {
	if !viper.IsSet("applications") {
		return nil, settings, false, nil
	}
	data, err := yaml.Marshal(viper.Get("applications"))
	if err != nil {
		return nil, settings, true, fmt.Errorf("invalid applications: %w", err)
	}
	var entries []yaml.Node
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, settings, true, fmt.Errorf("applications must be a list: %w", err)
	}
	known := settings.GetAllApps()
	requested := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind == yaml.ScalarNode && entry.Tag == "!!str" {
			requested = append(requested, entry.Value)
			continue
		}
		if entry.Kind != yaml.MappingNode {
			return nil, settings, true, fmt.Errorf("applications entries must be app names or definitions")
		}
		var app types.CrossPlatformApp
		if err := entry.Decode(&app); err != nil {
			return nil, settings, true, fmt.Errorf("invalid application definition: %w", err)
		}
		if err := app.Validate(); err != nil {
			return nil, settings, true, fmt.Errorf("invalid application %q: %w", app.Name, err)
		}
		replaced := false
		for i := range known {
			if normalizeInstallLookup(known[i].Name) == normalizeInstallLookup(app.Name) {
				known[i] = app
				replaced = true
			}
		}
		if !replaced {
			known = append(known, app)
		}
		requested = append(requested, app.Name)
	}
	// Flatten a copy so every selection path uses the same merged lookup, without
	// mutating the caller's settings slices.
	merged := config.CrossPlatformSettings{Terminal: config.TerminalApplicationsConfig{Utilities: known}}
	resolved, err := resolveAppsByName(merged, requested)
	if err != nil {
		return nil, settings, true, err
	}
	// Keep runtime and non-application settings for the installers.
	settings.ApplicationOverrides = known
	return resolved, settings, true, nil
}
