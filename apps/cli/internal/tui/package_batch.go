package tui

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
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
// BatchInstalled empty and each app installs on its own.
func PreparePackageBatch(ctx context.Context, apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) (PackageBatch, error) {
	si := NewStreamingInstaller(nil, repo, ctx, settings)
	defer si.cancel()
	err := si.preparePackageRun(ctx, apps)
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
