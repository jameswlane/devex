package brew

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

type BrewInstaller struct{}

// BrewVersion information
type BrewVersion struct {
	Major int
	Minor int
	Patch int
}

// Cached Brew version to avoid repeated detection
var cachedBrewVersion *BrewVersion

// ResetVersionCache resets the cached Brew version (useful for testing)
func ResetVersionCache() {
	cachedBrewVersion = nil
}

// getBrewVersion detects the Homebrew version
func getBrewVersion() (*BrewVersion, error) {
	if cachedBrewVersion != nil {
		return cachedBrewVersion, nil
	}

	output, err := utils.CommandExec.RunShellCommand("brew --version")
	if err != nil {
		return nil, fmt.Errorf("failed to detect Homebrew version: %w", err)
	}

	// Parse version from output like "Homebrew 4.0.0"
	versionRegex := regexp.MustCompile(`Homebrew\s+(\d+)\.(\d+)\.(\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) < 4 {
		return nil, fmt.Errorf("failed to parse Homebrew version from output: %s", output)
	}

	major := 0
	minor := 0
	patch := 0
	_, _ = fmt.Sscanf(matches[1], "%d", &major)
	_, _ = fmt.Sscanf(matches[2], "%d", &minor)
	_, _ = fmt.Sscanf(matches[3], "%d", &patch)

	cachedBrewVersion = &BrewVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}

	log.Debug("Detected Homebrew version", "version", fmt.Sprintf("%d.%d.%d", major, minor, patch))
	return cachedBrewVersion, nil
}

func New() *BrewInstaller {
	return &BrewInstaller{}
}

func (b *BrewInstaller) Install(command string, repo types.Repository) error {
	log.Debug("Brew Installer: Starting installation", "packageName", command)

	// Detect Homebrew version early to ensure optimal commands are used
	if _, err := getBrewVersion(); err != nil {
		log.Warn("Failed to detect Homebrew version, using defaults", "error", err)
	}

	// Run background validation for better performance
	validator := utilities.NewBackgroundValidator(30 * time.Second)
	validator.AddSuite(utilities.CreateSystemValidationSuite("brew"))
	validator.AddSuite(utilities.CreateNetworkValidationSuite())

	ctx := context.Background()
	if err := validator.RunValidations(ctx); err != nil {
		return utilities.WrapError(err, utilities.ErrorTypeSystem, "install", command, "brew")
	}

	// Validate package name
	if err := validateBrewPackageName(command); err != nil {
		return fmt.Errorf("invalid package name: %w", err)
	}

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "brew",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "packageName", command)
		return fmt.Errorf("failed to check if Brew package is installed: %w", err)
	}

	if isInstalled {
		log.Info("Package already installed, skipping installation", "packageName", command)
		return nil
	}

	// Update Homebrew using intelligent update system
	if err := utilities.EnsurePackageManagerUpdated(ctx, "brew", repo, 24*time.Hour); err != nil {
		log.Warn("Failed to update Homebrew", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Validate package availability
	if err := validateBrewPackageAvailability(command); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Run `brew install` command with proper context
	installCommand := fmt.Sprintf("brew install %s", command)
	_, err = utils.CommandExec.RunShellCommand(installCommand)
	if err != nil {
		log.Error("Failed to install package using Brew", err, "packageName", command, "command", installCommand)

		// Provide actionable error messages
		if strings.Contains(err.Error(), "No available formula") {
			return fmt.Errorf("failed to install Brew package '%s': formula not found (hint: check formula name or try 'brew search %s')", command, command)
		}
		if strings.Contains(err.Error(), "Permission denied") {
			return fmt.Errorf("failed to install Brew package '%s': permission denied (hint: ensure Homebrew is properly configured for your user)", command)
		}
		if strings.Contains(err.Error(), "Another active Homebrew process") {
			return fmt.Errorf("failed to install Brew package '%s': another Homebrew process is running (hint: wait for other operations to complete)", command)
		}
		return fmt.Errorf("failed to install Brew package '%s': %w", command, err)
	}

	log.Debug("Brew package installed successfully", "packageName", command)

	// Verify installation succeeded
	verifyConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "brew",
		InstallCommand: command,
	}
	if isInstalled, err := utilities.IsAppInstalled(verifyConfig); err != nil {
		log.Warn("Failed to verify installation", "error", err, "packageName", command)
	} else if !isInstalled {
		return fmt.Errorf("package installation verification failed for: %s (hint: check 'brew list' to see installed packages)", command)
	}

	// Perform post-installation setup
	if err := performPostInstallationSetup(command); err != nil {
		log.Warn("Post-installation setup failed", "package", command, "error", err)
		// Don't fail the installation, just warn
	}

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "packageName", command)
		return fmt.Errorf("failed to add Brew package '%s' to repository: %w", command, err)
	}

	log.Debug("Package added to repository successfully", "packageName", command)
	return nil
}

// Uninstall removes packages using brew
func (b *BrewInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("Brew Installer: Starting uninstallation", "packageName", command)

	// Check if the package is installed
	isInstalled, err := b.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "packageName", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "packageName", command)
		return nil
	}

	// Run `brew uninstall` command
	uninstallCommand := fmt.Sprintf("brew uninstall %s", command)
	_, err = utils.CommandExec.RunShellCommand(uninstallCommand)
	if err != nil {
		log.Error("Failed to uninstall package using Brew", err, "packageName", command, "command", uninstallCommand)

		// Provide actionable error messages
		if strings.Contains(err.Error(), "No such keg") {
			return fmt.Errorf("failed to uninstall Brew package '%s': package not found (hint: use 'brew list' to see installed packages)", command)
		}
		if strings.Contains(err.Error(), "Permission denied") {
			return fmt.Errorf("failed to uninstall Brew package '%s': permission denied (hint: check file permissions)", command)
		}
		return fmt.Errorf("failed to uninstall Brew package '%s': %w", command, err)
	}

	log.Debug("Brew package uninstalled successfully", "packageName", command)

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "packageName", command)
		return fmt.Errorf("failed to remove Brew package from repository: %w", err)
	}

	log.Debug("Package removed from repository successfully", "packageName", command)
	return nil
}

// IsInstalled checks if a package is installed using brew
func (b *BrewInstaller) IsInstalled(command string) (bool, error) {
	// Use brew list to check if package is installed
	checkCommand := fmt.Sprintf("brew list %s", command)
	_, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err != nil {
		// brew list returns non-zero exit code if package is not installed
		return false, nil
	}

	// If brew list succeeds, package is installed
	return true, nil
}

// PackageManager interface implementation methods

// InstallPackages installs multiple packages via Homebrew (implements PackageManager interface)
func (b *BrewInstaller) InstallPackages(ctx context.Context, packages []string, dryRun bool) error {
	if len(packages) == 0 {
		return nil
	}

	log.Info("Installing packages via Homebrew", "packages", packages, "dryRun", dryRun)

	if dryRun {
		log.Info("DRY RUN: Would install packages", "packages", packages)
		return nil
	}

	// Update Homebrew first
	if _, err := utils.CommandExec.RunShellCommand("brew update"); err != nil {
		log.Warn("Failed to update Homebrew", "error", err)
		// Continue anyway
	}

	// Install packages in batch - Homebrew supports multiple packages
	packageList := strings.Join(packages, " ")
	installCmd := fmt.Sprintf("brew install %s", packageList)

	if _, err := utils.CommandExec.RunShellCommand(installCmd); err != nil {
		// If batch install fails, try individual packages
		log.Warn("Batch install failed, trying individual packages", "error", err)
		for _, pkg := range packages {
			if _, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("brew install %s", pkg)); err != nil {
				// Don't fail the entire batch, just log the error
				log.Error("Failed to install Homebrew package", err, "package", pkg)
				continue
			}
			log.Info("Successfully installed Homebrew package", "package", pkg)
		}
	} else {
		log.Info("Successfully installed all Homebrew packages", "packages", packages)
	}

	return nil
}

// IsAvailable checks if Homebrew package manager is available
func (b *BrewInstaller) IsAvailable(ctx context.Context) bool {
	_, err := utils.CommandExec.RunShellCommand("which brew")
	if err != nil {
		return false
	}

	// Also check if Homebrew is properly initialized
	_, err = utils.CommandExec.RunShellCommand("brew --version")
	return err == nil
}

// GetName returns the package manager name
func (b *BrewInstaller) GetName() string {
	return "brew"
}

// Helper functions

// validateBrewPackageName validates package name format
func validateBrewPackageName(packageName string) error {
	if strings.TrimSpace(packageName) == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	// Check for potentially dangerous names
	if packageName == "." || packageName == ".." {
		return fmt.Errorf("invalid package name '%s'", packageName)
	}

	// Validate against regex for safe package names
	validName := regexp.MustCompile(`^[a-zA-Z0-9@._+/-]+$`)
	if !validName.MatchString(packageName) {
		return fmt.Errorf("package name '%s' contains invalid characters", packageName)
	}

	return nil
}

// validateBrewPackageAvailability checks if a package is available in formulae
func validateBrewPackageAvailability(packageName string) error {
	// Try to search for the package
	searchCmd := fmt.Sprintf("brew search %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(searchCmd)
	if err != nil {
		return fmt.Errorf("failed to search for package '%s': %w (hint: check internet connection and package name)", packageName, err)
	}

	// Check if any result was found
	if output == "" || strings.TrimSpace(output) == "" || strings.Contains(output, "No formula or cask found") {
		return fmt.Errorf("no installable candidate found for package '%s' (hint: check formula name or try 'brew search %s')", packageName, packageName)
	}

	log.Debug("Package availability validated", "package", packageName)
	return nil
}

// performPostInstallationSetup handles package-specific post-installation configuration
func performPostInstallationSetup(packageName string) error {
	// Run any post-install hooks that Homebrew might have
	if _, err := utils.CommandExec.RunShellCommand("brew cleanup 2>/dev/null || true"); err != nil {
		log.Debug("Failed to run brew cleanup", "error", err)
	}

	// Update shell completions if available
	if strings.Contains(packageName, "completion") || packageName == "bash-completion" || packageName == "zsh-completions" {
		if _, err := utils.CommandExec.RunShellCommand("brew --prefix 2>/dev/null || true"); err != nil {
			log.Debug("Failed to get brew prefix for completions", "error", err)
		}
	}

	return nil
}

// RunBrewUpdate updates Homebrew formulae
func RunBrewUpdate(forceUpdate bool, repo types.Repository) error {
	log.Debug("Starting Homebrew update", "forceUpdate", forceUpdate)

	ctx := context.Background()

	if forceUpdate {
		// Force update by using a very short max age
		return utilities.EnsurePackageManagerUpdated(ctx, "brew", repo, 1*time.Second)
	} else {
		// Use standard 24-hour cache
		return utilities.EnsurePackageManagerUpdated(ctx, "brew", repo, 24*time.Hour)
	}
}
