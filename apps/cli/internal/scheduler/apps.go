package scheduler

import (
	"context"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Applications orders selected dependencies; dependencies outside the selection
// remain the installer's responsibility. Hooks/conflicts form serial barriers.
func Applications(apps []types.CrossPlatformApp, install func(context.Context, int, types.CrossPlatformApp) error) []Task {
	names := map[string]string{}
	for _, app := range apps {
		names[strings.ToLower(app.Name)] = app.Name
	}
	tasks := make([]Task, len(apps))
	for i, app := range apps {
		cfg := app.GetOSConfig()
		t := Task{ID: app.Name, Resource: ManagerResource(cfg.InstallMethod), Exclusive: len(cfg.PreInstall)+len(cfg.PostInstall)+len(cfg.Themes)+len(cfg.ConfigFiles)+len(cfg.Conflicts) > 0}
		for _, dep := range cfg.Dependencies {
			if name, ok := names[strings.ToLower(dep)]; ok {
				t.DependsOn = append(t.DependsOn, name)
			}
		}
		t.Run = func(ctx context.Context) error { return install(ctx, i, app) }
		tasks[i] = t
	}
	return tasks
}
