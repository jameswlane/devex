package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// PluginConfig represents the plugin configuration structure
type PluginConfig struct {
	Priorities struct {
		PackageManagers     map[string]int `yaml:"package_managers"`
		DesktopEnvironments map[string]int `yaml:"desktop_environments"`
	} `yaml:"priorities"`

	Platforms map[string]map[string]PlatformConfig `yaml:"platforms"`

	Dependencies map[string]PluginDependency `yaml:"dependencies"`

	UserOverrides struct {
		PreferredPackageManager string   `yaml:"preferred_package_manager"`
		ExcludedPlugins         []string `yaml:"excluded_plugins"`
		IncludedPlugins         []string `yaml:"included_plugins"`
	} `yaml:"user_overrides"`

	SelectionRules struct {
		MaxPackageManagers int      `yaml:"max_package_managers"`
		AlwaysInclude      []string `yaml:"always_include"`
		NeverAutoSelect    []string `yaml:"never_auto_select"`
		PreferNative       bool     `yaml:"prefer_native"`
		IncludeDesktop     bool     `yaml:"include_desktop"`
	} `yaml:"selection_rules"`
}

// PlatformConfig represents platform-specific plugin configuration
type PlatformConfig struct {
	Required     []string `yaml:"required"`
	Optional     []string `yaml:"optional"`
	DesktopAware bool     `yaml:"desktop_aware"`
}

// PluginDependency represents plugin dependency information
type PluginDependency struct {
	Requires  []string `yaml:"requires"`
	Conflicts []string `yaml:"conflicts"`
}

// Selector handles intelligent plugin selection
type Selector struct {
	config         *PluginConfig
	registryClient *RegistryClient
	platform       *platform.Platform
}

// NewSelector creates a new plugin selector
func NewSelector(registryClient *RegistryClient, platformDetector *platform.Detector) (*Selector, error) {
	// Load plugin configuration
	config, err := loadPluginConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin config: %w", err)
	}

	// Detect current platform
	plat, err := platformDetector.DetectPlatform()
	if err != nil {
		return nil, fmt.Errorf("failed to detect platform: %w", err)
	}

	return &Selector{
		config:         config,
		registryClient: registryClient,
		platform:       plat,
	}, nil
}

// SelectPlugins returns the list of plugins to download for the current platform
func (s *Selector) SelectPlugins(ctx context.Context) ([]string, error) {
	selectedPlugins := make(map[string]bool)

	// 1. Start with platform-specific required plugins
	platformKey := s.getPlatformKey()
	if platConfig, ok := s.config.Platforms[s.platform.OS][platformKey]; ok {
		for _, plugin := range platConfig.Required {
			selectedPlugins[plugin] = true
		}

		// Add desktop environment plugin if detected and platform is desktop-aware
		if platConfig.DesktopAware && s.platform.DesktopEnv != "" && s.platform.DesktopEnv != "unknown" {
			desktopPlugin := fmt.Sprintf("desktop-%s", s.platform.DesktopEnv)
			selectedPlugins[desktopPlugin] = true
		}
	} else {
		// Fallback to auto-detection if no platform config exists
		for _, plugin := range s.platform.GetRequiredPlugins() {
			selectedPlugins[plugin] = true
		}
	}

	// 2. Apply user overrides
	if err := s.applyUserOverrides(selectedPlugins); err != nil {
		return nil, fmt.Errorf("failed to apply user overrides: %w", err)
	}

	// 3. Query registry for available plugins
	availablePlugins, err := s.registryClient.GetAvailablePlugins(ctx, s.platform.OS, s.platform.Distribution)
	if err != nil {
		// If registry is unavailable, use local selection
		return s.getLocalSelection(selectedPlugins), nil
	}

	// 4. Filter and prioritize plugins (convert to compatible format)
	compatiblePlugins := s.convertToCompatibleFormat(availablePlugins)
	finalPlugins := s.filterAndPrioritize(selectedPlugins, compatiblePlugins)

	// 5. Resolve dependencies
	if err := s.resolveDependencies(ctx, finalPlugins); err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// 6. Check for conflicts
	if err := s.checkConflicts(finalPlugins); err != nil {
		return nil, fmt.Errorf("plugin conflicts detected: %w", err)
	}

	return s.getPluginList(finalPlugins), nil
}

// loadPluginConfig loads the plugin configuration from file
func loadPluginConfig() (*PluginConfig, error) {
	config := &PluginConfig{}

	// Load default configuration
	configPath := filepath.Join("config", "plugins.yaml")
	defaultConfig, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read default config: %w", err)
	}

	if err := yaml.Unmarshal(defaultConfig, config); err != nil {
		return nil, fmt.Errorf("failed to parse default config: %w", err)
	}

	// Load user overrides if they exist
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userConfigPath := filepath.Join(homeDir, ".devex", "plugins.yaml")
		if userConfig, err := os.ReadFile(userConfigPath); err == nil {
			userOverrides := &PluginConfig{}
			if err := yaml.Unmarshal(userConfig, userOverrides); err == nil {
				// Merge user overrides
				if userOverrides.UserOverrides.PreferredPackageManager != "" {
					config.UserOverrides.PreferredPackageManager = userOverrides.UserOverrides.PreferredPackageManager
				}
				config.UserOverrides.ExcludedPlugins = append(config.UserOverrides.ExcludedPlugins, userOverrides.UserOverrides.ExcludedPlugins...)
				config.UserOverrides.IncludedPlugins = append(config.UserOverrides.IncludedPlugins, userOverrides.UserOverrides.IncludedPlugins...)
			}
		}
	}

	// Also check Viper for runtime overrides
	if viper.IsSet("plugins.excluded") {
		config.UserOverrides.ExcludedPlugins = viper.GetStringSlice("plugins.excluded")
	}
	if viper.IsSet("plugins.included") {
		config.UserOverrides.IncludedPlugins = viper.GetStringSlice("plugins.included")
	}

	return config, nil
}

// getPlatformKey returns the platform configuration key
func (s *Selector) getPlatformKey() string {
	if s.platform.Distribution != "" && s.platform.Distribution != "unknown" {
		return s.platform.Distribution
	}
	return s.platform.OS
}

// applyUserOverrides applies user preferences to plugin selection
func (s *Selector) applyUserOverrides(selectedPlugins map[string]bool) error {
	// Remove excluded plugins
	for _, excluded := range s.config.UserOverrides.ExcludedPlugins {
		delete(selectedPlugins, excluded)
	}

	// Add explicitly included plugins
	for _, included := range s.config.UserOverrides.IncludedPlugins {
		selectedPlugins[included] = true
	}

	// Handle preferred package manager
	if pref := s.config.UserOverrides.PreferredPackageManager; pref != "" {
		// Remove other package managers if preferred is available
		prefPlugin := fmt.Sprintf("package-manager-%s", pref)
		if selectedPlugins[prefPlugin] {
			// Remove other package managers
			for plugin := range selectedPlugins {
				if strings.HasPrefix(plugin, "package-manager-") && plugin != prefPlugin {
					delete(selectedPlugins, plugin)
				}
			}
		}
	}

	return nil
}

// convertToCompatibleFormat converts PluginMetadata to a simpler format for processing
func (s *Selector) convertToCompatibleFormat(plugins []PluginMetadata) []PluginMetadata {
	// Since we're using the existing PluginMetadata type, we just return as-is
	// This is a placeholder for any necessary conversions
	return plugins
}

// filterAndPrioritize filters and prioritizes plugins based on configuration
func (s *Selector) filterAndPrioritize(selectedPlugins map[string]bool, availablePlugins []PluginMetadata) map[string]bool {
	result := make(map[string]bool)

	// Group plugins by type
	packageManagers := []string{}
	desktopPlugins := []string{}
	otherPlugins := []string{}

	for plugin := range selectedPlugins {
		switch {
		case strings.HasPrefix(plugin, "package-manager-"):
			packageManagers = append(packageManagers, plugin)
		case strings.HasPrefix(plugin, "desktop-"):
			desktopPlugins = append(desktopPlugins, plugin)
		default:
			otherPlugins = append(otherPlugins, plugin)
		}
	}

	// Sort package managers by priority
	sort.Slice(packageManagers, func(i, j int) bool {
		pmI := strings.TrimPrefix(packageManagers[i], "package-manager-")
		pmJ := strings.TrimPrefix(packageManagers[j], "package-manager-")
		priorityI := s.config.Priorities.PackageManagers[pmI]
		priorityJ := s.config.Priorities.PackageManagers[pmJ]
		return priorityI > priorityJ
	})

	// Apply max package managers limit
	maxPMs := s.config.SelectionRules.MaxPackageManagers
	if maxPMs > 0 && len(packageManagers) > maxPMs {
		packageManagers = packageManagers[:maxPMs]
	}

	// Add selected plugins to result
	for _, pm := range packageManagers {
		result[pm] = true
	}

	// Add desktop plugins if enabled
	if s.config.SelectionRules.IncludeDesktop {
		for _, dp := range desktopPlugins {
			result[dp] = true
		}
	}

	// Add other plugins
	for _, plugin := range otherPlugins {
		// Check if plugin is in never auto-select list
		isNeverAuto := false
		for _, never := range s.config.SelectionRules.NeverAutoSelect {
			if plugin == never {
				isNeverAuto = true
				break
			}
		}
		if !isNeverAuto {
			result[plugin] = true
		}
	}

	// Always include certain plugins
	for _, always := range s.config.SelectionRules.AlwaysInclude {
		result[always] = true
	}

	return result
}

// resolveDependencies resolves plugin dependencies
func (s *Selector) resolveDependencies(ctx context.Context, selectedPlugins map[string]bool) error {
	changed := true
	for changed {
		changed = false
		for plugin := range selectedPlugins {
			if dep, ok := s.config.Dependencies[plugin]; ok {
				for _, required := range dep.Requires {
					if !selectedPlugins[required] {
						selectedPlugins[required] = true
						changed = true
					}
				}
			}
		}
	}
	return nil
}

// checkConflicts checks for plugin conflicts
func (s *Selector) checkConflicts(selectedPlugins map[string]bool) error {
	for plugin := range selectedPlugins {
		if dep, ok := s.config.Dependencies[plugin]; ok {
			for _, conflict := range dep.Conflicts {
				if selectedPlugins[conflict] {
					return fmt.Errorf("plugin %s conflicts with %s", plugin, conflict)
				}
			}
		}
	}
	return nil
}

// getLocalSelection returns local plugin selection when registry is unavailable
func (s *Selector) getLocalSelection(selectedPlugins map[string]bool) []string {
	result := []string{}
	for plugin := range selectedPlugins {
		result = append(result, plugin)
	}
	sort.Strings(result)
	return result
}

// getPluginList converts the plugin map to a sorted list
func (s *Selector) getPluginList(selectedPlugins map[string]bool) []string {
	result := []string{}
	for plugin := range selectedPlugins {
		result = append(result, plugin)
	}
	sort.Strings(result)
	return result
}
