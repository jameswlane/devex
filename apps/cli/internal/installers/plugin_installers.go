package installers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/system"
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

// pluginIOExecutor is implemented by executors that accept caller-owned streams.
type pluginIOExecutor interface {
	ExecutePluginWithIO(context.Context, string, []string, sdk.OperationIO) error
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

	if p.method != "script" && p.method != "curlpipe" {
		return p.InstallPackages(p.context(), command, nil, nil)
	}
	packages, err := catalogPackageArguments(p.method, command)
	if err != nil {
		return err
	}
	return p.pluginBootstrap.ExecutePluginWithContext(p.context(), "package-manager-"+p.method, append([]string{"install"}, packages...))
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
	case "apt", "appimage", "deb", "dnf", "flatpak", "mise", "pip":
		packages, err := catalogPackageArguments(p.method, command)
		if err != nil {
			return false, err
		}
		args := append([]string{"is-installed"}, packages...)

		// Plugins report an absent package with sdk.NotInstalledError (exit code
		// sdk.ExitCodeNotInstalled). Any other failure means the query itself
		// failed, which is not proof of absence: return it as an error so status
		// shows "error" and uninstall keeps its tracking record.
		if err := p.executeQuietly(pluginName, args); err != nil {
			var missing *sdk.NotInstalledError
			if errors.As(err, &missing) {
				return false, nil
			}
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

// executeQuietly runs a read-only plugin query with its output on stderr, so
// commands that print machine-readable results (for example
// `devex status --format json`) keep stdout clean.
func (p *PluginBasedInstaller) executeQuietly(pluginName string, args []string) error {
	if io, ok := p.pluginBootstrap.(pluginIOExecutor); ok {
		return io.ExecutePluginWithIO(p.context(), pluginName, args, sdk.OperationIO{Stdout: os.Stderr, Stderr: os.Stderr})
	}
	return p.pluginBootstrap.ExecutePluginWithContext(p.context(), pluginName, args)
}

// catalogPackageArguments returns identifiers for removal and discovery; only
// Homebrew's namespace selector is meaningful outside installation.
func catalogPackageArguments(method, command string) ([]string, error) {
	if method == "script" || method == "curlpipe" {
		parts := strings.Fields(command)
		packages := make([]string, 0, len(parts))
		for _, part := range parts {
			if !strings.HasPrefix(part, "-") && part != "sudo" && part != "install" && part != "remove" {
				packages = append(packages, part)
			}
		}
		return packages, nil
	}
	request, err := types.ResolvePackageInstall(method, command, nil, nil)
	if err != nil {
		return nil, err
	}
	if method == "brew" {
		return append(request.Options, request.Packages...), nil
	}
	return request.Packages, nil
}

// InstallPackages sends a validated request using the released argv contract.
func (p *PluginBasedInstaller) InstallPackages(ctx context.Context, command string, packages, options []string) error {
	if p.pluginBootstrap == nil {
		return fmt.Errorf("plugin bootstrap not initialized")
	}
	request, err := types.ResolvePackageInstall(p.method, command, packages, options)
	if err != nil {
		return err
	}
	if len(request.Packages) == 0 {
		return fmt.Errorf("no packages specified")
	}
	args := append([]string{"install"}, request.Options...)
	args = append(args, request.Packages...)
	return p.pluginBootstrap.ExecutePluginWithContext(ctx, "package-manager-"+p.method, args)
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
	if app.InstallMethod == "curlpipe" || app.InstallMethod == "deb" || app.InstallMethod == "appimage" {
		return InstallArtifact(ctx, app)
	}

	if plugin, ok := installer.(*PluginBasedInstaller); ok && (len(app.Packages) > 0 || len(app.Options) > 0) {
		return plugin.InstallPackages(ctx, app.InstallCommand, app.Packages, app.Options)
	}
	return installer.Install(app.InstallCommand, repo)
}

type packagesInstalledKey struct{}

// WithPackagesInstalled marks an app whose packages a shared batch transaction
// already installed, so InstallApp skips only its install step.
func WithPackagesInstalled(ctx context.Context, appName string) context.Context {
	return context.WithValue(ctx, packagesInstalledKey{}, appName)
}

// PackagesInstalled reports whether WithPackagesInstalled marked this app.
func PackagesInstalled(ctx context.Context, appName string) bool {
	name, ok := ctx.Value(packagesInstalledKey{}).(string)
	return ok && name == appName
}

// InstallCrossPlatformApp installs a cross-platform application using the appropriate OS-specific configuration
func InstallCrossPlatformApp(ctx context.Context, app types.CrossPlatformApp, settings config.CrossPlatformSettings, repo types.Repository) (result error) {
	ctx, finishAudit := state.BeginApplication(ctx, app.Name, "plain", app)
	defer func() { finishAudit(result) }()
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
		SystemRequirements: osConfig.SystemRequirements,
		Default:            app.Default,
		InstallMethod:      osConfig.InstallMethod,
		InstallCommand:     osConfig.InstallCommand,
		Packages:           osConfig.Packages,
		Options:            osConfig.Options,
		UninstallCommand:   osConfig.UninstallCommand,
		Dependencies:       osConfig.Dependencies,
		PreInstall:         osConfig.PreInstall,
		PostInstall:        osConfig.PostInstall,
		ConfigFiles:        osConfig.ConfigFiles,
		AptSources:         osConfig.AptSources,
		CleanupFiles:       osConfig.CleanupFiles,
		Conflicts:          osConfig.Conflicts,
		DownloadURL:        osConfig.DownloadURL,
		Verification:       osConfig.Verification,
		InstallDir:         osConfig.Destination,
	}

	// Install the app directly
	return InstallApp(ctx, appConfig, settings, repo)
}

// InstallApp installs a single application
func InstallApp(ctx context.Context, app types.AppConfig, settings config.CrossPlatformSettings, repo types.Repository) (result error) {
	ctx, finishAudit := state.BeginApplication(ctx, app.Name, "plain", app)
	defer func() { finishAudit(result) }()
	log.Info("Installing app", "app", app.Name)

	if err := system.CheckApplicationRequirements(ctx, app.Name, app.SystemRequirements, nil); err != nil {
		return err
	}

	// Resolve the same configured policy used by the interactive installer.
	validator, err := settings.GetCommandValidator()
	if err != nil {
		return err
	}
	policy := utils.InstallHookPolicy{Validator: validator, AppName: app.Name}

	// Execute pre-install commands
	if err := state.AuditStep(ctx, "pre-install", len(app.PreInstall) > 0, func() error { return runInstallCommands(ctx, app.PreInstall, policy) }); err != nil {
		return fmt.Errorf("failed to execute pre-install commands: %w", err)
	}

	// Execute the actual install command
	if err := state.AuditStep(ctx, "install", true, func() error {
		if PackagesInstalled(ctx, app.Name) {
			log.Info("Packages were installed in the batch transaction", "app", app.Name)
			return nil
		}
		return executeInstallCommand(ctx, app, repo)
	}); err != nil {
		return fmt.Errorf("failed to execute install command: %w", err)
	}

	// Shared lifecycle stage: package install -> config_files -> post-install hooks.
	// Config-file problems are warnings: the audit step records them as a failed
	// step, but they don't fail the install.
	var changed error
	_ = state.AuditStep(ctx, "config-files", len(app.ConfigFiles) > 0, func() error {
		result := configfiles.Apply(ctx, app.Name, app.ConfigFiles, false, func(message string) {
			if strings.HasPrefix(message, "Warning:") {
				log.Warn(message)
			} else {
				log.Info(message)
			}
		})
		changed = result.Changed
		if len(result.Warnings) > 0 {
			return errors.New(strings.Join(result.Warnings, "; "))
		}
		return nil
	})
	if changed != nil {
		return fmt.Errorf("apply configuration files: %w", changed)
	}

	// Execute post-install commands
	if err := state.AuditStep(ctx, "post-install", len(app.PostInstall) > 0, func() error { return runInstallCommands(ctx, app.PostInstall, policy) }); err != nil {
		return fmt.Errorf("failed to execute post-install commands: %w", err)
	}

	log.Info("App installed successfully", "app", app.Name)
	return nil
}

func runInstallCommands(ctx context.Context, commands []types.InstallCommand, policies ...utils.InstallHookPolicy) error {
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
		if err := utils.ApplyShellActions(ctx, cmd); err != nil {
			return err
		}
		if cmd.Shell != "" {
			log.Info("Executing shell command", "command", cmd.Shell)
			output, err := utils.ExecInstallHookWithContext(ctx, cmd, policies...)
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
	ctx := p.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if !interaction.IsInteractiveMode() {
		ctx = sdk.WithUnattended(ctx)
	}
	return ctx
}

// CatalogPackageArguments resolves native package identifiers from a catalog command.
func CatalogPackageArguments(method, command string) ([]string, error) {
	return catalogPackageArguments(method, command)
}

// InstallArtifact dispatches integrity metadata alongside direct artifact arguments.
func InstallArtifact(ctx context.Context, app types.AppConfig) error {
	installer := GetInstaller(ctx, app.InstallMethod)
	if plugin, ok := installer.(*PluginBasedInstaller); ok {
		return plugin.InstallArtifact(ctx, app)
	}

	if installer == nil {
		return fmt.Errorf("artifact installer unavailable: %s", app.InstallMethod)
	}
	return installer.Install(app.InstallCommand, nil)
}

// InstallArtifact preserves metadata and native argv in the plugin request.
func (p *PluginBasedInstaller) InstallArtifact(ctx context.Context, app types.AppConfig) error {
	if p.pluginBootstrap == nil {
		return fmt.Errorf("plugin bootstrap not initialized")
	}
	command := app.InstallCommand
	if app.DownloadURL != "" && app.InstallMethod != "appimage" {
		command = app.DownloadURL
	}
	packages := app.Packages
	options := app.Options
	var err error
	if app.InstallMethod != "curlpipe" {
		request, resolveErr := types.ResolvePackageInstall(app.InstallMethod, command, packages, options)
		if resolveErr != nil {
			return resolveErr
		}
		packages, options = request.Packages, request.Options
	} else if len(packages) == 0 {
		packages, err = catalogPackageArguments(app.InstallMethod, command)
	}
	if err != nil {
		return err
	}
	if len(app.Verification.Arguments()) > 0 && app.InstallMethod != "appimage" && len(packages) != 1 {
		return fmt.Errorf("verification metadata requires exactly one artifact")
	}
	if len(packages) > 0 {
		verification, cleanup, err := PrepareArtifact(ctx, packages[0], app.Verification)
		if err != nil {
			return err
		}
		defer cleanup()
		app.Verification = verification
	}
	args := append([]string{"install"}, app.Verification.Arguments()...)
	args = append(args, packages...)
	args = append(args, options...)
	if executor, ok := p.pluginBootstrap.(pluginIOExecutor); ok {
		return executor.ExecutePluginWithIO(ctx, "package-manager-"+app.InstallMethod, args, sdk.OperationIO{Stdin: os.Stdin, Stdout: os.Stderr, Stderr: os.Stderr})
	}
	return p.pluginBootstrap.ExecutePluginWithContext(ctx, "package-manager-"+app.InstallMethod, args)
}
