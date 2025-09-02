package dnf

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/metrics"
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

	// Start metrics tracking
	timer := metrics.StartInstallation("dnf", command)

	// Run background validation for better performance
	validator := utilities.NewBackgroundValidator(30 * time.Second)
	validator.AddSuite(createDnfFallbackValidationSuite())
	validator.AddSuite(utilities.CreateNetworkValidationSuite())

	ctx := context.Background()
	if err := validator.RunValidations(ctx); err != nil {
		timer.Failure(err)
		return utilities.WrapError(err, utilities.ErrorTypeSystem, "install", command, "dnf")
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
			return utilities.NewPackageError("install-check", command, "dnf", err)
		}
	}

	if isInstalled {
		log.Info("Package already installed, skipping installation", "command", command)
		return nil // Already installed is a success condition
	}

	// Ensure package metadata is up to date if it's stale
	if err := ensurePackageMetadataUpdated(repo); err != nil {
		log.Warn("Failed to update package metadata", "error", err)
		// Continue anyway, as the installation might still work
		// This is not a critical error, just log it
	}

	// Check if package is available in repositories
	if err := validatePackageAvailability(command); err != nil {
		return utilities.NewPackageError("availability-check", command, "dnf", err)
	}

	// Use secure installation method
	if err := secureInstallPackage(ctx, command); err != nil {
		log.Error("Failed to install package", err, "command", command)
		timer.Failure(err)
		return utilities.NewPackageError("install", command, "dnf", err)
	}

	log.Debug("Package installed successfully", "command", command, "package_manager", "dnf")

	// Verify installation succeeded
	if isInstalled, err := d.isPackageInstalled(command); err != nil {
		log.Warn("Failed to verify installation", "error", err, "command", command)
	} else if !isInstalled {
		return utilities.NewPackageError("verification", command, "dnf", utilities.ErrVerificationFailed)
	}

	// Perform post-installation setup for specific packages using registry pattern
	if err := utilities.ExecutePostInstallHandler(command); err != nil {
		log.Warn("Post-installation setup failed", "package", command, "error", err)
		// Don't fail the installation, just warn
	}

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "command", command)
		timer.Failure(err)
		return utilities.NewRepositoryError("add", command, "dnf", err)
	}

	log.Debug("Package added to repository successfully", "command", command)
	timer.Success()
	return nil
}

// Uninstall removes packages using dnf/yum
func (d *DnfInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("DNF Installer: Starting uninstallation", "command", command)

	// Run background validation for system availability
	validator := utilities.NewBackgroundValidator(15 * time.Second) // Shorter timeout for uninstall
	validator.AddSuite(utilities.CreateSystemValidationSuite("dnf"))

	ctx := context.Background()
	if err := validator.RunValidations(ctx); err != nil {
		return utilities.WrapError(err, utilities.ErrorTypeSystem, "uninstall", command, "dnf")
	}

	// Check if the package is installed
	isInstalled, err := d.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return utilities.NewPackageError("uninstall-check", command, "dnf", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "command", command)
		return nil // Not installed is success for uninstall
	}

	// Uninstall package using secure execution with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := secureUninstallPackage(ctx, command); err != nil {
		log.Error("Failed to uninstall package", err, "command", command)
		return utilities.NewPackageError("uninstall", command, "dnf/yum", err)
	}

	log.Debug("Package uninstalled successfully", "command", command, "package_manager", "dnf")

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "command", command)
		return utilities.NewRepositoryError("remove", command, "dnf", err)
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
		// For other errors, wrap with structured error
		return false, utilities.WrapError(err, utilities.ErrorTypePackage, "installation-check", packageName, "rpm")
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
// secureInstallPackage installs a package using DNF or YUM with secure execution
func secureInstallPackage(ctx context.Context, packageName string) error {
	// Validate package name
	if err := utils.ValidatePackageName(packageName); err != nil {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"installer": "dnf",
			"package":   packageName,
			"reason":    "invalid_package_name",
		})
		return fmt.Errorf("invalid package name: %w", err)
	}

	// Check if dnf is available
	if _, err := utils.CommandExec.RunCommand(ctx, "which", "dnf"); err == nil {
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "dnf", "install", "-y", packageName); err != nil {
			return fmt.Errorf("dnf install failed: %w", err)
		}
		return nil
	}

	// Check if yum is available as fallback
	if _, err := utils.CommandExec.RunCommand(ctx, "which", "yum"); err == nil {
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "yum", "install", "-y", packageName); err != nil {
			return fmt.Errorf("yum install failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("neither dnf nor yum package managers found")
}

// getDnfUpdateCommand returns the appropriate update command
func getDnfUpdateCommand() (string, string) {
	// Check if dnf is available
	if _, err := utils.CommandExec.RunShellCommand("which dnf"); err == nil {
		return "sudo dnf check-update", "dnf"
	}
	// Check if yum is available as fallback
	if _, err := utils.CommandExec.RunShellCommand("which yum"); err == nil {
		return "sudo yum check-update", "yum"
	}
	// Neither available - return error case (should not happen if validation passed)
	return "", ""
}

// getDnfUninstallCommand returns the appropriate uninstall command
// secureUninstallPackage uninstalls a package using DNF or YUM with secure execution
func secureUninstallPackage(ctx context.Context, packageName string) error {
	// Validate package name
	if err := utils.ValidatePackageName(packageName); err != nil {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"installer": "dnf",
			"package":   packageName,
			"reason":    "invalid_package_name",
		})
		return fmt.Errorf("invalid package name: %w", err)
	}

	// Check if dnf is available
	if _, err := utils.CommandExec.RunCommand(ctx, "which", "dnf"); err == nil {
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "dnf", "remove", "-y", packageName); err != nil {
			return fmt.Errorf("dnf remove failed: %w", err)
		}
		return nil
	}

	// Check if yum is available as fallback
	if _, err := utils.CommandExec.RunCommand(ctx, "which", "yum"); err == nil {
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "yum", "remove", "-y", packageName); err != nil {
			return fmt.Errorf("yum remove failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("neither dnf nor yum package managers found")
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

	// Use secure group installation method
	ctx := context.Background()
	if err := secureInstallGroup(ctx, groupName); err != nil {
		log.Error("Failed to install package group", err, "group", groupName)
		return fmt.Errorf("failed to install package group via dnf: %w", err)
	}

	log.Info("Package group installed successfully", "group", groupName, "package_manager", "dnf")

	// Add the group to the repository
	if err := repo.AddApp(groupName); err != nil {
		log.Error("Failed to add package group to repository", err, "group", groupName)
		return fmt.Errorf("failed to add package group to repository: %w", err)
	}

	return nil
}

// getDnfGroupInstallCommand returns the appropriate group install command
// secureInstallGroup installs a group using DNF or YUM with secure execution
func secureInstallGroup(ctx context.Context, groupName string) error {
	// Validate group name
	if err := utils.ValidatePackageName(groupName); err != nil {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"installer": "dnf",
			"group":     groupName,
			"reason":    "invalid_group_name",
		})
		return fmt.Errorf("invalid group name: %w", err)
	}

	// Quote group name if it contains spaces for shell safety
	quotedGroupName := groupName
	if strings.Contains(groupName, " ") {
		quotedGroupName = "'" + groupName + "'"
	}

	// Check if dnf is available
	if _, err := utils.CommandExec.RunCommand(ctx, "which", "dnf"); err == nil {
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "dnf", "group", "install", "-y", quotedGroupName); err != nil {
			return fmt.Errorf("dnf group install failed: %w", err)
		}
		return nil
	}

	// Fall back to yum
	if _, err := utils.CommandExec.RunCommand(ctx, "which", "yum"); err == nil {
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "yum", "groupinstall", "-y", quotedGroupName); err != nil {
			return fmt.Errorf("yum groupinstall failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("neither dnf nor yum package managers found")
}

// EnableEPEL enables the Extra Packages for Enterprise Linux repository
func (d *DnfInstaller) EnableEPEL() error {
	log.Debug("Enabling EPEL repository")

	// Check if dnf is available
	ctx := context.Background()
	if _, err := utils.CommandExec.RunCommand(ctx, "which", "dnf"); err == nil {
		// Use DNF with secure execution
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "dnf", "install", "-y", "epel-release"); err != nil {
			return fmt.Errorf("failed to enable EPEL via DNF: %w", err)
		}
	} else {
		// Use YUM
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "yum", "install", "-y", "epel-release"); err != nil {
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

// createDnfFallbackValidationSuite creates validation checks that support DNF/YUM fallback
func createDnfFallbackValidationSuite() utilities.ValidationSuite {
	checks := []utilities.ValidationCheck{
		{
			Name:        "package-manager-available",
			Description: "Check if DNF or YUM is available in PATH",
			Validator:   createDnfOrYumAvailabilityCheck(),
			Timeout:     5 * time.Second,
			Critical:    true,
		},
		{
			Name:        "rpm-available",
			Description: "Check if rpm is available for package verification",
			Validator:   createCommandAvailabilityCheck("rpm"),
			Timeout:     5 * time.Second,
			Critical:    true,
		},
		{
			Name:        "rpm-functional",
			Description: "Check if rpm responds to version command",
			Validator:   createRpmVersionCheck(),
			Timeout:     5 * time.Second,
			Critical:    true,
		},
		{
			Name:        "system-permissions",
			Description: "Check if user has necessary permissions",
			Validator:   createPermissionCheck(),
			Timeout:     5 * time.Second,
			Critical:    false,
		},
		{
			Name:        "disk-space",
			Description: "Check available disk space",
			Validator:   createDiskSpaceCheck(),
			Timeout:     5 * time.Second,
			Critical:    false,
		},
	}

	return utilities.ValidationSuite{
		Name:   "dnf-system",
		Checks: checks,
	}
}

// createDnfOrYumAvailabilityCheck creates a validator that checks if DNF or YUM is available
func createDnfOrYumAvailabilityCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Check if dnf is available
		if _, err := utils.CommandExec.RunCommand(ctx, "which", "dnf"); err == nil {
			return nil // DNF is available
		}

		// Check if yum is available as fallback
		if _, err := utils.CommandExec.RunShellCommand("which yum"); err == nil {
			return nil // YUM is available as fallback
		}

		return fmt.Errorf("neither dnf nor yum found")
	}
}

// createCommandAvailabilityCheck creates a validator that checks if a command is available
func createCommandAvailabilityCheck(command string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		_, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("which %s", command))
		if err != nil {
			return fmt.Errorf("command '%s' not found in PATH", command)
		}
		return nil
	}
}

// createRpmVersionCheck creates a validator that checks if rpm responds to version command
func createRpmVersionCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if _, err := utils.CommandExec.RunShellCommand("rpm --version"); err != nil {
			return fmt.Errorf("command 'rpm' does not respond to version commands")
		}
		return nil
	}
}

// createPermissionCheck creates a validator that checks basic system permissions
func createPermissionCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Test if we can write to /tmp (basic permission check)
		testFile := fmt.Sprintf("/tmp/devex-permission-test-%d", time.Now().UnixNano())
		cmd := fmt.Sprintf("touch %s && rm -f %s", testFile, testFile)

		if _, err := utils.CommandExec.RunShellCommand(cmd); err != nil {
			return fmt.Errorf("insufficient permissions for file operations")
		}

		return nil
	}
}

// createDiskSpaceCheck creates a validator that checks available disk space
func createDiskSpaceCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Check available space in /tmp and /usr/local (common install locations)
		locations := []string{"/tmp", "/usr/local", "/"}

		for _, location := range locations {
			output, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("df -h %s", location))
			if err != nil {
				log.Debug("Could not check disk space", "location", location, "error", err)
				continue
			}

			// Basic check - if df command succeeds, assume space is available
			// More sophisticated parsing could be added here
			if len(output) > 0 {
				log.Debug("Disk space check passed", "location", location)
			}
		}

		return nil
	}
}
