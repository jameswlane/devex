package installers

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

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
		ctx:             ctx,
		pluginBootstrap: pluginBootstrap,
	}
}

type pluginExecutor interface {
	ExecutePluginWithContext(context.Context, string, []string) error
}

// PluginBasedInstaller wraps plugin execution in the BaseInstaller interface
type PluginBasedInstaller struct {
	method          string
	pluginBootstrap pluginExecutor
	ctx             context.Context
}

// Install executes the plugin install command
func (p *PluginBasedInstaller) Install(command string, repo types.Repository) error {
	if p.pluginBootstrap == nil {
		return fmt.Errorf("plugin bootstrap not initialized")
	}

	pluginName := "package-manager-" + p.method
	packages, err := catalogPackageArguments(p.method, command)
	if err != nil {
		return err
	}
	args := append([]string{"install"}, packages...)

	return p.pluginBootstrap.ExecutePluginWithContext(p.context(), pluginName, args)
}

// Uninstall executes the plugin remove command
func (p *PluginBasedInstaller) Uninstall(command string, repo types.Repository) error {
	if p.pluginBootstrap == nil {
		return fmt.Errorf("plugin bootstrap not initialized")
	}

	pluginName := "package-manager-" + p.method
	packages, err := catalogPackageArguments(p.method, command)
	if err != nil {
		return err
	}
	args := append([]string{"remove"}, packages...)

	return p.pluginBootstrap.ExecutePluginWithContext(p.context(), pluginName, args)
}

// IsInstalled checks if a package is installed using the plugin
func (p *PluginBasedInstaller) IsInstalled(command string) (bool, error) {
	if p.pluginBootstrap == nil {
		return false, fmt.Errorf("plugin bootstrap not initialized")
	}

	pluginName := "package-manager-" + p.method
	switch p.method {
	case "apt", "appimage", "deb", "flatpak", "mise", "pip":
		packages, err := catalogPackageArguments(p.method, command)
		if err != nil {
			return false, err
		}
		args := append([]string{"is-installed"}, packages...)

		// Plugins currently use the same error exit for an absent package and a
		// discovery failure. Neither is proof of absence: preserve the error and
		// tracking until the plugin protocol can distinguish them.
		if err := p.pluginBootstrap.ExecutePluginWithContext(p.context(), pluginName, args); err != nil {
			return false, fmt.Errorf("plugin %s installation check failed: %w", pluginName, err)
		}
		return true, nil
	default:
		// For plugins without a package-specific installed query, skip the
		// probe (treating status as unknown) so offline uninstalls are not
		// blocked by a full package list query. Removal will report whether
		// the package was present.
		return true, nil
	}
}

// catalogPackageArguments normalizes package-manager command prefixes, not shell
// programs. Flags retain the existing behavior of being omitted from plugin argv.
// Package identifiers (including versions, URLs and local archives) stay intact.
func catalogPackageArguments(method, command string) ([]string, error) {
	parts := strings.Fields(command)
	if method == "script" || method == "curlpipe" {
		// These methods consume scripts rather than package identifiers. Preserve
		// their existing token handling; interpreting scripts is outside this parser.
		packages := make([]string, 0, len(parts))
		for _, part := range parts {
			if !strings.HasPrefix(part, "-") && part != "sudo" && part != "install" && part != "remove" {
				packages = append(packages, part)
			}
		}
		return packages, nil
	}
	if strings.ContainsAny(command, "\n\r|;&`") || strings.Contains(command, "$(") {
		return nil, fmt.Errorf("invalid %s catalog command: use package identifiers and move shell commands to pre_install or post_install", method)
	}

	// Only consume a manager name when it introduces a command. A package
	// named apt, pip, sudo, install or remove must remain a package identifier.
	start := 0
	if len(parts) > 1 && parts[0] == "sudo" {
		start++
	}
	manager := method
	if start < len(parts) && (parts[start] == method ||
		((method == "apt" || method == "deb") && (parts[start] == "apt" || parts[start] == "apt-get"))) {
		start++
	}
	verb := ""
	if start < len(parts)-1 {
		switch parts[start] {
		case "install", "remove", "uninstall":
			verb = parts[start]
			parts = parts[start+1:]
		}
	}
	packages := make([]string, 0, len(parts))
	for _, part := range parts {
		// Homebrew's --cask selects the cask namespace (Docker Desktop), so the
		// plugin must receive it for both install and removal.
		if !strings.HasPrefix(part, "-") || (manager == "brew" && part == "--cask") {
			packages = append(packages, part)
		}
	}
	// The only explicit Flatpak remote in the catalog is flathub. It is a
	// repository selector, never an application to install, remove or probe.
	if manager == "flatpak" && verb == "install" && len(packages) > 1 && packages[0] == "flathub" {
		packages = packages[1:]
	}
	return packages, nil
}

func executeInstallCommand(ctx context.Context, app types.AppConfig, repo types.Repository) error {
	installer := GetInstaller(ctx, app.InstallMethod)
	if installer == nil {
		pluginName := "package-manager-" + app.InstallMethod
		if pluginBootstrap != nil {
			if err := pluginBootstrap.PluginDownloadError(pluginName); err != nil {
				return fmt.Errorf("required plugin %s is unavailable: %w", pluginName, err)
			}
		}
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
	if err := runInstallCommands(ctx, app.PreInstall); err != nil {
		return fmt.Errorf("failed to execute pre-install commands: %w", err)
	}

	// Execute the actual install command
	if err := executeInstallCommand(ctx, app, repo); err != nil {
		return fmt.Errorf("failed to execute install command: %w", err)
	}

	// Execute post-install commands
	if err := runInstallCommands(ctx, app.PostInstall); err != nil {
		return fmt.Errorf("failed to execute post-install commands: %w", err)
	}

	log.Info("App installed successfully", "app", app.Name)
	return nil
}

func runInstallCommands(ctx context.Context, commands []types.InstallCommand) error {
	if len(commands) == 0 {
		return nil
	}
	plan, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: "install-hooks"})
	if err != nil {
		return err
	}
	log.Info("Starting runInstallCommands", "commands", len(commands))

	for _, cmd := range commands {
		if _, err := utils.InstallHookPrivilege(cmd); err != nil {
			return err
		}
		if cmd.Shell != "" {
			log.Info("Executing shell command", "command", cmd.Shell)
			output, err := utils.ExecInstallHookWithContext(ctx, cmd)
			if output != "" {
				fmt.Fprint(os.Stdout, output)
			}
			if err != nil {
				log.Error("Failed to execute shell command", err, "output", output)
				return fmt.Errorf("failed to execute shell command: %w", err)
			}
		}

		if cmd.Copy != nil {
			source := utils.ExpandUserPlaceholders(cmd.Copy.Source, *plan.IntendedUser)
			destination := utils.ExpandUserPlaceholders(cmd.Copy.Destination, *plan.IntendedUser)
			log.Info("Copying file", "source", source, "destination", destination)
			if err := utils.CopyUserFileWithContext(ctx, source, destination); err != nil {
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

func (p *PluginBasedInstaller) context() context.Context {
	if p.ctx == nil {
		return context.Background()
	}
	return p.ctx
}
