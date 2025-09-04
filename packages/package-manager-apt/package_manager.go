package main

import (
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

// Execute handles command execution
func (a *APTInstaller) Execute(command string, args []string) error {
	// Ensure APT is available
	a.EnsureAvailable()

	switch command {
	case "install":
		return a.handleInstall(args)
	case "remove":
		return a.handleRemove(args)
	case "update":
		return a.handleUpdate(args)
	case "upgrade":
		return a.handleUpgrade(args)
	case "search":
		return a.handleSearch(args)
	case "list":
		return a.handleList(args)
	case "info":
		return a.handleInfo(args)
	case "is-installed":
		return a.handleIsInstalled(args)
	case "add-repository":
		return a.handleAddRepository(args)
	case "remove-repository":
		return a.handleRemoveRepository(args)
	case "validate-repository":
		return a.handleValidateRepository(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// getAPTVersion detects the APT version with caching
func (a *APTInstaller) getAPTVersion() (*APTVersion, error) {
	if a.versionCached && a.aptVersion != nil {
		return a.aptVersion, nil
	}

	// Try apt --version first (available in APT 1.0+)
	output, err := sdk.ExecCommandOutput("apt", "--version")
	if err != nil {
		// Fallback to apt-get --version
		output, err = sdk.ExecCommandOutput("apt-get", "--version")
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

	a.logger.Debug("Detected APT version", "version", fmt.Sprintf("%d.%d.%d", major, minor, patch))
	return a.aptVersion, nil
}

// getAPTCommand returns the appropriate APT command based on version
func (a *APTInstaller) getAPTCommand() string {
	if version, err := a.getAPTVersion(); err == nil && version.Major < 1 {
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
	output, err := sdk.ExecCommandOutput("dpkg-query", "-W", "-f=${Status}", packageName)
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
	output, err := sdk.ExecCommandOutput("apt-cache", "policy", packageName)
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

	a.logger.Debug("Package availability validated", "package", packageName)
	return nil
}

// handleInstall installs packages
func (a *APTInstaller) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	a.logger.Printf("Installing packages: %s\\n", strings.Join(args, ", "))

	// Validate all package names first
	for _, pkg := range args {
		if err := a.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}
	}

	// Update package lists first
	a.logger.Println("Updating package lists...")
	if err := sdk.ExecCommand(true, "apt", "update"); err != nil {
		a.logger.Warning("Failed to update package lists: %v", err)
	}

	// Check availability of packages in parallel
	if err := a.validatePackagesParallel(args); err != nil {
		return err
	}

	// Install packages
	aptCmd := a.getAPTCommand()
	cmdArgs := append([]string{"install", "-y"}, args...)
	if err := sdk.ExecCommand(true, aptCmd, cmdArgs...); err != nil {
		return fmt.Errorf("failed to install packages [%s]: %w", strings.Join(args, ", "), err)
	}

	// Verify installation
	for _, pkg := range args {
		if installed, err := a.isPackageInstalled(pkg); err != nil {
			a.logger.Warning("Failed to verify installation of %s: %v", pkg, err)
		} else if !installed {
			return fmt.Errorf("installation verification failed for package: %s", pkg)
		}
	}

	a.logger.Success("Successfully installed packages: %s", strings.Join(args, ", "))
	return nil
}

// handleRemove removes packages
func (a *APTInstaller) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	a.logger.Printf("Removing packages: %s\\n", strings.Join(args, ", "))

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
			a.logger.Warning("Failed to check installation status of %s: %v", pkg, err)
			packagesToRemove = append(packagesToRemove, pkg) // Include anyway
		} else if installed {
			packagesToRemove = append(packagesToRemove, pkg)
		} else {
			a.logger.Printf("Package %s is not installed, skipping\\n", pkg)
		}
	}

	if len(packagesToRemove) == 0 {
		a.logger.Println("No packages to remove")
		return nil
	}

	// Remove packages
	aptCmd := a.getAPTCommand()
	cmdArgs := append([]string{"remove", "-y"}, packagesToRemove...)
	if err := sdk.ExecCommand(true, aptCmd, cmdArgs...); err != nil {
		return fmt.Errorf("failed to remove packages [%s]: %w", strings.Join(args, ", "), err)
	}

	a.logger.Success("Successfully removed packages: %s", strings.Join(packagesToRemove, ", "))
	return nil
}

// handleUpdate updates package lists
func (a *APTInstaller) handleUpdate(args []string) error {
	a.logger.Println("Updating package lists...")
	if err := sdk.ExecCommand(true, "apt", "update"); err != nil {
		return fmt.Errorf("failed to update package lists: %w", err)
	}
	a.logger.Success("Package lists updated successfully")
	return nil
}

// handleUpgrade upgrades installed packages
func (a *APTInstaller) handleUpgrade(args []string) error {
	a.logger.Println("Upgrading installed packages...")

	// Update first
	if err := sdk.ExecCommand(true, "apt", "update"); err != nil {
		return fmt.Errorf("failed to update package lists: %w", err)
	}

	// Then upgrade
	if err := sdk.ExecCommand(true, "apt", "upgrade", "-y"); err != nil {
		return fmt.Errorf("failed to upgrade packages: %w", err)
	}

	a.logger.Success("Packages upgraded successfully")
	return nil
}

// handleSearch searches for packages
func (a *APTInstaller) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	a.logger.Printf("Searching for: %s\\n", searchTerm)

	return sdk.ExecCommand(false, "apt", "search", searchTerm)
}

// handleList lists packages
func (a *APTInstaller) handleList(args []string) error {
	if len(args) == 0 {
		// List all installed packages
		return sdk.ExecCommand(false, "apt", "list", "--installed")
	}

	// Handle flags or search terms
	cmdArgs := append([]string{"list"}, args...)
	return sdk.ExecCommand(false, "apt", cmdArgs...)
}

// handleInfo shows package information
func (a *APTInstaller) handleInfo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package specified")
	}

	for i, pkg := range args {
		if err := a.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}

		a.logger.Printf("Package information for: %s\\n", pkg)
		if err := sdk.ExecCommand(false, "apt", "show", pkg); err != nil {
			a.logger.ErrorMsg("Failed to get info for %s: %v", pkg, err)
		}

		if i < len(args)-1 {
			fmt.Println("---")
		}
	}

	return nil
}

// handleIsInstalled checks if packages are installed
func (a *APTInstaller) handleIsInstalled(args []string) error {
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
			a.logger.Success("Package %s is installed", pkg)
		} else {
			a.logger.ErrorMsg("Package %s is not installed", pkg)
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
			a.logger.Printf("Package %s is already installed, skipping\n", result.pkg)
		}
	}

	return firstError
}
