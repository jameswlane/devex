package installers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers/appimage"
	"github.com/jameswlane/devex/pkg/installers/apt"
	"github.com/jameswlane/devex/pkg/installers/brew"
	"github.com/jameswlane/devex/pkg/installers/curlpipe"
	"github.com/jameswlane/devex/pkg/installers/deb"
	"github.com/jameswlane/devex/pkg/installers/docker"
	"github.com/jameswlane/devex/pkg/installers/flatpak"
	"github.com/jameswlane/devex/pkg/installers/mise"
	"github.com/jameswlane/devex/pkg/installers/pip"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var installerRegistry = map[string]types.BaseInstaller{}

func init() {
	initializeInstallers()
}

// initializeInstallers sets up platform-specific installers
func initializeInstallers() {
	// Cross-platform installers (available on all platforms)
	installerRegistry["curlpipe"] = curlpipe.New()
	installerRegistry["mise"] = mise.New()
	installerRegistry["pip"] = pip.New()
	installerRegistry["docker"] = docker.New()

	// Platform-specific installers
	switch runtime.GOOS {
	case "linux":
		// Linux package managers
		installerRegistry["apt"] = apt.New()
		installerRegistry["flatpak"] = flatpak.New()
		installerRegistry["deb"] = deb.New()
		installerRegistry["appimage"] = appimage.New()
		// Note: Could add dnf, pacman, etc. based on distribution detection

	case "darwin":
		// macOS package managers
		installerRegistry["brew"] = brew.New()
		// TODO: Add Mac App Store installer (mas)

	case "windows":
		// Windows package managers
		// TODO: Add winget, chocolatey, scoop installers
		log.Warn("Windows installers not yet implemented")
	}

	log.Info("Initialized installers for platform", "platform", runtime.GOOS, "count", len(installerRegistry))
}

// GetAvailableInstallers returns a list of available installer methods for the current platform
func GetAvailableInstallers() []string {
	var installers []string
	for method := range installerRegistry {
		installers = append(installers, method)
	}
	return installers
}

// IsInstallerSupported checks if an installer method is supported on the current platform
func IsInstallerSupported(method string) bool {
	_, exists := installerRegistry[method]
	return exists
}

func executeInstallCommand(app types.AppConfig, repo types.Repository) error {
	installer, exists := installerRegistry[app.InstallMethod]
	if !exists {
		log.Error("Unsupported install method", fmt.Errorf("method: %s", app.InstallMethod))
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
	log.Info("Executing installer", "method", app.InstallMethod)
	return installer.Install(app.InstallCommand, repo)
}

// InstallCrossPlatformApp installs a cross-platform application using the appropriate OS-specific configuration
func InstallCrossPlatformApp(app types.CrossPlatformApp, settings config.CrossPlatformSettings, repo types.Repository) error {
	log.Info("Installing cross-platform app", "app", app.Name, "platform", runtime.GOOS)

	// Validate that the app is supported on the current platform
	if !app.IsSupported() {
		return fmt.Errorf("app %s is not supported on %s", app.Name, runtime.GOOS)
	}

	// Validate the app configuration
	if err := app.Validate(); err != nil {
		return fmt.Errorf("app validation failed: %w", err)
	}

	// Get OS-specific configuration
	osConfig := app.GetOSConfig()

	// Create AppConfig for direct installation
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name:        app.Name,
			Description: app.Description,
			Category:    app.Category,
		},
		Default:          app.Default,
		InstallMethod:    osConfig.InstallMethod,
		InstallCommand:   osConfig.InstallCommand,
		UninstallCommand: osConfig.UninstallCommand,
		Dependencies:     osConfig.Dependencies,
		PreInstall:       osConfig.PreInstall,
		PostInstall:      osConfig.PostInstall,
		ConfigFiles:      osConfig.ConfigFiles,
		AptSources:       osConfig.AptSources,
		CleanupFiles:     osConfig.CleanupFiles,
		Conflicts:        osConfig.Conflicts,
		DownloadURL:      osConfig.DownloadURL,
		InstallDir:       osConfig.Destination,
	}

	// Install the app directly
	return InstallApp(appConfig, settings, repo)
}

// InstallCrossPlatformApps installs multiple cross-platform applications
func InstallCrossPlatformApps(apps []types.CrossPlatformApp, settings config.CrossPlatformSettings, repo types.Repository) error {
	log.Info("Installing cross-platform apps", "count", len(apps))

	var errors []string

	for _, app := range apps {
		log.Info("Processing app", "app", app.Name)

		// Skip unsupported apps
		if !app.IsSupported() {
			log.Warn("Skipping unsupported app", "app", app.Name, "platform", runtime.GOOS)
			continue
		}

		// Install the app
		if err := InstallCrossPlatformApp(app, settings, repo); err != nil {
			log.Error("Failed to install app", err, "app", app.Name)
			errors = append(errors, fmt.Sprintf("%s: %v", app.Name, err))
			continue
		}

		log.Info("App installed successfully", "app", app.Name)
	}

	// Return combined errors if any occurred
	if len(errors) > 0 {
		return fmt.Errorf("failed to install some apps: %s", strings.Join(errors, "; "))
	}

	return nil
}

func InstallApp(app types.AppConfig, settings config.CrossPlatformSettings, repo types.Repository) error {
	log.Info("Installing app", "app", app.Name)

	if err := validateSystemRequirements(app); err != nil {
		return fmt.Errorf("failed to validate system requirements: %w", err)
	}

	if err := backupExistingFiles(app); err != nil {
		return fmt.Errorf("failed to back up existing files: %w", err)
	}

	if len(app.Conflicts) > 0 {
		if err := RemoveConflictingPackages(app.Conflicts); err != nil {
			return fmt.Errorf("failed to remove conflicting packages: %w", err)
		}
	}

	if err := runInstallCommands(app.PreInstall); err != nil {
		return fmt.Errorf("failed to execute pre-install commands: %w", err)
	}

	if err := setupEnvironment(app); err != nil {
		return fmt.Errorf("failed to set up environment: %w", err)
	}

	if err := HandleDependencies(app, settings, repo); err != nil {
		return fmt.Errorf("failed to handle dependencies: %w", err)
	}

	if len(app.AptSources) > 0 {
		for _, source := range app.AptSources {
			if err := apt.AddAptSource(source.KeySource, source.KeyName, source.SourceRepo, source.SourceName, source.RequireDearmor); err != nil {
				log.Error("Failed to add APT source", err, "source", source)
				return fmt.Errorf("failed to add APT source: %w", err)
			}
		}

		if err := apt.RunAptUpdate(true, repo); err != nil {
			log.Error("Failed to update APT package lists", err)
			return fmt.Errorf("failed to update APT package lists: %w", err)
		}
	}

	if err := processConfigFiles(app); err != nil {
		return fmt.Errorf("failed to process config files: %w", err)
	}

	if err := processThemes(app); err != nil {
		return fmt.Errorf("failed to process themes: %w", err)
	}

	if err := executeInstallCommand(app, repo); err != nil {
		return fmt.Errorf("failed to execute install command: %w", err)
	}

	if err := runInstallCommands(app.PostInstall); err != nil {
		return fmt.Errorf("failed to execute post-install commands: %w", err)
	}

	if err := cleanupAfterInstall(app); err != nil {
		return fmt.Errorf("failed to clean up after installation: %w", err)
	}

	log.Info("App installed successfully", "app", app.Name)
	return nil
}

func runInstallCommands(commands []types.InstallCommand) error {
	log.Info("Starting runInstallCommands", "commands", commands)

	for _, cmd := range commands {
		if cmd.UpdateShellConfig != "" {
			processedCommand := utils.ReplacePlaceholders(cmd.UpdateShellConfig, map[string]string{})

			log.Info("Updating shell config", "command", processedCommand)
			if err := utils.UpdateShellConfig("shellPath", "configKey", []string{processedCommand}); err != nil {
				log.Error("Failed to update shell config", err, "command", processedCommand)
				return fmt.Errorf("failed to update shell config: %w", err)
			}
		}

		if cmd.Shell != "" {
			processedCommand := utils.ReplacePlaceholders(cmd.UpdateShellConfig, map[string]string{})
			log.Info("Executing shell command", "command", processedCommand)
			output, err := utils.ExecAsUser(processedCommand)
			if err != nil {
				log.Error("Failed to execute shell command", err, "output", output)
				return fmt.Errorf("failed to execute shell command: %w", err)
			}
		}

		if cmd.Copy != nil {
			source := utils.ReplacePlaceholders(cmd.Copy.Source, map[string]string{})
			destination := utils.ReplacePlaceholders(cmd.Copy.Destination, map[string]string{})
			log.Info("Copying file", "source", source, "destination", destination)
			if err := utils.CopyFile(source, destination); err != nil {
				log.Error("Failed to copy file", err, "source", source, "destination", destination)
				return fmt.Errorf("failed to copy file from %s to %s: %w", source, destination, err)
			}
		}
	}

	log.Info("Completed runInstallCommands successfully")
	return nil
}

func RemoveConflictingPackages(packages []string) error {
	if len(packages) == 0 {
		log.Info("No conflicting packages to remove")
		return nil
	}

	log.Info("Removing conflicting packages", "packages", packages)
	command := fmt.Sprintf("sudo apt-get remove -y %s", strings.Join(packages, " "))

	if _, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to remove conflicting packages", err, "command", command)
		return fmt.Errorf("failed to remove conflicting packages: %w", err)
	}

	log.Info("Conflicting packages removed successfully")
	return nil
}

func HandleDependencies(app types.AppConfig, settings config.CrossPlatformSettings, repo types.Repository) error {
	for _, dep := range app.Dependencies {
		// Retrieve the dependency's app configuration
		depApp, err := config.FindAppByName(settings, dep)
		if err != nil {
			return fmt.Errorf("failed to find dependency %s: %v", dep, err)
		}

		// Install the dependency using InstallApp
		if err := InstallApp(*depApp, settings, repo); err != nil {
			return fmt.Errorf("failed to install dependency %s: %v", dep, err)
		}
	}
	return nil
}

func processConfigFiles(app types.AppConfig) error {
	log.Info("Processing configuration files", "app", app.Name)

	if len(app.ConfigFiles) == 0 {
		log.Info("No configuration files to process", "app", app.Name)
		return nil
	}

	homeDir, err := utils.GetHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	for _, configFile := range app.ConfigFiles {
		// Process source path
		sourcePath := utils.ReplacePlaceholders(configFile.Source, map[string]string{})

		// Process destination path
		destPath := utils.ReplacePlaceholders(configFile.Destination, map[string]string{})
		destPath = strings.Replace(destPath, "~", homeDir, 1)

		log.Info("Processing config file", "app", app.Name, "source", sourcePath, "dest", destPath)

		// Check if source file exists
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			log.Warn("Source config file not found", "app", app.Name, "source", sourcePath)
			continue
		}

		// Create destination directory if it doesn't exist
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			log.Warn("Failed to create destination directory", "error", err, "dir", destDir)
			continue
		}

		// Copy configuration file
		if err := utils.CopyFile(sourcePath, destPath); err != nil {
			log.Warn("Failed to copy config file", "error", err, "source", sourcePath, "dest", destPath)
			continue
		}

		// Set appropriate permissions for config files
		if err := os.Chmod(destPath, 0644); err != nil {
			log.Warn("Failed to set permissions on config file", "error", err, "file", destPath)
		}

		log.Info("Config file processed successfully", "app", app.Name, "dest", destPath)
	}

	log.Info("Configuration files processing completed", "app", app.Name)
	return nil
}

func processThemes(app types.AppConfig) error {
	log.Info("Processing themes", "app", app.Name)

	if len(app.Themes) == 0 {
		log.Info("No themes to process", "app", app.Name)
		return nil
	}

	homeDir, err := utils.GetHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	for _, theme := range app.Themes {
		log.Info("Processing theme", "app", app.Name, "theme", theme.Name)

		// Process theme files
		for _, themeFile := range theme.Files {
			// Process source path
			sourcePath := utils.ReplacePlaceholders(themeFile.Source, map[string]string{})

			// Process destination path
			destPath := utils.ReplacePlaceholders(themeFile.Destination, map[string]string{})
			destPath = strings.Replace(destPath, "~", homeDir, 1)

			log.Info("Processing theme file", "app", app.Name, "theme", theme.Name, "source", sourcePath, "dest", destPath)

			// Check if source file exists
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				log.Warn("Source theme file not found", "app", app.Name, "theme", theme.Name, "source", sourcePath)
				continue
			}

			// Create destination directory if it doesn't exist
			destDir := filepath.Dir(destPath)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				log.Warn("Failed to create theme destination directory", "error", err, "dir", destDir)
				continue
			}

			// Copy theme file
			if err := utils.CopyFile(sourcePath, destPath); err != nil {
				log.Warn("Failed to copy theme file", "error", err, "source", sourcePath, "dest", destPath)
				continue
			}

			// Set executable permissions for shell scripts
			if strings.HasSuffix(destPath, ".sh") {
				if err := os.Chmod(destPath, 0755); err != nil {
					log.Warn("Failed to set executable permissions on theme script", "error", err, "file", destPath)
				}
			} else {
				// Set normal permissions for other files
				if err := os.Chmod(destPath, 0644); err != nil {
					log.Warn("Failed to set permissions on theme file", "error", err, "file", destPath)
				}
			}

			log.Info("Theme file processed successfully", "app", app.Name, "theme", theme.Name, "dest", destPath)
		}

		// If theme has a color and background, we could potentially apply them
		// For now, we just log them - specific theme application would be handled
		// by desktop environment specific code (GNOME, KDE, etc.)
		if theme.ThemeColor != "" || theme.ThemeBackground != "" {
			log.Info("Theme properties detected",
				"app", app.Name,
				"theme", theme.Name,
				"color", theme.ThemeColor,
				"background", theme.ThemeBackground)
			// TODO: Add desktop environment specific theme application
		}
	}

	log.Info("Themes processing completed", "app", app.Name)
	return nil
}

func validateSystemRequirements(app types.AppConfig) error {
	log.Info("Validating system requirements", "app", app.Name)

	// Check if installer method is supported on current platform
	if !IsInstallerSupported(app.InstallMethod) {
		return fmt.Errorf("installer method '%s' is not supported on this platform", app.InstallMethod)
	}

	// Validate APT-specific requirements
	if app.InstallMethod == "apt" {
		// Check if apt is available
		if _, err := utils.CommandExec.RunShellCommand("which apt-get"); err != nil {
			return fmt.Errorf("apt-get not found: %w", err)
		}

		// Validate APT sources if specified
		for _, source := range app.AptSources {
			if source.KeySource == "" || source.SourceRepo == "" {
				return fmt.Errorf("incomplete APT source configuration for %s", app.Name)
			}
		}
	}

	// Check for conflicting packages
	if len(app.Conflicts) > 0 {
		log.Info("Checking for conflicting packages", "app", app.Name, "conflicts", app.Conflicts)
		for _, conflict := range app.Conflicts {
			// Check if conflicting package is installed
			checkCmd := fmt.Sprintf("dpkg -l | grep -q '^ii.*%s'", conflict)
			if _, err := utils.CommandExec.RunShellCommand(checkCmd); err == nil {
				log.Warn("Conflicting package found", "app", app.Name, "conflict", conflict)
				// Note: We don't fail here, just warn. Removal happens later in the pipeline
			}
		}
	}

	// Validate download URLs if specified
	if app.DownloadURL != "" {
		log.Info("Validating download URL", "app", app.Name, "url", app.DownloadURL)
		// Simple URL format validation
		if !strings.HasPrefix(app.DownloadURL, "http://") && !strings.HasPrefix(app.DownloadURL, "https://") {
			return fmt.Errorf("invalid download URL format for %s: %s", app.Name, app.DownloadURL)
		}
	}

	log.Info("System requirements validated successfully", "app", app.Name)
	return nil
}

func backupExistingFiles(app types.AppConfig) error {
	log.Info("Backing up existing files", "app", app.Name)

	// Create backup directory if needed
	homeDir, err := utils.GetHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	backupDir := filepath.Join(homeDir, ".devex", "backups", app.Name)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Backup configuration files that will be overwritten
	for _, configFile := range app.ConfigFiles {
		destPath := utils.ReplacePlaceholders(configFile.Destination, map[string]string{})
		destPath = strings.Replace(destPath, "~", homeDir, 1)

		// Check if destination file exists
		if _, err := os.Stat(destPath); err == nil {
			log.Info("Backing up existing config file", "app", app.Name, "file", destPath)

			// Create backup filename with timestamp
			timestamp := time.Now().Format("20060102_150405")
			backupFilename := fmt.Sprintf("%s_%s", filepath.Base(destPath), timestamp)
			backupPath := filepath.Join(backupDir, backupFilename)

			// Create backup directory structure if needed
			backupFileDir := filepath.Dir(backupPath)
			if err := os.MkdirAll(backupFileDir, 0755); err != nil {
				log.Warn("Failed to create backup directory", "error", err, "dir", backupFileDir)
				continue
			}

			// Copy file to backup location
			if err := utils.CopyFile(destPath, backupPath); err != nil {
				log.Warn("Failed to backup file", "error", err, "source", destPath, "backup", backupPath)
				continue
			}

			log.Info("File backed up successfully", "app", app.Name, "original", destPath, "backup", backupPath)
		}
	}

	log.Info("File backup completed", "app", app.Name, "backupDir", backupDir)
	return nil
}

func setupEnvironment(app types.AppConfig) error {
	log.Info("Setting up environment", "app", app.Name)

	// Handle shell updates (add to PATH, set environment variables)
	if len(app.ShellUpdates) > 0 {
		log.Info("Processing shell updates", "app", app.Name, "updates", app.ShellUpdates)

		homeDir, err := utils.GetHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		// Get shell RC file path (e.g., ~/.bashrc, ~/.zshrc)
		shellRCPath, err := utils.GetShellRCPath("", homeDir)
		if err != nil || shellRCPath == "" {
			log.Warn("Could not determine shell RC file, skipping shell updates", "app", app.Name, "error", err)
			return nil
		}

		// Add shell updates to RC file
		for _, update := range app.ShellUpdates {
			processedUpdate := utils.ReplacePlaceholders(update, map[string]string{
				"HOME": homeDir,
				"USER": os.Getenv("USER"),
			})

			log.Info("Adding shell update", "app", app.Name, "update", processedUpdate)
			if err := utils.UpdateShellConfig(shellRCPath, "devex_"+app.Name, []string{processedUpdate}); err != nil {
				log.Warn("Failed to update shell config", "error", err, "app", app.Name)
				continue
			}
		}
	}

	// Handle symlinks (create symlinks for binaries)
	if app.Symlink != "" {
		log.Info("Creating symlink", "app", app.Name, "symlink", app.Symlink)

		// Process symlink paths
		symlinkParts := strings.Split(app.Symlink, ":")
		if len(symlinkParts) == 2 {
			source := strings.TrimSpace(symlinkParts[0])
			target := strings.TrimSpace(symlinkParts[1])

			// Replace placeholders
			homeDir, _ := utils.GetHomeDir()
			source = utils.ReplacePlaceholders(source, map[string]string{})
			target = utils.ReplacePlaceholders(target, map[string]string{})
			source = strings.Replace(source, "~", homeDir, 1)
			target = strings.Replace(target, "~", homeDir, 1)

			// Create target directory if it doesn't exist
			targetDir := filepath.Dir(target)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				log.Warn("Failed to create symlink target directory", "error", err, "dir", targetDir)
			} else {
				// Create symlink
				if err := os.Symlink(source, target); err != nil {
					if !os.IsExist(err) {
						log.Warn("Failed to create symlink", "error", err, "source", source, "target", target)
					} else {
						log.Info("Symlink already exists", "app", app.Name, "target", target)
					}
				} else {
					log.Info("Symlink created successfully", "app", app.Name, "source", source, "target", target)
				}
			}
		} else {
			log.Warn("Invalid symlink format, expected 'source:target'", "app", app.Name, "symlink", app.Symlink)
		}
	}

	// Handle install directory setup
	if app.InstallDir != "" {
		installDir := utils.ReplacePlaceholders(app.InstallDir, map[string]string{})
		homeDir, _ := utils.GetHomeDir()
		installDir = strings.Replace(installDir, "~", homeDir, 1)

		log.Info("Ensuring install directory exists", "app", app.Name, "dir", installDir)
		if err := os.MkdirAll(installDir, 0755); err != nil {
			log.Warn("Failed to create install directory", "error", err, "dir", installDir)
		}
	}

	log.Info("Environment setup completed", "app", app.Name)
	return nil
}

func cleanupAfterInstall(app types.AppConfig) error {
	log.Info("Cleaning up after installation", "app", app.Name)

	// Clean up specified cleanup files
	if len(app.CleanupFiles) > 0 {
		homeDir, err := utils.GetHomeDir()
		if err != nil {
			log.Warn("Failed to get home directory for cleanup", "error", err)
		} else {
			for _, cleanupFile := range app.CleanupFiles {
				// Process cleanup file path
				filePath := utils.ReplacePlaceholders(cleanupFile, map[string]string{})
				filePath = strings.Replace(filePath, "~", homeDir, 1)

				log.Info("Cleaning up file", "app", app.Name, "file", filePath)

				// Remove file or directory
				if err := os.RemoveAll(filePath); err != nil {
					if !os.IsNotExist(err) {
						log.Warn("Failed to clean up file", "error", err, "file", filePath)
					}
				} else {
					log.Info("File cleaned up successfully", "app", app.Name, "file", filePath)
				}
			}
		}
	}

	// Clean up any temporary files created during installation
	// This could include downloaded packages, extracted archives, etc.
	tempDir := os.TempDir()
	appTempPattern := fmt.Sprintf("devex_%s_*", strings.ReplaceAll(app.Name, " ", "_"))

	log.Info("Cleaning up temporary files", "app", app.Name, "pattern", appTempPattern, "tempDir", tempDir)

	// Note: In a production implementation, you might want to track temp files
	// created during installation and clean them up specifically
	// For now, we just log the cleanup intent

	// Refresh package database cache if this was an APT installation
	if app.InstallMethod == "apt" {
		log.Info("Refreshing package database cache", "app", app.Name)
		// Note: We don't actually run apt update here to avoid unnecessary network calls
		// This would be configurable in a production version
	}

	// Refresh font cache if fonts were installed
	if len(app.ConfigFiles) > 0 {
		for _, configFile := range app.ConfigFiles {
			if strings.Contains(configFile.Destination, "fonts") {
				log.Info("Refreshing font cache", "app", app.Name)
				if _, err := utils.CommandExec.RunShellCommand("fc-cache -fv"); err != nil {
					log.Warn("Failed to refresh font cache", "error", err)
				}
				break
			}
		}
	}

	// Update desktop database if .desktop files were installed
	if len(app.ConfigFiles) > 0 {
		for _, configFile := range app.ConfigFiles {
			if strings.HasSuffix(configFile.Destination, ".desktop") {
				log.Info("Updating desktop database", "app", app.Name)
				if _, err := utils.CommandExec.RunShellCommand("update-desktop-database ~/.local/share/applications/"); err != nil {
					log.Warn("Failed to update desktop database", "error", err)
				}
				break
			}
		}
	}

	log.Info("Cleanup completed", "app", app.Name)
	return nil
}
