package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/spf13/cobra"
)

// PluginInfo represents information about a loaded plugin
type PluginInfo struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Commands    []PluginCommand `json:"commands"`
	Author      string          `json:"author,omitempty"`
	Repository  string          `json:"repository,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Path        string          `json:"-"` // Path to the plugin executable
}

// PluginCommand represents a command provided by a plugin
type PluginCommand struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Usage       string            `json:"usage"`
	Flags       map[string]string `json:"flags,omitempty"`
}

// ExecutableManager manages executable plugins
type ExecutableManager struct {
	pluginDir string
	plugins   map[string]*PluginInfo
}

// NewExecutableManager creates a new executable plugin manager
func NewExecutableManager(pluginDir string) *ExecutableManager {
	return &ExecutableManager{
		pluginDir: pluginDir,
		plugins:   make(map[string]*PluginInfo),
	}
}

// DiscoverPlugins discovers all plugins in the plugin directory
func (m *ExecutableManager) DiscoverPlugins() error {
	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Look for plugin executables
		if !strings.HasPrefix(entry.Name(), "devex-plugin-") {
			continue
		}

		pluginPath := filepath.Join(m.pluginDir, entry.Name())
		if err := m.loadPlugin(pluginPath); err != nil {
			log.Warn("Failed to load plugin", "path", pluginPath, "error", err)
			continue
		}
	}

	log.Debug("Discovered plugins", "count", len(m.plugins))
	return nil
}

// loadPlugin loads a single plugin
func (m *ExecutableManager) loadPlugin(pluginPath string) error {
	// Get plugin info by running the plugin with --plugin-info flag
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, pluginPath, "--plugin-info")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}

	var info PluginInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return fmt.Errorf("failed to parse plugin info: %w", err)
	}

	info.Path = pluginPath
	m.plugins[info.Name] = &info

	log.Debug("Loaded plugin", "name", info.Name, "version", info.Version)
	return nil
}

// ListPlugins returns all loaded plugins
func (m *ExecutableManager) ListPlugins() map[string]*PluginInfo {
	return m.plugins
}

// ExecutePlugin executes a plugin with the given arguments
func (m *ExecutableManager) ExecutePlugin(pluginName string, args []string) error {
	plugin, exists := m.plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, plugin.Path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// RegisterCommands registers plugin commands with cobra
func (m *ExecutableManager) RegisterCommands(rootCmd *cobra.Command) {
	for pluginName, plugin := range m.plugins {
		// Create a command group for each plugin
		pluginCmd := &cobra.Command{
			Use:   pluginName,
			Short: plugin.Description,
		}

		// Add each plugin command as a subcommand
		for _, cmd := range plugin.Commands {
			cmdName := cmd.Name
			cmdDesc := cmd.Description
			pName := pluginName // Capture for closure

			subCmd := &cobra.Command{
				Use:   cmdName,
				Short: cmdDesc,
				Long:  cmd.Usage,
				RunE: func(cmd *cobra.Command, args []string) error {
					// Execute the plugin with the command name and args
					pluginArgs := append([]string{cmdName}, args...)
					return m.ExecutePlugin(pName, pluginArgs)
				},
			}

			// Add flags if specified
			for flagName, flagDesc := range cmd.Flags {
				subCmd.Flags().String(flagName, "", flagDesc)
			}

			pluginCmd.AddCommand(subCmd)
		}

		rootCmd.AddCommand(pluginCmd)
	}
}

// GetPluginDir returns the plugin directory path
func (m *ExecutableManager) GetPluginDir() string {
	return m.pluginDir
}

// InstallPlugin installs a plugin from a given path
func (m *ExecutableManager) InstallPlugin(sourcePath string, pluginName string) error {
	destPath := filepath.Join(m.pluginDir, fmt.Sprintf("devex-plugin-%s", pluginName))
	if runtime.GOOS == "windows" {
		destPath += ".exe"
	}

	// Copy the plugin to the plugin directory
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source plugin: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination plugin: %w", err)
	}
	defer destFile.Close()

	if _, err := destFile.ReadFrom(sourceFile); err != nil {
		return fmt.Errorf("failed to copy plugin: %w", err)
	}

	// Make executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("failed to make plugin executable: %w", err)
		}
	}

	// Load the plugin
	return m.loadPlugin(destPath)
}

// RemovePlugin removes a plugin
func (m *ExecutableManager) RemovePlugin(pluginName string) error {
	plugin, exists := m.plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	if err := os.Remove(plugin.Path); err != nil {
		return fmt.Errorf("failed to remove plugin file: %w", err)
	}

	delete(m.plugins, pluginName)
	return nil
}
