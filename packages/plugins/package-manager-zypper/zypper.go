package zypper

import (
	"fmt"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// ZypperInstaller implements the BaseInstaller interface for Zypper (SUSE)
type ZypperInstaller struct{}

var lastZypperRefreshTime time.Time

func getCurrentUser() string {
	return utilities.GetCurrentUser()
}

// NewZypperInstaller creates a new ZypperInstaller instance
func NewZypperInstaller() *ZypperInstaller {
	return &ZypperInstaller{}
}

// Install installs packages using zypper (implements BaseInstaller interface)
func (z *ZypperInstaller) Install(command string, repo types.Repository) error {
	log.Debug("Zypper Installer: Starting installation", "command", command)

	// Validate Zypper system availability
	if err := validateZypperSystem(); err != nil {
		return fmt.Errorf("zypper system validation failed: %w", err)
	}

	// Check if the package is already installed
	isInstalled, err := z.isPackageInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed via zypper: %w", err)
	}

	if isInstalled {
		log.Info("Package already installed, skipping installation", "command", command)
		return nil
	}

	// Ensure repository metadata is up to date if it's stale
	if err := ensureRepositoryRefreshed(repo); err != nil {
		log.Warn("Failed to refresh repository metadata", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Check if package is available in repositories
	if err := validatePackageAvailability(command); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Determine install command based on package type
	installCommand := getInstallCommand(command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package via zypper", err, "command", command)
		return fmt.Errorf("failed to install package via zypper: %w", err)
	}

	log.Debug("Zypper package installed successfully", "command", command)

	// Verify installation succeeded
	if isInstalled, err := z.isPackageInstalled(command); err != nil {
		log.Warn("Failed to verify installation", "error", err, "command", command)
	} else if !isInstalled {
		return fmt.Errorf("package installation verification failed for: %s", command)
	}

	// Perform post-installation setup for specific packages
	if err := performPostInstallationSetup(command); err != nil {
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

// Uninstall removes packages using zypper with dependency checking
func (z *ZypperInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("Zypper Installer: Starting uninstallation", "command", command)

	// Validate Zypper system availability
	if err := validateZypperSystem(); err != nil {
		return fmt.Errorf("zypper system validation failed: %w", err)
	}

	// Check if the package is installed
	isInstalled, err := z.isPackageInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "command", command)
		return nil
	}

	// Check for dependencies
	dependents, err := z.GetDependents(command)
	if err != nil {
		log.Warn("Failed to check package dependents", "error", err)
	} else if len(dependents) > 0 {
		log.Warn("Package has dependents that may be affected", "package", command, "dependents", dependents)
	}

	// Run zypper remove command with --clean-deps to remove unneeded dependencies
	uninstallCommand := fmt.Sprintf("sudo zypper remove --non-interactive --clean-deps %s", command)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall package via zypper", err, "command", command)
		return fmt.Errorf("failed to uninstall package via zypper: %w", err)
	}

	log.Debug("Zypper package uninstalled successfully", "command", command)

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "command", command)
		return fmt.Errorf("failed to remove package from repository: %w", err)
	}

	log.Debug("Package removed from repository successfully", "command", command)
	return nil
}

// IsInstalled checks if a package is installed using zypper
func (z *ZypperInstaller) IsInstalled(command string) (bool, error) {
	return z.isPackageInstalled(command)
}

// isPackageInstalled checks if a package is installed using rpm query (zypper uses RPM database)
func (z *ZypperInstaller) isPackageInstalled(packageName string) (bool, error) {
	// Use rpm to check if package is installed (zypper uses RPM database)
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

func RunZypperRefresh(forceRefresh bool, repo types.Repository) error {
	log.Debug("Starting Zypper repository refresh", "forceRefresh", forceRefresh)

	// Check if refresh is required
	if !forceRefresh && time.Since(lastZypperRefreshTime) < 24*time.Hour {
		log.Debug("Zypper refresh skipped (cached)")
		return nil
	}

	// Execute zypper refresh to update repository metadata
	refreshCommand := "sudo zypper refresh --non-interactive"
	if _, err := utils.CommandExec.RunShellCommand(refreshCommand); err != nil {
		log.Error("Failed to execute Zypper refresh", err, "command", refreshCommand)
		return fmt.Errorf("failed to execute Zypper refresh: %w", err)
	}

	// Update the last refresh time cache
	lastZypperRefreshTime = time.Now()
	if err := repo.Set("last_zypper_refresh", lastZypperRefreshTime.Format(time.RFC3339)); err != nil {
		log.Warn("Failed to store last refresh time in repository", err)
	}

	log.Debug("Zypper refresh completed successfully")
	return nil
}

// validateZypperSystem checks if Zypper is available and functional
func validateZypperSystem() error {
	// Check if zypper is available
	if _, err := utils.CommandExec.RunShellCommand("which zypper"); err != nil {
		return fmt.Errorf("zypper not found: %w", err)
	}

	// Check if we can access the RPM database (used by zypper)
	if _, err := utils.CommandExec.RunShellCommand("rpm --version"); err != nil {
		return fmt.Errorf("rpm not functional: %w", err)
	}

	return nil
}

// ensureRepositoryRefreshed updates repository metadata if it's stale
func ensureRepositoryRefreshed(repo types.Repository) error {
	// Check if we need to refresh (more than 6 hours old)
	if time.Since(lastZypperRefreshTime) > 6*time.Hour {
		log.Debug("Repository metadata is stale, refreshing")
		return RunZypperRefresh(false, repo)
	}
	return nil
}

// validatePackageAvailability checks if a package is available in repositories
func validatePackageAvailability(packageName string) error {
	// Use zypper info to check if package is available
	command := fmt.Sprintf("zypper info --non-interactive %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		return fmt.Errorf("failed to check package availability: %w", err)
	}

	// Check if the output indicates the package is available
	if strings.Contains(output, "package") && strings.Contains(output, "not found") {
		return fmt.Errorf("package '%s' not found in any repository", packageName)
	}

	if strings.Contains(output, "No provider of") {
		return fmt.Errorf("no provider found for package '%s'", packageName)
	}

	log.Debug("Package availability validated", "package", packageName)
	return nil
}

// getInstallCommand returns the appropriate install command based on package type
func getInstallCommand(packageName string) string {
	// Check if it's a pattern installation
	if strings.HasPrefix(packageName, "-t pattern") || strings.HasPrefix(packageName, "pattern:") {
		if strings.HasPrefix(packageName, "pattern:") {
			// Convert pattern:name to -t pattern name
			patternName := strings.TrimPrefix(packageName, "pattern:")
			return fmt.Sprintf("sudo zypper install --non-interactive -t pattern %s", patternName)
		}
		// Already in -t pattern format
		return fmt.Sprintf("sudo zypper install --non-interactive %s", packageName)
	}

	// Check if it's a product installation
	if strings.HasPrefix(packageName, "-t product") || strings.HasPrefix(packageName, "product:") {
		if strings.HasPrefix(packageName, "product:") {
			// Convert product:name to -t product name
			productName := strings.TrimPrefix(packageName, "product:")
			return fmt.Sprintf("sudo zypper install --non-interactive -t product %s", productName)
		}
		// Already in -t product format
		return fmt.Sprintf("sudo zypper install --non-interactive %s", packageName)
	}

	// Default package installation
	return fmt.Sprintf("sudo zypper install --non-interactive %s", packageName)
}

// performPostInstallationSetup handles package-specific post-installation configuration
func performPostInstallationSetup(packageName string) error {
	switch packageName {
	case "docker":
		return setupDockerService()
	case "nginx":
		return setupNginxService()
	case "postgresql", "postgresql-server":
		return setupPostgreSQLService()
	case "redis":
		return setupRedisService()
	case "apache2", "httpd":
		return setupApacheService()
	default:
		// No special setup required
		return nil
	}
}

// setupDockerService configures Docker service and user permissions
func setupDockerService() error {
	log.Debug("Configuring Docker service and permissions")

	// Enable Docker service to start on boot
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl enable docker"); err != nil {
		log.Warn("Failed to enable Docker service", "error", err)
	} else {
		log.Info("Docker service enabled for automatic startup")
	}

	// Start Docker service
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl start docker"); err != nil {
		log.Warn("Failed to start Docker service", "error", err)
	} else {
		log.Info("Docker service started successfully")
	}

	// Add current user to docker group
	currentUser := getCurrentUser()
	if currentUser == "" {
		log.Warn("Unable to determine current user, skipping docker group addition")
		return nil
	}

	addUserCmd := fmt.Sprintf("sudo usermod -aG docker %s", currentUser)
	if _, err := utils.CommandExec.RunShellCommand(addUserCmd); err != nil {
		log.Warn("Failed to add user to docker group", "user", currentUser, "error", err)
	} else {
		log.Info("User added to docker group", "user", currentUser)
		log.Info("Note: You may need to log out and log back in for docker group changes to take effect")
	}

	// Wait a moment for service to fully start
	time.Sleep(2 * time.Second)

	// Verify Docker daemon is accessible
	if _, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Server.Version}}'"); err == nil {
		log.Info("Docker daemon is running and accessible")
	} else {
		log.Warn("Docker daemon may not be fully ready yet", "hint", "Try running 'sudo systemctl status docker' to check service status")
	}

	return nil
}

// setupNginxService configures Nginx service
func setupNginxService() error {
	log.Debug("Configuring Nginx service")

	// Enable Nginx service to start on boot
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl enable nginx"); err != nil {
		log.Warn("Failed to enable Nginx service", "error", err)
		return err
	}

	// Start Nginx service
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl start nginx"); err != nil {
		log.Warn("Failed to start Nginx service", "error", err)
		return err
	}

	log.Info("Nginx service configured and started successfully")
	return nil
}

// setupPostgreSQLService configures PostgreSQL service
func setupPostgreSQLService() error {
	log.Debug("Configuring PostgreSQL service")

	// Initialize PostgreSQL database cluster (SUSE-specific path)
	if _, err := utils.CommandExec.RunShellCommand("sudo -u postgres initdb -D /var/lib/pgsql/data"); err != nil {
		log.Warn("Failed to initialize PostgreSQL database", "error", err)
		// Continue anyway, might be already initialized
	}

	// Enable PostgreSQL service to start on boot
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl enable postgresql"); err != nil {
		log.Warn("Failed to enable PostgreSQL service", "error", err)
		return err
	}

	// Start PostgreSQL service
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl start postgresql"); err != nil {
		log.Warn("Failed to start PostgreSQL service", "error", err)
		return err
	}

	log.Info("PostgreSQL service configured and started successfully")
	return nil
}

// setupRedisService configures Redis service
func setupRedisService() error {
	log.Debug("Configuring Redis service")

	// Enable Redis service to start on boot
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl enable redis"); err != nil {
		log.Warn("Failed to enable Redis service", "error", err)
		return err
	}

	// Start Redis service
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl start redis"); err != nil {
		log.Warn("Failed to start Redis service", "error", err)
		return err
	}

	log.Info("Redis service configured and started successfully")
	return nil
}

// setupApacheService configures Apache HTTP Server service
func setupApacheService() error {
	log.Debug("Configuring Apache HTTP Server service")

	// Enable Apache service to start on boot (SUSE uses apache2)
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl enable apache2"); err != nil {
		log.Warn("Failed to enable Apache service", "error", err)
		return err
	}

	// Start Apache service
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl start apache2"); err != nil {
		log.Warn("Failed to start Apache service", "error", err)
		return err
	}

	log.Info("Apache HTTP Server service configured and started successfully")
	return nil
}

// SUSE-Specific Advanced Features

// InstallPattern installs a SUSE software pattern
func (z *ZypperInstaller) InstallPattern(patternName string, repo types.Repository) error {
	log.Debug("Installing SUSE pattern", "pattern", patternName)

	// Validate system first
	if err := validateZypperSystem(); err != nil {
		return fmt.Errorf("zypper system validation failed: %w", err)
	}

	// Install pattern using zypper
	installCommand := fmt.Sprintf("sudo zypper install --non-interactive -t pattern %s", patternName)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install pattern", err, "pattern", patternName)
		return fmt.Errorf("failed to install pattern via zypper: %w", err)
	}

	log.Info("Pattern installed successfully", "pattern", patternName)

	// Add the pattern to the repository
	if err := repo.AddApp(fmt.Sprintf("pattern:%s", patternName)); err != nil {
		log.Error("Failed to add pattern to repository", err, "pattern", patternName)
		return fmt.Errorf("failed to add pattern to repository: %w", err)
	}

	return nil
}

// InstallProduct installs a SUSE product
func (z *ZypperInstaller) InstallProduct(productName string, repo types.Repository) error {
	log.Debug("Installing SUSE product", "product", productName)

	// Validate system first
	if err := validateZypperSystem(); err != nil {
		return fmt.Errorf("zypper system validation failed: %w", err)
	}

	// Install product using zypper
	installCommand := fmt.Sprintf("sudo zypper install --non-interactive -t product %s", productName)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install product", err, "product", productName)
		return fmt.Errorf("failed to install product via zypper: %w", err)
	}

	log.Info("Product installed successfully", "product", productName)

	// Add the product to the repository
	if err := repo.AddApp(fmt.Sprintf("product:%s", productName)); err != nil {
		log.Error("Failed to add product to repository", err, "product", productName)
		return fmt.Errorf("failed to add product to repository: %w", err)
	}

	return nil
}

// SystemUpdate performs a full system update using zypper
func (z *ZypperInstaller) SystemUpdate() error {
	log.Debug("Performing system update")

	// Validate system first
	if err := validateZypperSystem(); err != nil {
		return fmt.Errorf("zypper system validation failed: %w", err)
	}

	// Perform full system update
	updateCommand := "sudo zypper update --non-interactive"
	if _, err := utils.CommandExec.RunShellCommand(updateCommand); err != nil {
		log.Error("Failed to perform system update", err)
		return fmt.Errorf("failed to perform system update: %w", err)
	}

	log.Info("System update completed successfully")
	return nil
}

// SystemUpgrade performs a distribution upgrade (for openSUSE)
func (z *ZypperInstaller) SystemUpgrade() error {
	log.Debug("Performing distribution upgrade")

	// Validate system first
	if err := validateZypperSystem(); err != nil {
		return fmt.Errorf("zypper system validation failed: %w", err)
	}

	// Perform distribution upgrade (for Tumbleweed)
	upgradeCommand := "sudo zypper dup --non-interactive"
	if _, err := utils.CommandExec.RunShellCommand(upgradeCommand); err != nil {
		log.Error("Failed to perform distribution upgrade", err)
		return fmt.Errorf("failed to perform distribution upgrade: %w", err)
	}

	log.Info("Distribution upgrade completed successfully")
	return nil
}

// CleanCache cleans the zypper cache
func (z *ZypperInstaller) CleanCache() error {
	log.Debug("Cleaning package cache")

	// Clean package cache
	cleanCommand := "sudo zypper clean --all"
	if _, err := utils.CommandExec.RunShellCommand(cleanCommand); err != nil {
		log.Error("Failed to clean package cache", err)
		return fmt.Errorf("failed to clean package cache: %w", err)
	}

	log.Info("Package cache cleaned successfully")
	return nil
}

// ListInstalled lists all installed packages
func (z *ZypperInstaller) ListInstalled() ([]string, error) {
	log.Debug("Listing installed packages")

	// List all installed packages
	listCommand := "zypper search --installed-only --type package"
	output, err := utils.CommandExec.RunShellCommand(listCommand)
	if err != nil {
		log.Error("Failed to list installed packages", err)
		return nil, fmt.Errorf("failed to list installed packages: %w", err)
	}

	// Parse the output to extract package names
	var packages []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(line, "i") {
			// Extract package name from zypper search output
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				packageName := parts[2] // Third column is package name
				packages = append(packages, packageName)
			}
		}
	}

	log.Debug("Listed installed packages", "count", len(packages))
	return packages, nil
}

// SearchPackages searches for packages in repositories
func (z *ZypperInstaller) SearchPackages(query string) ([]string, error) {
	log.Debug("Searching for packages", "query", query)

	var packages []string

	// Search packages in repositories
	searchCommand := fmt.Sprintf("zypper search --type package %s", query)
	output, err := utils.CommandExec.RunShellCommand(searchCommand)
	if err == nil {
		// Parse zypper search results
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && (strings.HasPrefix(line, "v") || strings.HasPrefix(line, " ")) {
				// This is a package line
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					packageName := parts[2] // Third column is package name
					packages = append(packages, fmt.Sprintf("[package] %s", packageName))
				}
			}
		}
	}

	// Search patterns
	patternSearchCommand := fmt.Sprintf("zypper search --type pattern %s", query)
	patternOutput, err := utils.CommandExec.RunShellCommand(patternSearchCommand)
	if err == nil {
		// Parse pattern search results
		lines := strings.Split(patternOutput, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && (strings.HasPrefix(line, "v") || strings.HasPrefix(line, " ")) {
				// This is a pattern line
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					patternName := parts[2] // Third column is pattern name
					packages = append(packages, fmt.Sprintf("[pattern] %s", patternName))
				}
			}
		}
	}

	log.Debug("Found packages matching query", "query", query, "count", len(packages))
	return packages, nil
}

// Repository Management Functions

// AddRepository adds a new zypper repository
func (z *ZypperInstaller) AddRepository(name, url, alias string) error {
	log.Debug("Adding repository", "name", name, "url", url, "alias", alias)

	// Validate inputs to prevent injection
	if err := validateRepositoryInputs(name, url, alias); err != nil {
		return fmt.Errorf("invalid repository parameters: %w", err)
	}

	// Add repository using zypper
	addRepoCommand := fmt.Sprintf("sudo zypper addrepo --refresh %s %s", url, alias)
	if _, err := utils.CommandExec.RunShellCommand(addRepoCommand); err != nil {
		log.Error("Failed to add repository", err, "name", name, "url", url)
		return fmt.Errorf("failed to add repository: %w", err)
	}

	log.Info("Repository added successfully", "name", name, "alias", alias)
	return nil
}

// RemoveRepository removes a zypper repository
func (z *ZypperInstaller) RemoveRepository(alias string) error {
	log.Debug("Removing repository", "alias", alias)

	// Remove repository using zypper
	removeRepoCommand := fmt.Sprintf("sudo zypper removerepo %s", alias)
	if _, err := utils.CommandExec.RunShellCommand(removeRepoCommand); err != nil {
		log.Error("Failed to remove repository", err, "alias", alias)
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	log.Info("Repository removed successfully", "alias", alias)
	return nil
}

// AddGPGKey imports a GPG key for repository verification
func (z *ZypperInstaller) AddGPGKey(keyURL string) error {
	log.Debug("Adding GPG key", "keyURL", keyURL)

	// Validate key URL
	if !isValidURL(keyURL) {
		return fmt.Errorf("invalid GPG key URL: %s", keyURL)
	}

	// Download and import GPG key
	addKeyCommand := fmt.Sprintf("sudo rpm --import %s", keyURL)
	if _, err := utils.CommandExec.RunShellCommand(addKeyCommand); err != nil {
		log.Error("Failed to add GPG key", err, "keyURL", keyURL)
		return fmt.Errorf("failed to add GPG key: %w", err)
	}

	log.Info("GPG key added successfully", "keyURL", keyURL)
	return nil
}

// LockPackage locks a package to prevent updates
func (z *ZypperInstaller) LockPackage(packageName string) error {
	log.Debug("Locking package", "package", packageName)

	// Lock package using zypper
	lockCommand := fmt.Sprintf("sudo zypper addlock %s", packageName)
	if _, err := utils.CommandExec.RunShellCommand(lockCommand); err != nil {
		log.Error("Failed to lock package", err, "package", packageName)
		return fmt.Errorf("failed to lock package: %w", err)
	}

	log.Info("Package locked successfully", "package", packageName)
	return nil
}

// UnlockPackage unlocks a package to allow updates
func (z *ZypperInstaller) UnlockPackage(packageName string) error {
	log.Debug("Unlocking package", "package", packageName)

	// Unlock package using zypper
	unlockCommand := fmt.Sprintf("sudo zypper removelock %s", packageName)
	if _, err := utils.CommandExec.RunShellCommand(unlockCommand); err != nil {
		log.Error("Failed to unlock package", err, "package", packageName)
		return fmt.Errorf("failed to unlock package: %w", err)
	}

	log.Info("Package unlocked successfully", "package", packageName)
	return nil
}

// Validation Functions

// validateRepositoryInputs validates repository parameters to prevent injection attacks
func validateRepositoryInputs(name, url, alias string) error {
	// Validate repository name - only alphanumeric, dash, underscore, dots
	if !isValidRepositoryName(name) {
		return fmt.Errorf("invalid repository name: must contain only letters, numbers, dashes, underscores, and dots")
	}

	// Validate alias - similar to name
	if !isValidRepositoryName(alias) {
		return fmt.Errorf("invalid repository alias: must contain only letters, numbers, dashes, underscores, and dots")
	}

	// Validate URL
	if !isValidURL(url) {
		return fmt.Errorf("invalid repository URL: must be a valid HTTP/HTTPS URL")
	}

	return nil
}

// isValidRepositoryName checks if repository name/alias is safe for filesystem and commands
func isValidRepositoryName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}

	for _, char := range name {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' && char != '.' {
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

// GetDependents returns packages that depend on the given package
func (z *ZypperInstaller) GetDependents(packageName string) ([]string, error) {
	log.Debug("Checking package dependents", "package", packageName)

	// Use zypper to check what requires this package
	command := fmt.Sprintf("zypper search --requires %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// If error, try rpm approach
		return z.getDependentsViaRPM(packageName)
	}

	// Parse the output to find dependent packages
	dependents := []string{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Skip header lines and empty lines
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Loading") || strings.HasPrefix(line, "S |") || strings.Contains(line, "---") {
			continue
		}

		// Parse package name from zypper output format
		parts := strings.Split(line, "|")
		if len(parts) >= 2 {
			pkgName := strings.TrimSpace(parts[1])
			if pkgName != "" && pkgName != packageName {
				dependents = append(dependents, pkgName)
			}
		}
	}

	return dependents, nil
}

// getDependentsViaRPM uses rpm to check package dependencies (fallback method)
func (z *ZypperInstaller) getDependentsViaRPM(packageName string) ([]string, error) {
	// Use rpm to check what requires this package
	command := fmt.Sprintf("rpm -q --whatrequires %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// Check if it's just "no package requires" message
		if strings.Contains(output, "no package requires") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to check dependents: %w", err)
	}

	// Parse the output to get package names
	dependents := []string{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "no package requires") {
			// Extract just the package name without version
			parts := strings.Split(line, "-")
			if len(parts) >= 2 {
				// Reconstruct package name (may contain hyphens)
				pkgName := strings.Join(parts[:len(parts)-2], "-")
				if pkgName != "" {
					dependents = append(dependents, pkgName)
				}
			}
		}
	}

	return dependents, nil
}

// GetOrphans returns orphaned packages (installed as dependencies but no longer needed)
func (z *ZypperInstaller) GetOrphans() ([]string, error) {
	log.Debug("Finding orphaned packages")

	// Use zypper to find unneeded packages
	command := "zypper packages --unneeded"
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// Try alternative approach
		return z.getOrphansViaRPM()
	}

	// Parse the output to get package names
	orphans := []string{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip header lines
		if line == "" || strings.HasPrefix(line, "Loading") || strings.Contains(line, "---") || strings.HasPrefix(line, "S |") {
			continue
		}

		// Parse package name from zypper output
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			pkgName := strings.TrimSpace(parts[2])
			if pkgName != "" {
				orphans = append(orphans, pkgName)
			}
		}
	}

	log.Debug("Found orphaned packages", "count", len(orphans))
	return orphans, nil
}

// getOrphansViaRPM uses rpm to find leaf packages (fallback method)
func (z *ZypperInstaller) getOrphansViaRPM() ([]string, error) {
	// Use package-cleanup if available (from yum-utils)
	command := "package-cleanup --leaves --quiet"
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// package-cleanup not available, return empty list
		return []string{}, nil
	}

	// Parse the output
	orphans := []string{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			orphans = append(orphans, line)
		}
	}

	return orphans, nil
}

// RemoveOrphans removes orphaned packages
func (z *ZypperInstaller) RemoveOrphans() error {
	log.Debug("Removing orphaned packages")

	orphans, err := z.GetOrphans()
	if err != nil {
		return fmt.Errorf("failed to get orphans: %w", err)
	}

	if len(orphans) == 0 {
		log.Info("No orphaned packages to remove")
		return nil
	}

	log.Info("Removing orphaned packages", "packages", orphans)

	// Remove orphans using zypper
	orphansStr := strings.Join(orphans, " ")
	command := fmt.Sprintf("sudo zypper remove --non-interactive --clean-deps %s", orphansStr)
	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to remove orphans", err, "output", output)
		return fmt.Errorf("failed to remove orphans: %w", err)
	}

	log.Info("Successfully removed orphaned packages", "count", len(orphans))
	return nil
}
