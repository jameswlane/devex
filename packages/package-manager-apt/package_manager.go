// Package main implements the APT package manager plugin for the DevEx CLI.
// This plugin provides comprehensive Debian/Ubuntu package management with advanced
// features including repository management, GPG key handling, and security validation.
//
// The plugin supports:
//   - Package installation, removal, and updates
//   - Repository management (add, remove, list)
//   - GPG key management for repository security
//   - Package search and dependency resolution
//   - System upgrade operations
//   - Concurrent installation with rate limiting
//   - Input validation and security checks
//
// Security features include package name validation, repository URL verification,
// GPG signature validation, and protection against command injection attacks.
// All operations support context cancellation and proper error handling.
package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// APTVersion represents APT version information
type APTVersion struct {
	Major int
	Minor int
	Patch int
}

// APTInstaller implements APT package manager functionality as a plugin
type APTInstaller struct {
	*sdk.PackageManagerPlugin
	logger        sdk.Logger
	aptVersion    *APTVersion
	versionCached bool
}

// SetLogger sets the logger for the APT installer
func (a *APTInstaller) SetLogger(logger sdk.Logger) {
	a.logger = logger
}

// getLogger returns the logger, or a silent logger if none is set
func (a *APTInstaller) getLogger() sdk.Logger {
	if a.logger != nil {
		return a.logger
	}
	// Return a silent logger to prevent nil pointer dereferences
	return sdk.NewDefaultLogger(true)
}

// Execute handles command execution
func (a *APTInstaller) Execute(command string, args []string) error {
	ctx := context.Background()

	// Ensure APT is available
	a.EnsureAvailable()

	switch command {
	case "install":
		return a.handleInstall(ctx, args)
	case "remove":
		return a.handleRemove(ctx, args)
	case "update":
		return a.handleUpdate(ctx, args)
	case "upgrade":
		return a.handleUpgrade(ctx, args)
	case "search":
		return a.handleSearch(ctx, args)
	case "list":
		return a.handleList(ctx, args)
	case "info":
		return a.handleInfo(ctx, args)
	case "is-installed":
		return a.handleIsInstalled(ctx, args)
	case "add-repository":
		return a.handleAddRepository(ctx, args)
	case "remove-repository":
		return a.handleRemoveRepository(ctx, args)
	case "validate-repository":
		return a.handleValidateRepository(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// GetAPTVersion detects the APT version with caching
func (a *APTInstaller) GetAPTVersion() (*APTVersion, error) {
	if a.versionCached && a.aptVersion != nil {
		return a.aptVersion, nil
	}

	// Try apt --version first (available in APT 1.0+)
	output, err := a.ExecManagerCommandOutput("search", "--version")
	if err != nil {
		// Fallback to apt-get --version
		output, err = sdk.ExecCommandOutputWithTimeoutAndOperation(a.GetTimeout("search"), "search", "apt-get", "--version")
		if err != nil {
			return nil, fmt.Errorf("failed to detect APT version: %w", err)
		}
	}

	// Parse version from output like "apt 3.0.0 (amd64)" or "apt 1.6.12ubuntu0.2 (amd64)"
	versionRegex := regexp.MustCompile(`apt\\s+(\\d+)\\.(\\d+)\\.(\\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) < 4 {
		// Try alternate format
		versionRegex = regexp.MustCompile(`apt\\s+(\\d+)\\.(\\d+)`)
		matches = versionRegex.FindStringSubmatch(output)
		if len(matches) < 3 {
			return nil, fmt.Errorf("failed to parse APT version from output: %s", output)
		}
		// Default patch to 0 if not specified
		matches = append(matches, "0")
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	a.aptVersion = &APTVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}
	a.versionCached = true

	a.getLogger().Debug("Detected APT version", "version", fmt.Sprintf("%d.%d.%d", major, minor, patch))
	return a.aptVersion, nil
}

// GetAPTCommand returns the appropriate APT command based on version
func (a *APTInstaller) GetAPTCommand() string {
	if version, err := a.GetAPTVersion(); err == nil && version.Major < 1 {
		return "apt-get"
	}
	return "apt"
}

// isPackageInstalled checks if a package is installed using dpkg-query
func (a *APTInstaller) isPackageInstalled(packageName string) (bool, error) {
	if err := a.validatePackageName(packageName); err != nil {
		return false, err
	}

	// Use dpkg-query to check if package is installed
	output, err := sdk.ExecCommandOutputWithTimeoutAndOperation(a.GetTimeout("search"), "search", "dpkg-query", "-W", "-f=${Status}", packageName)
	if err != nil {
		// Package not found
		return false, nil
	}

	// Check if package is properly installed
	return strings.Contains(output, "install ok installed"), nil
}

// validatePackageAvailability checks if a package is available in repositories
func (a *APTInstaller) validatePackageAvailability(packageName string) error {
	if err := a.validatePackageName(packageName); err != nil {
		return err
	}

	// Use apt-cache policy to check package availability
	output, err := sdk.ExecCommandOutputWithTimeoutAndOperation(a.GetTimeout("search"), "search", "apt-cache", "policy", packageName)
	if err != nil {
		return fmt.Errorf("failed to check package availability: %w", err)
	}

	// Check if the output indicates the package is available
	outputStr := output
	if strings.Contains(outputStr, "Unable to locate package") ||
		strings.Contains(outputStr, "No packages found") ||
		strings.Contains(outputStr, "Package not found") {
		return fmt.Errorf("package '%s' not found in any repository", packageName)
	}

	// Check if any installable version is available
	if !strings.Contains(outputStr, "Candidate:") && !strings.Contains(outputStr, "Version table:") {
		return fmt.Errorf("no installable candidate found for package '%s'", packageName)
	}

	a.getLogger().Debug("Package availability validated", "package", packageName)
	return nil
}

// handleInstall installs packages
func (a *APTInstaller) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	a.getLogger().Printf("Installing packages: %s\\n", strings.Join(args, ", "))

	// Validate all package names first
	for _, pkg := range args {
		if err := a.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}
	}

	// Update package lists first
	a.getLogger().Println("Updating package lists...")
	if err := a.ExecManagerCommand("update", true, "update"); err != nil {
		a.getLogger().Warning("Failed to update package lists: %v", err)
	}

	// Check availability of packages in parallel
	if err := a.validatePackagesParallel(args); err != nil {
		return err
	}

	// Install packages
	cmdArgs := append([]string{"install", "-y"}, args...)
	if err := a.ExecManagerCommand("install", true, cmdArgs...); err != nil {
		return fmt.Errorf("failed to install packages [%s]: %w", strings.Join(args, ", "), err)
	}

	// Verify installation
	for _, pkg := range args {
		if installed, err := a.isPackageInstalled(pkg); err != nil {
			a.getLogger().Warning("Failed to verify installation of %s: %v", pkg, err)
		} else if !installed {
			return fmt.Errorf("installation verification failed for package: %s", pkg)
		}
	}

	a.getLogger().Success("Successfully installed packages: %s", strings.Join(args, ", "))
	return nil
}

// handleRemove removes packages
func (a *APTInstaller) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	a.getLogger().Printf("Removing packages: %s\\n", strings.Join(args, ", "))

	// Validate all package names first
	for _, pkg := range args {
		if err := a.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}
	}

	// Check which packages are actually installed
	var packagesToRemove []string
	for _, pkg := range args {
		if installed, err := a.isPackageInstalled(pkg); err != nil {
			a.getLogger().Warning("Failed to check installation status of %s: %v", pkg, err)
			packagesToRemove = append(packagesToRemove, pkg) // Include anyway
		} else if installed {
			packagesToRemove = append(packagesToRemove, pkg)
		} else {
			a.getLogger().Printf("Package %s is not installed, skipping\\n", pkg)
		}
	}

	if len(packagesToRemove) == 0 {
		a.getLogger().Println("No packages to remove")
		return nil
	}

	// Remove packages
	cmdArgs := append([]string{"remove", "-y"}, packagesToRemove...)
	if err := a.ExecManagerCommand("remove", true, cmdArgs...); err != nil {
		return fmt.Errorf("failed to remove packages [%s]: %w", strings.Join(args, ", "), err)
	}

	a.getLogger().Success("Successfully removed packages: %s", strings.Join(packagesToRemove, ", "))
	return nil
}

// handleUpdate updates package lists
func (a *APTInstaller) handleUpdate(ctx context.Context, args []string) error {
	a.getLogger().Println("Updating package lists...")
	if err := a.ExecManagerCommand("update", true, "update"); err != nil {
		return fmt.Errorf("failed to update package lists: %w", err)
	}
	a.getLogger().Success("Package lists updated successfully")
	return nil
}

// handleUpgrade upgrades installed packages
func (a *APTInstaller) handleUpgrade(ctx context.Context, args []string) error {
	a.getLogger().Println("Upgrading installed packages...")

	// Update first
	if err := a.ExecManagerCommand("update", true, "update"); err != nil {
		return fmt.Errorf("failed to update package lists: %w", err)
	}

	// Then upgrade
	if err := a.ExecManagerCommand("upgrade", true, "upgrade", "-y"); err != nil {
		return fmt.Errorf("failed to upgrade packages: %w", err)
	}

	a.getLogger().Success("Packages upgraded successfully")
	return nil
}

// handleSearch searches for packages
func (a *APTInstaller) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	a.getLogger().Printf("Searching for: %s\\n", searchTerm)

	return a.ExecManagerCommand("search", false, "search", searchTerm)
}

// handleList lists packages
func (a *APTInstaller) handleList(ctx context.Context, args []string) error {
	if len(args) == 0 {
		// List all installed packages
		return a.ExecManagerCommand("search", false, "list", "--installed")
	}

	// Handle flags or search terms
	cmdArgs := append([]string{"list"}, args...)
	return a.ExecManagerCommand("search", false, cmdArgs...)
}

// handleInfo shows package information
func (a *APTInstaller) handleInfo(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package specified")
	}

	for i, pkg := range args {
		if err := a.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}

		a.getLogger().Printf("Package information for: %s\\n", pkg)
		if err := sdk.ExecCommandWithContext(ctx, false, "apt", "show", pkg); err != nil {
			a.getLogger().ErrorMsg("Failed to get info for %s: %v", pkg, err)
		}

		if i < len(args)-1 {
			fmt.Println("---")
		}
	}

	return nil
}

// handleIsInstalled checks if packages are installed
func (a *APTInstaller) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	allInstalled := true
	for _, pkg := range args {
		if err := a.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}

		installed, err := a.isPackageInstalled(pkg)
		if err != nil {
			return fmt.Errorf("failed to check installation status of %s: %w", pkg, err)
		}

		if installed {
			a.getLogger().Success("Package %s is installed", pkg)
		} else {
			a.getLogger().ErrorMsg("Package %s is not installed", pkg)
			allInstalled = false
		}
	}

	if !allInstalled {
		return fmt.Errorf("one or more packages are not installed")
	}
	return nil
}

// validatePackagesParallel validates multiple packages concurrently for better performance
func (a *APTInstaller) validatePackagesParallel(packages []string) error {
	type validationResult struct {
		pkg       string
		err       error
		installed bool
	}

	const maxWorkers = 5 // Limit concurrent operations to avoid overwhelming the system
	workers := maxWorkers
	if len(packages) < workers {
		workers = len(packages)
	}

	packagesChan := make(chan string, len(packages))
	resultsChan := make(chan validationResult, len(packages))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pkg := range packagesChan {
				result := validationResult{pkg: pkg}

				// Validate package availability
				if err := a.validatePackageAvailability(pkg); err != nil {
					result.err = fmt.Errorf("package validation failed for '%s': %w", pkg, err)
					resultsChan <- result
					continue
				}

				// Check if already installed
				if installed, err := a.isPackageInstalled(pkg); err == nil {
					result.installed = installed
				}

				resultsChan <- result
			}
		}()
	}

	// Send packages to workers
	for _, pkg := range packages {
		packagesChan <- pkg
	}
	close(packagesChan)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var firstError error
	installedPackages := make(map[string]bool)

	for result := range resultsChan {
		if result.err != nil && firstError == nil {
			firstError = result.err
		}
		if result.installed {
			installedPackages[result.pkg] = true
			a.getLogger().Printf("Package %s is already installed, skipping\n", result.pkg)
		}
	}

	return firstError
}
