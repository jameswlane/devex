package tui

import (
	"context"
	"os/exec"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// InstalledApps checks native package state before setup confirmation and execution.
// APT uses one query; other supported managers use at most four concurrent probes.
// Unknown managers and failed probes are conservatively left in the install plan.
func InstalledApps(ctx context.Context, apps []types.CrossPlatformApp) map[string]bool {
	result := InstalledAptApps(ctx, apps)
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, 4)
	for _, app := range apps {
		cfg := app.GetOSConfig()
		if cfg.InstallMethod != "mise" && cfg.InstallMethod != "flatpak" {
			continue
		}
		request, err := types.ResolvePackageInstall(cfg.InstallMethod, cfg.InstallCommand, cfg.Packages, cfg.Options)
		if err != nil || len(request.Packages) == 0 || len(request.Options) > 0 {
			continue
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return result
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			for _, pkg := range request.Packages {
				args := []string{"info", pkg}
				if cfg.InstallMethod == "mise" {
					args = []string{"where", pkg}
				}
				if exec.CommandContext(ctx, cfg.InstallMethod, args...).Run() != nil {
					return
				} //nolint:gosec // fixed manager names and argv, no shell
			}
			mu.Lock()
			result[app.Name] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result
}
