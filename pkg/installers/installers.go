package installers

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/appimage"
	"github.com/jameswlane/devex/pkg/installers/apt"
	"github.com/jameswlane/devex/pkg/installers/brew"
	"github.com/jameswlane/devex/pkg/installers/curlpipe"
	"github.com/jameswlane/devex/pkg/installers/deb"
	"github.com/jameswlane/devex/pkg/installers/docker"
	"github.com/jameswlane/devex/pkg/installers/flatpak"
	"github.com/jameswlane/devex/pkg/installers/mise"
	"github.com/jameswlane/devex/pkg/installers/pip"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type BaseInstaller interface {
	Install(command string, repo repository.Repository) error
}

var installerRegistry = map[string]BaseInstaller{}

func init() {
	installerRegistry["apt"] = apt.New()
	installerRegistry["brew"] = brew.New()
	installerRegistry["curlpipe"] = curlpipe.New()
	installerRegistry["deb"] = deb.New()
	installerRegistry["docker"] = docker.New()
	installerRegistry["flatpak"] = flatpak.New()
	installerRegistry["mise"] = mise.New()
	installerRegistry["pip"] = pip.New()
	installerRegistry["appimage"] = appimage.New()
}

func executeInstallCommand(app types.AppConfig, repo repository.Repository) error {
	installer, exists := installerRegistry[app.InstallMethod]
	if !exists {
		log.Error("Unsupported install method", "method", app.InstallMethod)
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
	log.Info("Executing installer", "method", app.InstallMethod)
	return installer.Install(app.InstallCommand, repo)
}

// InstallApp installs the app based on the InstallMethod field.
func InstallApp(app types.AppConfig, settings config.Settings, repo repository.Repository) error {
	log.Info("Installing app", "app", app.Name)

	// Validate system requirements
	if err := validateSystemRequirements(app); err != nil {
		return fmt.Errorf("failed to validate system requirements: %v", err)
	}

	// Backup existing files
	if err := backupExistingFiles(app); err != nil {
		return fmt.Errorf("failed to back up existing files: %v", err)
	}

	// Remove conflicting packages
	if len(app.Conflicts) > 0 {
		if err := RemoveConflictingPackages(app.Conflicts); err != nil {
			return fmt.Errorf("failed to remove conflicting packages: %v", err)
		}
	}

	// Run pre-install commands
	if err := runInstallCommands(app.PreInstall, settings); err != nil {
		return fmt.Errorf("failed to execute pre-install commands: %v", err)
	}

	// Set up environment
	if err := setupEnvironment(app); err != nil {
		return fmt.Errorf("failed to set up environment: %v", err)
	}

	// Handle dependencies
	if err := HandleDependencies(app, settings, repo); err != nil {
		return fmt.Errorf("failed to handle dependencies: %v", err)
	}

	// Handle APT sources
	if len(app.AptSources) > 0 {
		for _, source := range app.AptSources {
			if err := apt.AddAptSource(source.KeySource, source.KeyName, source.SourceRepo, source.SourceName, source.RequireDearmor); err != nil {
				log.Error("Failed to add APT source", "source", source, "error", err)
				return fmt.Errorf("failed to add APT source: %v", err)
			}
		}

		// Run apt-get update to refresh package lists
		if err := apt.RunAptUpdate(true, repo); err != nil {
			log.Error("Failed to update APT package lists", "error", err)
			return fmt.Errorf("failed to update APT package lists: %v", err)
		}
	}

	// Process config files (placeholder)
	if err := processConfigFiles(app); err != nil {
		return fmt.Errorf("failed to process config files: %v", err)
	}

	// Process themes (placeholder)
	if err := processThemes(app); err != nil {
		return fmt.Errorf("failed to process themes: %v", err)
	}

	// Execute the installation
	if err := executeInstallCommand(app, repo); err != nil {
		return fmt.Errorf("failed to execute install command: %v", err)
	}

	// Run post-install commands
	if err := runInstallCommands(app.PostInstall, settings); err != nil {
		return fmt.Errorf("failed to execute post-install commands: %v", err)
	}

	// Perform cleanup
	if err := cleanupAfterInstall(app); err != nil {
		return fmt.Errorf("failed to clean up after installation: %v", err)
	}

	log.Info("App installed successfully", "app", app.Name)
	return nil
}

// runInstallCommands executes pre-install or post-install commands.
func runInstallCommands(commands []types.InstallCommand, settings config.Settings) error {
	log.Info("Starting runInstallCommands", "commands", commands, "dryRun", settings.DryRun, "homeDir", settings.HomeDir)

	for _, cmd := range commands {
		if cmd.UpdateShellConfig != "" {
			processedCommand := utils.ReplacePlaceholders(cmd.UpdateShellConfig)
			log.Info("Updating shell config", "command", processedCommand)
			if !settings.DryRun {
				if err := utils.UpdateShellConfig([]string{processedCommand}); err != nil {
					log.Error("Failed to update shell config", "command", processedCommand, "error", err)
					return fmt.Errorf("failed to update shell config: %v", err)
				}
			}
		}

		if cmd.Shell != "" {
			processedCommand := utils.ReplacePlaceholders(cmd.Shell)
			log.Info("Executing shell command", "command", processedCommand)
			if !settings.DryRun {
				if err := utils.ExecAsUser(processedCommand, settings.DryRun); err != nil {
					log.Error("Failed to execute shell command", "command", processedCommand, "error", err)
					return fmt.Errorf("failed to execute shell command: %v", err)
				}
			}
		}

		if cmd.Copy != nil {
			source := utils.ReplacePlaceholders(cmd.Copy.Source)
			destination := utils.ReplacePlaceholders(cmd.Copy.Destination)
			log.Info("Copying file", "source", source, "destination", destination)
			if !settings.DryRun {
				if err := utilities.CopyFile(source, destination); err != nil {
					log.Error("Failed to copy file", "source", source, "destination", destination, "error", err)
					return fmt.Errorf("failed to copy file from %s to %s: %v", source, destination, err)
				}
			}
		}
	}

	log.Info("Completed runInstallCommands successfully")
	return nil
}

// RemoveConflictingPackages removes specified conflicting packages before installation.
func RemoveConflictingPackages(packages []string) error {
	if len(packages) == 0 {
		log.Info("No conflicting packages to remove")
		return nil
	}

	log.Info("Removing conflicting packages", "packages", packages)
	command := fmt.Sprintf("sudo apt-get remove -y %s", strings.Join(packages, " "))

	if err := utilities.RunCommand(command); err != nil {
		log.Error("Failed to remove conflicting packages", "command", command, "error", err)
		return fmt.Errorf("failed to remove conflicting packages: %v", err)
	}

	log.Info("Conflicting packages removed successfully")
	return nil
}

func HandleDependencies(app types.AppConfig, settings config.Settings, repo repository.Repository) error {
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

//nolint:unparam
func processConfigFiles(app types.AppConfig) error {
	log.Warn("processConfigFiles is not yet implemented", "app", app.Name)
	// TODO: Implement configuration file processing
	return nil
}

//nolint:unparam
func processThemes(app types.AppConfig) error {
	log.Warn("processThemes is not yet implemented", "app", app.Name)
	// TODO: Implement theme processing
	return nil
}

//nolint:unparam
func validateSystemRequirements(app types.AppConfig) error {
	log.Warn("validateSystemRequirements is not yet implemented", "app", app.Name)
	// TODO: Validate system requirements for the app
	return nil
}

//nolint:unparam
func backupExistingFiles(app types.AppConfig) error {
	log.Warn("backupExistingFiles is not yet implemented", "app", app.Name)
	// TODO: Implement backup logic for existing files
	return nil
}

//nolint:unparam
func setupEnvironment(app types.AppConfig) error {
	log.Warn("setupEnvironment is not yet implemented", "app", app.Name)
	// TODO: Implement environment setup logic
	return nil
}

//nolint:unparam
func cleanupAfterInstall(app types.AppConfig) error {
	log.Warn("cleanupAfterInstall is not yet implemented", "app", app.Name)
	// TODO: Implement cleanup logic
	return nil
}
