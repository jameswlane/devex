package installers

import (
	"context"
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var pluginBootstrap *bootstrap.PluginBootstrap
var testMode bool

// InitializeWithPluginBootstrap initializes the installer system with plugin bootstrap
func InitializeWithPluginBootstrap(pb *bootstrap.PluginBootstrap) {
	pluginBootstrap = pb
	log.Debug("Installer system initialized with plugin bootstrap")
}

// EnableTestMode enables test mode with mock installers
func EnableTestMode() {
	testMode = true
	log.Debug("Test mode enabled for installer system")
}

// DisableTestMode disables test mode
func DisableTestMode() {
	testMode = false
	log.Debug("Test mode disabled for installer system")
}

// GetAvailableInstallers returns a list of available installer methods for the current platform
func GetAvailableInstallers(ctx context.Context) []string {
	if pluginBootstrap == nil {
		return []string{}
	}

	// Get available package manager plugins
	availablePlugins, err := pluginBootstrap.GetAvailablePlugins(ctx)
	if err != nil {
		// Return empty slice if unable to get plugins
		return []string{}
	}
	var installers []string

	for pluginName := range availablePlugins {
		// Only return package-manager plugins
		if strings.HasPrefix(pluginName, "package-manager-") {
			// Remove the "package-manager-" prefix
			method := strings.TrimPrefix(pluginName, "package-manager-")
			installers = append(installers, method)
		}
	}

	return installers
}

// IsInstallerSupported checks if an installer method is supported on the current platform
func IsInstallerSupported(ctx context.Context, method string) bool {
	// In test mode, support common package managers for testing
	if testMode {
		supportedTestMethods := []string{"apt", "dnf", "pacman", "snap", "brew", "yum", "zypper"}
		for _, supported := range supportedTestMethods {
			if method == supported {
				return true
			}
		}
		return false
	}

	if pluginBootstrap == nil {
		return false
	}

	// Check if package manager plugin exists
	pluginName := "package-manager-" + method
	return pluginBootstrap.IsPluginAvailable(ctx, pluginName)
}

// GetInstaller returns the installer instance for the given method, or nil if not found
// NOTE: This now uses the plugin system instead of direct installer instances
func GetInstaller(ctx context.Context, method string) types.BaseInstaller {
	if !IsInstallerSupported(ctx, method) {
		return nil
	}

	// In test mode, return a mock installer
	if testMode {
		return &MockInstaller{
			method: method,
		}
	}

	return &PluginBasedInstaller{
		method:          method,
		pluginBootstrap: pluginBootstrap,
	}
}

// PluginBasedInstaller wraps plugin execution in the BaseInstaller interface
type PluginBasedInstaller struct {
	method          string
	pluginBootstrap *bootstrap.PluginBootstrap
}

// Install executes the plugin install command
func (p *PluginBasedInstaller) Install(command string, repo types.Repository) error {
	if p.pluginBootstrap == nil {
		return fmt.Errorf("plugin bootstrap not initialized")
	}

	pluginName := "package-manager-" + p.method
	args := []string{"install"}

	// Parse command to extract package names
	if command != "" {
		// Simple command parsing - split on spaces and filter out common flags
		parts := strings.Fields(command)
		for _, part := range parts {
			if !strings.HasPrefix(part, "-") && part != "install" && part != "sudo" {
				args = append(args, part)
			}
		}
	}

	return p.pluginBootstrap.ExecutePlugin(pluginName, args)
}

// Uninstall executes the plugin remove command
func (p *PluginBasedInstaller) Uninstall(command string, repo types.Repository) error {
	if p.pluginBootstrap == nil {
		return fmt.Errorf("plugin bootstrap not initialized")
	}

	pluginName := "package-manager-" + p.method
	args := []string{"remove"}

	// Parse command to extract package names
	if command != "" {
		parts := strings.Fields(command)
		for _, part := range parts {
			if !strings.HasPrefix(part, "-") && part != "remove" && part != "sudo" {
				args = append(args, part)
			}
		}
	}

	return p.pluginBootstrap.ExecutePlugin(pluginName, args)
}

// IsInstalled checks if a package is installed using the plugin
func (p *PluginBasedInstaller) IsInstalled(command string) (bool, error) {
	if p.pluginBootstrap == nil {
		return false, fmt.Errorf("plugin bootstrap not initialized")
	}

	pluginName := "package-manager-" + p.method
	args := []string{"list"}

	// For now, we'll return true and let the plugin handle the check
	// This is a simplified implementation
	err := p.pluginBootstrap.ExecutePlugin(pluginName, args)
	return err == nil, nil
}

func executeInstallCommand(ctx context.Context, app types.AppConfig, repo types.Repository) error {
	installer := GetInstaller(ctx, app.InstallMethod)
	if installer == nil {
		log.Error("Unsupported install method", fmt.Errorf("method: %s", app.InstallMethod))
		return fmt.Errorf("install method '%s' is not supported on this platform", app.InstallMethod)
	}
	log.Info("Executing installer", "method", app.InstallMethod)
	return installer.Install(app.InstallCommand, repo)
}

// InstallCrossPlatformApp installs a cross-platform application using the appropriate OS-specific configuration
func InstallCrossPlatformApp(ctx context.Context, app types.CrossPlatformApp, settings config.CrossPlatformSettings, repo types.Repository) error {
	log.Info("Installing cross-platform app", "app", app.Name)

	// Validate that the app is supported on the current platform
	if !app.IsSupported() {
		return fmt.Errorf("app %s is not supported on current platform", app.Name)
	}

	// Validate the app configuration
	if err := app.Validate(); err != nil {
		return fmt.Errorf("app validation failed: %w", err)
	}

	// Get OS-specific configuration
	osConfig := app.GetOSConfig()

	// Create AppConfig for direct installation
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name:        app.Name,
			Description: app.Description,
			Category:    app.Category,
		},
		Default:          app.Default,
		InstallMethod:    osConfig.InstallMethod,
		InstallCommand:   osConfig.InstallCommand,
		UninstallCommand: osConfig.UninstallCommand,
		Dependencies:     osConfig.Dependencies,
		PreInstall:       osConfig.PreInstall,
		PostInstall:      osConfig.PostInstall,
		ConfigFiles:      osConfig.ConfigFiles,
		AptSources:       osConfig.AptSources,
		CleanupFiles:     osConfig.CleanupFiles,
		Conflicts:        osConfig.Conflicts,
		DownloadURL:      osConfig.DownloadURL,
		InstallDir:       osConfig.Destination,
	}

	// Install the app directly
	return InstallApp(ctx, appConfig, settings, repo)
}

// InstallApp installs a single application
func InstallApp(ctx context.Context, app types.AppConfig, settings config.CrossPlatformSettings, repo types.Repository) error {
	log.Info("Installing app", "app", app.Name)

	// Execute pre-install commands
	if err := runInstallCommands(app.PreInstall); err != nil {
		return fmt.Errorf("failed to execute pre-install commands: %w", err)
	}

	// Execute the actual install command
	if err := executeInstallCommand(ctx, app, repo); err != nil {
		return fmt.Errorf("failed to execute install command: %w", err)
	}

	// Execute post-install commands
	if err := runInstallCommands(app.PostInstall); err != nil {
		return fmt.Errorf("failed to execute post-install commands: %w", err)
	}

	log.Info("App installed successfully", "app", app.Name)
	return nil
}

func runInstallCommands(commands []types.InstallCommand) error {
	log.Info("Starting runInstallCommands", "commands", len(commands))

	for _, cmd := range commands {
		if cmd.Shell != "" {
			processedCommand := utils.ReplacePlaceholders(cmd.Shell, map[string]string{})
			log.Info("Executing shell command", "command", processedCommand)
			output, err := utils.ExecAsUser(processedCommand)
			if err != nil {
				log.Error("Failed to execute shell command", err, "output", output)
				return fmt.Errorf("failed to execute shell command: %w", err)
			}
		}

		if cmd.Copy != nil {
			source := utils.ReplacePlaceholders(cmd.Copy.Source, map[string]string{})
			destination := utils.ReplacePlaceholders(cmd.Copy.Destination, map[string]string{})
			log.Info("Copying file", "source", source, "destination", destination)
			if err := utils.CopyFile(source, destination); err != nil {
				log.Error("Failed to copy file", err, "source", source, "destination", destination)
				return fmt.Errorf("failed to copy file from %s to %s: %w", source, destination, err)
			}
		}
	}

	log.Info("Completed runInstallCommands successfully")
	return nil
}

// MockInstaller is a test-only installer that simulates package manager operations
type MockInstaller struct {
	method string
}

// Install simulates package installation
func (m *MockInstaller) Install(command string, repo types.Repository) error {
	log.Info("Mock install", "method", m.method, "command", command)
	// Simulate successful installation in test mode
	return nil
}

// Uninstall simulates package uninstallation
func (m *MockInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Mock uninstall", "method", m.method, "command", command)
	// Simulate successful uninstallation in test mode
	return nil
}

// IsInstalled simulates checking if a package is installed
func (m *MockInstaller) IsInstalled(command string) (bool, error) {
	log.Info("Mock check installed", "method", m.method, "command", command)
	// Simulate package being installed in test mode
	return true, nil
}
