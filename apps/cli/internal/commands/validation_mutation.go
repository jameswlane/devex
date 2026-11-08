package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
)

func validateBeforeMutation(ctx context.Context, settings config.CrossPlatformSettings) error {
	return config.RequireValidConfig(ctx, settings)
}

// validateImportCandidate prepares an isolated precedence stream; no destination
// directory, backup, undo log or version is written until this check succeeds.
func validateImportCandidate(ctx context.Context, settings config.CrossPlatformSettings, data map[string]interface{}) error {
	if err := validateImportData(data); err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "devex-validate-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dirs := config.EffectiveConfigDirs(settings)
	names := []string{"default", "default_env", "team", "team_env", "user", "user_env"}
	configurations, ok := data["configurations"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("configurations must be a map")
	}
	for i, src := range dirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		dst := filepath.Join(root, names[i])
		files, err := config.ConfigurationFiles(src)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, relative := range files {
			content, err := os.ReadFile(filepath.Join(src, relative))
			if err != nil {
				return err
			}
			target := filepath.Join(dst, relative)
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return err
			}
			if err := os.WriteFile(target, content, 0600); err != nil { // #nosec G703 -- relative paths are enumerated by ConfigurationFiles from fixed configuration roots
				return err
			}
		}
		if value, ok := configurations[names[i]]; ok {
			entries, ok := value.(map[string]interface{})
			if !ok {
				return fmt.Errorf("%s configurations must be a map", names[i])
			}
			for key, value := range entries {
				if err := writeImportedConfiguration(dst, key, value); err != nil {
					return err
				}
			}
		}
		dirs[i] = dst
	}
	candidate, err := config.LoadConfigDirs(ctx, settings.HomeDir, dirs, config.FileApplicationLoader{})
	if err != nil {
		return err
	}
	result, err := config.ValidateConfigDirs(ctx, candidate, dirs, false)
	if err != nil {
		return err
	}
	if !result.Valid {
		return fmt.Errorf("invalid imported configuration: %v", result.Diagnostics)
	}
	return nil
}
