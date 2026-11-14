package tui

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// PackageBatch reports what PreparePackageBatch did for each app.
type PackageBatch struct {
	// Preinstalled holds apps whose packages were all installed before the run.
	Preinstalled map[string]bool
	// BatchInstalled holds apps whose packages the shared transaction installed.
	// Their own install step is skipped; config files and hooks still run.
	BatchInstalled map[string]bool
}

// PreparePackageBatch runs the TUI's package batch planner without a TUI, so
// unattended installs get the same single apt-get update, single batched
// apt-get install, and batched mise/Flatpak installs. A failed batch leaves
// BatchInstalled empty and each app installs on its own. Apps whose installer
// plugin is unavailable stay out of the batch so they fail on their own, as
// before.
func PreparePackageBatch(ctx context.Context, apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) (PackageBatch, error) {
	available := make([]types.CrossPlatformApp, 0, len(apps))
	for _, app := range apps {
		switch method := app.GetOSConfig().InstallMethod; method {
		case "apt", "mise", "flatpak":
			if !installers.IsInstallerSupported(ctx, method) {
				cfg := app.GetOSConfig()
				// Its hooks or dependencies still order the run, so batch nothing.
				if len(cfg.Dependencies)+len(cfg.Conflicts)+len(cfg.PreInstall)+len(cfg.ConfigFiles)+len(cfg.Themes) > 0 {
					return PackageBatch{}, nil
				}
				continue
			}
		}
		available = append(available, app)
	}
	si := NewStreamingInstaller(nil, repo, ctx, settings)
	defer si.cancel()
	err := si.preparePackageRun(ctx, available)
	si.apt.mu.Lock()
	defer si.apt.mu.Unlock()
	return PackageBatch{Preinstalled: copySet(si.apt.preinstalled), BatchInstalled: copySet(si.apt.batchInstalled)}, err
}

func copySet(set map[string]bool) map[string]bool {
	result := make(map[string]bool, len(set))
	for key, value := range set {
		result[key] = value
	}
	return result
}
