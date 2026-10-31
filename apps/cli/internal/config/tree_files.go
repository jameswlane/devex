package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// ConfigurationFiles lists relative YAML paths in the tree and supported legacy files.
// Backups, templates, and repository metadata are deliberately excluded.
func ConfigurationFiles(dir string) ([]string, error) {
	var files []string
	for _, root := range ConfigDirectories {
		err := filepath.WalkDir(filepath.Join(dir, root), func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("configuration path %s is a symbolic link", path)
			}
			if entry.IsDir() {
				return nil
			}
			if strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") {
				relative, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				files = append(files, relative)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("list configuration in %s: %w", dir, err)
		}
	}
	for _, name := range append(append([]string{}, CrossPlatformFiles...), "devex.yaml") {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			files = append(files, name)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

// LoadApplicationConfigs reads only the applications configured in a single tier.
func LoadApplicationConfigs(dir string) ([]types.CrossPlatformApp, error) {
	legacy, err := loadLegacyInitApplications([]string{dir})
	if err != nil {
		return nil, err
	}
	canonical, err := (FileApplicationLoader{}).LoadApplications([]string{dir})
	if err != nil {
		return nil, err
	}
	return mergeApplications(legacy, canonical), nil
}

// DecodeApplicationConfigs accepts a legacy application list, category map, or wrapper.
func DecodeApplicationConfigs(data []byte) ([]types.CrossPlatformApp, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return nil, nil
	}
	node := document.Content[0]
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == "applications" {
				node = node.Content[i+1]
				break
			}
		}
	}
	return decodeLegacyApplicationNode(node)
}
