// Package installers provides legacy installer functionality
// This file contains the original installer functions that are still needed
// but don't depend on individual package manager implementations
package installers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/system"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

// InstallCrossPlatformApps installs multiple cross-platform applications
func InstallCrossPlatformApps(apps []types.CrossPlatformApp, settings config.CrossPlatformSettings, repo types.Repository) error {
	log.Info("Installing cross-platform apps", "count", len(apps))

	var errors []string

	for _, app := range apps {
		log.Info("Processing app", "app", app.Name)

		// Skip unsupported apps
		if !app.IsSupported() {
			log.Warn("Skipping unsupported app", "app", app.Name)
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

// RemoveConflictingPackages removes conflicting packages
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

// HandleDependencies handles app dependencies
func HandleDependencies(app types.AppConfig, settings config.CrossPlatformSettings, repo types.Repository) error {
	for _, dep := range app.Dependencies {
		// Retrieve the dependency's app configuration
		depApp, err := config.FindAppByName(settings, dep)
		if err != nil {
			return fmt.Errorf("failed to find dependency %s: %w", dep, err)
		}

		// Install the dependency using InstallApp
		if err := InstallApp(*depApp, settings, repo); err != nil {
			return fmt.Errorf("failed to install dependency %s: %w", dep, err)
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
		sourcePath = strings.Replace(sourcePath, "~", homeDir, 1)

		// Process destination path
		destPath := utils.ReplacePlaceholders(configFile.Destination, map[string]string{})
		destPath = strings.Replace(destPath, "~", homeDir, 1)

		log.Info("Processing config file", "app", app.Name, "source", sourcePath, "dest", destPath)

		// Check if source file exists
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			log.Warn("Source config file not found", "app", app.Name, "source", sourcePath)
			continue
		}

		// Check if destination file already exists - skip copying if it does
		if _, err := os.Stat(destPath); err == nil {
			log.Info("Config file already exists, skipping copy to preserve user configuration", "app", app.Name, "dest", destPath)
			continue
		}

		// Create destination directory if it doesn't exist
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0750); err != nil {
			log.Warn("Failed to create destination directory", "error", err, "dir", destDir)
			continue
		}

		// Copy configuration file
		if err := utils.CopyFile(sourcePath, destPath); err != nil {
			log.Warn("Failed to copy config file", "error", err, "source", sourcePath, "dest", destPath)
			continue
		}

		// Set appropriate permissions for config files
		if err := os.Chmod(destPath, 0600); err != nil {
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
			if err := os.MkdirAll(destDir, 0750); err != nil {
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
				if err := os.Chmod(destPath, 0700); err != nil {
					log.Warn("Failed to set executable permissions on theme script", "error", err, "file", destPath)
				}
			} else {
				// Set normal permissions for other files
				if err := os.Chmod(destPath, 0600); err != nil {
					log.Warn("Failed to set permissions on theme file", "error", err, "file", destPath)
				}
			}

			log.Info("Theme file processed successfully", "app", app.Name, "theme", theme.Name, "dest", destPath)
		}

		// Log theme properties for future desktop environment integration
		if theme.ThemeColor != "" || theme.ThemeBackground != "" {
			log.Info("Theme properties detected",
				"app", app.Name,
				"theme", theme.Name,
				"color", theme.ThemeColor,
				"background", theme.ThemeBackground)
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

	// Validate comprehensive system requirements using new validation system
	if hasSystemRequirements(app.SystemRequirements) {
		log.Info("Validating comprehensive system requirements", "app", app.Name)

		validator := system.NewRequirementsValidator()
		results, err := validator.ValidateRequirements(app.Name, app.SystemRequirements)
		if err != nil {
			return fmt.Errorf("failed to validate system requirements: %w", err)
		}

		// Check for failures
		if validator.HasFailures(results) {
			failures := validator.GetFailures(results)
			log.Error("System requirements validation failed", fmt.Errorf("validation failed"), "app", app.Name, "failures", len(failures))

			// Log each failure
			for _, failure := range failures {
				log.Error("Requirement failed", fmt.Errorf("requirement not met"),
					"requirement", failure.Requirement,
					"message", failure.Message,
					"suggestion", failure.Suggestion)
			}

			return fmt.Errorf("system requirements not met for %s: %d requirement(s) failed", app.Name, len(failures))
		}

		// Log warnings if any
		warnings := validator.GetWarnings(results)
		if len(warnings) > 0 {
			log.Warn("System requirements validation has warnings", "app", app.Name, "warnings", len(warnings))
			for _, warning := range warnings {
				log.Warn("Requirement warning",
					"requirement", warning.Requirement,
					"message", warning.Message,
					"suggestion", warning.Suggestion)
			}
		}

		log.Info("Comprehensive system requirements validated successfully", "app", app.Name)
	}

	log.Info("System requirements validated successfully", "app", app.Name)
	return nil
}

// hasSystemRequirements checks if the app has any system requirements defined
func hasSystemRequirements(requirements types.SystemRequirements) bool {
	return requirements.MinMemoryMB > 0 ||
		requirements.MinDiskSpaceMB > 0 ||
		requirements.DockerVersion != "" ||
		requirements.DockerComposeVersion != "" ||
		requirements.GoVersion != "" ||
		requirements.NodeVersion != "" ||
		requirements.PythonVersion != "" ||
		requirements.RubyVersion != "" ||
		requirements.JavaVersion != "" ||
		requirements.GitVersion != "" ||
		requirements.KubectlVersion != "" ||
		len(requirements.RequiredCommands) > 0 ||
		len(requirements.RequiredServices) > 0 ||
		len(requirements.RequiredPorts) > 0 ||
		len(requirements.RequiredEnvVars) > 0
}

func backupExistingFiles(app types.AppConfig) error {
	log.Info("Backing up existing files", "app", app.Name)

	// Create backup directory if needed
	homeDir, err := utils.GetHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	backupDir := filepath.Join(homeDir, ".devex", "backups", app.Name)
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
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
			if err := os.MkdirAll(targetDir, 0750); err != nil {
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
		if err := os.MkdirAll(installDir, 0750); err != nil {
			log.Warn("Failed to create install directory", "error", err, "dir", installDir)
		}
	}

	log.Info("Environment setup completed", "app", app.Name)
	return nil
}

func cleanupAfterInstall(app types.AppConfig) {
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

	log.Info("Cleanup completed", "app", app.Name)
}
