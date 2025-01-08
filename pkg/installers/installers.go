package installers

import (
	"fmt"
	"strings"

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

func executeInstallCommand(app types.AppConfig, repo types.Repository) error {
	installer, exists := installerRegistry[app.InstallMethod]
	if !exists {
		log.Error("Unsupported install method", fmt.Errorf("method: %s", app.InstallMethod))
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
	log.Info("Executing installer", "method", app.InstallMethod)
	return installer.Install(app.InstallCommand, repo)
}

func InstallApp(app types.AppConfig, settings config.Settings, repo types.Repository) error {
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

func HandleDependencies(app types.AppConfig, settings config.Settings, repo types.Repository) error {
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
