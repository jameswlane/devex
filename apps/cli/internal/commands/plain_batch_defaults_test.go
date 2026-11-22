package commands

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
)

// Plain-install specs must not run real package managers unless they opt in.
var _ = BeforeEach(func() {
	original := preparePackageBatch
	preparePackageBatch = func(context.Context, []types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) (tui.PackageBatch, error) {
		return tui.PackageBatch{}, nil
	}
	DeferCleanup(func() { preparePackageBatch = original })
	originalSources := prepareAptSources
	prepareAptSources = func(context.Context, types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) error {
		return nil
	}
	DeferCleanup(func() { prepareAptSources = originalSources })
})
