package utilities

import (
	"os"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

func IsAppInstalled(app types.AppConfig) (bool, error) {
	log.Info("Checking if app is installed", "app", app.Name, "method", app.InstallMethod)

	commands := strings.Fields(app.InstallCommand)

	// Check each component based on the installation method
	for _, cmd := range commands {
		switch app.InstallMethod {
		case "apt":
			if !isAptInstalled(cmd) {
				log.Info("APT package not installed", "package", cmd)
				return false, nil
			}
		case "pip":
			if !isPipInstalled(cmd) {
				log.Info("PIP package not installed", "package", cmd)
				return false, nil
			}
		case "flatpak":
			if !isFlatpakInstalled(cmd) {
				log.Info("Flatpak app not installed", "appID", cmd)
				return false, nil
			}
		case "docker":
			if !isDockerInstalled(cmd) {
				log.Info("Docker container not found", "container", cmd)
				return false, nil
			}
		case "dnf":
			if !isDnfInstalled(cmd) {
				log.Info("DNF package not installed", "package", cmd)
				return false, nil
			}
		case "appimage":
			if !isAppImageInstalled(cmd) {
				log.Info("AppImage not found", "binary", cmd)
				return false, nil
			}
		case "deb":
			if !isDebInstalled(cmd) {
				log.Info("Deb package not found", "command", cmd)
				return false, nil
			}
		case "brew":
			if !isBrewInstalled(cmd) {
				log.Info("Brew package not installed", "package", cmd)
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
	// First try dpkg-query with proper formatting
	command := "dpkg-query -W -f='${Status}' " + packageName
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// If dpkg-query fails, try alternative method
		log.Info("dpkg-query failed, trying alternative check", "package", packageName, "error", err)
		return isAptInstalledAlternative(packageName)
	}
	return strings.Contains(output, "install ok installed")
}

// isAptInstalledAlternative uses dpkg -l as fallback method
func isAptInstalledAlternative(packageName string) bool {
	command := "dpkg -l " + packageName
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Warn("Failed to check APT package with alternative method", "package", packageName, "error", err)
		return false
	}
	// Look for lines starting with 'ii' which indicates installed packages
	return strings.Contains(output, "ii  "+packageName)
}

func isDnfInstalled(packageName string) bool {
	// Use rpm to check if package is installed (both DNF and YUM use RPM backend)
	command := "rpm -q " + packageName
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// rpm -q returns non-zero exit code if package is not installed
		if strings.Contains(output, "not installed") || strings.Contains(output, "is not installed") {
			return false
		}
		// For other errors, log and return false
		log.Warn("Failed to check DNF/RPM package", "package", packageName, "error", err)
		return false
	}
	// If rpm -q succeeds, package is installed
	return true
}

func isPipInstalled(packageName string) bool {
	command := "pip show " + packageName
	_, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Warn("Failed to check PIP package", "package", packageName, "error", err)
		return false
	}
	return true
}

func isFlatpakInstalled(appID string) bool {
	command := "flatpak list --columns=application"
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Warn("Failed to check Flatpak app", "appID", appID, "error", err)
		return false
	}
	return strings.Contains(output, appID)
}

func isDockerInstalled(containerName string) bool {
	// First check if Docker daemon is running
	if !isDockerServiceAvailable() {
		log.Warn("Docker service is not available", "container", containerName)
		return false
	}

	command := "docker ps -a --format {{.Names}}"
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Warn("Failed to check Docker container", "container", containerName, "error", err)
		return false
	}
	return strings.Contains(output, containerName)
}

// isDockerServiceAvailable checks if Docker daemon is running and accessible
func isDockerServiceAvailable() bool {
	// Try to run a simple docker command to check if service is available
	_, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Server.Version}}'")
	return err == nil
}

func isAppImageInstalled(binaryPath string) bool {
	if _, err := os.Stat(binaryPath); err == nil {
		log.Info("AppImage binary found", "binaryPath", binaryPath)
		return true
	} else if os.IsNotExist(err) {
		log.Info("AppImage binary not found", "binaryPath", binaryPath)
		return false
	} else {
		log.Warn("Failed to check AppImage binary", "binaryPath", binaryPath, "error", err)
		return false
	}
}

func isDebInstalled(command string) bool {
	// For deb packages, check if the command is available in PATH
	// This is more reliable than checking package names since .deb files
	// might have different package names than their executable commands

	// First try with current PATH
	_, err := utils.CommandExec.RunShellCommand("which " + command)
	if err == nil {
		log.Info("Deb package command found in PATH", "command", command)
		return true
	}

	// Try with refreshed PATH including common binary locations
	pathCheckCommand := "export PATH=$PATH:/usr/bin:/usr/local/bin:/bin && which " + command
	_, pathErr := utils.CommandExec.RunShellCommand(pathCheckCommand)
	if pathErr == nil {
		log.Info("Deb package command found in extended PATH", "command", command)
		return true
	}

	// Check common installation locations directly
	commonPaths := []string{
		"/usr/bin/" + command,
		"/usr/local/bin/" + command,
		"/bin/" + command,
	}

	for _, path := range commonPaths {
		_, statErr := utils.CommandExec.RunShellCommand("test -x " + path)
		if statErr == nil {
			log.Info("Deb package executable found at path", "command", command, "path", path)
			return true
		}
	}

	// Fallback: check if command supports common help flags (safer approach)
	versionCommand := "export PATH=$PATH:/usr/bin:/usr/local/bin:/bin && (" + command + " --version 2>/dev/null || " + command + " --help 2>/dev/null || false)"
	_, execErr := utils.CommandExec.RunShellCommand(versionCommand)
	if execErr == nil {
		log.Info("Deb package command is executable with extended PATH", "command", command)
		return true
	}

	log.Info("Deb package command not found after exhaustive search", "command", command)
	return false
}

func isBrewInstalled(packageName string) bool {
	// Use brew list to check if package is installed
	command := "brew list " + packageName
	_, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// brew list returns non-zero exit code if package is not installed
		log.Info("Brew package not installed", "package", packageName, "error", err)
		return false
	}
	return true
}
