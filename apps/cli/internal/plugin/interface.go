package plugin

import (
	"context"

	"github.com/spf13/cobra"
)

// Plugin defines the interface that all plugins must implement
type Plugin interface {
	// Name returns the plugin name
	Name() string

	// Version returns the plugin version
	Version() string

	// Description returns a brief description of the plugin
	Description() string

	// Commands returns the cobra commands this plugin provides
	Commands() []*cobra.Command

	// Init is called when the plugin is loaded
	Init(ctx context.Context) error

	// Cleanup is called when the plugin is unloaded
	Cleanup(ctx context.Context) error
}

// Manager manages plugin loading and lifecycle
type Manager struct {
	plugins   map[string]Plugin
	pluginDir string
}

// NewManager creates a new plugin manager
func NewManager(pluginDir string) *Manager {
	return &Manager{
		plugins:   make(map[string]Plugin),
		pluginDir: pluginDir,
	}
}

// LoadPlugin loads a plugin from a .so file
func (m *Manager) LoadPlugin(ctx context.Context, pluginPath string) error {
	// Implementation for loading plugins
	return nil
}

// GetPlugin returns a loaded plugin by name
func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	plugin, exists := m.plugins[name]
	return plugin, exists
}

// ListPlugins returns all loaded plugins
func (m *Manager) ListPlugins() map[string]Plugin {
	return m.plugins
}

// RegisterCommands registers all plugin commands with the root command
func (m *Manager) RegisterCommands(rootCmd *cobra.Command) {
	for _, plugin := range m.plugins {
		for _, cmd := range plugin.Commands() {
			rootCmd.AddCommand(cmd)
		}
	}
}
