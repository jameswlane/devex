// Package sdk provides core functionality for DevEx plugin development and management
// Version: v0.1.0 - improved error handling and timeout configuration
package sdk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DownloadError represents an error that occurred during plugin download
type DownloadError struct {
	Plugin string
	Err    error
}

func (e *DownloadError) Error() string {
	return fmt.Sprintf("failed to download plugin %s: %v", e.Plugin, e.Err)
}

// Unwrap returns the underlying error for error wrapping support
func (e *DownloadError) Unwrap() error {
	return e.Err
}

// MultiError represents multiple errors that occurred
type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%d errors occurred: %v", len(e.Errors), e.Errors[0])
}

func (e *MultiError) Unwrap() []error {
	return e.Errors
}

// Add adds an error to the MultiError
func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// HasErrors returns true if there are any errors
func (e *MultiError) HasErrors() bool {
	return len(e.Errors) > 0
}

// DownloadStrategy defines how to handle download failures
type DownloadStrategy int

const (
	// ContinueOnError continues downloading other plugins if one fails
	ContinueOnError DownloadStrategy = iota
	// FailOnError stops downloading if any plugin fails
	FailOnError
	// RequireCritical only fails if critical plugins fail
	RequireCritical
)

// Logger interface for plugin SDK logging
// Plugins can provide their own logger implementation or use a default
type Logger interface {
	Printf(format string, args ...any)
	Println(msg string, args ...any)
	Success(msg string, args ...any)
	Warning(msg string, args ...any)
	ErrorMsg(msg string, args ...any)
	Info(msg string, keyvals ...any)
	Warn(msg string, keyvals ...any)
	Error(msg string, err error, keyvals ...any)
	Debug(msg string, keyvals ...any)
}

// DefaultLogger provides basic console logging for plugins
type DefaultLogger struct {
	silent bool
}

// NewDefaultLogger creates a new default logger
func NewDefaultLogger(silent bool) Logger {
	return &DefaultLogger{silent: silent}
}

// Printf implements Logger interface
func (l *DefaultLogger) Printf(format string, args ...any) {
	if !l.silent {
		fmt.Printf(format, args...)
	}
}

// Println implements Logger interface
func (l *DefaultLogger) Println(msg string, args ...any) {
	if !l.silent {
		if len(args) > 0 {
			fmt.Printf(msg+"\n", args...)
		} else {
			fmt.Println(msg)
		}
	}
}

// Success implements Logger interface
func (l *DefaultLogger) Success(msg string, args ...any) {
	if !l.silent {
		fmt.Printf("✅ "+msg+"\n", args...)
	}
}

// Warning implements Logger interface
func (l *DefaultLogger) Warning(msg string, args ...any) {
	if !l.silent {
		fmt.Printf("⚠️  "+msg+"\n", args...)
	}
}

// ErrorMsg implements Logger interface
func (l *DefaultLogger) ErrorMsg(msg string, args ...any) {
	if !l.silent {
		fmt.Printf("❌ "+msg+"\n", args...)
	}
}

// Info implements Logger interface
func (l *DefaultLogger) Info(msg string, keyvals ...any) {
	if !l.silent {
		fmt.Print("INFO: " + msg + "\n")
	}
}

// Warn implements Logger interface
func (l *DefaultLogger) Warn(msg string, keyvals ...any) {
	if !l.silent {
		fmt.Print("WARN: " + msg + "\n")
	}
}

// Error implements Logger interface
func (l *DefaultLogger) Error(msg string, err error, keyvals ...any) {
	if !l.silent {
		if err != nil {
			fmt.Printf("ERROR: %s - %v\n", msg, err)
		} else {
			fmt.Print("ERROR: " + msg + "\n")
		}
	}
}

// Debug implements Logger interface
func (l *DefaultLogger) Debug(msg string, keyvals ...any) {
	if !l.silent {
		fmt.Print("DEBUG: " + msg + "\n")
	}
}

// TimeoutConfig represents timeout configuration for different operation types
type TimeoutConfig struct {
	// Default timeout for all operations
	Default time.Duration `json:"default"`
	// Install operation timeout
	Install time.Duration `json:"install"`
	// Update operation timeout
	Update time.Duration `json:"update"`
	// Upgrade operation timeout
	Upgrade time.Duration `json:"upgrade"`
	// Search operation timeout
	Search time.Duration `json:"search"`
	// Network operation timeout
	Network time.Duration `json:"network"`
	// Build operation timeout
	Build time.Duration `json:"build"`
	// Shell command timeout
	Shell time.Duration `json:"shell"`
}

// DefaultTimeouts provides sensible default timeout values
func DefaultTimeouts() TimeoutConfig {
	return TimeoutConfig{
		Default: 5 * time.Minute,
		Install: 10 * time.Minute,
		Update:  2 * time.Minute,
		Upgrade: 15 * time.Minute,
		Search:  30 * time.Second,
		Network: 1 * time.Minute,
		Build:   30 * time.Minute,
		Shell:   5 * time.Minute,
	}
}

// GetTimeout returns the appropriate timeout for an operation type
func (tc TimeoutConfig) GetTimeout(operationType string) time.Duration {
	switch strings.ToLower(operationType) {
	case "install":
		if tc.Install > 0 {
			return tc.Install
		}
	case "update":
		if tc.Update > 0 {
			return tc.Update
		}
	case "upgrade":
		if tc.Upgrade > 0 {
			return tc.Upgrade
		}
	case "search":
		if tc.Search > 0 {
			return tc.Search
		}
	case "network":
		if tc.Network > 0 {
			return tc.Network
		}
	case "build":
		if tc.Build > 0 {
			return tc.Build
		}
	case "shell":
		if tc.Shell > 0 {
			return tc.Shell
		}
	}

	if tc.Default > 0 {
		return tc.Default
	}

	return DefaultTimeouts().Default
}

// PluginInfo represents the standard plugin information
type PluginInfo struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Commands    []PluginCommand `json:"commands"`
	Author      string          `json:"author,omitempty"`
	Repository  string          `json:"repository,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	// Timeout configuration for plugin operations
	Timeouts    TimeoutConfig   `json:"timeouts,omitempty"`
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
	info     PluginInfo
	logger   Logger
	timeouts TimeoutConfig
}

// NewBasePlugin creates a new base plugin
func NewBasePlugin(info PluginInfo) *BasePlugin {
	timeouts := info.Timeouts
	if timeouts.Default == 0 {
		timeouts = DefaultTimeouts()
	}

	return &BasePlugin{
		info:     info,
		logger:   NewDefaultLogger(false),
		timeouts: timeouts,
	}
}

// Info returns the plugin information
func (p *BasePlugin) Info() PluginInfo {
	return p.info
}

// GetLogger returns the plugin's logger
func (p *BasePlugin) GetLogger() Logger {
	return p.logger
}

// SetLogger sets a custom logger for the plugin
func (p *BasePlugin) SetLogger(logger Logger) {
	if logger != nil {
		p.logger = logger
	}
}

// GetTimeouts returns the plugin's timeout configuration
func (p *BasePlugin) GetTimeouts() TimeoutConfig {
	return p.timeouts
}

// SetTimeouts sets custom timeout configuration for the plugin
func (p *BasePlugin) SetTimeouts(timeouts TimeoutConfig) {
	p.timeouts = timeouts
}

// GetTimeout returns the appropriate timeout for an operation type
func (p *BasePlugin) GetTimeout(operationType string) time.Duration {
	return p.timeouts.GetTimeout(operationType)
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


// ExecCommandWithContext executes a command with context support for cancellation
func ExecCommandWithContext(ctx context.Context, useSudo bool, name string, args ...string) error {
	var cmd *exec.Cmd

	if useSudo && RequireSudo() {
		cmdArgs := append([]string{name}, args...)
		cmd = exec.CommandContext(ctx, "sudo", cmdArgs...)
	} else {
		cmd = exec.CommandContext(ctx, name, args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}


// ExecCommandOutputWithContext executes a command and returns output with context support
func ExecCommandOutputWithContext(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()
	return string(output), err
}

// TimeoutError represents a command execution timeout error
type TimeoutError struct {
	Command   string
	Args      []string
	Timeout   time.Duration
	Operation string
}

func (e *TimeoutError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("command '%s %s' timed out after %v during %s operation",
			e.Command, strings.Join(e.Args, " "), e.Timeout, e.Operation)
	}
	return fmt.Sprintf("command '%s %s' timed out after %v",
		e.Command, strings.Join(e.Args, " "), e.Timeout)
}

// ExecCommandWithTimeout executes a command with a specific timeout
func ExecCommandWithTimeout(timeout time.Duration, useSudo bool, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := ExecCommandWithContext(ctx, useSudo, name, args...)
	if ctx.Err() == context.DeadlineExceeded {
		return &TimeoutError{
			Command: name,
			Args:    args,
			Timeout: timeout,
		}
	}
	return err
}

// ExecCommandWithTimeoutAndOperation executes a command with timeout and operation context
func ExecCommandWithTimeoutAndOperation(timeout time.Duration, operation string, useSudo bool, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := ExecCommandWithContext(ctx, useSudo, name, args...)
	if ctx.Err() == context.DeadlineExceeded {
		return &TimeoutError{
			Command:   name,
			Args:      args,
			Timeout:   timeout,
			Operation: operation,
		}
	}
	return err
}

// ExecCommandOutputWithTimeout executes a command and returns output with timeout
func ExecCommandOutputWithTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	output, err := ExecCommandOutputWithContext(ctx, name, args...)
	if ctx.Err() == context.DeadlineExceeded {
		return output, &TimeoutError{
			Command: name,
			Args:    args,
			Timeout: timeout,
		}
	}
	return output, err
}

// ExecCommandOutputWithTimeoutAndOperation executes a command and returns output with timeout and operation context
func ExecCommandOutputWithTimeoutAndOperation(timeout time.Duration, operation string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	output, err := ExecCommandOutputWithContext(ctx, name, args...)
	if ctx.Err() == context.DeadlineExceeded {
		return output, &TimeoutError{
			Command:   name,
			Args:      args,
			Timeout:   timeout,
			Operation: operation,
		}
	}
	return output, err
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

// ExecManagerCommand executes a package manager command with timeout
func (p *PackageManagerPlugin) ExecManagerCommand(operation string, useSudo bool, args ...string) error {
	timeout := p.GetTimeout(operation)
	return ExecCommandWithTimeoutAndOperation(timeout, operation, useSudo, p.managerCommand, args...)
}

// ExecManagerCommandOutput executes a package manager command and returns output with timeout
func (p *PackageManagerPlugin) ExecManagerCommandOutput(operation string, args ...string) (string, error) {
	timeout := p.GetTimeout(operation)
	return ExecCommandOutputWithTimeoutAndOperation(timeout, operation, p.managerCommand, args...)
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
	Priority  int                       `json:"priority,omitempty"`  // Installation priority
	Type      string                    `json:"type,omitempty"`      // Plugin type (package-manager, desktop, etc.)
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
	logger           Logger
	strategy         DownloadStrategy
}

// DownloaderConfig configures the plugin downloader
type DownloaderConfig struct {
	RegistryURL      string
	PluginDir        string
	CacheDir         string
	VerifyChecksums  bool
	VerifySignatures bool
	PublicKeyPath    string
	Strategy         DownloadStrategy
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
		logger:           NewDefaultLogger(false), // Default logger
		strategy:         ContinueOnError,        // Default to continue on error
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
		logger:           NewDefaultLogger(false), // Default logger
		strategy:         config.Strategy,
	}
}

// SetLogger allows setting a custom logger for the downloader
func (d *Downloader) SetLogger(logger Logger) {
	if logger != nil {
		d.logger = logger
	}
}

// SetSilent enables/disables silent mode for the default logger
func (d *Downloader) SetSilent(silent bool) {
	if defaultLogger, ok := d.logger.(*DefaultLogger); ok {
		defaultLogger.silent = silent
	}
}

// SetStrategy sets the download strategy
func (d *Downloader) SetStrategy(strategy DownloadStrategy) {
	d.strategy = strategy
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

	// Validate plugin metadata to prevent unnecessary downloads
	if err := d.validatePluginBinary(pluginName, binary); err != nil {
		return err
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
	return d.DownloadRequiredPluginsWithContext(ctx, requiredPlugins)
}

// DownloadRequiredPluginsWithOptions downloads plugins with custom options
func (d *Downloader) DownloadRequiredPluginsWithOptions(requiredPlugins []string, criticalPlugins []string) error {
	ctx := context.Background()
	return d.DownloadRequiredPluginsWithContextAndOptions(ctx, requiredPlugins, criticalPlugins)
}

// DownloadRequiredPluginsWithContext downloads plugins with context support
func (d *Downloader) DownloadRequiredPluginsWithContext(ctx context.Context, requiredPlugins []string) error {
	return d.DownloadRequiredPluginsWithContextAndOptions(ctx, requiredPlugins, nil)
}

// DownloadRequiredPluginsWithContextAndOptions downloads plugins with full control
func (d *Downloader) DownloadRequiredPluginsWithContextAndOptions(ctx context.Context, requiredPlugins []string, criticalPlugins []string) error {
	if d.logger != nil {
		d.logger.Printf("Downloading %d required plugins...\n", len(requiredPlugins))
	}

	// Convert critical plugins to a map for quick lookup
	criticalMap := make(map[string]bool)
	for _, plugin := range criticalPlugins {
		criticalMap[plugin] = true
	}

	multiErr := &MultiError{}
	downloadedCount := 0

	for _, pluginName := range requiredPlugins {
		if err := d.DownloadPluginWithContext(ctx, pluginName); err != nil {
			downloadErr := &DownloadError{Plugin: pluginName, Err: err}
			multiErr.Add(downloadErr)

			if d.logger != nil {
				d.logger.Warning("Failed to download plugin %s: %v", pluginName, err)
			}

			// Check strategy and critical status
			switch d.strategy {
			case FailOnError:
				// Return immediately on any error
				return downloadErr
			case RequireCritical:
				// Return immediately if a critical plugin fails
				if criticalMap[pluginName] {
					return fmt.Errorf("critical plugin download failed: %w", downloadErr)
				}
			case ContinueOnError:
				// Continue with next plugin
				continue
			}
		} else {
			downloadedCount++
			if d.logger != nil {
				d.logger.Success("Downloaded plugin %s", pluginName)
			}
		}
	}

	// Log summary
	if d.logger != nil {
		d.logger.Printf("Downloaded %d/%d plugins successfully\n", downloadedCount, len(requiredPlugins))
	}

	// Return appropriate error based on strategy
	if multiErr.HasErrors() {
		switch d.strategy {
		case ContinueOnError:
			// Log errors but don't fail
			if d.logger != nil {
				d.logger.Warning("Some plugins failed to download: %v", multiErr)
			}
			return nil
		case RequireCritical:
			// Check if any critical plugins failed
			for _, err := range multiErr.Errors {
				if downloadErr, ok := err.(*DownloadError); ok {
					if criticalMap[downloadErr.Plugin] {
						return multiErr
					}
				}
			}
			// No critical plugins failed
			if d.logger != nil {
				d.logger.Warning("Non-critical plugins failed to download: %v", multiErr)
			}
			return nil
		default:
			return multiErr
		}
	}

	return nil
}

// UpdateRegistry updates the plugin registry with caching
func (d *Downloader) UpdateRegistry() error {
	_, err := d.fetchRegistry()
	return err
}

// getCacheDuration returns cache duration based on environment
func (d *Downloader) getCacheDuration() time.Duration {
	// Check for development environment with validation
	if env, err := SafeGetEnv("DEVEX_ENV"); err == nil && (env == "development" || env == "dev") {
		return 5 * time.Minute // Shorter cache in development
	}
	if env, err := SafeGetEnv("NODE_ENV"); err == nil && env == "development" {
		return 5 * time.Minute
	}

	return 1 * time.Hour // Default production cache duration
}

// fetchRegistry fetches the plugin registry with environment-aware caching
func (d *Downloader) fetchRegistry() (*PluginRegistry, error) {
	// Try to load from cache first
	cachedRegistry := d.loadCachedRegistry()
	cacheDuration := d.getCacheDuration()
	if cachedRegistry != nil && time.Since(cachedRegistry.LastUpdated) < cacheDuration {
		return cachedRegistry, nil
	}

	// Fetch fresh registry
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(d.registryURL + "/api/v1/registry")
	if err != nil {
		// Return cached registry if available
		if cachedRegistry != nil {
			if d.logger != nil {
				d.logger.Warning("Using cached registry (network unavailable)")
			}
			return cachedRegistry, nil
		}
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
		if d.logger != nil {
			d.logger.Printf("Plugin %s is already up to date\n", pluginName)
		}
		return nil
	}

	if d.logger != nil {
		d.logger.Printf("Downloading %s (%s)...\n", pluginName, binary.OS+"-"+binary.Arch)
	}

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Create temporary file for download
	tempPath := pluginPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = tempFile.Close() }()
	defer func() { _ = os.Remove(tempPath) }()

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

	if d.logger != nil {
		d.logger.Success("Successfully installed plugin %s", pluginName)
	}
	return nil
}

// validatePluginBinary validates plugin metadata to prevent unnecessary downloads
func (d *Downloader) validatePluginBinary(pluginName string, binary PlatformBinary) error {
	// Check if URL is empty or invalid
	if binary.URL == "" {
		return fmt.Errorf("plugin %s has no download URL for this platform", pluginName)
	}

	// Check if checksum is empty (indicates plugin not built yet)
	if binary.Checksum == "" {
		return fmt.Errorf("plugin %s has no checksum (binary not available)", pluginName)
	}

	// Check if size is zero (indicates empty or missing binary)
	if binary.Size == 0 {
		return fmt.Errorf("plugin %s has zero size (binary not available)", pluginName)
	}

	return nil
}

// isPluginUpToDate checks if a plugin is up to date by comparing checksums
func (d *Downloader) isPluginUpToDate(pluginPath, expectedChecksum string) bool {
	if !d.verifyChecksums || expectedChecksum == "" {
		return false // Always redownload if checksums are disabled or checksum is empty
	}

	file, err := os.Open(pluginPath)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	return actualChecksum == expectedChecksum
}

// ExecutableManager manages plugin executables with caching and timeouts
type ExecutableManager struct {
	pluginDir     string
	cachedPlugins map[string]PluginMetadata
	cacheTime     time.Time
	loadTimeout   time.Duration
	mu            sync.RWMutex
}

// NewExecutableManager creates a new executable manager with caching
func NewExecutableManager(pluginDir string) *ExecutableManager {
	return &ExecutableManager{
		pluginDir:     pluginDir,
		cachedPlugins: make(map[string]PluginMetadata),
		loadTimeout:   10 * time.Second, // Reduced from 30s for better CLI responsiveness
	}
}

// GetPluginDir returns the plugin directory
func (em *ExecutableManager) GetPluginDir() string {
	return em.pluginDir
}

// ListPlugins returns installed plugins with caching
func (em *ExecutableManager) ListPlugins() map[string]PluginMetadata {
	// Check cached plugins with read lock
	em.mu.RLock()
	if time.Since(em.cacheTime) < 30*time.Second {
		// Return a copy to prevent external mutations
		result := make(map[string]PluginMetadata, len(em.cachedPlugins))
		for k, v := range em.cachedPlugins {
			result[k] = v
		}
		em.mu.RUnlock()
		return result
	}
	em.mu.RUnlock()

	// Acquire write lock for cache refresh
	em.mu.Lock()
	defer em.mu.Unlock()

	// Double-check cache after acquiring write lock
	if time.Since(em.cacheTime) < 30*time.Second {
		result := make(map[string]PluginMetadata, len(em.cachedPlugins))
		for k, v := range em.cachedPlugins {
			result[k] = v
		}
		return result
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
	em.mu.Lock()
	em.cachedPlugins = make(map[string]PluginMetadata)
	em.mu.Unlock()
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
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to create destination plugin: %w", err)
	}
	defer func() { _ = destFile.Close() }()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy plugin: %w", err)
	}

	// Make executable
	if err := os.Chmod(pluginPath, 0755); err != nil {
		return fmt.Errorf("failed to make plugin executable: %w", err)
	}

	// Clear cache to force refresh
	em.mu.Lock()
	em.cachedPlugins = make(map[string]PluginMetadata)
	em.mu.Unlock()
	// Note: ExecutableManager doesn't have a logger, this would be handled by the calling code
	return nil
}

// DiscoverPluginsWithContext discovers plugins with context support
func (em *ExecutableManager) DiscoverPluginsWithContext(ctx context.Context) error {
	// Context-aware version of DiscoverPlugins
	return em.DiscoverPlugins()
}


// RegisterCommands registers plugin commands (placeholder)
func (em *ExecutableManager) RegisterCommands(rootCmd interface{}) error {
	// Placeholder implementation - will register plugin commands after release
	return nil
}

// IsDownloadError checks if an error is a DownloadError
func IsDownloadError(err error) bool {
	var downloadErr *DownloadError
	return errors.As(err, &downloadErr)
}

// GetDownloadErrors extracts all DownloadErrors from a MultiError
func GetDownloadErrors(err error) []*DownloadError {
	var multiErr *MultiError
	if !errors.As(err, &multiErr) {
		// Check if it's a single download error
		var downloadErr *DownloadError
		if errors.As(err, &downloadErr) {
			return []*DownloadError{downloadErr}
		}
		return nil
	}

	var downloadErrors []*DownloadError
	for _, e := range multiErr.Errors {
		var downloadErr *DownloadError
		if errors.As(e, &downloadErr) {
			downloadErrors = append(downloadErrors, downloadErr)
		}
	}
	return downloadErrors
}
