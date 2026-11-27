package tui

import (
	"context"
	"fmt"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// PackageBatch reports what PreparePackageBatch did for each app.
type PackageBatch struct {
	// Preinstalled holds apps whose packages were all installed before the run.
	Preinstalled map[string]bool
	// BatchInstalled holds apps whose packages the shared transaction installed.
	// Their own install step is skipped; config files and hooks still run.
	BatchInstalled map[string]bool
	// SourceErrors holds apps whose APT sources or keys could not be prepared.
	// Installing them on their own would use the distribution's package, so
	// they fail with this error instead.
	SourceErrors map[string]error
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
	available, err := policyBatchable(ctx, available, settings.HomeDir)
	if err != nil {
		return PackageBatch{}, err
	}
	si := NewStreamingInstaller(nil, repo, ctx, settings)
	defer si.cancel()
	err = si.preparePackageRun(ctx, available, settings.HomeDir)
	si.apt.mu.Lock()
	defer si.apt.mu.Unlock()
	sourceErrors := make(map[string]error, len(si.apt.sourceErrors))
	for name, sourceErr := range si.apt.sourceErrors {
		sourceErrors[name] = sourceErr
	}
	return PackageBatch{Preinstalled: copySet(si.apt.preinstalled), BatchInstalled: copySet(si.apt.batchInstalled), SourceErrors: sourceErrors}, err
}

func copySet(set map[string]bool) map[string]bool {
	result := make(map[string]bool, len(set))
	for key, value := range set {
		result[key] = value
	}
	return result
}

// PrepareAptSources writes an apt app's APT sources and keys and refreshes the
// package lists, for an app about to install on its own outside the TUI. The
// batch only prepares sources for apps it considers, so an app it skipped (an
// install command that is not plain package names, or a batch that failed
// early) would otherwise install the distribution's package. Sources already
// on disk are not written again, and fresh lists are not updated again.
// Apps the organization policy denies are left to their own install to refuse.
func PrepareAptSources(ctx context.Context, app types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) error {
	cfg := app.GetOSConfig()
	if cfg.InstallMethod != "apt" || len(cfg.AptSources) == 0 {
		return nil
	}
	// The app's own install refuses an app the organization policy denies.
	if allowed, err := policyBatchable(ctx, []types.CrossPlatformApp{app}, settings.HomeDir); err != nil || len(allowed) == 0 {
		return nil
	}
	si := NewStreamingInstaller(nil, repo, ctx, settings)
	defer si.cancel()
	if err := state.AuditStep(ctx, "apt-sources", true, func() error {
		if err := si.addAptSources(ctx, app.Name, cfg); err != nil {
			return err
		}
		return si.ensureAptLists(ctx)
	}); err != nil {
		return fmt.Errorf("prepare APT sources for %s: %w", app.Name, err)
	}
	return nil
}
