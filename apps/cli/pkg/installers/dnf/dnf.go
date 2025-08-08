package dnf

import (
	"fmt"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type DnfInstaller struct{}

var lastDnfUpdateTime time.Time

func NewDnfInstaller() *DnfInstaller {
	return &DnfInstaller{}
}

func (d *DnfInstaller) Install(command string, repo types.Repository) error {
	log.Debug("DNF Installer: Starting installation", "command", command)

	// Validate DNF system availability
	if err := validateDnfSystem(); err != nil {
		return fmt.Errorf("dnf system validation failed: %w", err)
	}

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "dnf",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		// If utilities doesn't support DNF yet, use our own check
		isInstalled, err = d.isPackageInstalled(command)
		if err != nil {
			log.Error("Failed to check if package is installed", err, "command", command)
			return fmt.Errorf("failed to check if package is installed via dnf: %w", err)
		}
	}

	if isInstalled {
		log.Info("Package already installed, skipping installation", "command", command)
		return nil
	}

	// Ensure package metadata is up to date if it's stale
	if err := ensurePackageMetadataUpdated(repo); err != nil {
		log.Warn("Failed to update package metadata", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Check if package is available in repositories
	if err := validatePackageAvailability(command); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Run dnf/yum install command with automatic fallback
	installCommand, packageManager := getDnfInstallCommand(command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package", err, "command", command, "package_manager", packageManager)
		return fmt.Errorf("failed to install package via %s: %w", packageManager, err)
	}

	log.Debug("Package installed successfully", "command", command, "package_manager", packageManager)

	// Verify installation succeeded
	if isInstalled, err := d.isPackageInstalled(command); err != nil {
		log.Warn("Failed to verify installation", "error", err, "command", command)
	} else if !isInstalled {
		return fmt.Errorf("package installation verification failed for: %s", command)
	}

	// Perform post-installation setup for specific packages using registry pattern
	if err := utilities.ExecutePostInstallHandler(command); err != nil {
		log.Warn("Post-installation setup failed", "package", command, "error", err)
		// Don't fail the installation, just warn
	}

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "command", command)
		return fmt.Errorf("failed to add package to repository: %w", err)
	}

	log.Debug("Package added to repository successfully", "command", command)
	return nil
}

// Uninstall removes packages using dnf/yum
func (d *DnfInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("DNF Installer: Starting uninstallation", "command", command)

	// Validate DNF system availability
	if err := validateDnfSystem(); err != nil {
		return fmt.Errorf("dnf system validation failed: %w", err)
	}

	// Check if the package is installed
	isInstalled, err := d.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "command", command)
		return nil
	}

	// Run dnf/yum remove command
	uninstallCommand, packageManager := getDnfUninstallCommand(command)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall package", err, "command", command, "package_manager", packageManager)
		return fmt.Errorf("failed to uninstall package via %s: %w", packageManager, err)
	}

	log.Debug("Package uninstalled successfully", "command", command, "package_manager", packageManager)

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "command", command)
		return fmt.Errorf("failed to remove package from repository: %w", err)
	}

	log.Debug("Package removed from repository successfully", "command", command)
	return nil
}

// IsInstalled checks if a package is installed using dnf/yum
func (d *DnfInstaller) IsInstalled(command string) (bool, error) {
	return d.isPackageInstalled(command)
}

// isPackageInstalled checks if a package is installed using rpm query
func (d *DnfInstaller) isPackageInstalled(packageName string) (bool, error) {
	// Use rpm to check if package is installed
	command := fmt.Sprintf("rpm -q %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// rpm -q returns non-zero exit code if package is not installed
		if strings.Contains(output, "not installed") || strings.Contains(output, "is not installed") {
			return false, nil
		}
		// For other errors, return the error
		return false, fmt.Errorf("failed to check package installation status: %w", err)
	}

	// If rpm -q succeeds, package is installed
	return true, nil
}

func RunDnfUpdate(forceUpdate bool, repo types.Repository) error {
	log.Debug("Starting DNF metadata update", "forceUpdate", forceUpdate)

	// Check if update is required
	if !forceUpdate && time.Since(lastDnfUpdateTime) < 24*time.Hour {
		log.Debug("DNF update skipped (cached)")
		return nil
	}

	// Execute dnf/yum check-update to refresh metadata
	updateCommand, packageManager := getDnfUpdateCommand()
	if _, err := utils.CommandExec.RunShellCommand(updateCommand); err != nil {
		// dnf/yum check-update returns exit code 100 when updates are available, which is not an error
		if !strings.Contains(err.Error(), "exit status 100") {
			log.Error("Failed to execute metadata refresh", err, "command", updateCommand, "package_manager", packageManager)
			return fmt.Errorf("failed to execute %s metadata refresh: %w", packageManager, err)
		}
	}

	// Update the last update time cache
	lastDnfUpdateTime = time.Now()
	if err := repo.Set("last_dnf_update", lastDnfUpdateTime.Format(time.RFC3339)); err != nil {
		log.Warn("Failed to store last update time in repository", err)
	}

	log.Debug("DNF metadata update completed successfully")
	return nil
}

// getDnfInstallCommand returns the appropriate install command and package manager name
func getDnfInstallCommand(packageName string) (string, string) {
	// Check if dnf is available
	if _, err := utils.CommandExec.RunShellCommand("which dnf"); err == nil {
		return fmt.Sprintf("sudo dnf install -y %s", packageName), "dnf"
	}
	// Fall back to yum
	return fmt.Sprintf("sudo yum install -y %s", packageName), "yum"
}

// getDnfUpdateCommand returns the appropriate update command
func getDnfUpdateCommand() (string, string) {
	// Check if dnf is available
	if _, err := utils.CommandExec.RunShellCommand("which dnf"); err == nil {
		return "sudo dnf check-update", "dnf"
	}
	// Fall back to yum
	return "sudo yum check-update", "yum"
}

// getDnfUninstallCommand returns the appropriate uninstall command
func getDnfUninstallCommand(packageName string) (string, string) {
	// Check if dnf is available
	if _, err := utils.CommandExec.RunShellCommand("which dnf"); err == nil {
		return fmt.Sprintf("sudo dnf remove -y %s", packageName), "dnf"
	}
	// Fall back to yum
	return fmt.Sprintf("sudo yum remove -y %s", packageName), "yum"
}

// validateDnfSystem checks if DNF is available and functional
func validateDnfSystem() error {
	// Check if dnf is available
	if _, err := utils.CommandExec.RunShellCommand("which dnf"); err != nil {
		// Try YUM as fallback for older RHEL/CentOS systems
		if _, err := utils.CommandExec.RunShellCommand("which yum"); err != nil {
			return fmt.Errorf("neither dnf nor yum found: %w", err)
		}
		log.Info("DNF not found, but YUM is available - will use YUM as fallback")
		return nil
	}

	// Check if rpm is available (needed for checking installation status)
	if _, err := utils.CommandExec.RunShellCommand("which rpm"); err != nil {
		return fmt.Errorf("rpm not found: %w", err)
	}

	// Check if we can access the rpm database
	if _, err := utils.CommandExec.RunShellCommand("rpm --version"); err != nil {
		return fmt.Errorf("rpm not functional: %w", err)
	}

	return nil
}

// ensurePackageMetadataUpdated updates package metadata if it's stale
func ensurePackageMetadataUpdated(repo types.Repository) error {
	// Check if we need to update (more than 6 hours old)
	if time.Since(lastDnfUpdateTime) > 6*time.Hour {
		log.Debug("Package metadata is stale, updating")
		return RunDnfUpdate(false, repo)
	}
	return nil
}

// validatePackageAvailability checks if a package is available in repositories
func validatePackageAvailability(packageName string) error {
	// Use dnf info to check if package is available
	command := fmt.Sprintf("dnf info %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// If dnf is not available, try yum
		command = fmt.Sprintf("yum info %s", packageName)
		output, err = utils.CommandExec.RunShellCommand(command)
		if err != nil {
			return fmt.Errorf("failed to check package availability: %w", err)
		}
	}

	// Check if the output indicates the package is available
	if strings.Contains(output, "No matching packages") || strings.Contains(output, "No package") {
		return fmt.Errorf("package '%s' not found in any repository", packageName)
	}

	// Check if package information is available
	if !strings.Contains(output, "Name") && !strings.Contains(output, "Available Packages") {
		return fmt.Errorf("no package information found for '%s'", packageName)
	}

	log.Debug("Package availability validated", "package", packageName)
	return nil
}

// Post-installation setup is now handled by the registry pattern in utilities.ExecutePostInstallHandler()
// This provides a more flexible and maintainable approach for package-specific configuration.
// See pkg/installers/utilities/handlers.go for handler implementations.

// Repository Management Functions

// AddRepository adds a new YUM/DNF repository
func (d *DnfInstaller) AddRepository(name, baseurl, gpgkey string) error {
	log.Debug("Adding repository", "name", name, "baseurl", baseurl)

	// Create repository configuration
	repoConfig := fmt.Sprintf(`[%s]
name=%s
baseurl=%s
enabled=1
gpgcheck=1
gpgkey=%s
`, name, name, baseurl, gpgkey)

	// Validate inputs to prevent directory traversal and injection
	if err := validateRepositoryInputs(name, baseurl, gpgkey); err != nil {
		return fmt.Errorf("invalid repository parameters: %w", err)
	}

	// Write repository file safely using temporary file approach
	systemPaths := utilities.GetSystemPaths()
	repoFile := systemPaths.GetRepositoryFilePath("dnf", name)
	if err := writeRepositoryFile(repoFile, repoConfig); err != nil {
		return fmt.Errorf("failed to create repository file: %w", err)
	}

	log.Info("Repository added successfully", "name", name, "file", repoFile)
	return nil
}

// InstallGroup installs a DNF/YUM package group
func (d *DnfInstaller) InstallGroup(groupName string, repo types.Repository) error {
	log.Debug("Installing package group", "group", groupName)

	// Validate system first
	if err := validateDnfSystem(); err != nil {
		return fmt.Errorf("dnf system validation failed: %w", err)
	}

	// Get the appropriate command for group installation
	installCommand, packageManager := getDnfGroupInstallCommand(groupName)

	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package group", err, "group", groupName, "package_manager", packageManager)
		return fmt.Errorf("failed to install package group via %s: %w", packageManager, err)
	}

	log.Info("Package group installed successfully", "group", groupName, "package_manager", packageManager)

	// Add the group to the repository
	if err := repo.AddApp(groupName); err != nil {
		log.Error("Failed to add package group to repository", err, "group", groupName)
		return fmt.Errorf("failed to add package group to repository: %w", err)
	}

	return nil
}

// getDnfGroupInstallCommand returns the appropriate group install command
func getDnfGroupInstallCommand(groupName string) (string, string) {
	// Check if dnf is available
	if _, err := utils.CommandExec.RunShellCommand("which dnf"); err == nil {
		return fmt.Sprintf("sudo dnf group install -y '%s'", groupName), "dnf"
	}
	// Fall back to yum
	return fmt.Sprintf("sudo yum groupinstall -y '%s'", groupName), "yum"
}

// EnableEPEL enables the Extra Packages for Enterprise Linux repository
func (d *DnfInstaller) EnableEPEL() error {
	log.Debug("Enabling EPEL repository")

	// Check if dnf is available
	if _, err := utils.CommandExec.RunShellCommand("which dnf"); err == nil {
		// Use DNF
		if _, err := utils.CommandExec.RunShellCommand("sudo dnf install -y epel-release"); err != nil {
			return fmt.Errorf("failed to enable EPEL via DNF: %w", err)
		}
	} else {
		// Use YUM
		if _, err := utils.CommandExec.RunShellCommand("sudo yum install -y epel-release"); err != nil {
			return fmt.Errorf("failed to enable EPEL via YUM: %w", err)
		}
	}

	log.Info("EPEL repository enabled successfully")
	return nil
}

// validateRepositoryInputs validates repository parameters to prevent injection attacks
func validateRepositoryInputs(name, baseurl, gpgkey string) error {
	// Validate repository name - only alphanumeric, dash, underscore
	if !isValidRepositoryName(name) {
		return fmt.Errorf("invalid repository name: must contain only letters, numbers, dashes, and underscores")
	}

	// Validate URLs
	if !isValidURL(baseurl) {
		return fmt.Errorf("invalid baseurl: must be a valid HTTP/HTTPS URL")
	}

	if !isValidURL(gpgkey) {
		return fmt.Errorf("invalid gpgkey: must be a valid HTTP/HTTPS URL")
	}

	return nil
}

// isValidRepositoryName checks if repository name is safe for filesystem
func isValidRepositoryName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}

	for _, char := range name {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' {
			return false
		}
	}
	return true
}

// isValidURL checks if URL is valid and uses safe schemes
func isValidURL(urlStr string) bool {
	if len(urlStr) == 0 || len(urlStr) > 2048 {
		return false
	}

	// Must start with http:// or https://
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return false
	}

	// Basic validation - no shell metacharacters
	dangerousChars := []string{"'", "\"", "`", "$", ";", "&", "|", "(", ")", "<", ">", "\n", "\r", "\t"}
	for _, char := range dangerousChars {
		if strings.Contains(urlStr, char) {
			return false
		}
	}

	return true
}

// writeRepositoryFile safely writes repository configuration to file
func writeRepositoryFile(repoFile, repoConfig string) error {
	// Use a more secure approach with temporary file creation
	tempFile := fmt.Sprintf("/tmp/repo-config-%d.tmp", time.Now().UnixNano())

	// Create temp file with config content
	writeCmd := fmt.Sprintf("printf %%s %s > %s",
		escapeShellArg(repoConfig),
		escapeShellArg(tempFile))

	if _, err := utils.CommandExec.RunShellCommand(writeCmd); err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Move temp file to final location with sudo
	moveCmd := fmt.Sprintf("sudo mv %s %s",
		escapeShellArg(tempFile),
		escapeShellArg(repoFile))

	if _, err := utils.CommandExec.RunShellCommand(moveCmd); err != nil {
		// Clean up temp file on error
		_, _ = utils.CommandExec.RunShellCommand(fmt.Sprintf("rm -f %s", escapeShellArg(tempFile)))
		return fmt.Errorf("failed to move repository file: %w", err)
	}

	// Set appropriate permissions
	chmodCmd := fmt.Sprintf("sudo chmod 644 %s", escapeShellArg(repoFile))
	if _, err := utils.CommandExec.RunShellCommand(chmodCmd); err != nil {
		log.Warn("Failed to set repository file permissions", "file", repoFile, "error", err)
		// Not a fatal error, continue
	}

	return nil
}

// escapeShellArg safely escapes shell arguments
func escapeShellArg(arg string) string {
	// Use single quotes and escape any single quotes in the argument
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}
