package flatpak

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type FlatpakInstaller struct{}

// FlatpakVersion information
type FlatpakVersion struct {
	Major int
	Minor int
	Patch int
}

// Cached Flatpak version to avoid repeated detection
var cachedFlatpakVersion *FlatpakVersion

// ResetVersionCache resets the cached Flatpak version (useful for testing)
func ResetVersionCache() {
	cachedFlatpakVersion = nil
}

// getFlatpakVersion detects the Flatpak version
func getFlatpakVersion() (*FlatpakVersion, error) {
	if cachedFlatpakVersion != nil {
		return cachedFlatpakVersion, nil
	}

	output, err := utils.CommandExec.RunShellCommand("flatpak --version")
	if err != nil {
		return nil, fmt.Errorf("failed to detect Flatpak version: %w", err)
	}

	// Parse version from output like "Flatpak 1.14.4"
	versionRegex := regexp.MustCompile(`Flatpak\s+(\d+)\.(\d+)\.(\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) < 4 {
		// Try alternate format
		versionRegex = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
		matches = versionRegex.FindStringSubmatch(output)
		if len(matches) < 4 {
			return nil, fmt.Errorf("failed to parse Flatpak version from output: %s", output)
		}
	}

	major := 0
	minor := 0
	patch := 0
	_, _ = fmt.Sscanf(matches[1], "%d", &major)
	_, _ = fmt.Sscanf(matches[2], "%d", &minor)
	_, _ = fmt.Sscanf(matches[3], "%d", &patch)

	cachedFlatpakVersion = &FlatpakVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}

	log.Debug("Detected Flatpak version", "version", fmt.Sprintf("%d.%d.%d", major, minor, patch))
	return cachedFlatpakVersion, nil
}

func New() *FlatpakInstaller {
	return &FlatpakInstaller{}
}

func (f *FlatpakInstaller) Install(command string, repo types.Repository) error {
	log.Debug("Flatpak Installer: Starting installation", "appID", command)

	// Detect Flatpak version early to ensure optimal commands are used
	if _, err := getFlatpakVersion(); err != nil {
		log.Warn("Failed to detect Flatpak version, using defaults", "error", err)
	}

	// Run background validation for better performance
	validator := utilities.NewBackgroundValidator(30 * time.Second)
	validator.AddSuite(utilities.CreateSystemValidationSuite("flatpak"))
	validator.AddSuite(utilities.CreateNetworkValidationSuite())

	ctx := context.Background()
	if err := validator.RunValidations(ctx); err != nil {
		return utilities.WrapError(err, utilities.ErrorTypeSystem, "install", command, "flatpak")
	}

	// Parse the app ID to determine if we need to specify a remote
	remote, appID := parseFlatpakCommand(command)

	// Wrap the command into a types.AppConfig object
	// Use the extracted appID for installation checks
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "flatpak",
		InstallCommand: appID, // Use extracted appID for checks
	}

	// Check if the app is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if app is installed", err, "appID", command)
		return fmt.Errorf("failed to check if Flatpak app is installed: %w", err)
	}

	if isInstalled {
		log.Info("App is already installed, skipping installation", "appID", command)
		return nil
	}

	// Ensure remotes are configured and updated
	if err := ensureFlatpakRemotes(ctx); err != nil {
		log.Warn("Failed to ensure Flatpak remotes", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Update Flatpak metadata using intelligent update system
	if err := utilities.EnsurePackageManagerUpdated(ctx, "flatpak", repo, 6*time.Hour); err != nil {
		log.Warn("Failed to update Flatpak metadata", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Check if app is available in repositories
	if err := validateFlatpakAvailability(command); err != nil {
		return fmt.Errorf("app validation failed: %w", err)
	}

	// Run flatpak install command with proper context
	installCommand := buildFlatpakInstallCommand(remote, appID)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install Flatpak app", err, "appID", command)

		// Provide actionable error messages
		if strings.Contains(err.Error(), "No remote refs found") {
			return fmt.Errorf("failed to install Flatpak app '%s': app not found in configured remotes (hint: check app ID or add required remote)", command)
		}
		if strings.Contains(err.Error(), "Permission denied") {
			return fmt.Errorf("failed to install Flatpak app '%s': permission denied (hint: Flatpak may require user-level installation, try without sudo)", command)
		}
		return fmt.Errorf("failed to install Flatpak app '%s': %w", command, err)
	}

	log.Debug("Flatpak app installed successfully", "appID", command)

	// Verify installation succeeded (use appConfig with extracted appID)
	verifyConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "flatpak",
		InstallCommand: appID, // Use extracted appID for verification
	}
	if isInstalled, err := utilities.IsAppInstalled(verifyConfig); err != nil {
		log.Warn("Failed to verify installation", "error", err, "appID", command)
	} else if !isInstalled {
		return fmt.Errorf("app installation verification failed for: %s (hint: check 'flatpak list' to see installed apps)", command)
	}

	// Perform post-installation setup for specific apps
	if err := performPostInstallationSetup(appID); err != nil {
		log.Warn("Post-installation setup failed", "app", appID, "error", err)
		// Don't fail the installation, just warn
	}

	// Add the app to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add Flatpak app to repository", err, "appID", command)
		return fmt.Errorf("failed to add Flatpak app '%s' to repository: %w", command, err)
	}

	log.Debug("Flatpak app added to repository successfully", "appID", command)
	return nil
}

// Uninstall removes apps using flatpak
func (f *FlatpakInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("Flatpak Installer: Starting uninstallation", "appID", command)

	// Check if the app is installed
	isInstalled, err := f.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if app is installed", err, "appID", command)
		return fmt.Errorf("failed to check if app is installed: %w", err)
	}

	if !isInstalled {
		log.Info("App not installed, skipping uninstallation", "appID", command)
		return nil
	}

	// Parse the app ID
	_, appID := parseFlatpakCommand(command)

	// Run flatpak uninstall command
	uninstallCommand := fmt.Sprintf("flatpak uninstall -y %s", appID)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall Flatpak app", err, "appID", command)

		// Provide actionable error messages
		if strings.Contains(err.Error(), "is not installed") {
			return fmt.Errorf("failed to uninstall Flatpak app '%s': app not found (hint: use 'flatpak list' to see installed apps)", command)
		}
		return fmt.Errorf("failed to uninstall Flatpak app '%s': %w", command, err)
	}

	log.Debug("Flatpak app uninstalled successfully", "appID", command)

	// Remove the app from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove Flatpak app from repository", err, "appID", command)
		return fmt.Errorf("failed to remove Flatpak app from repository: %w", err)
	}

	log.Debug("Flatpak app removed from repository successfully", "appID", command)
	return nil
}

// IsInstalled checks if an app is installed using flatpak
func (f *FlatpakInstaller) IsInstalled(command string) (bool, error) {
	// Parse the app ID
	_, appID := parseFlatpakCommand(command)

	// Use flatpak list command to check if app is installed
	// Check both user and system installations
	checkCommand := fmt.Sprintf("flatpak list --columns=application | grep -q '^%s$'", appID)
	_, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err != nil {
		// Check user installation specifically
		checkUserCommand := fmt.Sprintf("flatpak list --user --columns=application | grep -q '^%s$'", appID)
		_, userErr := utils.CommandExec.RunShellCommand(checkUserCommand)
		if userErr != nil {
			// Neither system nor user installation found
			return false, nil
		}
	}

	// App is installed (either system or user level)
	return true, nil
}

// PackageManager interface implementation methods

// InstallPackages installs multiple packages via Flatpak (implements PackageManager interface)
func (f *FlatpakInstaller) InstallPackages(ctx context.Context, packages []string, dryRun bool) error {
	if len(packages) == 0 {
		return nil
	}

	log.Info("Installing packages via Flatpak", "packages", packages, "dryRun", dryRun)

	if dryRun {
		log.Info("DRY RUN: Would install packages", "packages", packages)
		return nil
	}

	// Update Flatpak metadata first
	if _, err := utils.CommandExec.RunShellCommand("flatpak update --appstream"); err != nil {
		log.Warn("Failed to update Flatpak metadata", "error", err)
		// Continue anyway
	}

	// Install each app (Flatpak doesn't support multiple apps in one command reliably)
	for _, pkg := range packages {
		remote, appID := parseFlatpakCommand(pkg)
		installCmd := buildFlatpakInstallCommand(remote, appID)

		if _, err := utils.CommandExec.RunShellCommand(installCmd); err != nil {
			// Don't fail the entire batch, just log the error
			log.Error("Failed to install Flatpak app", err, "app", pkg)
			continue
		}
		log.Info("Successfully installed Flatpak app", "app", pkg)
	}

	return nil
}

// IsAvailable checks if Flatpak package manager is available
func (f *FlatpakInstaller) IsAvailable(ctx context.Context) bool {
	_, err := utils.CommandExec.RunShellCommand("which flatpak")
	if err != nil {
		return false
	}

	// Also check if Flatpak is properly initialized
	_, err = utils.CommandExec.RunShellCommand("flatpak remotes")
	return err == nil
}

// GetName returns the package manager name
func (f *FlatpakInstaller) GetName() string {
	return "flatpak"
}

// Helper functions

// ensureFlatpakRemotes ensures common Flatpak remotes are configured
func ensureFlatpakRemotes(ctx context.Context) error {
	// Check if flathub remote is configured
	output, err := utils.CommandExec.RunShellCommand("flatpak remotes")
	if err != nil {
		return fmt.Errorf("failed to list Flatpak remotes: %w", err)
	}

	// Add flathub if not present
	if !strings.Contains(output, "flathub") {
		log.Info("Adding Flathub remote to Flatpak")
		addRemoteCmd := "flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"
		if _, err := utils.CommandExec.RunShellCommand(addRemoteCmd); err != nil {
			log.Warn("Failed to add Flathub remote", "error", err)
			// Non-fatal, continue
		}
	}

	return nil
}

// validateFlatpakAvailability checks if an app is available in repositories
func validateFlatpakAvailability(appID string) error {
	// Parse the app ID
	remote, app := parseFlatpakCommand(appID)

	// Build search command
	searchCmd := "flatpak search " + app
	if remote != "" {
		searchCmd = fmt.Sprintf("flatpak search --arch=x86_64 %s", app)
	}

	output, err := utils.CommandExec.RunShellCommand(searchCmd)
	if err != nil {
		// Search might fail on older Flatpak versions, try remote-ls
		remoteCmd := fmt.Sprintf("flatpak remote-ls %s | grep -i %s", getRemoteOrDefault(remote), app)
		output, err = utils.CommandExec.RunShellCommand(remoteCmd)
		if err != nil {
			return fmt.Errorf("app '%s' not found in any configured remote (hint: check app ID or add required remote)", appID)
		}
	}

	// Check if any result was found
	if output == "" || strings.TrimSpace(output) == "" {
		return fmt.Errorf("no installable candidate found for app '%s'", appID)
	}

	log.Debug("App availability validated", "app", appID)
	return nil
}

// parseFlatpakCommand parses a Flatpak command to extract remote and app ID
func parseFlatpakCommand(command string) (string, string) {
	// Check if command contains remote specification (e.g., "flathub:org.mozilla.firefox")
	if strings.Contains(command, ":") {
		parts := strings.SplitN(command, ":", 2)
		return parts[0], parts[1]
	}

	// Check if it's a full app ID (e.g., "org.mozilla.firefox")
	if strings.Contains(command, ".") {
		return "", command
	}

	// Simple app name, will use default remote
	return "", command
}

// buildFlatpakInstallCommand builds the appropriate install command
func buildFlatpakInstallCommand(remote, appID string) string {
	if remote != "" {
		return fmt.Sprintf("flatpak install -y %s %s", remote, appID)
	}
	// Try flathub by default for simple names
	if !strings.Contains(appID, ".") {
		return fmt.Sprintf("flatpak install -y flathub %s", appID)
	}
	return fmt.Sprintf("flatpak install -y %s", appID)
}

// getRemoteOrDefault returns the remote name or default
func getRemoteOrDefault(remote string) string {
	if remote != "" {
		return remote
	}
	return "flathub"
}

// performPostInstallationSetup handles app-specific post-installation configuration
func performPostInstallationSetup(appID string) error {
	// Add any app-specific setup here
	// For example, setting up desktop integration, permissions, etc.

	// Ensure desktop integration is enabled
	if strings.Contains(appID, "org.") || strings.Contains(appID, "com.") {
		// Update desktop database for better integration
		if _, err := utils.CommandExec.RunShellCommand("update-desktop-database 2>/dev/null || true"); err != nil {
			log.Debug("Failed to update desktop database", "error", err)
		}
	}

	return nil
}

// RunFlatpakUpdate updates Flatpak metadata
func RunFlatpakUpdate(forceUpdate bool, repo types.Repository) error {
	log.Debug("Starting Flatpak update", "forceUpdate", forceUpdate)

	ctx := context.Background()

	if forceUpdate {
		// Force update by using a very short max age
		return utilities.EnsurePackageManagerUpdated(ctx, "flatpak", repo, 1*time.Second)
	} else {
		// Use standard 24-hour cache
		return utilities.EnsurePackageManagerUpdated(ctx, "flatpak", repo, 24*time.Hour)
	}
}
