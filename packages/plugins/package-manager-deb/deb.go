package deb

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
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

	// Execute post-install commands if they exist
	if err := executePostInstallCommands(*appConfig); err != nil {
		log.Warn("Post-install commands failed - installation succeeded but configuration may be incomplete",
			"app", appConfig.Name,
			"error", err,
			"suggestion", "Check the application configuration or run post-install commands manually")
		// Don't fail the installation for post-install command failures
	}

	// Add to repository
	if err := repo.AddApp(appConfig.Name); err != nil {
		log.Error("Failed to add package to repository", err, "app", appConfig.Name)
		return fmt.Errorf("failed to add .deb package to repository: %w", err)
	}

	log.Info("Package added to repository successfully", "app", appConfig.Name)
	return nil
}

// Uninstall removes packages using dpkg
func (d *DebInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Deb Installer: Starting uninstallation", "installCommand", command)

	// Retrieve the app configuration
	appConfig, err := config.GetAppInfo(command)
	if err != nil {
		log.Error("App configuration not found", err, "installCommand", command)
		return fmt.Errorf("app configuration not found: %w", err)
	}

	// Check if the package is installed
	isInstalled, err := d.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "app", appConfig.Name)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "app", appConfig.Name)
		return nil
	}

	// Run dpkg --remove command
	uninstallCommand := fmt.Sprintf("sudo dpkg --remove %s", appConfig.Name)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall package", err, "app", appConfig.Name, "command", uninstallCommand)
		return fmt.Errorf("failed to uninstall .deb package '%s': %w", appConfig.Name, err)
	}

	log.Info("Deb package uninstalled successfully", "app", appConfig.Name)

	// Remove from repository
	if err := repo.DeleteApp(appConfig.Name); err != nil {
		log.Error("Failed to remove package from repository", err, "app", appConfig.Name)
		return fmt.Errorf("failed to remove package from repository: %w", err)
	}

	log.Info("Package removed from repository successfully", "app", appConfig.Name)
	return nil
}

// IsInstalled checks if a package is installed using dpkg
func (d *DebInstaller) IsInstalled(command string) (bool, error) {
	// Retrieve the app configuration to get the actual package name
	appConfig, err := config.GetAppInfo(command)
	if err != nil {
		log.Error("App configuration not found", err, "installCommand", command)
		return false, fmt.Errorf("app configuration not found: %w", err)
	}

	// Use dpkg-query to check if package is installed
	checkCommand := fmt.Sprintf("dpkg-query -W -f='${Status}' %s 2>/dev/null", appConfig.Name)
	output, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err != nil {
		// dpkg-query returns non-zero exit code if package is not installed
		return false, nil
	}

	// Check if the package is installed and configured properly
	return strings.Contains(output, "install ok installed"), nil
}

// executePostInstallCommands executes any post-install commands specified in the app configuration
func executePostInstallCommands(appConfig types.AppConfig) error {
	if len(appConfig.PostInstall) == 0 {
		log.Debug("No post-install commands to execute", "app", appConfig.Name)
		return nil
	}

	log.Info("Executing post-install commands", "app", appConfig.Name, "commandCount", len(appConfig.PostInstall))

	for i, cmd := range appConfig.PostInstall {
		log.Debug("Executing post-install command", "app", appConfig.Name, "step", i+1, "command", cmd.Shell)

		if cmd.Shell != "" {
			if _, err := utils.CommandExec.RunShellCommand(cmd.Shell); err != nil {
				log.Warn("Post-install shell command failed", "app", appConfig.Name, "command", cmd.Shell, "step", i+1, "error", err)
				return fmt.Errorf("post-install command failed at step %d/%d (command: %s): %w", i+1, len(appConfig.PostInstall), cmd.Shell, err)
			}
		}

		if cmd.Sleep > 0 {
			log.Debug("Post-install sleep", "app", appConfig.Name, "duration", cmd.Sleep)
			// Note: Sleep duration should be handled by the command execution framework
			// For now, we'll skip sleep commands in post-install
		}
	}

	log.Info("Post-install commands completed successfully", "app", appConfig.Name)
	return nil
}
