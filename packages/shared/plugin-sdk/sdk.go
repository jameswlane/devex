package sdk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

// PluginMetadata represents plugin metadata with path and platform information
type PluginMetadata struct {
	PluginInfo
	Path      string                    `json:"path"`
	Platforms map[string]PlatformBinary `json:"platforms,omitempty"` // Platform-specific binaries
	Priority  int                       `json:"priority,omitempty"`   // Installation priority
	Type      string                    `json:"type,omitempty"`       // Plugin type (package-manager, desktop, etc.)
}

// PluginRegistry represents the plugin registry structure
type PluginRegistry struct {
	BaseURL     string                    `json:"base_url"`
	Version     string                    `json:"version"`
	LastUpdated time.Time                 `json:"last_updated"`
	Plugins     map[string]PluginMetadata `json:"plugins"`
}

// PlatformBinary represents a binary for a specific platform
type PlatformBinary struct {
	URL      string `json:"url"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// Downloader handles secure plugin downloading from registry with verification
type Downloader struct {
	registryURL      string
	pluginDir        string
	cacheDir         string
	verifyChecksums  bool
	verifySignatures bool
	publicKeyPath    string
}

// DownloaderConfig configures the plugin downloader
type DownloaderConfig struct {
	RegistryURL      string
	PluginDir        string
	CacheDir         string
	VerifyChecksums  bool
	VerifySignatures bool
	PublicKeyPath    string
}

// NewDownloader creates a new plugin downloader with default security settings
func NewDownloader(registryURL, pluginDir string) *Downloader {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".devex", "plugin-cache")
	
	return &Downloader{
		registryURL:      registryURL,
		pluginDir:        pluginDir,
		cacheDir:         cacheDir,
		verifyChecksums:  true,  // Enable checksum verification by default
		verifySignatures: false, // Signature verification optional for now
		publicKeyPath:    "",
	}
}

// NewSecureDownloader creates a downloader with custom security configuration
func NewSecureDownloader(config DownloaderConfig) *Downloader {
	return &Downloader{
		registryURL:      config.RegistryURL,
		pluginDir:        config.PluginDir,
		cacheDir:         config.CacheDir,
		verifyChecksums:  config.VerifyChecksums,
		verifySignatures: config.VerifySignatures,
		publicKeyPath:    config.PublicKeyPath,
	}
}

// GetAvailablePlugins returns available plugins from registry with caching
func (d *Downloader) GetAvailablePlugins() (map[string]PluginMetadata, error) {
	registry, err := d.fetchRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}
	return registry.Plugins, nil
}

// SearchPlugins searches for plugins by query with caching
func (d *Downloader) SearchPlugins(query string) (map[string]PluginMetadata, error) {
	allPlugins, err := d.GetAvailablePlugins()
	if err != nil {
		return nil, err
	}

	results := make(map[string]PluginMetadata)
	query = strings.ToLower(query)

	for name, metadata := range allPlugins {
		if strings.Contains(strings.ToLower(name), query) ||
			strings.Contains(strings.ToLower(metadata.Description), query) {
			results[name] = metadata
			continue
		}

		for _, tag := range metadata.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results[name] = metadata
				break
			}
		}
	}

	return results, nil
}

// DownloadPlugin securely downloads a plugin with checksum and signature verification
func (d *Downloader) DownloadPlugin(pluginName string) error {
	ctx := context.Background()
	return d.DownloadPluginWithContext(ctx, pluginName)
}

// DownloadPluginWithContext downloads a plugin with context for cancellation
func (d *Downloader) DownloadPluginWithContext(ctx context.Context, pluginName string) error {
	registry, err := d.fetchRegistry()
	if err != nil {
		return fmt.Errorf("failed to fetch registry: %w", err)
	}

	metadata, exists := registry.Plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s not found in registry", pluginName)
	}

	// Get platform-specific binary
	platformKey := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	binary, exists := metadata.Platforms[platformKey]
	if !exists {
		return fmt.Errorf("plugin %s not available for platform %s", pluginName, platformKey)
	}

	// Ensure plugin directory exists
	if err := os.MkdirAll(d.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download and verify plugin
	return d.downloadAndVerifyPlugin(ctx, pluginName, binary)
}

// DownloadRequiredPlugins downloads all required plugins for the platform with verification
func (d *Downloader) DownloadRequiredPlugins(requiredPlugins []string) error {
	ctx := context.Background()
	fmt.Printf("Downloading %d required plugins...\n", len(requiredPlugins))
	
	for _, pluginName := range requiredPlugins {
		if err := d.DownloadPluginWithContext(ctx, pluginName); err != nil {
			fmt.Printf("Warning: failed to download plugin %s: %v\n", pluginName, err)
			// Continue with other plugins rather than failing completely
		}
	}

	return nil
}

// UpdateRegistry updates the plugin registry with caching
func (d *Downloader) UpdateRegistry() error {
	_, err := d.fetchRegistry()
	return err
}

// fetchRegistry fetches the plugin registry with caching
func (d *Downloader) fetchRegistry() (*PluginRegistry, error) {
	// Try to load from cache first
	cachedRegistry := d.loadCachedRegistry()
	if cachedRegistry != nil && time.Since(cachedRegistry.LastUpdated) < 1*time.Hour {
		return cachedRegistry, nil
	}

	// Fetch fresh registry
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(d.registryURL + "/v1/registry.json")
	if err != nil {
		// Return cached registry if available
		if cachedRegistry != nil {
			fmt.Printf("Warning: using cached registry (network unavailable)\n")
			return cachedRegistry, nil
		}
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry server returned HTTP %d", resp.StatusCode)
	}

	// Parse registry
	var registry PluginRegistry
	if err := json.NewDecoder(resp.Body).Decode(&registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry: %w", err)
	}

	// Cache the registry
	d.cacheRegistry(&registry)
	return &registry, nil
}

// loadCachedRegistry loads registry from cache
func (d *Downloader) loadCachedRegistry() *PluginRegistry {
	cachePath := filepath.Join(d.cacheDir, "registry.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}

	var registry PluginRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil
	}

	return &registry
}

// cacheRegistry saves registry to cache
func (d *Downloader) cacheRegistry(registry *PluginRegistry) {
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		return // Ignore cache errors
	}

	cachePath := filepath.Join(d.cacheDir, "registry.json")
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(cachePath, data, 0644) // Ignore cache write errors
}

// downloadAndVerifyPlugin downloads and verifies a plugin binary
func (d *Downloader) downloadAndVerifyPlugin(ctx context.Context, pluginName string, binary PlatformBinary) error {
	pluginPath := filepath.Join(d.pluginDir, fmt.Sprintf("devex-plugin-%s", pluginName))
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	// Check if plugin already exists and is up to date
	if d.isPluginUpToDate(pluginPath, binary.Checksum) {
		fmt.Printf("Plugin %s is already up to date\n", pluginName)
		return nil
	}

	fmt.Printf("Downloading %s (%s)...\n", pluginName, binary.OS+"-"+binary.Arch)

	// Create HTTP client with timeout
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, "GET", binary.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Create temporary file for download
	tempPath := pluginPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()
	defer os.Remove(tempPath) // Cleanup on any error

	// Download with checksum verification
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	// Verify checksum if enabled
	if d.verifyChecksums && binary.Checksum != "" {
		actualChecksum := hex.EncodeToString(hasher.Sum(nil))
		if actualChecksum != binary.Checksum {
			return fmt.Errorf("checksum verification failed: expected %s, got %s", binary.Checksum, actualChecksum)
		}
	}

	// Verify signature if enabled
	if d.verifySignatures {
		if err := d.verifyPluginSignature(tempPath, binary.URL); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
	}

	// Make executable and move to final location
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to make plugin executable: %w", err)
	}

	if err := os.Rename(tempPath, pluginPath); err != nil {
		return fmt.Errorf("failed to move plugin to final location: %w", err)
	}

	fmt.Printf("Successfully installed plugin %s\n", pluginName)
	return nil
}

// isPluginUpToDate checks if a plugin is up to date by comparing checksums
func (d *Downloader) isPluginUpToDate(pluginPath, expectedChecksum string) bool {
	if !d.verifyChecksums || expectedChecksum == "" {
		return false // Always redownload if checksums are disabled
	}

	file, err := os.Open(pluginPath)
	if err != nil {
		return false
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	return actualChecksum == expectedChecksum
}


// ExecutableManager manages plugin executables with caching and timeouts
type ExecutableManager struct {
	pluginDir       string
	cachedPlugins   map[string]PluginMetadata
	cacheTime       time.Time
	loadTimeout     time.Duration
}

// NewExecutableManager creates a new executable manager with caching
func NewExecutableManager(pluginDir string) *ExecutableManager {
	return &ExecutableManager{
		pluginDir:     pluginDir,
		cachedPlugins: make(map[string]PluginMetadata),
		loadTimeout:   30 * time.Second,
	}
}

// GetPluginDir returns the plugin directory
func (em *ExecutableManager) GetPluginDir() string {
	return em.pluginDir
}

// ListPlugins returns installed plugins with caching
func (em *ExecutableManager) ListPlugins() map[string]PluginMetadata {
	// Return cached plugins if recent
	if time.Since(em.cacheTime) < 30*time.Second {
		return em.cachedPlugins
	}

	plugins := make(map[string]PluginMetadata)
	
	// Scan plugin directory for executables
	entries, err := os.ReadDir(em.pluginDir)
	if err != nil {
		return plugins // Return empty map on error
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, "devex-plugin-") && 
		   (runtime.GOOS != "windows" || strings.HasSuffix(name, ".exe")) {
			
			pluginName := strings.TrimPrefix(name, "devex-plugin-")
			if runtime.GOOS == "windows" {
				pluginName = strings.TrimSuffix(pluginName, ".exe")
			}

			pluginPath := filepath.Join(em.pluginDir, name)
			metadata := em.getPluginMetadata(pluginName, pluginPath)
			if metadata != nil {
				plugins[pluginName] = *metadata
			}
		}
	}

	// Cache results
	em.cachedPlugins = plugins
	em.cacheTime = time.Now()
	
	return plugins
}

// getPluginMetadata gets metadata from a plugin executable with timeout
func (em *ExecutableManager) getPluginMetadata(pluginName, pluginPath string) *PluginMetadata {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), em.loadTimeout)
	defer cancel()

	// Execute plugin with --plugin-info flag
	cmd := exec.CommandContext(ctx, pluginPath, "--plugin-info")
	output, err := cmd.Output()
	if err != nil {
		// Return basic metadata if plugin doesn't respond
		return &PluginMetadata{
			PluginInfo: PluginInfo{
				Name:        pluginName,
				Version:     "unknown",
				Description: fmt.Sprintf("DevEx plugin: %s", pluginName),
			},
			Path: pluginPath,
		}
	}

	var info PluginInfo
	if err := json.Unmarshal(output, &info); err != nil {
		// Return basic metadata if JSON is invalid
		return &PluginMetadata{
			PluginInfo: PluginInfo{
				Name:        pluginName,
				Version:     "unknown",
				Description: fmt.Sprintf("DevEx plugin: %s", pluginName),
			},
			Path: pluginPath,
		}
	}

	return &PluginMetadata{
		PluginInfo: info,
		Path:       pluginPath,
	}
}

// ExecutePlugin executes a plugin with given arguments and timeout
func (em *ExecutableManager) ExecutePlugin(pluginName string, args []string) error {
	plugins := em.ListPlugins()
	pluginInfo, exists := plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s is not installed", pluginName)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), em.loadTimeout)
	defer cancel()

	// Execute plugin
	cmd := exec.CommandContext(ctx, pluginInfo.Path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// DiscoverPlugins discovers and caches plugins in the plugin directory
func (em *ExecutableManager) DiscoverPlugins() error {
	// Just refresh the cache by calling ListPlugins
	em.ListPlugins()
	return nil
}

// RemovePlugin removes a plugin executable
func (em *ExecutableManager) RemovePlugin(pluginName string) error {
	plugins := em.ListPlugins()
	pluginInfo, exists := plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s is not installed", pluginName)
	}

	if err := os.Remove(pluginInfo.Path); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	// Clear cache to force refresh
	em.cachedPlugins = make(map[string]PluginMetadata)
	return nil
}

// InstallPlugin installs a plugin from a source path
func (em *ExecutableManager) InstallPlugin(sourcePath, pluginName string) error {
	pluginPath := filepath.Join(em.pluginDir, fmt.Sprintf("devex-plugin-%s", pluginName))
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	// Ensure plugin directory exists
	if err := os.MkdirAll(em.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Copy plugin
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source plugin: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to create destination plugin: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy plugin: %w", err)
	}

	// Make executable
	if err := os.Chmod(pluginPath, 0755); err != nil {
		return fmt.Errorf("failed to make plugin executable: %w", err)
	}

	// Clear cache to force refresh
	em.cachedPlugins = make(map[string]PluginMetadata)
	fmt.Printf("Successfully installed plugin %s\n", pluginName)
	return nil
}

// DiscoverPluginsWithContext discovers plugins with context support
func (em *ExecutableManager) DiscoverPluginsWithContext(ctx context.Context) error {
	// Context-aware version of DiscoverPlugins
	return em.DiscoverPlugins()
}

// DownloadRequiredPluginsWithContext downloads plugins with context support  
func (d *Downloader) DownloadRequiredPluginsWithContext(ctx context.Context, plugins []string) error {
	// Context-aware version - placeholder implementation
	for _, plugin := range plugins {
		if err := d.DownloadPluginWithContext(ctx, plugin); err != nil {
			return fmt.Errorf("failed to download plugin %s: %w", plugin, err)
		}
	}
	return nil
}

// RegisterCommands registers plugin commands (placeholder)
func (em *ExecutableManager) RegisterCommands(rootCmd interface{}) error {
	// Placeholder implementation - will register plugin commands after release
	return nil
}
