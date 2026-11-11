package config

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/catalog"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// LoadCatalogSettings loads registry defaults followed by local configuration.
// The local-only loader remains available to config tooling and offline callers.
func LoadCatalogSettings(ctx context.Context, home string) (CrossPlatformSettings, error) {
	settings, err := LoadCrossPlatformSettingsContext(ctx, home)
	if err != nil {
		return settings, err
	}
	settings.Catalog = &catalog.Result{}
	err = RefreshCatalog(ctx, settings)
	return settings, err
}

// RefreshCatalog updates the shared snapshot after command flags and config are parsed.
func RefreshCatalog(ctx context.Context, settings CrossPlatformSettings) error {
	return loadCatalog(ctx, settings, viper.GetBool("offline"))
}

// LoadCatalogOffline resolves the catalog from the local cache, or from the
// bundled applications when no cache exists, without network access or writes.
func LoadCatalogOffline(ctx context.Context, settings CrossPlatformSettings) error {
	return loadCatalog(ctx, settings, true)
}

func loadCatalog(ctx context.Context, settings CrossPlatformSettings, offline bool) error {
	if settings.Catalog == nil {
		return nil
	}
	bundled, err := bundledApplications()
	if err != nil {
		return err
	}
	store := catalog.Store{Client: catalog.NewClient(bootstrap.GetRegistryURL()), Dir: devexpaths.ForHome(settings.HomeDir).Cache, TTL: time.Hour, Offline: offline}
	result := store.Load(ctx, bundled)
	*settings.Catalog = result
	return nil
}

func bundledApplications() ([]types.CrossPlatformApp, error) {
	var apps []types.CrossPlatformApp
	err := fs.WalkDir(shipconfig.Files, "applications", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, err := shipconfig.Files.ReadFile(path)
		if err != nil {
			return err
		}
		var app types.CrossPlatformApp
		if err := yaml.Unmarshal(data, &app); err != nil {
			return fmt.Errorf("bundled application %s: %w", path, err)
		}
		apps = append(apps, app)
		return nil
	})
	return apps, err
}
