package package_manager_pacman

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// Configuration constants
const (
	// Timeouts
	DefaultPackageUpdateTimeout = 6 * time.Hour    // Default timeout for package manager updates
	DefaultYAYBuildTimeout      = 10 * time.Minute // Default timeout for building YAY from source
	DefaultValidationTimeout    = 30 * time.Second // Timeout for background system validation
	DefaultServiceStartupDelay  = 2 * time.Second  // Delay after starting a service

	// Cache durations
	DefaultSystemValidationCacheDuration = 1 * time.Hour   // How long to cache system validation results
	DefaultCacheMaxAge                   = 24 * time.Hour  // Maximum age before cache entries are stale
	DefaultForceUpdateCacheDuration      = 1 * time.Second // Cache duration when force updating
	DefaultStandardCacheDuration         = 24 * time.Hour  // Standard cache duration for package updates
)

// Security validation patterns
var (
	// validUsername ensures usernames contain only safe characters
	validUsername = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	// validPackageName ensures package names are safe for shell commands
	validPackageName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
)

type PacmanInstaller struct{}

// Pacman version information
type PacmanVersion struct {
	Major int
	Minor int
	Patch int
}

// Cached validation state
type systemValidationCache struct {
	lastValidated time.Time
	result        error
}

// Thread-safe cache for version and validation
var (
	cachedPacmanVersion    *PacmanVersion
	cachedSystemValidation *systemValidationCache
	versionMutex           sync.RWMutex
	validationMutex        sync.RWMutex
)

// ResetVersionCache resets the cached Pacman version (useful for testing)
func ResetVersionCache() {
	versionMutex.Lock()
	defer versionMutex.Unlock()
	cachedPacmanVersion = nil

	validationMutex.Lock()
	defer validationMutex.Unlock()
	cachedSystemValidation = nil
}

func getCurrentUser() string {
	return utilities.GetCurrentUser()
}

func New() *PacmanInstaller {
	return &PacmanInstaller{}
}

// getPacmanVersion detects the Pacman version with thread-safe caching
func getPacmanVersion() (*PacmanVersion, error) {
	// First, try to read from cache with read lock
	versionMutex.RLock()
	if cachedPacmanVersion != nil {
		version := cachedPacmanVersion
		versionMutex.RUnlock()
		return version, nil
	}
	versionMutex.RUnlock()

	// Need to detect version, acquire write lock
	versionMutex.Lock()
	defer versionMutex.Unlock()

	// Double-check in case another goroutine already detected it
	if cachedPacmanVersion != nil {
		return cachedPacmanVersion, nil
	}

	// Try pacman --version
	output, err := utils.CommandExec.RunShellCommand("pacman --version")
	if err != nil {
		return nil, fmt.Errorf("failed to detect Pacman version: %w", err)
	}

	// Parse version from output like "Pacman v6.0.2 - libalpm v13.0.2"
	versionRegex := regexp.MustCompile(`Pacman v(\d+)\.(\d+)\.(\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) < 4 {
		// Try alternate format
		versionRegex = regexp.MustCompile(`Pacman v(\d+)\.(\d+)`)
		matches = versionRegex.FindStringSubmatch(output)
		if len(matches) < 3 {
			return nil, fmt.Errorf("failed to parse Pacman version from output: %s", output)
		}
		// Default patch to 0 if not specified
		matches = append(matches, "0")
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	cachedPacmanVersion = &PacmanVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}

	log.Debug("Detected Pacman version", "version", fmt.Sprintf("%d.%d.%d", major, minor, patch))
	return cachedPacmanVersion, nil
}

// Install installs packages using pacman (implements BaseInstaller interface)
func (p *PacmanInstaller) Install(command string, repo types.Repository) error {
	log.Debug("pacman installer: starting installation", "command", command)

	// Validate package name early to prevent injection
	if !validPackageName.MatchString(command) {
		return fmt.Errorf("invalid package name: %s", command)
	}

	// Detect Pacman version early to ensure optimal commands are used
	if _, err := getPacmanVersion(); err != nil {
		log.Warn("failed to detect Pacman version, using defaults", "error", err)
	}

	// Run background validation for better performance
	validator := utilities.NewBackgroundValidator(DefaultValidationTimeout)
	validator.AddSuite(utilities.CreateSystemValidationSuite("pacman"))
	validator.AddSuite(utilities.CreateNetworkValidationSuite())

	ctx := context.Background()
	if err := validator.RunValidations(ctx); err != nil {
		return utilities.WrapError(err, utilities.ErrorTypeSystem, "install", command, "pacman")
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
		isInstalled, err = p.isPackageInstalled(context.Background(), command)
		if err != nil {
			log.Error("Failed to check if package is installed", err, "command", command)
			return fmt.Errorf("failed to check if package is installed via pacman: %w", err)
		}
	}

	if isInstalled {
		log.Info("package already installed, skipping installation", "command", command)
		return nil
	}

	// Ensure package database is up to date using intelligent update system
	if err := utilities.EnsurePackageManagerUpdated(ctx, "pacman", repo, DefaultPackageUpdateTimeout); err != nil {
		log.Warn("failed to update package database", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Attempt installation from official repositories first, then AUR if needed
	return p.installPackageWithFallback(command, repo)
}

// installPackageWithFallback attempts installation from official repos first, then AUR
func (p *PacmanInstaller) installPackageWithFallback(packageName string, repo types.Repository) error {
	// Check if package is available in repositories (try official repos first)
	if err := validatePackageAvailability(packageName); err != nil {
		// Package not found in official repos, try AUR if available
		log.Info("package not found in official repositories, attempting AUR installation", "package", packageName)
		return p.installFromAUR(packageName, repo)
	}

	// Install from official repositories
	return p.installFromOfficialRepos(packageName, repo)
}

// installFromOfficialRepos installs a package from official Pacman repositories
func (p *PacmanInstaller) installFromOfficialRepos(packageName string, repo types.Repository) error {
	log.Debug("Installing package from official repositories", "package", packageName)

	// Validate package name to prevent injection
	if !validPackageName.MatchString(packageName) {
		return fmt.Errorf("invalid package name: %s", packageName)
	}

	// Use secure command construction to prevent shell injection
	command := fmt.Sprintf("sudo pacman -S --noconfirm %s", packageName)
	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("failed to install package via pacman", err, "package", packageName, "output", output)
		return fmt.Errorf("failed to install package via pacman: %w", err)
	}

	log.Debug("pacman package installed successfully", "package", packageName)
	return p.finalizeInstallation(packageName, repo)
}

// installFromAUR installs a package from AUR with better error handling and context support
func (p *PacmanInstaller) installFromAUR(packageName string, repo types.Repository) error {
	log.Debug("attempting AUR installation", "package", packageName)

	// Validate package name to prevent injection
	if !validPackageName.MatchString(packageName) {
		return fmt.Errorf("invalid package name: %s", packageName)
	}

	// Check if YAY is installed, install if not (with context support)
	if err := ensureYayInstalledWithContext(context.Background()); err != nil {
		return fmt.Errorf("failed to ensure YAY is available: %w (hint: check internet connection and ensure git/base-devel are installed)", err)
	}

	// Check if package is available in AUR
	if err := validateAURPackageAvailability(packageName); err != nil {
		return fmt.Errorf("package not available in AUR: %w (hint: check package name spelling or try 'yay -Ss %s' to search)", err, packageName)
	}

	// Install from AUR using YAY with secure command execution
	log.Info("installing package from AUR", "package", packageName)
	command := fmt.Sprintf("yay -S --noconfirm %s", packageName)
	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("failed to install package from AUR", err, "package", packageName, "output", output)
		return fmt.Errorf("failed to install package from AUR: %w", err)
	}

	log.Info("package installed successfully from AUR", "package", packageName)
	return p.finalizeInstallation(packageName, repo)
}

// finalizeInstallation handles post-installation tasks common to both official and AUR packages
func (p *PacmanInstaller) finalizeInstallation(packageName string, repo types.Repository) error {
	// Verify installation succeeded
	if isInstalled, err := p.isPackageInstalled(context.Background(), packageName); err != nil {
		log.Warn("failed to verify installation", "error", err, "package", packageName)
	} else if !isInstalled {
		return fmt.Errorf("package installation verification failed for: %s", packageName)
	}

	// Perform post-installation setup for specific packages
	if err := performPostInstallationSetup(packageName); err != nil {
		log.Warn("post-installation setup failed", "package", packageName, "error", err)
		// Don't fail the installation, just warn
	}

	// Add the package to the repository
	if err := repo.AddApp(packageName); err != nil {
		log.Error("failed to add package to repository", err, "package", packageName)
		return fmt.Errorf("failed to add package to repository: %w", err)
	}

	log.Debug("package added to repository successfully", "package", packageName)
	return nil
}

// isPackageInstalled checks if a package is installed using pacman query
func (p *PacmanInstaller) isPackageInstalled(ctx context.Context, packageName string) (bool, error) {
	// Validate package name to prevent injection
	if !validPackageName.MatchString(packageName) {
		return false, fmt.Errorf("invalid package name: %s", packageName)
	}

	// Use safe command construction for validation
	command := fmt.Sprintf("pacman -Q %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// pacman -Q returns non-zero exit code if package is not installed
		if strings.Contains(output, "was not found") || strings.Contains(output, "not found") || strings.Contains(err.Error(), "was not found") {
			return false, nil
		}
		// For other errors, return the error
		return false, fmt.Errorf("failed to check package installation status: %w", err)
	}

	log.Debug("package installation status checked", "package", packageName, "installed", true)
	// If pacman -Q succeeds, package is installed
	return true, nil
}

func RunPacmanUpdate(forceUpdate bool, repo types.Repository) error {
	log.Debug("Starting Pacman database update", "forceUpdate", forceUpdate)

	ctx := context.Background()

	if forceUpdate {
		// Force update by using a very short max age
		return utilities.EnsurePackageManagerUpdated(ctx, "pacman", repo, DefaultForceUpdateCacheDuration)
	} else {
		// Use standard 24-hour cache
		return utilities.EnsurePackageManagerUpdated(ctx, "pacman", repo, DefaultStandardCacheDuration)
	}
}

// validatePacmanSystem checks if Pacman is available and functional with caching
func validatePacmanSystem() error {
	// Check cache first with TTL validation
	validationMutex.RLock()
	if cachedSystemValidation != nil {
		cacheAge := time.Since(cachedSystemValidation.lastValidated)
		if cacheAge < DefaultSystemValidationCacheDuration && cacheAge < DefaultCacheMaxAge {
			result := cachedSystemValidation.result
			validationMutex.RUnlock()
			log.Debug("Using cached system validation", "age", cacheAge)
			return result
		}
		log.Debug("System validation cache expired", "age", cacheAge)
	}
	validationMutex.RUnlock()

	// Need to validate, acquire write lock
	validationMutex.Lock()
	defer validationMutex.Unlock()

	// Double-check in case another goroutine already validated
	if cachedSystemValidation != nil {
		cacheAge := time.Since(cachedSystemValidation.lastValidated)
		if cacheAge < DefaultSystemValidationCacheDuration && cacheAge < DefaultCacheMaxAge {
			return cachedSystemValidation.result
		}
	}

	// Perform actual validation
	var validationError error

	// Check if pacman is available
	if _, err := utils.CommandExec.RunShellCommand("which pacman"); err != nil {
		validationError = fmt.Errorf("pacman not found: %w", err)
	} else {
		// Check if we can access the pacman database
		if _, err := utils.CommandExec.RunShellCommand("pacman --version"); err != nil {
			validationError = fmt.Errorf("pacman not functional: %w", err)
		}
	}

	// Cache the result
	cachedSystemValidation = &systemValidationCache{
		lastValidated: time.Now(),
		result:        validationError,
	}

	return validationError
}

// validatePackageAvailability checks if a package is available in official repositories
func validatePackageAvailability(packageName string) error {
	// Validate package name to prevent injection
	if !validPackageName.MatchString(packageName) {
		return fmt.Errorf("invalid package name: %s", packageName)
	}

	// Use safe command construction for validation
	command := fmt.Sprintf("pacman -Si %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// For mock testing compatibility, check both error and output
		if strings.Contains(output, "was not found") || strings.Contains(output, "not found") || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("package '%s' not found in official repositories (hint: try 'pacman -Ss %s' to search for similar packages)", packageName, packageName)
		}
		return fmt.Errorf("failed to check package availability: %w", err)
	}

	log.Debug("Package availability validated in official repos", "package", packageName, "outputSize", len(output))
	return nil
}

// validateAURPackageAvailability checks if a package is available in AUR
func validateAURPackageAvailability(packageName string) error {
	// Validate package name to prevent injection
	if !validPackageName.MatchString(packageName) {
		return fmt.Errorf("invalid package name: %s", packageName)
	}

	// Use safe command construction for validation
	command := fmt.Sprintf("yay -Si %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// For mock testing compatibility, check both error and output
		if strings.Contains(output, "was not found") || strings.Contains(output, "not found") || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("package '%s' not found in AUR (hint: ensure YAY is installed and try 'yay -Ss %s' to search)", packageName, packageName)
		}
		return fmt.Errorf("failed to check AUR package availability: %w", err)
	}

	log.Debug("Package availability validated in AUR", "package", packageName, "outputSize", len(output))
	return nil
}

// ensureYayInstalledWithContext checks if YAY is installed and installs it if necessary with context timeout
func ensureYayInstalledWithContext(ctx context.Context) error {
	// Check if yay is already available
	if _, err := utils.CommandExec.RunShellCommand("which yay"); err == nil {
		log.Debug("YAY is already installed")
		return nil
	}

	log.Info("YAY not found, installing from AUR")

	// Create context with timeout for the entire YAY installation process
	ctx, cancel := context.WithTimeout(ctx, DefaultYAYBuildTimeout)
	defer cancel()

	// Install base development tools if not present
	if err := installDevelopmentTools(ctx); err != nil {
		return fmt.Errorf("failed to install development tools: %w", err)
	}

	// Get and validate user information securely
	currentUser := getCurrentUser()
	if !validUsername.MatchString(currentUser) {
		return fmt.Errorf("invalid username detected: %s", currentUser)
	}

	homeDir, err := utils.GetHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get secure home directory: %w", err)
	}

	// Create and validate secure build directory
	buildDir, err := createSecureBuildDirectory(homeDir)
	if err != nil {
		return fmt.Errorf("failed to create secure build directory: %w", err)
	}

	// Ensure cleanup happens even if context is cancelled
	defer func() {
		if err := cleanupBuildDirectory(buildDir, homeDir); err != nil {
			log.Error("Critical: Failed to clean up YAY build directory", err, "dir", buildDir,
				"hint", "Please manually remove the directory to free disk space")
		}
	}()

	// Clone YAY repository
	if err := cloneYayRepository(ctx, buildDir); err != nil {
		return fmt.Errorf("failed to clone YAY repository: %w", err)
	}

	// Build and install YAY
	if err := buildAndInstallYay(ctx, buildDir); err != nil {
		return fmt.Errorf("failed to build and install YAY: %w (hint: ensure makepkg dependencies are installed and you have sufficient disk space)", err)
	}

	log.Info("YAY installed successfully")
	return nil
}

// createSecureBuildDirectory creates a validated build directory
func createSecureBuildDirectory(homeDir string) (string, error) {
	// Validate home directory path
	if err := utilities.ValidatePath(homeDir, "/"); err != nil {
		return "", fmt.Errorf("invalid home directory: %w", err)
	}

	// Create secure build path
	buildDir := filepath.Join(homeDir, ".cache", "yay-build")

	// Validate the build directory path
	if err := utilities.ValidatePath(buildDir, homeDir); err != nil {
		return "", fmt.Errorf("build directory validation failed: %w", err)
	}

	// Create directory with secure permissions
	if err := os.MkdirAll(buildDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create build directory: %w", err)
	}

	return buildDir, nil
}

// cloneYayRepository clones the YAY repository from AUR
func cloneYayRepository(ctx context.Context, buildDir string) error {
	log.Info("Cloning YAY repository from AUR...")

	// Clone YAY repository using secure command execution
	command := fmt.Sprintf("cd %s && git clone https://aur.archlinux.org/yay.git", buildDir)

	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		return fmt.Errorf("git clone failed: %w (output: %s)", err, output)
	}

	return nil
}

// buildAndInstallYay builds and installs YAY from source
func buildAndInstallYay(ctx context.Context, buildDir string) error {
	// Check context before proceeding to build
	select {
	case <-ctx.Done():
		return fmt.Errorf("YAY installation cancelled: %w", ctx.Err())
	default:
	}

	log.Info("Building YAY from source (this may take several minutes)...")

	// Build and install YAY using secure command execution
	yayBuildDir := filepath.Join(buildDir, "yay")
	command := fmt.Sprintf("cd %s && makepkg -si --noconfirm", yayBuildDir)

	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		return fmt.Errorf("makepkg failed: %w (output: %s)", err, output)
	}

	return nil
}

// cleanupBuildDirectory safely removes the build directory with better error handling and security validation
func cleanupBuildDirectory(buildDir, homeDir string) error {
	// Validate build directory path to prevent directory traversal
	if err := utilities.ValidatePath(buildDir, homeDir); err != nil {
		return fmt.Errorf("cleanup validation failed: %w", err)
	}

	// Additional safety checks
	if buildDir == "" || buildDir == "/" || buildDir == homeDir {
		return fmt.Errorf("refusing to remove unsafe directory: %s", buildDir)
	}

	// Ensure directory is within expected cache location
	expectedPrefix := filepath.Join(homeDir, ".cache")
	if !strings.HasPrefix(buildDir, expectedPrefix) {
		return fmt.Errorf("refusing to remove directory outside cache: %s", buildDir)
	}

	if err := os.RemoveAll(buildDir); err != nil {
		// Try to make the directory writable and remove again
		// #nosec G302 -- Directory needs owner-only access for secure cleanup
		if chmodErr := os.Chmod(buildDir, 0700); chmodErr == nil {
			if retryErr := os.RemoveAll(buildDir); retryErr == nil {
				log.Debug("Build directory cleaned up after chmod fix", "dir", buildDir)
				return nil
			}
		}
		return fmt.Errorf("failed to remove build directory %s: %w (hint: check directory permissions or use sudo)", buildDir, err)
	}

	log.Debug("Build directory cleaned up successfully", "dir", buildDir)
	return nil
}

// installDevelopmentTools installs base development tools needed for AUR
func installDevelopmentTools(ctx context.Context) error {
	log.Debug("installing base development tools")

	// Install base-devel group and git if not present
	devTools := []string{"base-devel", "git"}

	for _, tool := range devTools {
		// Validate tool name
		if !validPackageName.MatchString(tool) {
			return fmt.Errorf("invalid development tool name: %s", tool)
		}

		// Check if already installed
		if installed, err := (&PacmanInstaller{}).isPackageInstalled(ctx, tool); err == nil && installed {
			log.Debug("development tool already installed", "tool", tool)
			continue
		}

		// Install the tool using secure command execution
		command := fmt.Sprintf("sudo pacman -S --noconfirm %s", tool)
		if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
			return fmt.Errorf("failed to install %s: %w (output: %s)", tool, err, output)
		}
		log.Debug("development tool installed", "tool", tool)
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

	// Add current user to docker group with validation
	currentUser := getCurrentUser()
	if !validUsername.MatchString(currentUser) {
		log.Warn("Invalid username detected, skipping docker group addition", "user", currentUser)
		return nil
	}

	// Validate username
	if err := utils.ValidateUsername(currentUser); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	// Add user to docker group using secure command execution via utils interface
	command := fmt.Sprintf("sudo usermod -aG docker %s", currentUser)
	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Warn("Failed to add user to docker group", "user", currentUser, "error", err, "output", output)
	} else {
		log.Info("User added to docker group", "user", currentUser)
		log.Info("Note: You may need to log out and log back in for docker group changes to take effect")
	}

	// Wait a moment for service to fully start
	time.Sleep(DefaultServiceStartupDelay)

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

// Uninstall removes packages using pacman with dependency checking
func (p *PacmanInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("pacman installer: starting uninstallation", "command", command)

	// Validate package name to prevent injection
	if !validPackageName.MatchString(command) {
		return fmt.Errorf("invalid package name: %s", command)
	}

	// Validate Pacman system availability
	if err := validatePacmanSystem(); err != nil {
		return fmt.Errorf("pacman system validation failed: %w", err)
	}

	// Check if the package is installed
	isInstalled, err := p.isPackageInstalled(context.Background(), command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("package not installed, skipping uninstallation", "command", command)
		return nil
	}

	// Check for dependencies
	dependents, err := p.GetDependents(command)
	if err != nil {
		log.Warn("Failed to check package dependents", "error", err)
	} else if len(dependents) > 0 {
		log.Warn("Package has dependents that may be affected", "package", command, "dependents", dependents)
	}

	// Run pacman remove command using secure execution
	// -Rs removes dependencies that were installed with the package and are not required by other packages
	commandStr := fmt.Sprintf("sudo pacman -Rs --noconfirm %s", command)
	if output, err := utils.CommandExec.RunShellCommand(commandStr); err != nil {
		log.Error("failed to uninstall package via pacman", err, "command", command, "output", output)
		return fmt.Errorf("failed to uninstall package via pacman: %w", err)
	}

	log.Debug("pacman package uninstalled successfully", "command", command)

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
	return p.isPackageInstalled(context.Background(), command)
}

// InstallGroup installs a Pacman package group
func (p *PacmanInstaller) InstallGroup(groupName string, repo types.Repository) error {
	log.Debug("Installing package group", "group", groupName)

	// Validate system first
	if err := validatePacmanSystem(); err != nil {
		return fmt.Errorf("pacman system validation failed: %w", err)
	}

	// Validate group name
	if err := utils.ValidatePackageName(groupName); err != nil {
		log.Error("Invalid group name", err, "group", groupName)
		return fmt.Errorf("invalid group name: %w", err)
	}

	// Install group using pacman with secure execution via utils interface
	command := fmt.Sprintf("sudo pacman -S --noconfirm %s", groupName)
	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to install package group", err, "group", groupName, "output", output)
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

// PackageManager interface implementation methods

// InstallPackages installs multiple packages via Pacman (implements PackageManager interface)
func (p *PacmanInstaller) InstallPackages(ctx context.Context, packages []string, dryRun bool) error {
	if len(packages) == 0 {
		return nil
	}

	log.Info("Installing packages via Pacman", "packages", packages, "dryRun", dryRun)

	if dryRun {
		log.Info("DRY RUN: Would install packages", "packages", packages)
		return nil
	}

	// Update package database first
	updateCmd := "sudo pacman -Sy"
	if _, err := utils.CommandExec.RunShellCommand(updateCmd); err != nil {
		return fmt.Errorf("failed to update Pacman package database: %w", err)
	}

	// Install all packages in one command for efficiency using secure execution via utils interface
	packageList := strings.Join(packages, " ")
	command := fmt.Sprintf("sudo pacman -S --noconfirm %s", packageList)
	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		return fmt.Errorf("failed to install packages %v: %w (output: %s)", packages, err, output)
	}

	log.Info("Successfully installed packages", "packages", packages)
	return nil
}

// IsAvailable checks if Pacman package manager is available
func (p *PacmanInstaller) IsAvailable(ctx context.Context) bool {
	_, err := utils.CommandExec.RunShellCommand("which pacman")
	return err == nil
}

// GetName returns the package manager name
func (p *PacmanInstaller) GetName() string {
	return "pacman"
}

// GetDependents returns packages that depend on the given package
func (p *PacmanInstaller) GetDependents(packageName string) ([]string, error) {
	log.Debug("Checking package dependents", "package", packageName)

	// Validate package name to prevent injection
	if !validPackageName.MatchString(packageName) {
		return nil, fmt.Errorf("invalid package name: %s", packageName)
	}

	// Use pacman -Qi to get package info including Required By
	command := fmt.Sprintf("pacman -Qi %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		return nil, fmt.Errorf("failed to get package info: %w", err)
	}

	// Parse the output to find Required By line
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Required By") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				deps := strings.TrimSpace(parts[1])
				if deps == "None" {
					return []string{}, nil
				}
				// Split by spaces to get individual packages
				dependents := strings.Fields(deps)
				return dependents, nil
			}
		}
	}

	return []string{}, nil
}

// GetOrphans returns orphaned packages (installed as dependencies but no longer needed)
func (p *PacmanInstaller) GetOrphans() ([]string, error) {
	log.Debug("Finding orphaned packages")

	// Use pacman -Qtdq to find orphans
	command := "pacman -Qtdq"
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// No orphans returns error code, check if output is empty
		if strings.TrimSpace(output) == "" {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to find orphans: %w", err)
	}

	// Parse the output to get package names
	orphans := []string{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			orphans = append(orphans, line)
		}
	}

	log.Debug("Found orphaned packages", "count", len(orphans))
	return orphans, nil
}

// RemoveOrphans removes orphaned packages
func (p *PacmanInstaller) RemoveOrphans() error {
	log.Debug("Removing orphaned packages")

	orphans, err := p.GetOrphans()
	if err != nil {
		return fmt.Errorf("failed to get orphans: %w", err)
	}

	if len(orphans) == 0 {
		log.Info("No orphaned packages to remove")
		return nil
	}

	log.Info("Removing orphaned packages", "packages", orphans)

	// Validate all orphan package names
	validOrphans := make([]string, 0, len(orphans))
	for _, orphan := range orphans {
		if err := utils.ValidatePackageName(orphan); err != nil {
			log.Warn("Skipping invalid orphan package name", "package", orphan, "error", err)
			continue
		}
		validOrphans = append(validOrphans, orphan)
	}

	if len(validOrphans) == 0 {
		log.Info("No valid orphaned packages to remove")
		return nil
	}

	// Remove orphans using pacman with secure execution via utils interface
	orphanList := strings.Join(validOrphans, " ")
	command := fmt.Sprintf("sudo pacman -Rs --noconfirm %s", orphanList)
	if output, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to remove orphans", err, "output", output)
		return fmt.Errorf("failed to remove orphans: %w", err)
	}

	log.Info("Successfully removed orphaned packages", "count", len(orphans))
	return nil
}
