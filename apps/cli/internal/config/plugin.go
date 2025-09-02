package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PluginConfig represents configuration for plugins
type PluginConfig struct {
	Plugins map[string]PluginSettings `yaml:"plugins"`
}

// PluginSettings represents settings for a specific plugin
type PluginSettings struct {
	Enabled  bool                   `yaml:"enabled"`
	Config   map[string]interface{} `yaml:"config"`
	Priority int                    `yaml:"priority"`
}

// PluginConfigManager manages plugin configurations
type PluginConfigManager struct {
	configPath string
	config     *PluginConfig
}

// NewPluginConfigManager creates a new plugin config manager
func NewPluginConfigManager() (*PluginConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".devex", "plugins.yaml")
	manager := &PluginConfigManager{
		configPath: configPath,
		config: &PluginConfig{
			Plugins: make(map[string]PluginSettings),
		},
	}

	// Load existing config if it exists
	if err := manager.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load plugin config: %w", err)
	}

	return manager, nil
}

// Load loads the plugin configuration from disk
func (m *PluginConfigManager) Load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, m.config)
}

// Save saves the plugin configuration to disk
func (m *PluginConfigManager) Save() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// EnablePlugin enables a plugin
func (m *PluginConfigManager) EnablePlugin(name string) error {
	settings := m.config.Plugins[name]
	settings.Enabled = true
	m.config.Plugins[name] = settings
	return m.Save()
}

// DisablePlugin disables a plugin
func (m *PluginConfigManager) DisablePlugin(name string) error {
	settings := m.config.Plugins[name]
	settings.Enabled = false
	m.config.Plugins[name] = settings
	return m.Save()
}

// IsPluginEnabled checks if a plugin is enabled
func (m *PluginConfigManager) IsPluginEnabled(name string) bool {
	settings, exists := m.config.Plugins[name]
	if !exists {
		return true // Default to enabled
	}
	return settings.Enabled
}

// GetPluginConfig gets configuration for a specific plugin
func (m *PluginConfigManager) GetPluginConfig(name string) map[string]interface{} {
	settings, exists := m.config.Plugins[name]
	if !exists {
		return make(map[string]interface{})
	}
	return settings.Config
}

// SetPluginConfig sets configuration for a specific plugin
func (m *PluginConfigManager) SetPluginConfig(name string, config map[string]interface{}) error {
	settings := m.config.Plugins[name]
	settings.Config = config
	m.config.Plugins[name] = settings
	return m.Save()
}

// SetPluginPriority sets the priority for a plugin (higher number = higher priority)
func (m *PluginConfigManager) SetPluginPriority(name string, priority int) error {
	settings := m.config.Plugins[name]
	settings.Priority = priority
	m.config.Plugins[name] = settings
	return m.Save()
}

// GetPluginPriority gets the priority for a plugin
func (m *PluginConfigManager) GetPluginPriority(name string) int {
	settings, exists := m.config.Plugins[name]
	if !exists {
		return 0 // Default priority
	}
	return settings.Priority
}

// ListPluginConfigs returns all plugin configurations
func (m *PluginConfigManager) ListPluginConfigs() map[string]PluginSettings {
	return m.config.Plugins
}
