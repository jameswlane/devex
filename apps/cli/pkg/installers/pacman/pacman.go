package pacman

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type PacmanInstaller struct{}

var lastPacmanUpdateTime time.Time

// getCurrentUser attempts to determine the current user through multiple methods
func getCurrentUser() string {
	// Method 1: Try USER environment variable
	if username := os.Getenv("USER"); username != "" {
		return username
	}

	// Method 2: Try LOGNAME environment variable
	if username := os.Getenv("LOGNAME"); username != "" {
		return username
	}

	// Method 3: Use os/user package
	if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
		return currentUser.Username
	}

	// Method 4: Try whoami command as fallback
	if output, err := utils.CommandExec.RunShellCommand("whoami"); err == nil {
		username := strings.TrimSpace(output)
		if username != "" {
			return username
		}
	}

	return ""
}

func NewPacmanInstaller() *PacmanInstaller {
	return &PacmanInstaller{}
}

// Install installs packages using pacman (implements BaseInstaller interface)
func (p *PacmanInstaller) Install(command string, repo types.Repository) error {
	log.Debug("Pacman Installer: Starting installation", "command", command)

	// Validate Pacman system availability
	if err := validatePacmanSystem(); err != nil {
		return fmt.Errorf("pacman system validation failed: %w", err)
	}

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "pacman",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		// If utilities doesn't support Pacman yet, use our own check
		isInstalled, err = p.isPackageInstalled(command)
		if err != nil {
			log.Error("Failed to check if package is installed", err, "command", command)
			return fmt.Errorf("failed to check if package is installed via pacman: %w", err)
		}
	}

	if isInstalled {
		log.Info("Package already installed, skipping installation", "command", command)
		return nil
	}

	// Ensure package database is up to date if it's stale
	if err := ensurePackageDatabaseUpdated(repo); err != nil {
		log.Warn("Failed to update package database", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Check if package is available in repositories (try official repos first)
	if err := validatePackageAvailability(command); err != nil {
		// Package not found in official repos, try AUR if available
		if err := tryAURInstallation(command, repo); err != nil {
			return fmt.Errorf("package not found in official repositories or AUR: %w", err)
		}
		return nil // AUR installation succeeded
	}

	// Run pacman install command
	installCommand := fmt.Sprintf("sudo pacman -S --noconfirm %s", command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package via pacman", err, "command", command)
		return fmt.Errorf("failed to install package via pacman: %w", err)
	}

	log.Debug("Pacman package installed successfully", "command", command)

	// Verify installation succeeded
	if isInstalled, err := p.isPackageInstalled(command); err != nil {
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

// isPackageInstalled checks if a package is installed using pacman query
func (p *PacmanInstaller) isPackageInstalled(packageName string) (bool, error) {
	// Use pacman -Q to check if package is installed
	command := fmt.Sprintf("pacman -Q %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// pacman -Q returns non-zero exit code if package is not installed
		if strings.Contains(output, "was not found") || strings.Contains(output, "not found") {
			return false, nil
		}
		// For other errors, return the error
		return false, fmt.Errorf("failed to check package installation status: %w", err)
	}

	// If pacman -Q succeeds, package is installed
	return true, nil
}

// tryAURInstallation attempts to install a package from AUR using YAY
func tryAURInstallation(packageName string, repo types.Repository) error {
	log.Debug("Attempting AUR installation", "package", packageName)

	// Check if YAY is installed, install if not
	if err := ensureYayInstalled(); err != nil {
		return fmt.Errorf("failed to ensure YAY is available: %w", err)
	}

	// Check if package is available in AUR
	if err := validateAURPackageAvailability(packageName); err != nil {
		return fmt.Errorf("package not available in AUR: %w", err)
	}

	// Install from AUR using YAY
	installCommand := fmt.Sprintf("yay -S --noconfirm %s", packageName)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package from AUR", err, "package", packageName)
		return fmt.Errorf("failed to install package from AUR: %w", err)
	}

	log.Info("Package installed successfully from AUR", "package", packageName)

	// Add the package to the repository
	if err := repo.AddApp(packageName); err != nil {
		log.Error("Failed to add AUR package to repository", err, "package", packageName)
		return fmt.Errorf("failed to add AUR package to repository: %w", err)
	}

	return nil
}

func RunPacmanUpdate(forceUpdate bool, repo types.Repository) error {
	log.Debug("Starting Pacman database update", "forceUpdate", forceUpdate)

	// Check if update is required
	if !forceUpdate && time.Since(lastPacmanUpdateTime) < 24*time.Hour {
		log.Debug("Pacman update skipped (cached)")
		return nil
	}

	// Execute pacman -Sy to refresh package database
	updateCommand := "sudo pacman -Sy"
	if _, err := utils.CommandExec.RunShellCommand(updateCommand); err != nil {
		log.Error("Failed to execute Pacman database refresh", err, "command", updateCommand)
		return fmt.Errorf("failed to execute Pacman database refresh: %w", err)
	}

	// Update the last update time cache
	lastPacmanUpdateTime = time.Now()
	if err := repo.Set("last_pacman_update", lastPacmanUpdateTime.Format(time.RFC3339)); err != nil {
		log.Warn("Failed to store last update time in repository", err)
	}

	log.Debug("Pacman database update completed successfully")
	return nil
}

// validatePacmanSystem checks if Pacman is available and functional
func validatePacmanSystem() error {
	// Check if pacman is available
	if _, err := utils.CommandExec.RunShellCommand("which pacman"); err != nil {
		return fmt.Errorf("pacman not found: %w", err)
	}

	// Check if we can access the pacman database
	if _, err := utils.CommandExec.RunShellCommand("pacman --version"); err != nil {
		return fmt.Errorf("pacman not functional: %w", err)
	}

	return nil
}

// ensurePackageDatabaseUpdated updates package database if it's stale
func ensurePackageDatabaseUpdated(repo types.Repository) error {
	// Check if we need to update (more than 6 hours old)
	if time.Since(lastPacmanUpdateTime) > 6*time.Hour {
		log.Debug("Package database is stale, updating")
		return RunPacmanUpdate(false, repo)
	}
	return nil
}

// validatePackageAvailability checks if a package is available in official repositories
func validatePackageAvailability(packageName string) error {
	// Use pacman -Si to check if package is available in repositories
	command := fmt.Sprintf("pacman -Si %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		return fmt.Errorf("failed to check package availability: %w", err)
	}

	// Check if the output indicates the package is available
	if strings.Contains(output, "was not found") || strings.Contains(output, "not found") {
		return fmt.Errorf("package '%s' not found in official repositories", packageName)
	}

	log.Debug("Package availability validated in official repos", "package", packageName)
	return nil
}

// validateAURPackageAvailability checks if a package is available in AUR
func validateAURPackageAvailability(packageName string) error {
	// Use yay -Si to check if package is available in AUR
	command := fmt.Sprintf("yay -Si %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		return fmt.Errorf("failed to check AUR package availability: %w", err)
	}

	// Check if the output indicates the package is available
	if strings.Contains(output, "was not found") || strings.Contains(output, "not found") {
		return fmt.Errorf("package '%s' not found in AUR", packageName)
	}

	log.Debug("Package availability validated in AUR", "package", packageName)
	return nil
}

// ensureYayInstalled checks if YAY is installed and installs it if necessary
func ensureYayInstalled() error {
	// Check if yay is already available
	if _, err := utils.CommandExec.RunShellCommand("which yay"); err == nil {
		log.Debug("YAY is already installed")
		return nil
	}

	log.Info("YAY not found, installing from AUR")

	// Install base development tools if not present
	if err := installDevelopmentTools(); err != nil {
		return fmt.Errorf("failed to install development tools: %w", err)
	}

	// Clone yay from AUR and build it
	currentUser := getCurrentUser()
	if currentUser == "" {
		return fmt.Errorf("unable to determine current user for YAY installation")
	}

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return fmt.Errorf("HOME environment variable not set")
	}

	// Create temporary directory for building yay
	buildDir := fmt.Sprintf("%s/.cache/yay-build", homeDir)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Clone yay repository
	cloneCommand := fmt.Sprintf("cd %s && git clone https://aur.archlinux.org/yay.git", buildDir)
	if _, err := utils.CommandExec.RunShellCommand(cloneCommand); err != nil {
		return fmt.Errorf("failed to clone yay repository: %w", err)
	}

	// Build and install yay
	buildCommand := fmt.Sprintf("cd %s/yay && makepkg -si --noconfirm", buildDir)
	if _, err := utils.CommandExec.RunShellCommand(buildCommand); err != nil {
		return fmt.Errorf("failed to build and install yay: %w", err)
	}

	// Clean up build directory
	if err := os.RemoveAll(buildDir); err != nil {
		log.Warn("Failed to clean up YAY build directory", "error", err, "dir", buildDir)
	}

	log.Info("YAY installed successfully")
	return nil
}

// installDevelopmentTools installs base development tools needed for AUR
func installDevelopmentTools() error {
	log.Debug("Installing base development tools")

	// Install base-devel group and git if not present
	devTools := []string{"base-devel", "git"}

	for _, tool := range devTools {
		// Check if already installed
		if installed, err := (&PacmanInstaller{}).isPackageInstalled(tool); err == nil && installed {
			log.Debug("Development tool already installed", "tool", tool)
			continue
		}

		// Install the tool
		installCommand := fmt.Sprintf("sudo pacman -S --noconfirm %s", tool)
		if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
			return fmt.Errorf("failed to install %s: %w", tool, err)
		}
		log.Debug("Development tool installed", "tool", tool)
	}

	return nil
}

// performPostInstallationSetup handles package-specific post-installation configuration
func performPostInstallationSetup(packageName string) error {
	switch packageName {
	case "docker":
		return setupDockerService()
	case "nginx":
		return setupNginxService()
	case "postgresql", "postgres":
		return setupPostgreSQLService()
	case "redis":
		return setupRedisService()
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

	// Initialize PostgreSQL database cluster
	if _, err := utils.CommandExec.RunShellCommand("sudo -u postgres initdb -D /var/lib/postgres/data"); err != nil {
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

// Uninstall removes packages using pacman
func (p *PacmanInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("Pacman Installer: Starting uninstallation", "command", command)

	// Validate Pacman system availability
	if err := validatePacmanSystem(); err != nil {
		return fmt.Errorf("pacman system validation failed: %w", err)
	}

	// Check if the package is installed
	isInstalled, err := p.isPackageInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "command", command)
		return nil
	}

	// Run pacman remove command
	uninstallCommand := fmt.Sprintf("sudo pacman -Rs --noconfirm %s", command)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall package via pacman", err, "command", command)
		return fmt.Errorf("failed to uninstall package via pacman: %w", err)
	}

	log.Debug("Pacman package uninstalled successfully", "command", command)

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "command", command)
		return fmt.Errorf("failed to remove package from repository: %w", err)
	}

	log.Debug("Package removed from repository successfully", "command", command)
	return nil
}

// IsInstalled checks if a package is installed using pacman
func (p *PacmanInstaller) IsInstalled(command string) (bool, error) {
	return p.isPackageInstalled(command)
}

// InstallGroup installs a Pacman package group
func (p *PacmanInstaller) InstallGroup(groupName string, repo types.Repository) error {
	log.Debug("Installing package group", "group", groupName)

	// Validate system first
	if err := validatePacmanSystem(); err != nil {
		return fmt.Errorf("pacman system validation failed: %w", err)
	}

	// Install group using pacman
	installCommand := fmt.Sprintf("sudo pacman -S --noconfirm %s", groupName)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package group", err, "group", groupName)
		return fmt.Errorf("failed to install package group via pacman: %w", err)
	}

	log.Info("Package group installed successfully", "group", groupName)

	// Add the group to the repository
	if err := repo.AddApp(groupName); err != nil {
		log.Error("Failed to add package group to repository", err, "group", groupName)
		return fmt.Errorf("failed to add package group to repository: %w", err)
	}

	return nil
}

// SystemUpgrade performs a full system upgrade using pacman
func (p *PacmanInstaller) SystemUpgrade() error {
	log.Debug("Performing system upgrade")

	// Validate system first
	if err := validatePacmanSystem(); err != nil {
		return fmt.Errorf("pacman system validation failed: %w", err)
	}

	// Perform full system upgrade
	upgradeCommand := "sudo pacman -Syu --noconfirm"
	if _, err := utils.CommandExec.RunShellCommand(upgradeCommand); err != nil {
		log.Error("Failed to perform system upgrade", err)
		return fmt.Errorf("failed to perform system upgrade: %w", err)
	}

	log.Info("System upgrade completed successfully")
	return nil
}

// CleanCache cleans the pacman package cache
func (p *PacmanInstaller) CleanCache() error {
	log.Debug("Cleaning package cache")

	// Clean package cache
	cleanCommand := "sudo pacman -Sc --noconfirm"
	if _, err := utils.CommandExec.RunShellCommand(cleanCommand); err != nil {
		log.Error("Failed to clean package cache", err)
		return fmt.Errorf("failed to clean package cache: %w", err)
	}

	log.Info("Package cache cleaned successfully")
	return nil
}

// ListInstalled lists all installed packages
func (p *PacmanInstaller) ListInstalled() ([]string, error) {
	log.Debug("Listing installed packages")

	// List all installed packages
	listCommand := "pacman -Q"
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
		if line != "" {
			// Extract package name (first part before space)
			parts := strings.Split(line, " ")
			if len(parts) > 0 {
				packages = append(packages, parts[0])
			}
		}
	}

	log.Debug("Listed installed packages", "count", len(packages))
	return packages, nil
}

// SearchPackages searches for packages in repositories and AUR
func (p *PacmanInstaller) SearchPackages(query string) ([]string, error) {
	log.Debug("Searching for packages", "query", query)

	var packages []string

	// Search official repositories first
	searchCommand := fmt.Sprintf("pacman -Ss %s", query)
	output, err := utils.CommandExec.RunShellCommand(searchCommand)
	if err == nil {
		// Parse pacman search results
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "/") && !strings.HasPrefix(line, " ") {
				// This is a package line (contains repo/packagename)
				parts := strings.Split(line, " ")
				if len(parts) > 0 {
					repoPackage := parts[0]
					if strings.Contains(repoPackage, "/") {
						packageName := strings.Split(repoPackage, "/")[1]
						packages = append(packages, fmt.Sprintf("[official] %s", packageName))
					}
				}
			}
		}
	}

	// Search AUR if yay is available
	if _, err := utils.CommandExec.RunShellCommand("which yay"); err == nil {
		aurSearchCommand := fmt.Sprintf("yay -Ss %s", query)
		aurOutput, err := utils.CommandExec.RunShellCommand(aurSearchCommand)
		if err == nil {
			// Parse yay search results
			lines := strings.Split(aurOutput, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.Contains(line, "aur/") && !strings.HasPrefix(line, " ") {
					// This is an AUR package line
					parts := strings.Split(line, " ")
					if len(parts) > 0 {
						aurPackage := parts[0]
						if strings.Contains(aurPackage, "/") {
							packageName := strings.Split(aurPackage, "/")[1]
							packages = append(packages, fmt.Sprintf("[AUR] %s", packageName))
						}
					}
				}
			}
		}
	}

	log.Debug("Found packages matching query", "query", query, "count", len(packages))
	return packages, nil
}
