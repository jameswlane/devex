package installers

import (
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/appimage"
	"github.com/jameswlane/devex/pkg/installers/apt"
	"github.com/jameswlane/devex/pkg/installers/curlpipe"
	"github.com/jameswlane/devex/pkg/installers/deb"
	"github.com/jameswlane/devex/pkg/installers/docker"
	"github.com/jameswlane/devex/pkg/installers/flatpak"
	"github.com/jameswlane/devex/pkg/installers/mise"
	"github.com/jameswlane/devex/pkg/installers/pip"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// InstallApp installs the app based on the InstallMethod field.
func InstallApp(app types.AppConfig, settings config.Settings, repo repository.Repository) error {
	log.Info("Starting InstallApp", "app", app.Name, "method", app.InstallMethod)

	// Step 1: Execute pre-install commands
	if err := runInstallCommands(app.PreInstall, settings); err != nil {
		log.Error("Failed to execute pre-install commands", "app", app.Name, "error", err)
		return fmt.Errorf("failed to execute pre-install commands for %s: %v", app.Name, err)
	}

	// Step 2: Handle dependencies
	if err := handleDependencies(app, settings, repo); err != nil {
		log.Error("Failed to handle dependencies", "app", app.Name, "error", err)
		return fmt.Errorf("failed to handle dependencies for %s: %v", app.Name, err)
	}

	// Step 3: Install the app
	if len(app.AptSources) > 0 {
		for _, source := range app.AptSources {
			err := apt.AddAptSource(source.KeySource, source.KeyName, source.SourceRepo, source.SourceName)
			if err != nil {
				log.Error("Failed to handle APT source", "source", source, "error", err)
				return fmt.Errorf("failed to handle APT source: %v", err)
			}
		}
	}
	if err := executeInstallCommand(app, settings, repo); err != nil {
		log.Error("Failed to execute install command", "app", app.Name, "error", err)
		return fmt.Errorf("failed to install %s: %v", app.Name, err)
	}

	// Step 4: Execute post-install commands
	if err := runInstallCommands(app.PostInstall, settings); err != nil {
		log.Error("Failed to execute post-install commands", "app", app.Name, "error", err)
		return fmt.Errorf("failed to execute post-install commands for %s: %v", app.Name, err)
	}

	log.Info("App installed successfully", "app", app.Name)
	return nil
}

// handleDependencies ensures all dependencies are installed.
func handleDependencies(app types.AppConfig, settings config.Settings, repo repository.Repository) error {
	if len(app.Dependencies) == 0 {
		log.Info("No dependencies to handle", "app", app.Name)
		return nil
	}

	log.Info("Checking dependencies for app", "app", app.Name, "dependencies", app.Dependencies)

	for _, dep := range app.Dependencies {
		dependencyApp, err := config.FindAppByName(settings, dep)
		if err != nil {
			log.Error("Dependency not found", "dependency", dep)
			return fmt.Errorf("dependency %s not found: %v", dep, err)
		}

		log.Info("Installing dependency", "dependency", dep)
		if err := InstallApp(*dependencyApp, settings, repo); err != nil {
			return fmt.Errorf("failed to install dependency %s: %v", dep, err)
		}
	}

	log.Info("All dependencies handled successfully", "app", app.Name)
	return nil
}

// executeInstallCommand runs the installation logic based on the method.
func executeInstallCommand(app types.AppConfig, settings config.Settings, repo repository.Repository) error {
	log.Info("Executing install command", "app", app.Name, "method", app.InstallMethod)
	switch app.InstallMethod {
	case "appimage":
		return appimage.Install(app.DownloadURL, app.InstallDir, app.Symlink, app.Name, settings.DryRun, repo)
	case "apt":
		return apt.Install(app.InstallCommand, settings.DryRun, repo)
	case "curlpipe":
		return retryWithBackoff(func() error {
			return curlpipe.Install(app.DownloadURL, settings.DryRun, repo)
		})
	case "deb":
		return retryWithBackoff(func() error {
			return deb.Install(app.InstallCommand, settings.DryRun, repo)
		})
	case "docker":
		return docker.Install(app, settings.DryRun, repo)
	case "flatpak":
		return flatpak.Install(app.InstallCommand, app.Name, settings.DryRun, repo)
	case "mise":
		return mise.Install(app.InstallCommand, settings.DryRun, repo)
	case "pip":
		return pip.Install(app.InstallCommand, settings.DryRun, repo)
	default:
		log.Error("Unsupported install method", "method", app.InstallMethod)
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
}

// retryWithBackoff retries a function with exponential backoff.
func retryWithBackoff(f func() error) error {
	const maxRetries = 3
	const initialDelay = time.Second

	delay := initialDelay
	for i := 0; i < maxRetries; i++ {
		err := f()
		if err == nil {
			log.Info("Operation succeeded", "attempt", i+1)
			return nil
		}
		log.Warn("Retry failed", "attempt", i+1, "error", err)
		time.Sleep(delay)
		delay *= 2
	}
	log.Error("Max retries exceeded")
	return errors.New("max retries exceeded")
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
				if err := utils.CopyFile(source, destination); err != nil {
					log.Error("Failed to copy file", "source", source, "destination", destination, "error", err)
					return fmt.Errorf("failed to copy file from %s to %s: %v", source, destination, err)
				}
			}
		}
	}

	log.Info("Completed runInstallCommands successfully")
	return nil
}
