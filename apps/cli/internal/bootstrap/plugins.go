package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"text/tabwriter"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultRegistryURL Default plugin registry URL - you would host this
	DefaultRegistryURL = "https://registry.devex.sh"
	// BootstrapCacheDuration Cache duration for bootstrap initialization
	BootstrapCacheDuration = 24 * time.Hour
)

// BootstrapCache stores cached bootstrap results
type BootstrapCache struct {
	mu            sync.RWMutex
	platformCache *platform.Platform
	platformTime  time.Time
	pluginsCache  []string
	pluginsTime   time.Time
}

// getCachedPlatform returns cached platform if valid, nil otherwise
func (c *BootstrapCache) getCachedPlatform() *platform.Platform {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.platformCache != nil && time.Since(c.platformTime) < BootstrapCacheDuration {
		return c.platformCache
	}
	return nil
}

// setCachedPlatform stores platform in cache
func (c *BootstrapCache) setCachedPlatform(p *platform.Platform) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.platformCache = p
	c.platformTime = time.Now()
}

// getCachedPlugins returns cached required plugins if valid, nil otherwise
func (c *BootstrapCache) getCachedPlugins() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.pluginsCache != nil && time.Since(c.pluginsTime) < BootstrapCacheDuration {
		// Return a copy to prevent external modification
		plugins := make([]string, len(c.pluginsCache))
		copy(plugins, c.pluginsCache)
		return plugins
	}
	return nil
}

// setCachedPlugins stores required plugins in cache
func (c *BootstrapCache) setCachedPlugins(plugins []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pluginsCache = make([]string, len(plugins))
	copy(c.pluginsCache, plugins)
	c.pluginsTime = time.Now()
}

// PluginProgressEvent reports progress for one required plugin.
type PluginProgressEvent = sdk.PluginProgressEvent

// PluginProgressCallback receives required-plugin progress events.
type PluginProgressCallback = sdk.PluginProgressCallback

// PluginBootstrap handles the automatic plugin discovery and loading
type PluginBootstrap struct {
	detector       *platform.Detector
	downloader     *sdk.Downloader
	manager        *sdk.ExecutableManager
	platform       *platform.Platform
	skipDownload   bool
	cache          *BootstrapCache
	downloadErrors map[string]error
}

// GetRegistryURL returns the plugin registry URL from environment or default
func GetRegistryURL() string {
	// Check environment variable first
	if url := os.Getenv("DEVEX_PLUGIN_REGISTRY_URL"); url != "" {
		return url
	}

	// Check for registry URL in config file
	if configURL := getRegistryURLFromConfig(); configURL != "" {
		return configURL
	}

	// Fall back to default
	return DefaultRegistryURL
}

// getRegistryURLFromConfig attempts to read registry URL from config file
func getRegistryURLFromConfig() string {
	identity, err := sdk.NewPrivilegeResolver().ResolveUser(nil)
	if err != nil {
		return ""
	}

	// Check for config file
	configPath := devexpaths.ForHome(identity.HomeDir).ReadPath(filepath.Join(devexpaths.ForHome(identity.HomeDir).Config, "config.yaml"))
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	// Parse YAML properly to extract registry URL
	var config struct {
		PluginRegistryURL string `yaml:"plugin_registry_url"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		// If YAML parsing fails, return empty string (fallback to default)
		return ""
	}

	if config.PluginRegistryURL != "" {
		return config.PluginRegistryURL
	}

	return ""
}

// NewPluginBootstrap creates a new plugin bootstrap instance
func NewPluginBootstrap(skipDownload bool) (*PluginBootstrap, error) {
	resolver := sdk.NewPrivilegeResolver()
	identity, err := resolver.ResolveUser(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	homeDir := identity.HomeDir
	// Verify the home directory is accessible
	if _, err := os.Stat(homeDir); err != nil {
		return nil, fmt.Errorf("failed to get user home directory: home directory not accessible: %w", err)
	}

	pluginDir := devexpaths.ForHome(homeDir).ReadPath(devexpaths.ForHome(homeDir).Plugins())

	// Get registry URL with fallback to default
	registryURL := GetRegistryURL()

	// Log the registry URL being used (only in non-test mode)
	if !log.IsTestMode() && registryURL != DefaultRegistryURL {
		log.Info("Using custom plugin registry", "url", registryURL)
	}

	downloader := sdk.NewDownloader(registryURL, pluginDir)
	if resolver.Elevated && identity.UID != "0" {
		if err := downloader.SetInstallOwner(*identity); err != nil {
			return nil, err
		}
	}
	if !skipDownload {
		if err := downloader.PrepareInstallDirectory(); err != nil {
			return nil, fmt.Errorf("failed to create plugin directory: %w", err)
		}
	}

	// Configure downloader to use CLI logger adapter
	// In test mode, silence stdout but still log to file
	// In production, log everything to file (silent mode for TUI compatibility)
	loggerAdapter := NewSDKLoggerAdapter(true) // Always silent to avoid TUI conflicts
	downloader.SetLogger(loggerAdapter)

	return &PluginBootstrap{
		detector:     platform.NewDetector(),
		downloader:   downloader,
		manager:      sdk.NewExecutableManager(pluginDir),
		skipDownload: skipDownload,
		cache:        &BootstrapCache{},
	}, nil
}

// Initialize performs the complete plugin bootstrap process with caching
func (b *PluginBootstrap) Initialize(ctx context.Context) error {
	return b.InitializeWithProgress(ctx, nil)
}

// InitializeWithProgress performs plugin bootstrap and reports per-plugin download progress.
func (b *PluginBootstrap) InitializeWithProgress(ctx context.Context, progress sdk.PluginProgressCallback) error {
	// Try to get cached platform first
	platform := b.cache.getCachedPlatform()
	if platform == nil {
		// Cache miss - detect platform
		var err error
		platform, err = b.detector.DetectPlatform() //nolint:contextcheck // Platform detection doesn't require context
		if err != nil {
			return fmt.Errorf("failed to detect platform: %w", err)
		}
		// Cache the detected platform
		b.cache.setCachedPlatform(platform)
		log.Debug("Platform detection cached")
	} else {
		log.Debug("Using cached platform detection")
	}
	b.platform = platform

	// Download required plugins (unless skipped)
	if !b.skipDownload {
		// Try to get cached required plugins
		requiredPlugins := b.cache.getCachedPlugins()
		if requiredPlugins == nil {
			// Cache miss - get required plugins
			requiredPlugins = platform.GetRequiredPlugins()
			// Cache the required plugins list
			b.cache.setCachedPlugins(requiredPlugins)
			log.Debug("Required plugins list cached")
		} else {
			log.Debug("Using cached required plugins list")
		}

		if err := b.downloader.DownloadRequiredPluginsWithContextAndProgress(ctx, requiredPlugins, progress); err != nil {
			b.recordDownloadErrors(err)
			var unavailableErr *sdk.RegistryUnavailableError
			if errors.As(err, &unavailableErr) {
				for _, pluginName := range requiredPlugins {
					if _, recorded := b.downloadErrors[pluginName]; !recorded {
						b.downloadErrors[pluginName] = &sdk.DownloadError{Plugin: pluginName, Err: unavailableErr}
					}
				}
			}
			// Continue anyway - commands that do not need these plugins remain available.
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
func (b *PluginBootstrap) GetAvailablePlugins(ctx context.Context) (map[string]sdk.PluginMetadata, error) {
	return b.downloader.GetAvailablePlugins(ctx)
}

// IsPluginAvailable checks if a plugin is available for installation
func (b *PluginBootstrap) IsPluginAvailable(ctx context.Context, pluginName string) bool {
	if err := validatePluginName(pluginName); err != nil {
		log.Warn("Invalid plugin name provided", "name", pluginName, "error", err)
		return false
	}

	// An installed plugin is usable without asking the registry, including offline.
	if info, err := os.Stat(filepath.Join(b.manager.GetPluginDir(), "devex-plugin-"+pluginName)); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
		return true
	}

	plugins, err := b.downloader.GetAvailablePlugins(ctx)
	if err != nil {
		return false
	}
	_, exists := plugins[pluginName]
	return exists
}

func (b *PluginBootstrap) recordDownloadErrors(err error) {
	if b.downloadErrors == nil {
		b.downloadErrors = make(map[string]error)
	}
	var multiErr *sdk.MultiError
	if errors.As(err, &multiErr) {
		for _, downloadErr := range multiErr.Errors {
			b.recordDownloadErrors(downloadErr)
		}
		return
	}
	var downloadErr *sdk.DownloadError
	if errors.As(err, &downloadErr) {
		b.downloadErrors[downloadErr.Plugin] = downloadErr
	}
}

// PluginDownloadError returns the bootstrap download failure for a plugin, if any.
func (b *PluginBootstrap) PluginDownloadError(pluginName string) error {
	return b.downloadErrors[pluginName]
}

// ExecutePlugin executes a plugin with given arguments
func (b *PluginBootstrap) ExecutePlugin(pluginName string, args []string) error {
	return b.ExecutePluginWithContext(context.Background(), pluginName, args)
}

// ExecutePluginWithContext executes a plugin with cancellation from the caller.
func (b *PluginBootstrap) ExecutePluginWithContext(ctx context.Context, pluginName string, args []string) error {
	if err := validatePluginName(pluginName); err != nil {
		return fmt.Errorf("invalid plugin name: %w", err)
	}
	if _, installed := b.manager.ListPluginsWithContext(ctx)[pluginName]; !installed {
		if err := b.PluginDownloadError(pluginName); err != nil {
			return fmt.Errorf("required plugin %s is unavailable: %w", pluginName, err)
		}
	}

	return b.manager.ExecutePluginWithContext(ctx, pluginName, args)
}

// GetPlatform returns the detected platform
func (b *PluginBootstrap) GetPlatform() *platform.Platform {
	return b.platform
}

// SetSilent enables/disables silent mode for plugin downloading
func (b *PluginBootstrap) SetSilent(silent bool) {
	if b.downloader != nil {
		b.downloader.SetSilent(silent)
	}
}

// NewPluginManagementCmd registers the management surface before Cobra resolves arguments.
// Initialization is deferred so help works without network or filesystem access.
func NewPluginManagementCmd() *cobra.Command {
	return (&PluginBootstrap{}).createPluginManagementCmd()
}

func (b *PluginBootstrap) createPluginManagementCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "plugin", Short: "Install, inspect, run, update, and remove DevEx plugins",
		Example: "  devex plugin list --format json\n  devex plugin install tool-stackdetector\n  devex plugin run tool-stackdetector detect --path .\n  devex plugin update --all",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Preserve the root's configuration setup when overriding its hook.
			if root := cmd.Root(); root.Name() != "plugin" && root.PersistentPreRunE != nil {
				if err := root.PersistentPreRunE(cmd, args); err != nil {
					return err
				}
			}
			if b.manager == nil {
				initialized, err := NewPluginBootstrap(true)
				if err != nil {
					return err
				}
				*b = *initialized
			}
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			if err := b.manager.DiscoverPluginsWithContext(cmd.Context()); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return cmd.Context().Err()
		},
	}
	list := &cobra.Command{Use: "list", Short: "List installed versions, paths, and registry updates", Args: cobra.NoArgs, RunE: b.handleListPlugins}
	list.Flags().String("format", "table", "Output format (table, json)")
	search := &cobra.Command{Use: "search [query]", Short: "Search the plugin registry", Args: cobra.MaximumNArgs(1), RunE: b.handleSearchPlugins}
	install := &cobra.Command{Use: "install <name>", Short: "Install a checksum-verified plugin", Args: pluginNamesArgs(1), RunE: b.handleInstallPlugin}
	remove := &cobra.Command{Use: "remove <name>", Short: "Remove an installed plugin and its install record", Args: pluginNamesArgs(1), RunE: b.handleRemovePlugin}
	remove.Flags().Bool("force", false, "Allow removal of critical package-manager and system-setup plugins")
	update := &cobra.Command{Use: "update [name...]", Short: "Update named plugins or every installed plugin with --all", Args: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if (len(args) == 0) == !all {
			return fmt.Errorf("specify plugin names or --all, for example: devex plugin update --all")
		}
		return pluginNamesArgs(-1)(cmd, args)
	}, RunE: b.handleUpdatePlugins}
	update.Flags().Bool("all", false, "Update all installed plugins")
	info := &cobra.Command{Use: "info <name>", Short: "Show installed plugin metadata and commands", Args: pluginNamesArgs(1), RunE: b.handlePluginInfo}
	run := &cobra.Command{Use: "run <name> <command> [args...]", Short: "Run an installed plugin command (flags are passed through)", DisableFlagParsing: true, Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.MinimumNArgs(2)(cmd, args); err != nil {
			return err
		}
		return validatePluginName(args[0])
	}, RunE: b.handleRunPlugin}
	registry := &cobra.Command{Use: "registry", Short: "Show plugin registry configuration", Args: cobra.NoArgs, RunE: b.handleRegistryInfo}
	cmd.AddCommand(list, search, install, remove, update, info, run, registry)
	return cmd
}

func pluginNamesArgs(count int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if count >= 0 {
			if err := cobra.ExactArgs(count)(cmd, args); err != nil {
				return err
			}
		}
		for _, name := range args {
			if err := validatePluginName(name); err != nil {
				return err
			}
		}
		return nil
	}
}

type installedPlugin struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Path            string `json:"path"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
}

func (b *PluginBootstrap) handleListPlugins(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	if format != "table" && format != "json" {
		return fmt.Errorf("invalid format %q: use table or json", format)
	}
	installed := b.manager.ListPluginsWithContext(cmd.Context())
	latest := map[string]sdk.PluginMetadata{}
	if !viper.GetBool("offline") && len(installed) > 0 {
		var err error
		latest, err = b.downloader.GetAvailablePlugins(cmd.Context())
		if err != nil {
			if cmd.Context().Err() != nil {
				return cmd.Context().Err()
			}
			cmd.PrintErrf("Warning: latest registry versions unavailable: %v\n", err)
		}
	}
	rows := make([]installedPlugin, 0, len(installed))
	for _, name := range sortedPluginNames(installed) {
		metadata := installed[name]
		row := installedPlugin{Name: name, Version: metadata.Version, Path: metadata.Path, LatestVersion: "unknown"}
		if remote, ok := latest[name]; ok {
			row.LatestVersion = remote.Version
			localVersion, localErr := semver.NewVersion(metadata.Version)
			remoteVersion, remoteErr := semver.NewVersion(remote.Version)
			row.UpdateAvailable = localErr == nil && remoteErr == nil && remoteVersion.GreaterThan(localVersion)
		}
		rows = append(rows, row)
	}
	if format == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
	}
	if len(rows) == 0 {
		cmd.Println("No plugins installed. Run 'devex plugin search' to find plugins.")
		return nil
	}
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "NAME\tVERSION\tLATEST\tSTATUS\tPATH")
	for _, row := range rows {
		status := ""
		if row.UpdateAvailable {
			status = "update available"
		}
		_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", row.Name, row.Version, row.LatestVersion, status, row.Path)
	}
	return writer.Flush()
}

func sortedPluginNames(plugins map[string]sdk.PluginMetadata) []string {
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func requirePluginNetwork() error {
	if viper.GetBool("offline") {
		return fmt.Errorf("registry access is disabled by --offline; retry without offline mode")
	}
	return nil
}

func (b *PluginBootstrap) handleSearchPlugins(cmd *cobra.Command, args []string) error {
	if err := requirePluginNetwork(); err != nil {
		return err
	}
	query := ""
	if len(args) > 0 {
		query = args[0]
	}
	results, err := b.downloader.SearchPlugins(cmd.Context(), query)
	if err != nil {
		return fmt.Errorf("search plugin registry (check DEVEX_PLUGIN_REGISTRY_URL and your connection): %w", err)
	}
	if len(results) == 0 {
		cmd.Printf("No plugins found for query: %s\n", query)
	}
	for _, name := range sortedPluginNames(results) {
		metadata := results[name]
		key := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
		_, supported := metadata.Binaries[key]
		cmd.Printf("%s %s — %s (available for %s: %t)\n", name, metadata.Version, metadata.Description, key, supported)
	}
	return nil
}

func (b *PluginBootstrap) handleInstallPlugin(cmd *cobra.Command, args []string) error {
	if err := requirePluginNetwork(); err != nil {
		return err
	}
	if err := b.downloader.DownloadValidatedPluginWithContext(cmd.Context(), args[0]); err != nil {
		return fmt.Errorf("install plugin %s: %w", args[0], err)
	}
	if err := b.manager.DiscoverPluginsWithContext(cmd.Context()); err != nil {
		return err
	}
	cmd.Printf("Plugin %s installed successfully. Run 'devex plugin info %s' for commands.\n", args[0], args[0])
	return nil
}

func (b *PluginBootstrap) installed(cmd *cobra.Command, name string) (sdk.PluginMetadata, error) {
	metadata, ok := b.manager.ListPluginsWithContext(cmd.Context())[name]
	if !ok {
		return metadata, fmt.Errorf("plugin %s is not installed; run 'devex plugin install %s'", name, name)
	}
	return metadata, nil
}

func (b *PluginBootstrap) handleRunPlugin(cmd *cobra.Command, args []string) error {
	metadata, err := b.installed(cmd, args[0])
	if err != nil {
		return err
	}
	if !slices.Contains(getCommandNames(metadata.Commands), args[1]) {
		return fmt.Errorf("unknown command %q for plugin %s; run 'devex plugin info %s' for available commands", args[1], args[0], args[0])
	}
	return b.manager.ExecutePluginWithIO(cmd.Context(), args[0], args[1:], sdk.OperationIO{Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()})
}

func (b *PluginBootstrap) handleRemovePlugin(cmd *cobra.Command, args []string) error {
	metadata, err := b.installed(cmd, args[0])
	if err != nil {
		return err
	}
	affected := "devex plugin run " + args[0] + " {" + strings.Join(getCommandNames(metadata.Commands), ", ") + "}"
	critical := strings.HasPrefix(args[0], "package-manager-") || args[0] == "system-setup"
	if critical {
		affected += "; devex install, uninstall, setup, and maintain may require this plugin"
	}
	force, _ := cmd.Flags().GetBool("force")
	if critical && !force {
		return fmt.Errorf("refusing to remove critical plugin %s without --force; commands that will stop working: %s", args[0], affected)
	}
	// Root confines cleanup to the plugin directory, including sidecar paths.
	root, err := os.OpenRoot(b.manager.GetPluginDir())
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := root.Remove(filepath.Base(metadata.Path)); err != nil {
		return fmt.Errorf("remove plugin binary: %w", err)
	}
	if err := root.Remove(".devex-plugin-" + args[0] + ".install.json"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("binary removed but install record cleanup failed: %w", err)
	}
	if err := b.manager.DiscoverPluginsWithContext(cmd.Context()); err != nil {
		return err
	}
	cmd.Printf("Removed %s. Commands that will stop working: %s\n", args[0], affected)
	return nil
}

func (b *PluginBootstrap) handleUpdatePlugins(cmd *cobra.Command, args []string) error {
	if err := requirePluginNetwork(); err != nil {
		return err
	}
	plugins := b.manager.ListPluginsWithContext(cmd.Context())
	all, _ := cmd.Flags().GetBool("all")
	if all {
		args = sortedPluginNames(plugins)
	}
	// Validate the entire selection before starting downloads.
	for _, name := range args {
		if _, err := b.installed(cmd, name); err != nil {
			return err
		}
	}
	if len(args) == 0 {
		cmd.Println("No plugins installed to update")
		return nil
	}
	var failures []error
	for _, name := range args {
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		if err := b.downloader.DownloadValidatedPluginWithContext(cmd.Context(), name); err != nil {
			failures = append(failures, fmt.Errorf("update plugin %s: %w", name, err))
			continue
		}
		cmd.Printf("Updated %s\n", name)
	}
	if err := b.manager.DiscoverPluginsWithContext(cmd.Context()); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

func (b *PluginBootstrap) handlePluginInfo(cmd *cobra.Command, args []string) error {
	metadata, err := b.installed(cmd, args[0])
	if err != nil {
		return err
	}
	cmd.Printf("Plugin: %s\nVersion: %s\nDescription: %s\nPath: %s\nCommands:\n", args[0], metadata.Version, metadata.Description, metadata.Path)
	for _, command := range metadata.Commands {
		cmd.Printf("  %s - %s\n", command.Name, command.Description)
	}
	return nil
}

func getCommandNames(commands []sdk.PluginCommand) []string {
	names := make([]string, len(commands))
	for i, command := range commands {
		names[i] = command.Name
	}
	return names
}

// Regular expression for valid plugin names
// Allows alphanumeric characters, hyphens, and underscores
// Prevents directory traversal attacks and special characters
var validPluginNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// handleRegistryInfo shows registry configuration information
func (b *PluginBootstrap) handleRegistryInfo(cmd *cobra.Command, args []string) error {
	registryURL := GetRegistryURL()

	cmd.Println("Plugin Registry Configuration")
	cmd.Println("=============================")
	cmd.Printf("Current URL: %s\n\n", registryURL)

	// Check environment variable
	if envURL := os.Getenv("DEVEX_PLUGIN_REGISTRY_URL"); envURL != "" {
		cmd.Printf("✓ Environment variable set: DEVEX_PLUGIN_REGISTRY_URL=%s\n", envURL)
	} else {
		cmd.Println("✗ Environment variable not set: DEVEX_PLUGIN_REGISTRY_URL")
	}

	// Check config file
	dirs, err := devexpaths.Resolve()
	if err != nil {
		return err
	}
	homeDir := dirs.Home
	configPath := filepath.Join(devexpaths.ForHome(homeDir).Config, "config.yaml")
	if configURL := getRegistryURLFromConfig(); configURL != "" {
		cmd.Printf("✓ Config file setting: %s\n", configURL)
		cmd.Printf("  Location: %s\n", configPath)
	} else {
		cmd.Printf("✗ No registry URL in config file: %s\n", configPath)
	}

	// Show default
	cmd.Printf("\nDefault URL: %s\n", DefaultRegistryURL)

	// Show example configuration
	cmd.Println("\nTo use a custom registry:")
	cmd.Println("1. Set environment variable:")
	cmd.Println("   export DEVEX_PLUGIN_REGISTRY_URL=https://your-registry.com/v1")
	cmd.Println("\n2. Or add to config file ($DEVEX_CONFIG_DIR/config.yaml):")
	cmd.Println("   plugin_registry_url: https://your-registry.com/v1")

	return nil
}

// validatePluginName validates plugin name to prevent directory traversal and injection attacks
func validatePluginName(pluginName string) error {
	if pluginName == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	// Check length constraints
	if len(pluginName) > 64 {
		return fmt.Errorf("plugin name too long (max 64 characters): %s", pluginName)
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
