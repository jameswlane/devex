package deb

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type DebInstaller struct{}

func New() *DebInstaller {
	return &DebInstaller{}
}

func (d *DebInstaller) Install(command string, repo types.Repository) error {
	log.Info("Deb Installer: Starting installation", "installCommand", command)

	// Retrieve the app configuration
	appConfig, err := config.GetAppInfo(command)
	if err != nil {
		log.Error("App configuration not found", err, "installCommand", command)
		return fmt.Errorf("app configuration not found: %w", err)
	}

	// Resolve the download URL with the correct architecture
	architecture := runtime.GOARCH
	resolvedURL := strings.ReplaceAll(appConfig.DownloadURL, "%ARCHITECTURE%", architecture)

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(*appConfig)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "app", appConfig.Name)
		return fmt.Errorf("failed to check if .deb package is installed: %w", err)
	}

	if isInstalled {
		log.Info("Package already installed, skipping", "app", appConfig.Name)
		return nil
	}

	// Download the .deb file to a temporary directory
	tempDir := fs.GetTempDir("")
	fileName := filepath.Join(tempDir, filepath.Base(resolvedURL))
	if err := utils.DownloadFile(resolvedURL, fileName); err != nil {
		log.Error("Failed to download .deb file", err, "url", resolvedURL)
		return fmt.Errorf("failed to download .deb file: %w", err)
	}
	defer func() {
		if err := fs.Remove(fileName); err != nil {
			log.Warn("Failed to remove temporary file", err, "filePath", fileName)
		}
	}()

	// Run dpkg -i command
	installCommand := fmt.Sprintf("sudo dpkg -i %s", fileName)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package", err, "filePath", fileName, "command", installCommand)
		return fmt.Errorf("failed to install .deb package '%s': %w", fileName, err)
	}

	// Run apt-get install -f to fix dependencies
	fixDependenciesCommand := "sudo apt-get install -f -y"
	if _, err := utils.CommandExec.RunShellCommand(fixDependenciesCommand); err != nil {
		log.Error("Failed to fix dependencies", err, "filePath", fileName, "command", fixDependenciesCommand)
		return fmt.Errorf("failed to fix dependencies after installing .deb package '%s': %w", fileName, err)
	}

	log.Info("Deb package installed successfully", "app", appConfig.Name)

	// Add to repository
	if err := repo.AddApp(appConfig.Name); err != nil {
		log.Error("Failed to add package to repository", err, "app", appConfig.Name)
		return fmt.Errorf("failed to add .deb package to repository: %w", err)
	}

	log.Info("Package added to repository successfully", "app", appConfig.Name)
	return nil
}
