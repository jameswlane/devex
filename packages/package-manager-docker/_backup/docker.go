package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/metrics"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// ContainerStatus represents cached container status information
type ContainerStatus struct {
	IsRunning bool
	CachedAt  time.Time
}

// DaemonStatus represents cached Docker daemon status information
type DaemonStatus struct {
	IsAccessible bool
	Version      string
	CachedAt     time.Time
	Error        error
}

type DockerInstaller struct {
	// ServiceTimeout is the timeout for waiting for Docker daemon to become ready
	ServiceTimeout time.Duration
	// containerCache caches container status to avoid repeated Docker API calls
	containerCache map[string]*ContainerStatus
	// daemonCache caches Docker daemon status to avoid repeated daemon checks
	daemonCache *DaemonStatus
	// cacheMutex protects concurrent access to both container and daemon caches
	cacheMutex sync.RWMutex
	// cacheTimeout determines how long cached status remains valid
	cacheTimeout time.Duration
	// daemonCacheTimeout determines how long daemon status cache remains valid
	daemonCacheTimeout time.Duration
	// cleanupTicker for automatic cache cleanup
	cleanupTicker *time.Ticker
	// cleanupDone channel to stop the cleanup goroutine
	cleanupDone chan bool
	// cleanupInterval determines how often to clean expired cache entries
	cleanupInterval time.Duration
	// engineInstaller for Docker Engine installation
	engineInstaller *DockerEngineInstaller
}

func New() *DockerInstaller {
	d := &DockerInstaller{
		ServiceTimeout:     DefaultServiceTimeout,
		containerCache:     make(map[string]*ContainerStatus),
		cacheTimeout:       ContainerCacheTimeout,
		daemonCacheTimeout: 30 * time.Second, // Daemon status cached for 30 seconds
		cleanupInterval:    5 * time.Minute,  // Clean expired cache entries every 5 minutes
		cleanupDone:        make(chan bool, 1),
		engineInstaller:    NewEngineInstaller(),
	}
	// Only start cleanup if interval is valid - safer lifecycle management
	if d.cleanupInterval > 0 {
		d.startCleanupRoutine()
	}

	return d
}

// NewWithTimeout creates a new DockerInstaller with a custom timeout
func NewWithTimeout(timeout time.Duration) *DockerInstaller {
	d := &DockerInstaller{
		ServiceTimeout:     timeout,
		containerCache:     make(map[string]*ContainerStatus),
		cacheTimeout:       ContainerCacheTimeout,
		daemonCacheTimeout: 30 * time.Second,
		cleanupInterval:    5 * time.Minute,
		cleanupDone:        make(chan bool, 1),
		engineInstaller:    NewEngineInstaller(),
	}
	// Only start cleanup if interval is valid - safer lifecycle management
	if d.cleanupInterval > 0 {
		d.startCleanupRoutine()
	}

	return d
}

// NewWithCacheTimeout creates a new DockerInstaller with custom timeout and cache duration
func NewWithCacheTimeout(serviceTimeout, cacheTimeout time.Duration) *DockerInstaller {
	d := &DockerInstaller{
		ServiceTimeout:     serviceTimeout,
		containerCache:     make(map[string]*ContainerStatus),
		cacheTimeout:       cacheTimeout,
		daemonCacheTimeout: 30 * time.Second,
		cleanupInterval:    5 * time.Minute,
		cleanupDone:        make(chan bool, 1),
		engineInstaller:    NewEngineInstaller(),
	}
	// Only start cleanup if interval is valid - safer lifecycle management
	if d.cleanupInterval > 0 {
		d.startCleanupRoutine()
	}

	return d
}

// handleDockerInContainer handles Docker daemon setup in container environments
func (d *DockerInstaller) handleDockerInContainer() error {
	// Check if Docker socket is mounted using the command executor
	if _, err := utils.CommandExec.RunShellCommand("test -S " + DockerSocketPath); err == nil {
		log.Warn("Docker socket is available, but daemon access failed")
		// The socket exists but we can't access it - likely a permission issue
		return fmt.Errorf("docker socket exists but not accessible - container may need to run as root or with proper socket permissions")
	}

	log.Debug("Docker socket not found at /var/run/docker.sock")
	log.Debug("Attempting to start Docker daemon in container environment")

	// Attempt to start Docker daemon - this might work in privileged containers
	return d.attemptDockerDaemonStartup()
}

// attemptDockerDaemonStartup tries to start Docker daemon in privileged containers
func (d *DockerInstaller) attemptDockerDaemonStartup() error {
	ctx, cancel := context.WithTimeout(context.Background(), d.ServiceTimeout)
	defer cancel()

	// Try different methods to start Docker daemon
	if err := d.tryStartDockerService(ctx); err != nil {
		log.Warn("Failed to start Docker daemon in container", "error", err)
		return fmt.Errorf("unable to start Docker daemon in container")
	}

	log.Info("Attempted to start Docker daemon in container")

	// Wait for Docker daemon to become ready
	if err := utils.WaitForDockerDaemon(ctx, d.ServiceTimeout); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			metrics.RecordCount(metrics.MetricTimeoutOccurred, map[string]string{
				"installer": "docker",
				"operation": "daemon_startup",
			})
		}
		return fmt.Errorf("docker daemon startup attempt failed - daemon not responsive: %w", err)
	}

	log.Info("Docker daemon started successfully in container")
	metrics.RecordCount(metrics.MetricDockerDaemonReady, map[string]string{})
	return nil
}

// tryStartDockerService attempts to start Docker using various methods
func (d *DockerInstaller) tryStartDockerService(ctx context.Context) error {
	log.Debug("Attempting to start Docker daemon using multiple methods")

	// Try each method individually - SECURE: No shell injection risk
	startupMethods := []struct {
		name string
		cmd  []string
	}{
		{"service", []string{"sudo", "service", "docker", "start"}},
		{"systemctl", []string{"sudo", "systemctl", "start", "docker"}},
		{"dockerd", []string{"sudo", "dockerd", "--host=unix:///var/run/docker.sock"}},
	}

	for _, method := range startupMethods {
		log.Debug("Trying Docker startup method", "method", method.name)

		// Use mockable command executor for proper cancellation and security
		if _, err := utils.CommandExec.RunCommand(ctx, method.cmd[0], method.cmd[1:]...); err == nil {
			log.Debug("Docker startup method succeeded", "method", method.name)
			return nil
		}
	}

	return fmt.Errorf("all Docker startup methods failed")
}

// Install installs Docker Engine or containers with comprehensive validation and error handling
func (d *DockerInstaller) Install(command string, repo types.Repository) error {
	log.Info("Starting Docker installation", "command", command)

	// Validate user permissions and system database consistency before any Docker operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := validateCurrentUserPermissions(ctx); err != nil {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"operation": "user_validation",
			"reason":    "permission_check_failed",
		})
		return fmt.Errorf("user validation failed: %w", err)
	}

	log.Debug("User validation successful - proceeding with Docker installation")
	metrics.RecordCount(metrics.MetricSecurityValidationSuccess, map[string]string{
		"operation": "user_validation",
		"type":      "permission_check",
	})

	// Check if this is a Docker Engine installation request
	if d.isDockerEngineInstallCommand(command) {
		return d.installDockerEngine(repo)
	}

	// Otherwise, handle container installation
	return d.installContainer(command, repo)
}

// isDockerEngineInstallCommand checks if the command is for Docker Engine installation
func (d *DockerInstaller) isDockerEngineInstallCommand(command string) bool {
	engineCommands := []string{"docker-ce", "docker-engine", "docker.io", "install-engine", "engine"}
	cmdLower := strings.ToLower(command)

	for _, engineCmd := range engineCommands {
		if strings.Contains(cmdLower, engineCmd) {
			return true
		}
	}

	return false
}

// installDockerEngine installs Docker Engine using the engine installer
func (d *DockerInstaller) installDockerEngine(repo types.Repository) error {
	log.Info("Installing Docker Engine")
	ctx := context.Background()
	return d.engineInstaller.InstallDockerEngine(ctx, repo)
}

// installContainer installs Docker containers with comprehensive validation and error handling
func (d *DockerInstaller) installContainer(command string, repo types.Repository) error {
	log.Info("Starting Docker container installation", "command", command)

	// Start metrics tracking with panic recovery
	timer := metrics.StartInstallation("docker", command)
	defer d.recoverFromPanic(timer)

	// Validate Docker service availability
	if err := d.validateDockerServiceWithMetrics(command, timer); err != nil {
		return err
	}

	// Build and validate Docker command
	finalCommand, containerName, err := d.buildDockerCommand(command)
	if err != nil {
		timer.Failure(err)
		return err
	}

	// First, validate the command for security - this should happen before any other validation
	if err := validateDockerCommand(finalCommand); err != nil {
		err := fmt.Errorf("invalid docker command: %w", err)
		timer.Failure(err)
		return err
	}

	// Validate that we have a container name for Docker operations
	if containerName == "" {
		err := fmt.Errorf("failed to extract container name from command: %s", command)
		timer.Failure(err)
		return err
	}

	// Check if container is already running
	if isInstalled, err := d.checkContainerStatus(containerName); err != nil {
		timer.Failure(err)
		return err
	} else if isInstalled {
		log.Info("Docker container is already running, skipping installation", "containerName", containerName)
		timer.Success()
		return nil
	}

	// Execute Docker command
	if err := d.executeDockerCommandSafely(finalCommand); err != nil {
		timer.Failure(err)
		return err
	}

	// Register container in repository
	if err := d.registerContainer(containerName, repo); err != nil {
		timer.Failure(err)
		return err
	}

	// Clear cache after successful installation to ensure fresh status
	d.clearCachedStatus(containerName)

	log.Info("Docker container installed successfully", "containerName", containerName)
	timer.Success()
	return nil
}

// recoverFromPanic handles panic recovery during installation
func (d *DockerInstaller) recoverFromPanic(timer *metrics.InstallationTimer) {
	if r := recover(); r != nil {
		timer.Failure(fmt.Errorf("panic during Docker installation: %v", r))
		panic(r)
	}
}

// validateDockerServiceWithMetrics validates Docker service and records appropriate metrics
func (d *DockerInstaller) validateDockerServiceWithMetrics(command string, timer *metrics.InstallationTimer) error {
	metrics.RecordCount(metrics.MetricDockerSetupStarted, map[string]string{"command": command})

	if err := d.validateDockerServiceCached(); err != nil {
		// Handle container environment gracefully
		if isRunningInContainer() {
			log.Info("Docker daemon not available in container, skipping Docker-based installation", "app", command)
			log.Debug("To enable Docker-in-Docker, run container with: --privileged -v /var/run/docker.sock:/var/run/docker.sock")
			metrics.RecordCount(metrics.MetricDockerSetupSkipped, map[string]string{"command": command, "reason": "container_environment"})
			timer.Skip("Docker installation skipped in container environment")
			return nil
		}

		metrics.RecordCount(metrics.MetricDockerSetupFailed, map[string]string{"command": command})
		return fmt.Errorf("docker service validation failed: %w", err)
	}

	metrics.RecordCount(metrics.MetricDockerSetupSucceeded, map[string]string{"command": command})
	return nil
}

// buildDockerCommand constructs the final Docker command and extracts container name
func (d *DockerInstaller) buildDockerCommand(command string) (finalCommand, containerName string, err error) {
	// Try to get app configuration for DockerOptions
	if appConfig, configErr := config.GetAppInfo(command); configErr == nil && appConfig.DockerOptions.ContainerName != "" {
		log.Info("Building Docker command from configuration", "app", appConfig.Name)

		dockerCmd, buildErr := buildDockerRunCommand(appConfig.InstallCommand, appConfig.DockerOptions)
		if buildErr != nil {
			return "", "", fmt.Errorf("failed to build Docker command from options: %w", buildErr)
		}

		finalCommand = dockerCmd
		containerName = appConfig.DockerOptions.ContainerName
		log.Debug("Built Docker command from configuration", "command", finalCommand, "container", containerName)
	} else {
		// Use command as-is for full docker run commands
		finalCommand = command
		containerName = extractContainerName(command)
		// Note: empty container name is acceptable for commands without DockerOptions
		log.Debug("Using command as-is", "command", finalCommand, "container", containerName)
	}

	return finalCommand, containerName, nil
}

// checkContainerStatus verifies if a container is already running with caching
func (d *DockerInstaller) checkContainerStatus(containerName string) (bool, error) {
	// Check cache first
	if cached, valid := d.getCachedStatus(containerName); valid {
		log.Debug("Using cached container status", "containerName", containerName, "status", cached)
		metrics.RecordCount(metrics.MetricContainerCacheHit, map[string]string{"container": containerName})
		return cached, nil
	}

	metrics.RecordCount(metrics.MetricContainerCacheMiss, map[string]string{"container": containerName})

	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: containerName,
		},
		InstallMethod:  "docker",
		InstallCommand: containerName,
	}

	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		return false, fmt.Errorf("failed to check if Docker container is running: %w", err)
	}

	// Cache the result
	d.setCachedStatus(containerName, isInstalled)
	log.Debug("Cached container status", "containerName", containerName, "status", isInstalled)

	return isInstalled, nil
}

// executeDockerCommandSafely executes Docker command with proper error handling
func (d *DockerInstaller) executeDockerCommandSafely(finalCommand string) error {
	if err := executeDockerCommand(finalCommand); err != nil {
		log.Error("Failed to execute Docker command", err, "command", finalCommand)
		return fmt.Errorf("failed to execute Docker command: %w", err)
	}

	log.Info("Docker command executed successfully", "command", finalCommand)
	return nil
}

// registerContainer adds the container to the repository
func (d *DockerInstaller) registerContainer(containerName string, repo types.Repository) error {
	if err := repo.AddApp(containerName); err != nil {
		log.Error("Failed to add Docker container to repository", err, "containerName", containerName)
		return fmt.Errorf("failed to add Docker container to repository: %w", err)
	}

	log.Debug("Container registered in repository", "containerName", containerName)
	return nil
}

// Uninstall removes Docker containers
func (d *DockerInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Starting Docker container uninstallation", "command", command)

	// Check if Docker is available and running
	if err := d.validateDockerServiceCached(); err != nil {
		return fmt.Errorf("docker service validation failed: %w", err)
	}

	// Extract container name from the command
	containerName := extractContainerName(command)
	if containerName == "" {
		log.Error("Failed to extract container name from command", fmt.Errorf("command: %s", command))
		return fmt.Errorf("failed to extract container name from command")
	}

	// Check if the container is running
	isInstalled, err := d.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if Docker container is running", err, "containerName", containerName)
		return fmt.Errorf("failed to check if Docker container is running: %w", err)
	}

	if !isInstalled {
		log.Info("Docker container not running, skipping uninstallation", "containerName", containerName)
		return nil
	}

	// Validate container name to prevent command injection
	if err := utils.ValidatePackageName(containerName); err != nil {
		return fmt.Errorf("invalid container name: %w", err)
	}

	// Stop and remove the Docker container using secure execution
	ctx, cancel := context.WithTimeout(context.Background(), DockerCommandTimeout)
	defer cancel()

	// Try to stop the container
	if _, err := utils.CommandExec.RunCommand(ctx, "docker", "stop", containerName); err != nil {
		// Try with sudo
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "docker", "stop", containerName); err != nil {
			log.Warn("Failed to stop Docker container, continuing with removal", "containerName", containerName, "error", err)
			// Continue with removal attempt even if stop failed
		}
	}

	// Remove the container
	if _, err := utils.CommandExec.RunCommand(ctx, "docker", "rm", containerName); err != nil {
		// Try with sudo
		if _, err := utils.CommandExec.RunCommand(ctx, "sudo", "docker", "rm", containerName); err != nil {
			log.Error("Failed to remove Docker container", err, "containerName", containerName)
			return fmt.Errorf("failed to remove Docker container: %w", err)
		}
	}

	log.Info("Docker container removed successfully", "containerName", containerName)

	// Remove the container from the repository
	if err := repo.DeleteApp(containerName); err != nil {
		log.Error("Failed to remove Docker container from repository", err, "containerName", containerName)
		return fmt.Errorf("failed to remove Docker container from repository: %w", err)
	}

	// Clear cache after successful uninstallation
	d.clearCachedStatus(containerName)

	log.Debug("Container removed from repository", "containerName", containerName)
	return nil
}

// IsInstalled checks if a Docker container is running with caching
func (d *DockerInstaller) IsInstalled(command string) (bool, error) {
	// Extract container name from the command
	containerName := extractContainerName(command)
	if containerName == "" {
		return false, fmt.Errorf("failed to extract container name from command: %s", command)
	}

	// Validate container name to prevent command injection
	if err := utils.ValidatePackageName(containerName); err != nil {
		return false, fmt.Errorf("invalid container name: %w", err)
	}

	// Check cache first
	if cached, valid := d.getCachedStatus(containerName); valid {
		log.Debug("Using cached container status for IsInstalled", "containerName", containerName, "status", cached)
		metrics.RecordCount(metrics.MetricContainerCacheHit, map[string]string{"container": containerName})
		return cached, nil
	}

	metrics.RecordCount(metrics.MetricContainerCacheMiss, map[string]string{"container": containerName})

	// Check if the container is running using docker ps (secure execution)
	ctx, cancel := context.WithTimeout(context.Background(), DockerGroupTimeout)
	defer cancel()

	output, err := utils.CommandExec.RunCommand(ctx, "docker", "ps",
		"--filter", fmt.Sprintf("name=%s", containerName),
		"--filter", "status=running",
		"--format", "{{.Names}}")
	if err != nil {
		// Try with sudo if regular command failed
		output, err = utils.CommandExec.RunCommand(ctx, "sudo", "docker", "ps",
			"--filter", fmt.Sprintf("name=%s", containerName),
			"--filter", "status=running",
			"--format", "{{.Names}}")
		if err != nil {
			// If both fail, container is likely not running or Docker is not available
			isRunning := false
			d.setCachedStatus(containerName, isRunning)
			return isRunning, nil
		}
	}

	// Check if the container name appears in the output
	isRunning := strings.Contains(output, containerName)

	// Cache the result
	d.setCachedStatus(containerName, isRunning)
	log.Debug("Cached container status for IsInstalled", "containerName", containerName, "status", isRunning)

	return isRunning, nil
}

func extractContainerName(command string) string {
	parts := strings.Fields(command)

	// Handle Docker run commands with --name flag
	for i, part := range parts {
		if part == "--name" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	// Handle Docker uninstall commands (stop/rm commands)
	// Format: "stop containerName" or "stop containerName && rm containerName"
	for i, part := range parts {
		if (part == "stop" || part == "rm") && i+1 < len(parts) {
			nextPart := parts[i+1]
			// Skip shell operators
			if nextPart != "&&" && nextPart != "||" && nextPart != ";" {
				return nextPart
			}
		}
	}

	return ""
}

// validateDockerServiceCached checks Docker daemon status with caching support
func (d *DockerInstaller) validateDockerServiceCached() error {
	d.cacheMutex.RLock()

	// Check if we have valid cached daemon status
	if d.daemonCache != nil && time.Since(d.daemonCache.CachedAt) < d.daemonCacheTimeout {
		cached := d.daemonCache
		d.cacheMutex.RUnlock()

		log.Debug("Using cached Docker daemon status",
			"is_accessible", cached.IsAccessible,
			"version", cached.Version,
			"cached_age", time.Since(cached.CachedAt))

		if cached.IsAccessible {
			return nil
		}
		return cached.Error
	}
	d.cacheMutex.RUnlock()

	// No valid cache, perform actual validation
	return d.validateDockerServiceAndCache()
}

// validateDockerServiceAndCache performs Docker validation and updates cache
func (d *DockerInstaller) validateDockerServiceAndCache() error {
	log.Debug("Performing fresh Docker daemon validation")

	ctx, cancel := context.WithTimeout(context.Background(), DockerGroupTimeout)
	defer cancel()

	var daemonStatus *DaemonStatus

	// Check if docker command is available using the command executor interface
	if _, err := utils.CommandExec.RunShellCommand("which docker"); err != nil {
		daemonStatus = &DaemonStatus{
			IsAccessible: false,
			CachedAt:     time.Now(),
			Error:        fmt.Errorf("docker command not found: %w", err),
		}
	} else {
		// Check if Docker daemon is accessible and get version
		version, err := d.checkDockerAccessAndVersion(ctx)
		if err == nil {
			daemonStatus = &DaemonStatus{
				IsAccessible: true,
				Version:      version,
				CachedAt:     time.Now(),
				Error:        nil,
			}
		} else {
			// Check if we're in a container environment and handle accordingly
			if isRunningInContainer() {
				log.Info("Running in container environment - Docker-in-Docker may require special setup")
				log.Debug("Docker-in-Docker setup help", "hint", "Ensure your container runs with: --privileged -v /var/run/docker.sock:/var/run/docker.sock")
				containerErr := d.handleDockerInContainer()
				daemonStatus = &DaemonStatus{
					IsAccessible: containerErr == nil,
					CachedAt:     time.Now(),
					Error:        containerErr,
				}
			} else {
				daemonStatus = &DaemonStatus{
					IsAccessible: false,
					CachedAt:     time.Now(),
					Error:        fmt.Errorf("docker daemon not accessible: For Docker-in-Docker, run container with --privileged -v /var/run/docker.sock:/var/run/docker.sock"),
				}
			}
		}
	}

	// Update cache
	d.cacheMutex.Lock()
	d.daemonCache = daemonStatus
	d.cacheMutex.Unlock()

	log.Debug("Docker daemon status cached",
		"is_accessible", daemonStatus.IsAccessible,
		"version", daemonStatus.Version)

	if daemonStatus.IsAccessible {
		return nil
	}
	return daemonStatus.Error
}

// checkDockerAccessAndVersion verifies Docker daemon accessibility and returns version
func (d *DockerInstaller) checkDockerAccessAndVersion(ctx context.Context) (string, error) {
	// Try regular docker access first (user in docker group)
	if version, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Server.Version}}'"); err == nil {
		log.Info("Docker daemon accessible with user permissions", "version", version)
		return strings.TrimSpace(version), nil
	}

	// Try with sudo if regular access fails
	if version, err := utils.CommandExec.RunShellCommand("sudo docker version --format '{{.Server.Version}}'"); err == nil {
		log.Warn("Docker daemon accessible only with sudo - consider adding user to docker group", "version", version)
		return strings.TrimSpace(version), nil
	}

	// Try basic docker version command as fallback
	if output, err := utils.CommandExec.RunShellCommand("docker version"); err == nil && strings.Contains(output, "Server:") {
		log.Info("Docker daemon accessible via basic version command")
		return "detected", nil
	}

	return "", fmt.Errorf("docker daemon not accessible with any method")
}

// addUserToDockerGroup adds the current user to the docker group
func (d *DockerInstaller) addUserToDockerGroup() error {
	// Use the enhanced user detection with fallback methods
	username := getCurrentUserWithFallback()
	if username == "" || username == "root" {
		return nil // Skip for root or empty username
	}

	// Validate username for security
	if err := utils.ValidateUsername(username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	log.Info("Adding user to docker group", "user", username)

	ctx, cancel := context.WithTimeout(context.Background(), DockerGroupTimeout)
	defer cancel()

	output, err := utils.CommandExec.RunCommand(ctx, "sudo", "usermod", "-aG", "docker", username)
	if err != nil {
		return fmt.Errorf("failed to add user to docker group: %w (output: %s)", err, output)
	}

	log.Info("User added to docker group. Session refresh may be required for permissions to take effect.", "user", username)
	metrics.RecordCount(metrics.MetricDockerGroupAdded, map[string]string{"user": username})
	return nil
}

// addUserToDockerGroupWithContext adds the current user to the docker group with context
func (d *DockerInstaller) addUserToDockerGroupWithContext(ctx context.Context) error {
	// Use the enhanced user detection with fallback methods
	username := getCurrentUserWithFallbackContext(ctx)
	if username == "" || username == "root" {
		return nil // Skip for root or empty username
	}

	// Validate username for security
	if err := utils.ValidateUsername(username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	log.Info("Adding user to docker group", "user", username)

	// Use provided context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, DockerGroupTimeout)
	defer cancel()

	output, err := utils.CommandExec.RunCommand(cmdCtx, "sudo", "usermod", "-aG", "docker", username)
	if err != nil {
		return fmt.Errorf("failed to add user to docker group: %w (output: %s)", err, output)
	}

	log.Info("User successfully added to docker group", "user", username, "note", "Group changes take effect after next login")
	return nil
}

// getCurrentUserWithFallback attempts multiple methods to determine the current user.
// This function provides robust user detection using multiple fallback mechanisms:
// 1. Checks USER environment variable (most common)
// 2. Checks USERNAME environment variable (Windows compatibility)
// 3. Uses Go's user.Current() function
// 4. Executes 'whoami' command as fallback
// 5. Uses 'id -un' command as final fallback
// This ensures reliable user detection across different environments and configurations.
func getCurrentUserWithFallback() string {
	// Try environment variables first
	if currentUser := os.Getenv("USER"); currentUser != "" {
		return currentUser
	}
	if currentUser := os.Getenv("USERNAME"); currentUser != "" {
		return currentUser
	}

	// Try using the user package
	if currentUser, err := user.Current(); err == nil {
		return currentUser.Username
	}

	// Create context for command execution
	ctx, cancel := context.WithTimeout(context.Background(), UserDetectionTimeout)
	defer cancel()

	// Try using whoami command
	if output, err := utils.CommandExec.RunCommand(ctx, "whoami"); err == nil {
		username := strings.TrimSpace(output)
		if username != "" {
			return username
		}
	}

	// Try using id -un command
	if output, err := utils.CommandExec.RunCommand(ctx, "id", "-un"); err == nil {
		username := strings.TrimSpace(output)
		if username != "" {
			return username
		}
	}

	return ""
}

// validateUserExistence validates that a user exists in the system databases
func validateUserExistence(ctx context.Context, username string) error {
	if username == "" {
		return fmt.Errorf("empty username provided")
	}

	// Validate username format (basic sanitization)
	if strings.ContainsAny(username, ":;|&$`\"'\\(){}[]") {
		return fmt.Errorf("invalid characters in username: %s", username)
	}

	// Method 1: Use Go's user.Lookup function (most reliable)
	if userInfo, err := user.Lookup(username); err == nil {
		log.Debug("User validation successful via Go user.Lookup",
			"username", username,
			"uid", userInfo.Uid,
			"gid", userInfo.Gid,
			"home_dir", userInfo.HomeDir)

		// Additional validation: check if home directory exists
		if userInfo.HomeDir != "" {
			if _, err := os.Stat(userInfo.HomeDir); err != nil {
				log.Warn("User home directory does not exist",
					"username", username,
					"home_dir", userInfo.HomeDir,
					"error", err)
			}
		}
		return nil
	}

	// Method 2: Check /etc/passwd file directly (Linux/Unix systems)
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if err := validateUserInPasswdFile(username); err == nil {
			log.Debug("User validation successful via /etc/passwd", "username", username)
			return nil
		}
	}

	// Method 3: Use id command to verify user existence
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := utils.CommandExec.RunCommand(timeoutCtx, "id", username)
	if err == nil {
		idOutput := strings.TrimSpace(output)
		if strings.Contains(idOutput, "uid=") {
			log.Debug("User validation successful via id command",
				"username", username,
				"id_output", idOutput)
			return nil
		}
	}

	// Method 4: Use getent command (if available) for NSS databases
	output, err = utils.CommandExec.RunCommand(timeoutCtx, "getent", "passwd", username)
	if err == nil {
		passwdEntry := strings.TrimSpace(output)
		if strings.Contains(passwdEntry, username) {
			log.Debug("User validation successful via getent",
				"username", username,
				"passwd_entry", passwdEntry)
			return nil
		}
	}

	return fmt.Errorf("user %s does not exist in system databases", username)
}

// validateUserInPasswdFile checks if user exists in /etc/passwd
func validateUserInPasswdFile(username string) error {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return fmt.Errorf("cannot read /etc/passwd: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, username+":") {
			// Parse passwd entry: username:password:uid:gid:gecos:home:shell
			parts := strings.Split(line, ":")
			if len(parts) >= 6 {
				uid := parts[2]
				gid := parts[3]
				homeDir := parts[5]

				log.Debug("User found in /etc/passwd",
					"username", username,
					"uid", uid,
					"gid", gid,
					"home_dir", homeDir)
				return nil
			}
		}
	}

	return fmt.Errorf("user %s not found in /etc/passwd", username)
}

// validateCurrentUserPermissions checks if the current user has sufficient permissions
func validateCurrentUserPermissions(ctx context.Context) error {
	username := getCurrentUserWithFallbackContext(ctx)
	if username == "" {
		return fmt.Errorf("unable to determine current user")
	}

	// Validate user exists in system databases
	if err := validateUserExistence(ctx, username); err != nil {
		return fmt.Errorf("current user validation failed: %w", err)
	}

	// Check if user is root (not recommended but allowed)
	if username == "root" {
		log.Warn("Running as root user - consider using sudo with regular user")
		return nil
	}

	// Check sudo capabilities (required for Docker installation)
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := utils.CommandExec.RunCommand(timeoutCtx, "sudo", "-n", "echo", "test")
	if err != nil {
		return fmt.Errorf("current user %s lacks sudo privileges required for Docker installation", username)
	}

	// Check if user is already in docker group (optional optimization)
	if userInfo, err := user.Lookup(username); err == nil {
		groupIDs, err := userInfo.GroupIds()
		if err == nil {
			for _, gid := range groupIDs {
				if group, err := user.LookupGroupId(gid); err == nil {
					if group.Name == "docker" {
						log.Debug("User already in docker group", "username", username)
						break
					}
				}
			}
		}
	}

	log.Debug("User permissions validation successful", "username", username)
	return nil
}

// getCurrentUserWithFallbackContext is a context-aware version of getCurrentUserWithFallback
func getCurrentUserWithFallbackContext(ctx context.Context) string {
	// Try environment variables first
	if currentUser := os.Getenv("USER"); currentUser != "" {
		return currentUser
	}
	if currentUser := os.Getenv("USERNAME"); currentUser != "" {
		return currentUser
	}

	// Try using the user package
	if currentUser, err := user.Current(); err == nil {
		return currentUser.Username
	}

	// Use provided context for command execution
	cmdCtx, cancel := context.WithTimeout(ctx, UserDetectionTimeout)
	defer cancel()

	// Try using whoami command
	if output, err := utils.CommandExec.RunCommand(cmdCtx, "whoami"); err == nil {
		username := strings.TrimSpace(output)
		if username != "" {
			return username
		}
	}

	// Try using id -un command
	if output, err := utils.CommandExec.RunCommand(cmdCtx, "id", "-un"); err == nil {
		username := strings.TrimSpace(output)
		if username != "" {
			return username
		}
	}

	return ""
}

// executeDockerCommand runs a Docker command, using sudo if necessary
func executeDockerCommandWithContext(ctx context.Context, command string) error {
	// Create a global instance for this function
	d := &DockerInstaller{ServiceTimeout: 30 * time.Second}

	// First try without sudo
	if _, err := utils.CommandExec.RunShellCommand(command); err == nil {
		log.Info("Docker command executed with user permissions")
		return nil
	}

	// Add user to docker group if not already a member
	if err := d.addUserToDockerGroupWithContext(ctx); err != nil {
		log.Warn("Failed to add user to docker group, continuing with sudo", "error", err)
	}

	// If that fails, try with sudo - use safer command construction
	// Note: Command validation is now done earlier in the installContainer method

	// Use provided context for better cancellation support
	cmdCtx, cancel := context.WithTimeout(ctx, DockerCommandTimeout)
	defer cancel()

	args := strings.Fields(command)
	if len(args) == 0 {
		return fmt.Errorf("empty docker command")
	}

	// Try with sudo using the mockable interface
	if _, err := utils.CommandExec.RunCommand(cmdCtx, "sudo", args...); err != nil {
		log.Error("Docker command failed with both user and sudo access", err, "command", command)
		return fmt.Errorf("docker command failed even with sudo - check if Docker daemon is running and accessible: %w", err)
	}

	log.Info("Docker command executed with sudo - consider refreshing group membership", "hint", "Run 'newgrp docker' or log out and back in")
	return nil
}

// executeDockerCommand provides backward compatibility with a default context
func executeDockerCommand(command string) error {
	ctx := context.Background()
	return executeDockerCommandWithContext(ctx, command)
}

// validateDockerCommand validates that a Docker command contains only safe operations.
// This function provides comprehensive security validation by:
// 1. Ensuring the command starts with 'docker'
// 2. Whitelisting only safe Docker subcommands (run, stop, start, etc.)
// 3. Blocking suspicious patterns that could indicate command injection
// 4. Preventing shell metacharacters and path traversal attempts
// This is a critical security control that prevents malicious command execution.
func validateDockerCommand(command string) error {
	if command == "" {
		return fmt.Errorf("docker command cannot be empty")
	}

	// Split command into parts for validation
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("docker command cannot be empty after splitting")
	}

	// First part should be 'docker'
	if parts[0] != "docker" {
		return fmt.Errorf("command must start with 'docker', got: %s", parts[0])
	}

	// Allow only safe Docker subcommands
	if len(parts) < 2 {
		return fmt.Errorf("docker command must include a subcommand")
	}

	allowedCommands := AllowedDockerSubcommands

	subcommand := parts[1]
	if !allowedCommands[subcommand] {
		return fmt.Errorf("docker subcommand '%s' not allowed for security reasons", subcommand)
	}

	// Check for suspicious patterns
	fullCommand := strings.Join(parts, " ")
	suspiciousPatterns := SuspiciousPatterns

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(fullCommand, pattern) {
			return fmt.Errorf("docker command contains suspicious pattern: %s", pattern)
		}
	}

	// Additional validation for 'run' subcommand
	if subcommand == "run" {
		// Validate container name if --name flag is present
		for i, part := range parts {
			if part == "--name" {
				if i+1 >= len(parts) {
					return fmt.Errorf("container name validation failed: --name flag requires a value")
				}

				containerName := parts[i+1]

				// Check if the "container name" is actually another flag or image name
				// This happens when the original command had an empty name: "docker run -d --name  postgres:16"
				if strings.HasPrefix(containerName, "-") {
					return fmt.Errorf("container name validation failed: --name flag requires a value, found flag: %s", containerName)
				}

				// Check if the "container name" looks like an image name (contains : or /)
				// This indicates the container name was empty and we're seeing the image name
				if strings.Contains(containerName, ":") && (strings.Contains(containerName, "/") || !strings.Contains(containerName, " ")) {
					// This looks like an image name, which means the container name was likely empty
					return fmt.Errorf("container name validation failed: container name appears to be empty (found image name instead: %s)", containerName)
				}

				if err := validateContainerName(containerName); err != nil {
					return fmt.Errorf("container name validation failed: %w", err)
				}

				// Check for potential unquoted container names with spaces
				// Look ahead to see if the next few parts could be continuation of container name
				if i+2 < len(parts) {
					nextPart := parts[i+2]
					// If the next part doesn't look like a flag, environment variable, or image name,
					// it might be part of an unquoted container name with spaces
					if !strings.HasPrefix(nextPart, "-") && !strings.Contains(nextPart, "=") &&
						!strings.Contains(nextPart, ":") && !strings.Contains(nextPart, "/") &&
						!isLikelyImageName(nextPart) {
						return fmt.Errorf("container name validation failed: container name appears to contain unquoted spaces")
					}
				}
			}
			// Validate port mappings if -p flag is present
			if part == "-p" && i+1 < len(parts) {
				portMapping := parts[i+1]
				if err := validatePortMapping(portMapping); err != nil {
					return fmt.Errorf("port mapping validation failed: %w", err)
				}
			}
		}
	}

	return nil
}

// validateImageName validates that an image name is safe and legitimate
func validateImageName(imageName string) error {
	if imageName == "" {
		return fmt.Errorf("image name is required")
	}

	// Check for path traversal attempts
	if strings.Contains(imageName, "..") {
		return fmt.Errorf("invalid image name format: path traversal detected")
	}

	// Check for names that start with hyphens (could be confused for flags)
	if strings.HasPrefix(imageName, "-") {
		return fmt.Errorf("invalid image name format: cannot start with hyphen")
	}

	// Check for suspicious patterns that could indicate typosquatting
	if strings.Contains(imageName, " ") || strings.Contains(imageName, "\t") || strings.Contains(imageName, "\n") {
		return fmt.Errorf("invalid image name format: contains whitespace")
	}

	// Check for shell metacharacters
	if strings.ContainsAny(imageName, ";&|`$()[]{}*?<>\"'\\") {
		return fmt.Errorf("invalid image name format: contains shell metacharacters")
	}

	// Basic length check to prevent extremely long names
	if len(imageName) > 255 {
		return fmt.Errorf("image name too long")
	}

	return nil
}

// validateContainerName validates that a container name is safe and legitimate
func validateContainerName(containerName string) error {
	if containerName == "" {
		return fmt.Errorf("container name is required")
	}

	// Check for path traversal attempts
	if strings.Contains(containerName, "..") {
		return fmt.Errorf("invalid container name: path traversal detected")
	}

	// Check for command injection patterns
	if strings.Contains(containerName, "$(") || strings.Contains(containerName, "`") {
		return fmt.Errorf("invalid container name: command injection pattern detected")
	}

	// Check for names that start with hyphens (could be confused for flags)
	if strings.HasPrefix(containerName, "-") {
		return fmt.Errorf("invalid container name: cannot start with hyphen")
	}

	// Check for whitespace (spaces, tabs, newlines)
	if strings.Contains(containerName, " ") || strings.Contains(containerName, "\t") || strings.Contains(containerName, "\n") {
		return fmt.Errorf("invalid container name: contains whitespace")
	}

	// Check for shell metacharacters
	if strings.ContainsAny(containerName, ";&|`$()[]{}*?<>\"'\\") {
		return fmt.Errorf("invalid container name: contains shell metacharacters")
	}

	// Basic length check to prevent extremely long names
	if len(containerName) > 64 {
		return fmt.Errorf("container name too long (max 64 characters)")
	}

	return nil
}

// validatePortMapping validates that port mappings are safe
func validatePortMapping(portMapping string) error {
	if portMapping == "" {
		return fmt.Errorf("port mapping cannot be empty")
	}

	// Check for path injection attempts
	if strings.Contains(portMapping, "..") {
		return fmt.Errorf("invalid port mapping: path traversal detected")
	}

	// Check for shell metacharacters
	if strings.ContainsAny(portMapping, ";&|`$()[]{}*?<>\"'\\") {
		return fmt.Errorf("invalid port mapping: contains shell metacharacters")
	}

	// Check for dangerous exposed ports on all interfaces
	// Explicit all-interface bindings
	dangerousPorts := []string{"0.0.0.0:22:", "0.0.0.0:3389:", ":22:", ":3389:"} // SSH, RDP
	for _, dangerous := range dangerousPorts {
		if strings.Contains(portMapping, dangerous) {
			return fmt.Errorf("invalid port mapping: exposing dangerous service on all interfaces")
		}
	}

	// Check for implicit all-interface bindings (port:port format without host)
	// These are dangerous for common service ports
	dangerousServicePorts := []string{"22:", "3306:", "3389:", "5432:", "6379:", "27017:"} // SSH, MySQL, RDP, PostgreSQL, Redis, MongoDB
	for _, port := range dangerousServicePorts {
		// Check if the port mapping starts with the dangerous port (implicit all interfaces)
		if strings.HasPrefix(portMapping, port) {
			return fmt.Errorf("invalid port mapping: exposing dangerous service port %s on all interfaces", strings.TrimSuffix(port, ":"))
		}
		// Also check for the pattern "port:port" which exposes to all interfaces
		portNum := strings.TrimSuffix(port, ":")
		implicitPattern := portNum + ":" + portNum
		if portMapping == implicitPattern {
			return fmt.Errorf("invalid port mapping: exposing dangerous service port %s on all interfaces", portNum)
		}
	}

	// Check for invalid port numbers (basic validation)
	if strings.Contains(portMapping, "999999:") {
		return fmt.Errorf("invalid port mapping: port number out of range")
	}

	return nil
}

// isLikelyImageName checks if a string looks like a Docker image name
func isLikelyImageName(name string) bool {
	// Common Docker image patterns:
	// - Official images: nginx, redis, postgres, mysql, mongo, etc.
	// - Registry images with explicit tags/digests: already handled by : and / checks
	// - Images with versions: handled by : check

	// Common official Docker Hub images (incomplete list for common cases)
	commonImages := []string{
		"nginx", "redis", "postgres", "mysql", "mongo", "mariadb",
		"alpine", "ubuntu", "debian", "centos", "busybox", "scratch",
		"node", "python", "java", "golang", "php", "ruby",
		"httpd", "memcached", "elasticsearch", "kibana", "logstash",
		"traefik", "caddy", "jenkins", "sonarqube", "grafana",
		"prometheus", "consul", "vault", "etcd", "rabbitmq",
	}

	// Check against common image names
	for _, img := range commonImages {
		if name == img {
			return true
		}
	}

	// If it contains common image patterns (even without : or /)
	// Look for version-like suffixes or known patterns
	if strings.Contains(name, "latest") ||
		strings.Contains(name, "alpine") ||
		strings.Contains(name, "slim") ||
		strings.Contains(name, "ubuntu") ||
		strings.Contains(name, "debian") {
		return true
	}

	// Additional heuristics for likely image names
	// Look for common image naming patterns or characteristics
	if len(name) > 2 && !strings.ContainsAny(name, " \t\n;|&()[]{}*?<>\"'\\") {
		// Check if it looks like a typical image name (has common patterns)
		if strings.Contains(name, "-") || // Many images have hyphens: redis-alpine, mysql-server
			strings.HasSuffix(name, "db") || // Common database suffixes: mariadb, influxdb
			strings.HasSuffix(name, "sql") || // mysql, postgresql variations
			(len(name) >= 6 && !isCommonEnglishWord(name)) { // Longer names that aren't common words
			return true
		}
	}

	return false
}

// isCommonEnglishWord checks if a word is a common English word that's unlikely to be an image name
func isCommonEnglishWord(word string) bool {
	commonWords := []string{
		"with", "from", "into", "over", "under", "about", "after", "before",
		"during", "through", "without", "within", "between", "against",
		"include", "exclude", "should", "could", "would", "might", "will",
		"have", "been", "were", "they", "them", "this", "that", "these",
		"those", "what", "when", "where", "which", "while", "until",
		"because", "although", "however", "therefore", "otherwise",
		"spaces", "name", "container", "image", "command", "error",
	}

	wordLower := strings.ToLower(word)
	for _, common := range commonWords {
		if wordLower == common {
			return true
		}
	}
	return false
}

// buildDockerRunCommand constructs a complete docker run command from DockerOptions
func buildDockerRunCommand(imageName string, options types.DockerOptions) (string, error) {
	// Validate image name for security
	if err := validateImageName(imageName); err != nil {
		return "", fmt.Errorf("image validation failed: %w", err)
	}

	if err := options.Validate(); err != nil {
		return "", fmt.Errorf("invalid docker options: %w", err)
	}

	// Build command parts securely
	var cmdParts []string
	cmdParts = append(cmdParts, "docker", "run", "-d")
	cmdParts = append(cmdParts, "--name", options.ContainerName)

	// Add restart policy if specified
	if options.RestartPolicy != "" {
		cmdParts = append(cmdParts, "--restart", options.RestartPolicy)
	}

	// Add port mappings
	for _, port := range options.Ports {
		if port != "" {
			cmdParts = append(cmdParts, "-p", port)
		}
	}

	// Add environment variables
	for _, env := range options.Environment {
		if env != "" {
			cmdParts = append(cmdParts, "-e", env)
		}
	}

	// Add the image name
	cmdParts = append(cmdParts, imageName)

	// Join with spaces - this is safe since all parts are validated
	return strings.Join(cmdParts, " "), nil
}

// getCachedStatus retrieves cached container status if still valid
func (d *DockerInstaller) getCachedStatus(containerName string) (bool, bool) {
	d.cacheMutex.RLock()
	defer d.cacheMutex.RUnlock()

	status, exists := d.containerCache[containerName]
	if !exists {
		return false, false
	}

	// Check if cache entry is still valid
	if time.Since(status.CachedAt) > d.cacheTimeout {
		return false, false
	}

	return status.IsRunning, true
}

// setCachedStatus stores container status in cache
func (d *DockerInstaller) setCachedStatus(containerName string, isRunning bool) {
	d.cacheMutex.Lock()
	defer d.cacheMutex.Unlock()

	d.containerCache[containerName] = &ContainerStatus{
		IsRunning: isRunning,
		CachedAt:  time.Now(),
	}
}

// clearCachedStatus removes a specific container from cache (used after install/uninstall)
func (d *DockerInstaller) clearCachedStatus(containerName string) {
	d.cacheMutex.Lock()
	defer d.cacheMutex.Unlock()

	delete(d.containerCache, containerName)
	log.Debug("Cleared cached status for container", "containerName", containerName)
}

// clearExpiredCache removes expired entries from both container and daemon caches
func (d *DockerInstaller) clearExpiredCache() {
	d.cacheMutex.Lock()
	defer d.cacheMutex.Unlock()

	now := time.Now()

	// Clean expired container cache entries
	for containerName, status := range d.containerCache {
		if now.Sub(status.CachedAt) > d.cacheTimeout {
			delete(d.containerCache, containerName)
			log.Debug("Removed expired container cache entry", "containerName", containerName)
		}
	}

	// Clean expired daemon cache
	if d.daemonCache != nil && now.Sub(d.daemonCache.CachedAt) > d.daemonCacheTimeout {
		log.Debug("Removed expired daemon cache entry",
			"cached_age", now.Sub(d.daemonCache.CachedAt),
			"timeout", d.daemonCacheTimeout)
		d.daemonCache = nil
	}
}

// InvalidateDaemonCache clears the daemon status cache to force fresh validation
func (d *DockerInstaller) InvalidateDaemonCache() {
	d.cacheMutex.Lock()
	defer d.cacheMutex.Unlock()

	if d.daemonCache != nil {
		log.Debug("Invalidating daemon cache",
			"was_accessible", d.daemonCache.IsAccessible,
			"cached_age", time.Since(d.daemonCache.CachedAt))
		d.daemonCache = nil
	}
}

// startCleanupRoutine starts the background goroutine for automatic cache cleanup
func (d *DockerInstaller) startCleanupRoutine() {
	if d.cleanupInterval <= 0 {
		log.Debug("Cache cleanup disabled - invalid interval")
		return
	}

	d.cleanupTicker = time.NewTicker(d.cleanupInterval)
	log.Debug("Starting cache cleanup routine", "interval", d.cleanupInterval.String())

	go func() {
		defer func() {
			if d.cleanupTicker != nil {
				d.cleanupTicker.Stop()
			}
		}()

		// Capture ticker reference to avoid race condition
		ticker := d.cleanupTicker
		if ticker == nil {
			log.Debug("Cleanup ticker not initialized, exiting cleanup routine")
			return
		}

		for {
			select {
			case <-ticker.C:
				// Check if we should still be running before cleanup
				if d.cleanupTicker == nil {
					log.Debug("Cleanup routine stopped during execution")
					return
				}
				log.Debug("Running automatic cache cleanup")
				d.clearExpiredCache()
				log.Debug("Automatic cache cleanup completed")

			case <-d.cleanupDone:
				log.Debug("Cache cleanup routine stopping")
				return
			}
		}
	}()
}

// StopCleanup stops the background cleanup routine
func (d *DockerInstaller) StopCleanup() {
	if d.cleanupTicker != nil {
		d.cleanupTicker.Stop()
		d.cleanupTicker = nil
	}

	select {
	case d.cleanupDone <- true:
		log.Debug("Sent cleanup stop signal")
	default:
		// Channel already has a value or is closed
		log.Debug("Cleanup already stopped or stopping")
	}
}
