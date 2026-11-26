package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
)

// ShippedManifestFile records SHA-256 digests of unmodified shipped files
// written to a configuration directory so CLI upgrades can refresh them
// without overwriting user edits.
const ShippedManifestFile = ".devex-shipped.json"

type shippedManifest struct {
	Files map[string]string `json:"files"`
}

// WriteDefaultConfig writes missing files from the shipped canonical config tree,
// refreshes unedited copies of older shipped files tracked in the manifest, and
// preserves user-edited or legacy files.
func WriteDefaultConfig(configDir string) error {
	return writeDefaultConfig(configDir, false)
}

// OverwriteDefaultConfig replaces every shipped file in configDir with the
// current embedded version and updates the shipped manifest.
func OverwriteDefaultConfig(configDir string) error {
	return writeDefaultConfig(configDir, true)
}

// RefreshShippedConfig updates existing, unmodified shipped files in configDir
// whose tracked digest differs from the currently embedded content, and writes
// shipped files that are new since the manifest was recorded. Deleted tracked
// files and user-edited files are never touched.
func RefreshShippedConfig(configDir string) ([]string, error) {
	if configDir == "" {
		return nil, nil
	}
	info, err := os.Stat(configDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect config directory %s: %w", configDir, err)
	}
	if !info.IsDir() {
		return nil, nil
	}

	manifest, exists, err := readShippedManifest(configDir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	var updated []string
	walkErr := fs.WalkDir(shipconfig.Files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk embedded config %s: %w", path, walkErr)
		}
		if entry.IsDir() {
			return nil
		}

		recordedHash, tracked := manifest.Files[path]
		shipped, err := shipconfig.Files.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded config %s: %w", path, err)
		}
		shippedHash := sha256Hex(shipped)
		target := filepath.Join(configDir, filepath.FromSlash(path))
		if !tracked {
			// A shipped file the manifest has never seen is new in this CLI
			// version (refreshed files may already reference it); write it
			// once. Deleted shipped files stay tracked and are not recreated.
			if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return fmt.Errorf("create config directory %s: %w", filepath.Dir(target), err)
			}
			if err := os.WriteFile(target, shipped, 0600); err != nil {
				return fmt.Errorf("write config file %s: %w", target, err)
			}
			manifest.Files[path] = shippedHash
			updated = append(updated, path)
			return nil
		}
		if recordedHash == "" {
			return nil
		}
		if shippedHash == recordedHash {
			return nil
		}

		existing, err := os.ReadFile(target)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read config file %s: %w", target, err)
		}

		if sha256Hex(existing) != recordedHash {
			return nil
		}

		if err := os.WriteFile(target, shipped, 0600); err != nil {
			return fmt.Errorf("write config file %s: %w", target, err)
		}
		manifest.Files[path] = shippedHash
		updated = append(updated, path)
		return nil
	})

	if len(updated) > 0 {
		if manifestErr := writeShippedManifest(configDir, manifest); manifestErr != nil {
			if walkErr != nil {
				return updated, walkErr
			}
			return updated, manifestErr
		}
	}
	if walkErr != nil {
		return updated, walkErr
	}
	return updated, nil
}

func writeDefaultConfig(configDir string, force bool) error {
	manifest, _, err := readShippedManifest(configDir)
	if err != nil {
		if !force {
			return err
		}
		manifest = shippedManifest{Files: map[string]string{}}
	}

	err = fs.WalkDir(shipconfig.Files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
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

		data, err := shipconfig.Files.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded config %s: %w", path, err)
		}
		shippedHash := sha256Hex(data)

		if force {
			if err := os.WriteFile(target, data, 0600); err != nil {
				return fmt.Errorf("write config file %s: %w", target, err)
			}
			manifest.Files[path] = shippedHash
			return nil
		}

		existing, err := os.ReadFile(target)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(target, data, 0600); err != nil {
				return fmt.Errorf("write config file %s: %w", target, err)
			}
			manifest.Files[path] = shippedHash
			return nil
		}
		if err != nil {
			return fmt.Errorf("read config file %s: %w", target, err)
		}

		existingHash := sha256Hex(existing)
		if existingHash == shippedHash {
			return nil
		}
		if recordedHash, ok := manifest.Files[path]; ok && existingHash == recordedHash {
			if err := os.WriteFile(target, data, 0600); err != nil {
				return fmt.Errorf("write config file %s: %w", target, err)
			}
			manifest.Files[path] = shippedHash
		}
		return nil
	})
	if err != nil {
		return err
	}

	return writeShippedManifest(configDir, manifest)
}

func readShippedManifest(configDir string) (shippedManifest, bool, error) {
	path := filepath.Join(configDir, ShippedManifestFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return shippedManifest{Files: map[string]string{}}, false, nil
	}
	if err != nil {
		return shippedManifest{}, false, fmt.Errorf("read shipped manifest %s: %w", path, err)
	}
	var manifest shippedManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return shippedManifest{}, false, fmt.Errorf("parse shipped manifest %s: %w", path, err)
	}
	if manifest.Files == nil {
		manifest.Files = map[string]string{}
	}
	return manifest, true, nil
}

func writeShippedManifest(configDir string, manifest shippedManifest) error {
	if manifest.Files == nil {
		manifest.Files = map[string]string{}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal shipped manifest: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(configDir, ".devex-shipped-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp shipped manifest: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp shipped manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp shipped manifest: %w", err)
	}

	target := filepath.Join(configDir, ShippedManifestFile)
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("rename shipped manifest: %w", err)
	}
	cleanup = false
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
