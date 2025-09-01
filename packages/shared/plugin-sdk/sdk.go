package sdk

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// PluginInfo represents the standard plugin information
type PluginInfo struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Commands    []PluginCommand `json:"commands"`
	Author      string          `json:"author,omitempty"`
	Repository  string          `json:"repository,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
}

// PluginCommand represents a command provided by a plugin
type PluginCommand struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Usage       string            `json:"usage"`
	Flags       map[string]string `json:"flags,omitempty"`
}

// Plugin interface for implementing plugins
type Plugin interface {
	Info() PluginInfo
	Execute(command string, args []string) error
}

// DesktopPlugin interface for desktop environment plugins
type DesktopPlugin interface {
	Plugin
	
	// Desktop-specific methods
	IsAvailable() bool
	GetDesktopEnvironment() string
}

// BasePlugin provides common functionality for plugins
type BasePlugin struct {
	info PluginInfo
}

// NewBasePlugin creates a new base plugin
func NewBasePlugin(info PluginInfo) *BasePlugin {
	return &BasePlugin{info: info}
}

// Info returns the plugin information
func (p *BasePlugin) Info() PluginInfo {
	return p.info
}

// OutputPluginInfo outputs plugin info as JSON (for --plugin-info)
func (p *BasePlugin) OutputPluginInfo() {
	output, err := json.MarshalIndent(p.info, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal plugin info: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(string(output))
}

// CommandExists checks if a command exists in PATH
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// RequireSudo checks if sudo is required and available
func RequireSudo() bool {
	return CommandExists("sudo") && os.Getuid() != 0
}

// ExecCommand executes a command with optional sudo
func ExecCommand(useSudo bool, name string, args ...string) error {
	var cmd *exec.Cmd

	if useSudo && RequireSudo() {
		cmdArgs := append([]string{name}, args...)
		cmd = exec.Command("sudo", cmdArgs...)
	} else {
		cmd = exec.Command(name, args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// ExecCommandOutput executes a command and returns output
func ExecCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	return string(output), err
}

// PackageManagerPlugin provides common functionality for package manager plugins
type PackageManagerPlugin struct {
	*BasePlugin
	managerCommand string
}

// NewPackageManagerPlugin creates a new package manager plugin
func NewPackageManagerPlugin(info PluginInfo, managerCommand string) *PackageManagerPlugin {
	return &PackageManagerPlugin{
		BasePlugin:     NewBasePlugin(info),
		managerCommand: managerCommand,
	}
}

// IsAvailable checks if the package manager is available on the system
func (p *PackageManagerPlugin) IsAvailable() bool {
	return CommandExists(p.managerCommand)
}

// EnsureAvailable ensures the package manager is available or exits with error
func (p *PackageManagerPlugin) EnsureAvailable() {
	if !p.IsAvailable() {
		fmt.Fprintf(os.Stderr, "Error: %s is not available on this system\n", p.managerCommand)
		os.Exit(1)
	}
}

// HandleArgs provides standard argument handling for plugins
func HandleArgs(plugin Plugin, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", os.Args[0])
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "--plugin-info":
		// Get plugin info directly from the interface
		info := plugin.Info()
		output, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal plugin info: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(string(output))
	default:
		if err := plugin.Execute(command, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetEnv gets an environment variable with a default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// RunCommand runs a command and returns its output
func RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// IsRoot checks if the current user is root
func IsRoot() bool {
	return os.Getuid() == 0
}

// PluginMetadata represents plugin metadata with path information
type PluginMetadata struct {
	PluginInfo
	Path      string   `json:"path"`
	Platforms []string `json:"platforms,omitempty"` // Supported platforms (linux, darwin, windows)
}

// Downloader handles plugin downloading from registry
type Downloader struct {
	registryURL string
	pluginDir   string
}

// NewDownloader creates a new plugin downloader
func NewDownloader(registryURL, pluginDir string) *Downloader {
	return &Downloader{
		registryURL: registryURL,
		pluginDir:   pluginDir,
	}
}

// GetAvailablePlugins returns available plugins from registry
func (d *Downloader) GetAvailablePlugins() (map[string]PluginMetadata, error) {
	// Placeholder implementation - will connect to registry after release
	return make(map[string]PluginMetadata), nil
}

// SearchPlugins searches for plugins by query
func (d *Downloader) SearchPlugins(query string) (map[string]PluginMetadata, error) {
	// Placeholder implementation - will search registry after release
	return make(map[string]PluginMetadata), nil
}

// DownloadPlugin downloads a plugin from the registry
func (d *Downloader) DownloadPlugin(pluginName string) error {
	// Placeholder implementation - will download from registry after release
	return fmt.Errorf("plugin downloading not yet implemented - registry will be available after release")
}

// DownloadRequiredPlugins downloads all required plugins for the platform
func (d *Downloader) DownloadRequiredPlugins(requiredPlugins []string) error {
	// Placeholder implementation - will download required plugins after release
	return fmt.Errorf("plugin downloading not yet implemented - registry will be available after release")
}

// UpdateRegistry updates the plugin registry
func (d *Downloader) UpdateRegistry() error {
	// Placeholder implementation - will update registry after release
	return nil
}

// ExecutableManager manages plugin executables
type ExecutableManager struct {
	pluginDir string
}

// NewExecutableManager creates a new executable manager
func NewExecutableManager(pluginDir string) *ExecutableManager {
	return &ExecutableManager{
		pluginDir: pluginDir,
	}
}

// GetPluginDir returns the plugin directory
func (em *ExecutableManager) GetPluginDir() string {
	return em.pluginDir
}

// ListPlugins returns installed plugins
func (em *ExecutableManager) ListPlugins() map[string]PluginMetadata {
	// Placeholder implementation - will scan plugin directory after release
	return make(map[string]PluginMetadata)
}

// ExecutePlugin executes a plugin with given arguments
func (em *ExecutableManager) ExecutePlugin(pluginName string, args []string) error {
	// Placeholder implementation - will execute plugin after release
	return fmt.Errorf("plugin execution not yet implemented - plugins will be available after release")
}

// DiscoverPlugins discovers plugins in the plugin directory
func (em *ExecutableManager) DiscoverPlugins() error {
	// Placeholder implementation - will discover plugins after release
	return nil
}

// RemovePlugin removes a plugin
func (em *ExecutableManager) RemovePlugin(pluginName string) error {
	// Placeholder implementation - will remove plugin after release
	return fmt.Errorf("plugin removal not yet implemented - plugins will be available after release")
}

// InstallPlugin installs a plugin from a source path
func (em *ExecutableManager) InstallPlugin(sourcePath, pluginName string) error {
	// Placeholder implementation - will install plugin after release
	return fmt.Errorf("plugin installation not yet implemented - plugins will be available after release")
}

// RegisterCommands registers plugin commands (placeholder)
func (em *ExecutableManager) RegisterCommands(rootCmd interface{}) error {
	// Placeholder implementation - will register plugin commands after release
	return nil
}
