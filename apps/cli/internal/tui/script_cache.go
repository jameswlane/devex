package tui

import (
	"context"
	"os"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

func (si *StreamingInstaller) validateCachedScript(ctx context.Context, app, source, path string, metadata sdk.ArtifactVerification) error {
	if metadata.SHA256 == "" || installers.InstallationCacheOptions().Bypass {
		return si.validateScriptContent(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	manager, err := cache.NewCacheManager(config.CrossPlatformSettings{HomeDir: home})
	if err != nil {
		return err
	}
	return manager.Validate(ctx, app, cache.ArtifactKey(source, metadata.SHA256), "script-content-v1", installers.InstallationCacheOptions(), func() error { return si.validateScriptContent(path) })
}
