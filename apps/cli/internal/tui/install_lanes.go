package tui

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// runInstallLanes keeps APT and all hooks on one serial lane. The second lane
// runs independent user-space installers, preserving their input order (e.g.
// mise itself before tools). Declared dependencies/conflicts require serial
// execution until the planner has a dependency graph.
func runInstallLanes(ctx context.Context, apps []types.CrossPlatformApp, prepare func(context.Context, []types.CrossPlatformApp) error, install func(context.Context, int, types.CrossPlatformApp) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	parallel := true
	for _, app := range apps {
		cfg := app.GetOSConfig()
		independentMethod := cfg.InstallMethod == "mise" || cfg.InstallMethod == "curlpipe" || cfg.InstallMethod == "flatpak"
		hasHooks := len(cfg.PreInstall) > 0 || len(cfg.PostInstall) > 0 || len(cfg.Themes) > 0 || len(cfg.ConfigFiles) > 0
		if len(cfg.Dependencies) > 0 || len(cfg.Conflicts) > 0 || (independentMethod && hasHooks) {
			parallel = false
		}
	}
	var lanes [2][]int
	for i, app := range apps {
		cfg := app.GetOSConfig()
		lane := 0
		if parallel && len(cfg.PreInstall) == 0 && len(cfg.PostInstall) == 0 && len(cfg.Themes) == 0 && len(cfg.ConfigFiles) == 0 {
			switch cfg.InstallMethod {
			case "mise", "curlpipe", "flatpak":
				lane = 1
			}
		}
		lanes[lane] = append(lanes[lane], i)
	}
	var results [2]error
	var wg sync.WaitGroup
	for lane, indices := range lanes {
		if len(indices) == 0 {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Panics must be recovered in the goroutine where they occur.
			// Keep failures already collected before cancelling the other lane.
			var failures []error
			operation := "preparation"
			defer func() {
				if recovered := recover(); recovered != nil {
					failures = append(failures, fmt.Errorf("%s: installation lane panic: %v", operation, recovered))
					cancel()
				}
				results[lane] = errors.Join(failures...)
			}()
			if lane == 0 {
				if err := prepare(ctx, apps); err != nil {
					failures = append(failures, err)
					cancel()
					return
				}
			}
			for _, i := range indices {
				operation = apps[i].Name
				if ctx.Err() != nil {
					failures = append(failures, ctx.Err())
					break
				}
				if err := install(ctx, i, apps[i]); err != nil {
					failures = append(failures, err)
					if errors.Is(err, ErrSudoAuthorization) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
						cancel()
						break
					}
				}
			}
		}()
	}
	wg.Wait()
	return errors.Join(results[:]...)
}
