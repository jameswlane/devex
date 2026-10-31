package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
)

// WriteDefaultConfig writes missing files from the shipped canonical config tree.
func WriteDefaultConfig(configDir string) error {
	return fs.WalkDir(shipconfig.Files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk embedded config %s: %w", path, walkErr)
		}
		target := filepath.Join(configDir, filepath.FromSlash(path))
		if entry.IsDir() {
			if err := os.MkdirAll(target, 0750); err != nil {
				return fmt.Errorf("create config directory %s: %w", target, err)
			}
			return nil
		}

		if _, err := os.Stat(target); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check config file %s: %w", target, err)
		}
		data, err := shipconfig.Files.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded config %s: %w", path, err)
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return fmt.Errorf("write config file %s: %w", target, err)
		}
		return nil
	})
}
