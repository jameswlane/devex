package deb

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
)

type DebInstaller struct{}

func New() *DebInstaller {
	return &DebInstaller{}
}

func (d *DebInstaller) Install(command string, repo repository.Repository) error {
	log.Info("Deb Installer: Starting installation", "installCommand", command)

	// Retrieve the app configuration using GetAppInfo
	appConfig, err := config.GetAppInfo(command)
	if err != nil {
		log.Error("Deb Installer: App configuration not found", "installCommand", command, "error", err)
		return fmt.Errorf("app configuration not found: %v", err)
	}

	// Resolve the download URL with the correct architecture
	architecture := runtime.GOARCH
	resolvedURL := strings.ReplaceAll(appConfig.DownloadURL, "%ARCHITECTURE%", architecture)

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(*appConfig)
	if err != nil {
		log.Error("Deb Installer: Failed to check if package is installed", "app", appConfig.Name, "error", err)
		return fmt.Errorf("failed to check if .deb package is installed: %v", err)
	}

	if isInstalled {
		log.Info("Deb Installer: Package already installed, skipping", "app", appConfig.Name)
		return nil
	}

	// Download the .deb file to a temporary directory
	tempDir := os.TempDir()
	fileName := filepath.Join(tempDir, filepath.Base(resolvedURL))
	if err := utilities.DownloadFile(resolvedURL, fileName); err != nil {
		log.Error("Deb Installer: Failed to download .deb file", "url", resolvedURL, "error", err)
		return fmt.Errorf("failed to download .deb file: %v", err)
	}

	defer func() {
		if err := os.Remove(fileName); err != nil {
			log.Warn("Deb Installer: Failed to remove temporary file", "filePath", fileName, "error", err)
		}
	}()

	// Run dpkg -i command
	err = utilities.RunCommand(fmt.Sprintf("sudo dpkg -i %s", fileName))
	if err != nil {
		log.Error("Deb Installer: Failed to install package", "filePath", fileName, "error", err)
		return fmt.Errorf("failed to install .deb package: %v", err)
	}

	// Run apt-get install -f to fix dependencies
	err = utilities.RunCommand("sudo apt-get install -f -y")
	if err != nil {
		log.Error("Deb Installer: Failed to fix dependencies", "filePath", fileName, "error", err)
		return fmt.Errorf("failed to fix dependencies after installing .deb package: %v", err)
	}

	log.Info("Deb Installer: Installation successful", "filePath", fileName)

	// Add to repository
	if err := repo.AddApp(appConfig.Name); err != nil {
		log.Error("Deb Installer: Failed to add package to repository", "app", appConfig.Name, "error", err)
		return fmt.Errorf("failed to add .deb package to repository: %v", err)
	}

	log.Info("Deb Installer: Package added to repository", "app", appConfig.Name)
	return nil
}
