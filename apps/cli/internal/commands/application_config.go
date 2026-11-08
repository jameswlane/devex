package commands

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func configuredApplications(ctx context.Context, s config.CrossPlatformSettings) ([]types.CrossPlatformApp, config.CrossPlatformSettings, bool, error) {
	return agentapi.ConfiguredApplications(ctx, s)
}
