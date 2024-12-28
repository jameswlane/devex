package utilities

import (
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/types"
)

func IsAppInstalled(app types.AppConfig) (bool, error) {
	log.Info("Checking if app is installed", "app", app.Name, "method", app.InstallMethod)

	// Split the install_command into individual components
	commands := strings.Fields(app.InstallCommand)

	// Check each component based on the installation method
	for _, cmd := range commands {
		switch app.InstallMethod {
		case "apt":
			installed := isAptInstalled(cmd)
			if !installed {
				log.Info("APT package not installed", "package", cmd)
				return false, nil
			}
		case "pip":
			installed := isPipInstalled(cmd)
			if !installed {
				log.Info("PIP package not installed", "package", cmd)
				return false, nil
			}
		case "flatpak":
			installed := isFlatpakInstalled(cmd)
			if !installed {
				log.Info("Flatpak app not installed", "appID", cmd)
				return false, nil
			}
		case "docker":
			installed := isDockerInstalled(cmd)
			if !installed {
				log.Info("Docker container not found", "container", cmd)
				return false, nil
			}
		case "appimage":
			installed := isAppImageInstalled(cmd)
			if !installed {
				log.Info("AppImage not found", "binary", cmd)
				return false, nil
			}
		default:
			log.Warn("Unknown install method, skipping check", "method", app.InstallMethod)
			return false, nil
		}
	}

	log.Info("All components are installed", "app", app.Name)
	return true, nil
}

func isAptInstalled(packageName string) bool {
	cmd := exec.Command("dpkg-query", "-W", "-f=${Status}", packageName)
	output, err := cmd.Output()
	if err != nil {
		log.Warn("Failed to check APT package", "package", packageName, "error", err)
		return false
	}
	// Check for "install ok installed"
	return strings.Contains(string(output), "install ok installed")
}

func isPipInstalled(packageName string) bool {
	cmd := exec.Command("pip", "show", packageName)
	err := cmd.Run()
	return err == nil
}

func isFlatpakInstalled(appID string) bool {
	cmd := exec.Command("flatpak", "list", "--columns=application")
	output, err := cmd.Output()
	if err != nil {
		log.Warn("Failed to check Flatpak app", "appID", appID, "error", err)
		return false
	}
	// Check if the appID is in the list of installed apps
	return strings.Contains(string(output), appID)
}

func isDockerInstalled(containerName string) bool {
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		log.Warn("Failed to check Docker container", "container", containerName, "error", err)
		return false
	}
	// Check if the container name is in the list of existing containers
	return strings.Contains(string(output), containerName)
}

func isAppImageInstalled(binaryPath string) bool {
	if _, err := os.Stat(binaryPath); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return false
}
