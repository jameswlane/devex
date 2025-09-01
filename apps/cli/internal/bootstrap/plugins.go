package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
	"github.com/spf13/cobra"
)

const (
	// Default plugin registry URL - you would host this
	DefaultRegistryURL = "https://registry.devex.sh/v1"
)

// PluginBootstrap handles the automatic plugin discovery and loading
type PluginBootstrap struct {
	detector     *platform.Detector
	downloader   *sdk.Downloader
	manager      *sdk.ExecutableManager
	platform     *platform.Platform
	skipDownload bool
}

// NewPluginBootstrap creates a new plugin bootstrap instance
func NewPluginBootstrap(skipDownload bool) (*PluginBootstrap, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Verify the home directory is accessible
	if _, err := os.Stat(homeDir); err != nil {
		return nil, fmt.Errorf("failed to get user home directory: home directory not accessible: %w", err)
	}

	pluginDir := filepath.Join(homeDir, ".devex", "plugins")

	// Create plugin directory if download is enabled (real use case)
	if !skipDownload {
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create plugin directory: %w", err)
		}
	}

	downloader := sdk.NewDownloader(DefaultRegistryURL, pluginDir)

	// Configure downloader logger based on test mode
	if log.IsTestMode() {
		downloader.SetSilent(true)
	}

	return &PluginBootstrap{
		detector:     platform.NewDetector(),
		downloader:   downloader,
		manager:      sdk.NewExecutableManager(pluginDir),
		skipDownload: skipDownload,
	}, nil
}

// Initialize performs the complete plugin bootstrap process
func (b *PluginBootstrap) Initialize(ctx context.Context) error {
	// Detect platform
	platform, err := b.detector.DetectPlatform()
	if err != nil {
		return fmt.Errorf("failed to detect platform: %w", err)
	}
	b.platform = platform

	// Download required plugins (unless skipped)
	if !b.skipDownload {
		requiredPlugins := platform.GetRequiredPlugins()
		if err := b.downloader.DownloadRequiredPluginsWithContext(ctx, requiredPlugins); err != nil {
			log.Warning("Failed to download some plugins: %v", err)
			// Continue anyway - some plugins might still be available
		}
	}

	// Discover and load available plugins
	if err := b.manager.DiscoverPluginsWithContext(ctx); err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}

	return nil
}

// RegisterCommands registers all plugin commands with the root command
func (b *PluginBootstrap) RegisterCommands(rootCmd *cobra.Command) {
	if err := b.manager.RegisterCommands(rootCmd); err != nil {
		log.Warning("Failed to register some plugin commands: %v", err)
		// Continue anyway - plugin commands are optional
	}

	// Add plugin management commands
	rootCmd.AddCommand(b.createPluginManagementCmd())
}

// GetManager returns the plugin manager
func (b *PluginBootstrap) GetManager() *sdk.ExecutableManager {
	return b.manager
}

// GetAvailablePlugins returns available plugins from registry
func (b *PluginBootstrap) GetAvailablePlugins() (map[string]sdk.PluginMetadata, error) {
	return b.downloader.GetAvailablePlugins()
}

// IsPluginAvailable checks if a plugin is available for installation
func (b *PluginBootstrap) IsPluginAvailable(pluginName string) bool {
	if err := validatePluginName(pluginName); err != nil {
		log.Warn("Invalid plugin name provided", "name", pluginName, "error", err)
		return false
	}

	plugins, err := b.downloader.GetAvailablePlugins()
	if err != nil {
		return false
	}
	_, exists := plugins[pluginName]
	return exists
}

// ExecutePlugin executes a plugin with given arguments
func (b *PluginBootstrap) ExecutePlugin(pluginName string, args []string) error {
	if err := validatePluginName(pluginName); err != nil {
		return fmt.Errorf("invalid plugin name: %w", err)
	}

	return b.manager.ExecutePlugin(pluginName, args)
}

// GetPlatform returns the detected platform
func (b *PluginBootstrap) GetPlatform() *platform.Platform {
	return b.platform
}

// createPluginManagementCmd creates the plugin management command tree
func (b *PluginBootstrap) createPluginManagementCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage DevEx plugins",
		Long:  `Install, remove, list, and manage DevEx plugins`,
	}

	// plugin list command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE:  b.handleListPlugins,
	}

	// plugin search command
	searchCmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search available plugins",
		Args:  cobra.MaximumNArgs(1),
		RunE:  b.handleSearchPlugins,
	}

	// plugin install command
	installCmd := &cobra.Command{
		Use:   "install [plugin-name]",
		Short: "Install a plugin",
		Args:  cobra.ExactArgs(1),
		RunE:  b.handleInstallPlugin,
	}

	// plugin remove command
	removeCmd := &cobra.Command{
		Use:   "remove [plugin-name]",
		Short: "Remove a plugin",
		Args:  cobra.ExactArgs(1),
		RunE:  b.handleRemovePlugin,
	}

	// plugin update command
	updateCmd := &cobra.Command{
		Use:   "update [plugin-name]",
		Short: "Update plugins (or specific plugin)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  b.handleUpdatePlugins,
	}

	// plugin info command
	infoCmd := &cobra.Command{
		Use:   "info [plugin-name]",
		Short: "Show plugin information",
		Args:  cobra.ExactArgs(1),
		RunE:  b.handlePluginInfo,
	}

	cmd.AddCommand(listCmd, searchCmd, installCmd, removeCmd, updateCmd, infoCmd)
	return cmd
}

// handleListPlugins lists all installed plugins
func (b *PluginBootstrap) handleListPlugins(cmd *cobra.Command, args []string) error {
	plugins := b.manager.ListPlugins()

	if len(plugins) == 0 {
		log.Println("No plugins installed")
		log.Printf("Platform: %s", b.platform.String())
		log.Printf("Available package managers: %s", strings.Join(b.platform.PackageManagers, ", "))
		log.Println("\nRun 'devex plugin search' to find available plugins")
		return nil
	}

	log.Printf("Platform: %s", b.platform.String())
	log.Printf("Plugin directory: %s\n", b.manager.GetPluginDir())
	log.Println("Installed plugins:")

	for name, pluginInfo := range plugins {
		fmt.Printf("📦 %s v%s\n", name, pluginInfo.Version)
		fmt.Printf("   %s\n", pluginInfo.Description)
		if len(pluginInfo.Commands) > 0 {
			fmt.Printf("   Commands: %s\n", strings.Join(getCommandNames(pluginInfo.Commands), ", "))
		}
		fmt.Println()
	}

	return nil
}

// handleSearchPlugins searches for available plugins
func (b *PluginBootstrap) handleSearchPlugins(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	var results map[string]sdk.PluginMetadata
	var err error

	if query == "" {
		results, err = b.downloader.GetAvailablePlugins()
	} else {
		results, err = b.downloader.SearchPlugins(query)
	}

	if err != nil {
		return fmt.Errorf("failed to search plugins: %w", err)
	}

	if len(results) == 0 {
		if query == "" {
			fmt.Println("No plugins available")
		} else {
			fmt.Printf("No plugins found for query: %s\n", query)
		}
		return nil
	}

	fmt.Printf("Available plugins:\n\n")
	for name, metadata := range results {
		fmt.Printf("📦 %s v%s\n", name, metadata.Version)
		fmt.Printf("   %s\n", metadata.Description)
		if len(metadata.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", strings.Join(metadata.Tags, ", "))
		}

		// Check if available for current platform
		platformSupported := false
		for platformKey := range metadata.Platforms {
			if strings.HasPrefix(platformKey, runtime.GOOS) {
				platformSupported = true
				break
			}
		}
		if platformSupported {
			fmt.Printf("   ✅ Available for your platform\n")
		} else {
			fmt.Printf("   ❌ Not available for your platform (%s)\n", runtime.GOOS)
		}
		fmt.Println()
	}

	return nil
}

// handleInstallPlugin installs a specific plugin
func (b *PluginBootstrap) handleInstallPlugin(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	if err := b.downloader.UpdateRegistry(); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	if err := b.downloader.DownloadPlugin(pluginName); err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	// Reload plugins
	if err := b.manager.DiscoverPlugins(); err != nil {
		fmt.Printf("Warning: failed to reload plugins: %v\n", err)
	}

	fmt.Printf("Plugin %s installed successfully!\n", pluginName)
	fmt.Println("Restart devex or run 'devex plugin list' to see new commands")
	return nil
}

// handleRemovePlugin removes a specific plugin
func (b *PluginBootstrap) handleRemovePlugin(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	plugins := b.manager.ListPlugins()
	pluginInfo, exists := plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s is not installed", pluginName)
	}

	if err := os.Remove(pluginInfo.Path); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	fmt.Printf("Plugin %s removed successfully\n", pluginName)
	return nil
}

// handleUpdatePlugins updates plugins
func (b *PluginBootstrap) handleUpdatePlugins(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// Update all plugins
		return b.updateAllPlugins()
	}

	// Update specific plugin
	pluginName := args[0]
	if err := b.downloader.UpdateRegistry(); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	return b.downloader.DownloadPlugin(pluginName)
}

// handlePluginInfo shows information about a specific plugin
func (b *PluginBootstrap) handlePluginInfo(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	plugins := b.manager.ListPlugins()
	pluginInfo, exists := plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s is not installed", pluginName)
	}

	fmt.Printf("Plugin: %s\n", pluginInfo.Name)
	fmt.Printf("Version: %s\n", pluginInfo.Version)
	fmt.Printf("Description: %s\n", pluginInfo.Description)
	fmt.Printf("Path: %s\n", pluginInfo.Path)
	fmt.Printf("Commands:\n")

	for _, pluginCmd := range pluginInfo.Commands {
		fmt.Printf("  %s - %s\n", pluginCmd.Name, pluginCmd.Description)
	}

	return nil
}

// updateAllPlugins updates all installed plugins
func (b *PluginBootstrap) updateAllPlugins() error {
	if err := b.downloader.UpdateRegistry(); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	plugins := b.manager.ListPlugins()

	if len(plugins) == 0 {
		fmt.Println("No plugins installed to update")
		return nil
	}

	fmt.Printf("Updating %d plugins...\n", len(plugins))

	for name := range plugins {
		if err := b.downloader.DownloadPlugin(name); err != nil {
			fmt.Printf("Warning: failed to update plugin %s: %v\n", name, err)
		}
	}

	fmt.Println("Plugin update complete!")
	return nil
}

// getCommandNames extracts command names from PluginCommand slice
func getCommandNames(commands []sdk.PluginCommand) []string {
	names := make([]string, len(commands))
	for i, cmd := range commands {
		names[i] = cmd.Name
	}
	return names
}

// Regular expression for valid plugin names
// Allows alphanumeric characters, hyphens, and underscores
// Prevents directory traversal attacks and special characters
var validPluginNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// validatePluginName validates plugin name to prevent directory traversal and injection attacks
func validatePluginName(pluginName string) error {
	if pluginName == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	// Check length constraints
	if len(pluginName) > 64 {
		return fmt.Errorf("plugin name too long (max 64 characters): %s", pluginName)
	}

	if len(pluginName) < 1 {
		return fmt.Errorf("plugin name too short (min 1 character)")
	}

	// Check for directory traversal attempts
	if strings.Contains(pluginName, "..") ||
		strings.Contains(pluginName, "/") ||
		strings.Contains(pluginName, "\\") {
		return fmt.Errorf("plugin name contains invalid characters (directory traversal detected): %s", pluginName)
	}

	// Check for null bytes and control characters
	for _, char := range pluginName {
		if char == 0 || char < 32 {
			return fmt.Errorf("plugin name contains null bytes or control characters: %s", pluginName)
		}
	}

	// Validate against regex pattern
	if !validPluginNameRegex.MatchString(pluginName) {
		return fmt.Errorf("plugin name contains invalid characters (must be alphanumeric with hyphens/underscores, start with alphanumeric): %s", pluginName)
	}

	// Check for reserved names
	reservedNames := []string{
		".", "..", "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}

	upperPluginName := strings.ToUpper(pluginName)
	for _, reserved := range reservedNames {
		if upperPluginName == reserved {
			return fmt.Errorf("plugin name is reserved and cannot be used: %s", pluginName)
		}
	}

	return nil
}
