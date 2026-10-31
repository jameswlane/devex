package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var uninstallApplication = func(cmd *cobra.Command, app types.CrossPlatformApp, repo types.Repository) error {
	osConfig := app.GetOSConfig()
	legacy := app.ToLegacyAppConfig()
	if err := validateCurlpipeRemoval(legacy); err != nil {
		return err
	}
	installer := installers.GetInstaller(cmd.Context(), osConfig.InstallMethod)
	if installer == nil && osConfig.InstallMethod != "curlpipe" {
		return fmt.Errorf("install method %q is not supported on this platform", osConfig.InstallMethod)
	}
	return uninstallConfiguredApplication(cmd.Context(), legacy, installer, repo)
}

func removeInstalledApplication(cmd *cobra.Command, name string, settings config.CrossPlatformSettings, repo types.Repository, force, dryRun bool, provider pluginBootstrapProvider) (bool, error) {
	// Bundled apps keep the regular removal flow (backups, cascade, prompts);
	// this path only handles apps defined inline in a config file.
	if _, err := resolveAppsByName(settings, []string{name}); err == nil {
		return false, nil
	}
	_, merged, present, err := configuredApplications(settings)
	if err != nil {
		return true, err
	}
	var app types.CrossPlatformApp
	if present {
		apps, err := resolveAppsByName(merged, []string{name})
		if err != nil {
			return true, err
		}
		app = apps[0]
	} else {
		// Keep the installed definition so removal can outlive a temporary config file.
		data, err := repo.Get("installed-app-definition:" + normalizeInstallLookup(name))
		if err != nil || data == "" {
			return false, nil
		}
		if err := json.Unmarshal([]byte(data), &app); err != nil {
			return true, fmt.Errorf("invalid installed definition for %s: %w", name, err)
		}
	}
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Would remove application '%s'\n", app.Name)
		return true, nil
	}
	if !force {
		return true, fmt.Errorf("removing installed application %q requires --force", app.Name)
	}
	if err := prepareApplicationInstallers(cmd.Context(), provider); err != nil {
		return true, err
	}
	if err := uninstallApplication(cmd, app, repo); err != nil {
		return true, fmt.Errorf("failed to remove %s: %w", app.Name, err)
	}
	if err := repo.DeleteApp(app.Name); err != nil {
		return true, err
	}
	if err := repo.Set("installed-app-definition:"+normalizeInstallLookup(app.Name), ""); err != nil {
		return true, err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", app.Name)
	return true, nil
}
