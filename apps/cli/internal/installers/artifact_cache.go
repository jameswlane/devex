package installers

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/viper"
)

// InstallationCacheOptions uses the root flags and environment Viper keys.
func InstallationCacheOptions() cache.InstallCacheOptions {
	return cache.InstallCacheOptions{Bypass: viper.GetBool("no-cache"), Offline: viper.GetBool("offline"), Hit: func(key string) { log.Debug("Installation cache hit", "key", key) }}
}

// PrepareArtifact owns a private copy until the plugin exits.
func PrepareArtifact(ctx context.Context, source string, metadata sdk.ArtifactVerification) (sdk.ArtifactVerification, func(), error) {
	cleanup := func() {}
	options := InstallationCacheOptions()
	if options.Bypass && options.Offline {
		return metadata, cleanup, fmt.Errorf("--offline and --no-cache cannot be combined")
	}
	if options.Offline && metadata.SignatureURL != "" {
		return metadata, cleanup, fmt.Errorf("offline signature verification is unavailable; retry online")
	}
	if options.Bypass || metadata.SHA256 == "" {
		if options.Offline {
			return metadata, cleanup, fmt.Errorf("offline installation needs a cached artifact with a pinned SHA256")
		}
		return metadata, cleanup, nil
	}
	parsed, err := url.Parse(source)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return metadata, cleanup, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return metadata, cleanup, err
	}
	manager, err := cache.NewCacheManager(config.CrossPlatformSettings{HomeDir: home})
	if err != nil {
		return metadata, cleanup, err
	}
	file, err := os.CreateTemp("", "devex-install-artifact-*")
	if err != nil {
		return metadata, cleanup, err
	}
	_ = file.Close()
	cleanup = func() { _ = os.Remove(file.Name()) }
	if err := manager.FetchArtifact(ctx, source, file.Name(), metadata, options, cache.DownloadArtifact); err != nil {
		cleanup()
		return metadata, func() {}, err
	}
	metadata.CachedPath = file.Name()
	return metadata, cleanup, nil
}
