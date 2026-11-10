package tui

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/scheduler"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/viper"
)

// Preparation (including APT batching and privilege prompts) completes before
// concurrent workers start. All app-level hooks retain their serial barriers.
func runInstallLanes(ctx context.Context, apps []types.CrossPlatformApp, prepare func(context.Context, []types.CrossPlatformApp) error, install func(context.Context, int, types.CrossPlatformApp) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	limit := 1
	if viper.IsSet("install-concurrency") {
		limit = viper.GetInt("install-concurrency")
	}
	if limit < 1 {
		return scheduler.Run(ctx, nil, limit)
	}
	if prepare != nil {
		if err := prepare(ctx, apps); err != nil {
			return err
		}
	}
	return scheduler.Run(ctx, scheduler.Applications(apps, install), limit)
}
