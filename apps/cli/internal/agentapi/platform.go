package agentapi

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

func DetectPlatform(ctx context.Context) (*platform.Platform, error) {
	result, err := platform.NewDetector().DetectPlatformContext(ctx)
	if err != nil {
		return nil, APIError(err)
	}
	if result.PackageManagers == nil {
		result.PackageManagers = []string{}
	}
	return result, nil
}
