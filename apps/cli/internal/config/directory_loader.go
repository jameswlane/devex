package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// ApplicationLoader loads an application catalog from configuration tiers ordered
// from lowest to highest precedence.
type ApplicationLoader interface {
	LoadApplications(configDirs []string) ([]types.CrossPlatformApp, error)
}

// FileApplicationLoader loads the canonical per-application directory tree.
type FileApplicationLoader struct{}

// LoadApplications loads applications/category/app.yaml files and merges overrides by name.
func (FileApplicationLoader) LoadApplications(configDirs []string) ([]types.CrossPlatformApp, error) {
	var apps []types.CrossPlatformApp
	for _, configDir := range configDirs {
		tierApps, err := loadCrossPlatformAppDirectory(filepath.Join(configDir, "applications"))
		if err != nil {
			return nil, err
		}
		apps = mergeApplications(apps, tierApps)
	}
	return apps, nil
}

func loadCrossPlatformAppDirectory(dir string) ([]types.CrossPlatformApp, error) {
	files, err := configFilesIfPresent(dir, 4)
	if err != nil {
		return nil, err
	}

	apps := make([]types.CrossPlatformApp, 0, len(files))
	for _, relativePath := range files {
		path := filepath.Join(dir, relativePath)
		fileViper, err := loadYamlFileToViper(path)
		if err != nil {
			return nil, fmt.Errorf("load application config %s: %w", path, err)
		}
		var app types.CrossPlatformApp
		if err := fileViper.Unmarshal(&app); err != nil {
			return nil, fmt.Errorf("decode application config %s: %w", path, err)
		}
		if strings.TrimSpace(app.Name) == "" {
			return nil, fmt.Errorf("application config %s must define name", path)
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func loadEnvironmentAppDirectory(dir string) ([]types.CrossPlatformApp, error) {
	files, err := configFilesIfPresent(dir, 1)
	if err != nil {
		return nil, err
	}

	apps := make([]types.CrossPlatformApp, 0, len(files))
	for _, relativePath := range files {
		path := filepath.Join(dir, relativePath)
		fileViper, err := loadYamlFileToViper(path)
		if err != nil {
			return nil, fmt.Errorf("load environment config %s: %w", path, err)
		}
		var app types.CrossPlatformApp
		if err := fileViper.Unmarshal(&app); err != nil {
			return nil, fmt.Errorf("decode environment config %s: %w", path, err)
		}
		var legacy types.AppConfig
		if err := fileViper.Unmarshal(&legacy); err != nil {
			return nil, fmt.Errorf("decode environment config %s: %w", path, err)
		}
		if strings.TrimSpace(legacy.Name) == "" {
			return nil, fmt.Errorf("environment config %s must define name", path)
		}
		if app.AllPlatforms.InstallMethod == "" && app.Linux.InstallMethod == "" && app.MacOS.InstallMethod == "" && app.Windows.InstallMethod == "" {
			app = CrossPlatformAppFromLegacy(legacy)
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func configFilesIfPresent(dir string, maxDepth int) ([]string, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("access config directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("config path %s is not a directory", dir)
	}
	files, err := getDirectoryFilesRecursive(dir, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("list config directory %s: %w", dir, err)
	}
	return files, nil
}

func loadLegacyConfigDirectory(configDir string) (*viper.Viper, error) {
	v := viper.New()
	for _, filename := range CrossPlatformFiles {
		path := filepath.Join(configDir, filename)
		exists, err := fs.Exists(path)
		if err != nil {
			return nil, fmt.Errorf("check configuration %s: %w", path, err)
		}
		if !exists {
			continue
		}
		if isLegacyConfigFile(filename) {
			warnLegacyConfig(path)
		}
		fileViper, err := loadYamlFileToViper(path)
		if err != nil {
			return nil, err
		}
		for key, value := range fileViper.AllSettings() {
			// Old init wrote shell/editor/languages preferences here rather than
			// installable shell definitions. Keep those preferences out of Shell.
			if filename == "environment.yaml" && key == "shell" {
				if _, preference := value.(string); preference {
					continue
				}
			}
			v.Set(key, value)
		}
	}
	return v, nil
}

func isLegacyConfigFile(filename string) bool {
	switch filename {
	case "dotfiles.yaml", "security.yaml":
		return false
	default:
		return true
	}
}

func warnLegacyConfig(path string) {
	log.Warning("Deprecated configuration file %s; move its settings into the applications/, environments/, system/, or desktop/ directory", path)
}

func loadLegacyInitApplications(configDirs []string) ([]types.CrossPlatformApp, error) {
	var apps []types.CrossPlatformApp
	for _, configDir := range configDirs {
		path := filepath.Join(configDir, "applications.yaml")
		exists, err := fs.Exists(path)
		if err != nil {
			return nil, fmt.Errorf("check legacy application config %s: %w", path, err)
		}
		if !exists {
			continue
		}

		fileViper, err := loadYamlFileToViper(path)
		if err != nil {
			return nil, fmt.Errorf("load legacy application config %s: %w", path, err)
		}
		data, err := yaml.Marshal(fileViper.AllSettings())
		if err != nil {
			return nil, fmt.Errorf("read legacy application config %s: %w", path, err)
		}
		var root struct {
			Applications yaml.Node `yaml:"applications"`
		}
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("decode legacy application config %s: %w", path, err)
		}
		tierApps, err := decodeLegacyApplicationNode(&root.Applications)
		if err != nil {
			return nil, fmt.Errorf("decode legacy application config %s: %w", path, err)
		}
		apps = mergeApplications(apps, tierApps)
	}
	return apps, nil
}

func decodeLegacyApplicationNode(node *yaml.Node) ([]types.CrossPlatformApp, error) {
	if node == nil {
		return nil, nil
	}
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil, nil
		}
		return decodeLegacyApplicationNode(node.Content[0])
	}

	switch node.Kind {
	case yaml.SequenceNode:
		var apps []types.CrossPlatformApp
		for _, entry := range node.Content {
			if entry.Kind == yaml.ScalarNode {
				continue
			}
			app, err := decodeLegacyApplication(entry)
			if err != nil {
				return nil, err
			}
			apps = append(apps, app)
		}
		return apps, nil
	case yaml.MappingNode:
		var apps []types.CrossPlatformApp
		for i := 1; i < len(node.Content); i += 2 {
			categoryApps, err := decodeLegacyApplicationNode(node.Content[i])
			if err != nil {
				return nil, err
			}
			apps = append(apps, categoryApps...)
		}
		return apps, nil
	case 0:
		return nil, nil
	default:
		return nil, fmt.Errorf("applications must be a list or category map")
	}
}

func decodeLegacyApplication(node *yaml.Node) (types.CrossPlatformApp, error) {
	var app types.CrossPlatformApp
	if err := node.Decode(&app); err != nil {
		return types.CrossPlatformApp{}, err
	}
	if app.AllPlatforms.InstallMethod != "" || app.Linux.InstallMethod != "" || app.MacOS.InstallMethod != "" || app.Windows.InstallMethod != "" {
		return app, nil
	}

	var legacy types.AppConfig
	if err := node.Decode(&legacy); err != nil {
		return types.CrossPlatformApp{}, err
	}
	return CrossPlatformAppFromLegacy(legacy), nil
}

func CrossPlatformAppFromLegacy(legacy types.AppConfig) types.CrossPlatformApp {
	return types.CrossPlatformApp{
		Name:        legacy.Name,
		Description: legacy.Description,
		Category:    legacy.Category,
		Default:     legacy.Default,
		AllPlatforms: types.OSConfig{
			InstallMethod:      legacy.InstallMethod,
			InstallCommand:     legacy.InstallCommand,
			UninstallCommand:   legacy.UninstallCommand,
			Dependencies:       legacy.Dependencies,
			SystemRequirements: legacy.SystemRequirements,
			PreInstall:         legacy.PreInstall,
			PostInstall:        legacy.PostInstall,
			ConfigFiles:        legacy.ConfigFiles,
			Themes:             legacy.Themes,
			AptSources:         legacy.AptSources,
			CleanupFiles:       legacy.CleanupFiles,
			Conflicts:          legacy.Conflicts,
			DownloadURL:        legacy.DownloadURL,
			Destination:        legacy.InstallDir,
		},
	}
}

func mergeApplications(groups ...[]types.CrossPlatformApp) []types.CrossPlatformApp {
	var merged []types.CrossPlatformApp
	indices := make(map[string]int)
	for _, group := range groups {
		for _, app := range group {
			key := strings.ToLower(strings.TrimSpace(app.Name))
			if index, ok := indices[key]; ok {
				merged[index] = app
				continue
			}
			indices[key] = len(merged)
			merged = append(merged, app)
		}
	}
	return merged
}

func loadDirectorySettings(settings *CrossPlatformSettings, configDirs []string) error {
	for _, configDir := range configDirs {
		languages, err := loadEnvironmentAppDirectory(filepath.Join(configDir, "environments", "programming-languages"))
		if err != nil {
			return err
		}
		settings.ProgrammingLanguages = mergeApplications(settings.ProgrammingLanguages, languages)
		shellApps, err := loadEnvironmentAppDirectory(filepath.Join(configDir, "environments", "shell"))
		if err != nil {
			return err
		}
		settings.Shell = mergeApplications(settings.Shell, shellApps)

		if err := loadFontDirectory(settings, filepath.Join(configDir, "environments", "fonts")); err != nil {
			return err
		}
		if err := loadSystemDirectory(settings, filepath.Join(configDir, "system")); err != nil {
			return err
		}
		if err := loadDesktopDirectory(settings, filepath.Join(configDir, "desktop")); err != nil {
			return err
		}
	}
	return nil
}

func loadFontDirectory(settings *CrossPlatformSettings, dir string) error {
	files, err := configFilesIfPresent(dir, 1)
	if err != nil {
		return err
	}
	indices := make(map[string]int, len(settings.Fonts))
	for i, font := range settings.Fonts {
		indices[strings.ToLower(strings.TrimSpace(font.Name))] = i
	}
	for _, relativePath := range files {
		path := filepath.Join(dir, relativePath)
		fileViper, err := loadYamlFileToViper(path)
		if err != nil {
			return fmt.Errorf("load font config %s: %w", path, err)
		}
		var font types.Font
		if err := fileViper.Unmarshal(&font); err != nil {
			return fmt.Errorf("decode font config %s: %w", path, err)
		}
		key := strings.ToLower(strings.TrimSpace(font.Name))
		if index, ok := indices[key]; ok {
			settings.Fonts[index] = font
			continue
		}
		indices[key] = len(settings.Fonts)
		settings.Fonts = append(settings.Fonts, font)
	}
	return nil
}

func loadSystemDirectory(settings *CrossPlatformSettings, dir string) error {
	files, err := configFilesIfPresent(dir, 3)
	if err != nil {
		return err
	}
	for _, relativePath := range files {
		path := filepath.Join(dir, relativePath)
		if relativePath == "git.yaml" {
			gitConfigs, err := loadGitConfig(path)
			if err != nil {
				return err
			}
			settings.Dotfiles.Git = gitConfigs
			continue
		}
		fileViper, err := loadYamlFileToViper(path)
		if err != nil {
			return fmt.Errorf("load system config %s: %w", path, err)
		}
		switch relativePath {
		case "global.yaml":
			settings.Dotfiles.GlobalTheme = fileViper.GetString("global_theme")
		case "ssh.yaml":
			settings.Dotfiles.SSH = fileViper.AllSettings()
		case "terminal.yaml":
			settings.Dotfiles.Terminal = fileViper.AllSettings()
		}
	}
	return nil
}

func loadGitConfig(path string) ([]types.GitConfig, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect Git config %s: %w", path, err)
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("git config %s exceeds maximum size (%d bytes)", path, maxFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Git config %s: %w", path, err)
	}
	var git types.GitConfig
	if err := yaml.Unmarshal(data, &git); err == nil {
		return []types.GitConfig{git}, nil
	}
	var configs []types.GitConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("decode Git config %s as an object or list: %w", path, err)
	}
	return configs, nil
}

func loadDesktopDirectory(settings *CrossPlatformSettings, dir string) error {
	files, err := configFilesIfPresent(dir, 4)
	if err != nil {
		return err
	}
	for _, relativePath := range files {
		parts := strings.Split(filepath.ToSlash(relativePath), "/")
		if len(parts) < 2 {
			continue
		}
		path := filepath.Join(dir, relativePath)
		fileViper, err := loadYamlFileToViper(path)
		if err != nil {
			return fmt.Errorf("load desktop config %s: %w", path, err)
		}
		var desktop DesktopEnvConfig
		if err := fileViper.Unmarshal(&desktop); err != nil {
			return fmt.Errorf("decode desktop config %s: %w", path, err)
		}
		switch strings.ToLower(parts[0]) {
		case "gnome":
			mergeDesktopEnvConfig(&settings.DesktopEnvironments.GNOME, desktop)
		case "kde":
			mergeDesktopEnvConfig(&settings.DesktopEnvironments.KDE, desktop)
		case "macos":
			mergeDesktopEnvConfig(&settings.DesktopEnvironments.MacOS, desktop)
		case "windows":
			mergeDesktopEnvConfig(&settings.DesktopEnvironments.Windows, desktop)
		}
	}
	return nil
}

func mergeDesktopEnvConfig(current *DesktopEnvConfig, override DesktopEnvConfig) {
	if override.Platforms != nil {
		current.Platforms = override.Platforms
	}
	if override.Themes != nil {
		current.Themes = override.Themes
	}
	if override.Settings != nil {
		current.Settings = override.Settings
	}
	if override.Extensions != nil {
		current.Extensions = override.Extensions
	}
	if override.Dock != nil {
		current.Dock = override.Dock
	}
}
