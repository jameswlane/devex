package installers

import (
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"

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
)

// InstallApp installs the app based on the InstallMethod field
func InstallApp(app types.AppConfig, dryRun bool, repo repository.Repository) error {
	log.Info(fmt.Sprintf("Installing app %s using method %s", app.Name, app.InstallMethod))

	// Step 1: Handle dependencies
	if err := handleDependencies(app, repo, dryRun); err != nil {
		return fmt.Errorf("failed to handle dependencies for %s: %v", app.Name, err)
	}

	// Step 2: Execute the appropriate installation command
	if err := executeInstallCommand(app, repo, dryRun); err != nil {
		return fmt.Errorf("failed to install %s: %v", app.Name, err)
	}

	return nil
}

// handleDependencies ensures all dependencies are installed
func handleDependencies(app types.AppConfig, repo repository.Repository, dryRun bool) error {
	if len(app.Dependencies) == 0 {
		return nil
	}

	log.Info("Checking dependencies for app", "app", app.Name, "dependencies", app.Dependencies)

	// Batch preload all dependencies from the repository
	dependencySet := make(map[string]bool)
	for _, dep := range app.Dependencies {
		dependencySet[dep] = false
	}

	installedDeps, err := preloadDependenciesFromRepo(dependencySet, repo)
	if err != nil {
		return err
	}

	// Install missing dependencies
	for dep, installed := range installedDeps {
		if !installed {
			log.Info("Installing missing dependency", "dependency", dep)
			dependencyApp := findAppByName(dep)
			if dependencyApp == nil {
				return fmt.Errorf("dependency %s not found in app configurations", dep)
			}
			if err := InstallApp(*dependencyApp, dryRun, repo); err != nil {
				return fmt.Errorf("failed to install dependency %s: %v", dep, err)
			}
		}
	}

	return nil
}

// executeInstallCommand runs the installation logic based on the method
func executeInstallCommand(app types.AppConfig, repo repository.Repository, dryRun bool) error {
	switch app.InstallMethod {
	case "appimage":
		return appimage.Install(app.DownloadURL, app.InstallDir, app.Symlink, app.Name, dryRun, repo)
	case "apt":
		return apt.Install(app.InstallCommand, dryRun, repo)
	case "curlpipe":
		return retryWithBackoff(func() error {
			return curlpipe.Install(app.DownloadURL, dryRun, repo)
		})
	case "deb":
		return retryWithBackoff(func() error {
			return deb.Install(app.InstallCommand, dryRun, repo)
		})
	case "docker":
		return docker.Install(app, dryRun, repo)
	case "flatpak":
		return flatpak.Install(app.InstallCommand, app.Name, dryRun, repo)
	case "mise":
		return mise.Install(app.InstallCommand, dryRun, repo)
	case "pip":
		return pip.Install(app.InstallCommand, dryRun, repo)
	default:
		return fmt.Errorf("unsupported install method: %s", app.InstallMethod)
	}
}

// preloadDependenciesFromRepo checks which dependencies are already installed in the repository
func preloadDependenciesFromRepo(dependencySet map[string]bool, repo repository.Repository) (map[string]bool, error) {
	for dep := range dependencySet {
		exists, err := repo.GetApp(dep)
		if err != nil {
			return nil, fmt.Errorf("failed to check app existence: %v", err)
		}
		dependencySet[dep] = exists
	}
	return dependencySet, nil
}

// findAppByName finds an app by name from the global configuration
func findAppByName(name string) *types.AppConfig {
	var apps []types.AppConfig
	if err := viper.UnmarshalKey("apps", &apps); err != nil {
		log.Warn("Failed to load app configurations", "error", err)
		return nil
	}
	for _, app := range apps {
		if app.Name == name {
			return &app
		}
	}
	return nil
}

// retryWithBackoff retries a function with exponential backoff
func retryWithBackoff(f func() error) error {
	const maxRetries = 3
	const initialDelay = time.Second

	delay := initialDelay
	for i := 0; i < maxRetries; i++ {
		err := f()
		if err == nil {
			return nil
		}
		log.Warn(fmt.Sprintf("Retry %d/%d failed: %v", i+1, maxRetries, err))
		time.Sleep(delay)
		delay *= 2
	}
	return errors.New("max retries exceeded")
}
