package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/utils"
)

// getSelectedShell returns the shell name corresponding to the selected index
func (m *SetupModel) getSelectedShell() string {
	if m.selectedShell >= 0 && m.selectedShell < len(m.shells) {
		return m.shells[m.selectedShell]
	}
	return "zsh" // Default fallback
}

// ensureShellInstalled checks if a shell is available and installs it if necessary
func (m *SetupModel) ensureShellInstalled(ctx context.Context, shell string) error {
	if isToolAvailable(shell) {
		log.Info("Shell already available", "shell", shell)
		return nil
	}

	log.Info("Installing shell", "shell", shell)

	// Get shell app from configuration
	allApps := m.settings.GetAllApps()
	for _, app := range allApps {
		if app.Name == shell {
			return installers.InstallCrossPlatformApp(app, m.settings, m.repo)
		}
	}

	log.Warn("Shell not found in configuration, installing via system package manager", "shell", shell)

	// Fallback to system package manager
	installCmd := fmt.Sprintf("sudo apt-get update && sudo apt-get install -y %s", shell)
	output, err := exec.CommandContext(ctx, "bash", "-c", installCmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to install shell via apt", err, "shell", shell, "output", string(output))
		return fmt.Errorf("failed to install %s: %w", shell, err)
	}

	return nil
}

// copyShellConfiguration copies shell-specific configuration files
func (m *SetupModel) copyShellConfiguration(shell string) error {
	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"

	switch shell {
	case "zsh":
		return m.copyZshConfiguration(homeDir, devexDir)
	case "bash":
		return m.copyBashConfiguration(homeDir, devexDir)
	case "fish":
		return m.copyFishConfiguration(homeDir, devexDir)
	default:
		log.Warn("No specific configuration available for shell", "shell", shell)
		return nil
	}
}

// copyZshConfiguration copies zsh configuration files and modules
func (m *SetupModel) copyZshConfiguration(homeDir, devexDir string) error {
	log.Info("Copying zsh configuration files")

	// Copy main zshrc
	srcZshrc := devexDir + "/assets/zsh/zshrc"
	dstZshrc := homeDir + "/.zshrc"

	if err := m.copyFile(srcZshrc, dstZshrc); err != nil {
		return fmt.Errorf("failed to copy .zshrc: %w", err)
	}

	// Create a destination directory for zsh config modules
	zshConfigDir := devexDir + "/defaults/zsh"
	if err := os.MkdirAll(zshConfigDir, DirectoryPermissions); err != nil {
		return fmt.Errorf("failed to create zsh config directory: %w", err)
	}

	// Copy zsh configuration modules
	zshFiles := []string{"aliases", "extra", "init", "oh-my-zsh", "prompt", "rc", "shell", "zplug"}
	for _, file := range zshFiles {
		src := devexDir + "/assets/zsh/zsh/" + file
		dst := zshConfigDir + "/" + file
		if err := m.copyFile(src, dst); err != nil {
			log.Warn("Failed to copy zsh config file", "file", file, "error", err)
		}
	}

	// Copy inputrc
	inputrcSrc := devexDir + "/assets/zsh/inputrc"
	inputrcDst := homeDir + "/.inputrc"
	if err := m.copyFile(inputrcSrc, inputrcDst); err != nil {
		log.Warn("Failed to copy .inputrc", "error", err)
	}

	return nil
}

// copyBashConfiguration copies bash configuration files and modules
func (m *SetupModel) copyBashConfiguration(homeDir, devexDir string) error {
	log.Info("Copying bash configuration files")

	// Copy main bashrc
	srcBashrc := devexDir + "/assets/bash/bashrc"
	dstBashrc := homeDir + "/.bashrc"

	if err := m.copyFile(srcBashrc, dstBashrc); err != nil {
		return fmt.Errorf("failed to copy .bashrc: %w", err)
	}

	// Create a destination directory for bash config modules
	bashConfigDir := devexDir + "/defaults/bash"
	if err := os.MkdirAll(bashConfigDir, DirectoryPermissions); err != nil {
		return fmt.Errorf("failed to create bash config directory: %w", err)
	}

	// Copy bash configuration modules
	bashFiles := []string{"aliases", "extra", "init", "oh-my-bash", "prompt", "rc", "shell"}
	for _, file := range bashFiles {
		src := devexDir + "/assets/bash/bash/" + file
		dst := bashConfigDir + "/" + file
		if err := m.copyFile(src, dst); err != nil {
			log.Warn("Failed to copy bash config file", "file", file, "error", err)
		}
	}

	// Copy inputrc
	inputrcSrc := devexDir + "/assets/bash/inputrc"
	inputrcDst := homeDir + "/.inputrc"
	if err := m.copyFile(inputrcSrc, inputrcDst); err != nil {
		log.Warn("Failed to copy .inputrc", "error", err)
	}

	// Copy bash_profile if it exists
	bashProfileSrc := devexDir + "/assets/bash/bash_profile"
	bashProfileDst := homeDir + "/.bash_profile"
	if err := m.copyFile(bashProfileSrc, bashProfileDst); err != nil {
		log.Warn("Failed to copy .bash_profile", "error", err)
	}

	return nil
}

// copyFishConfiguration copies fish configuration files and modules
func (m *SetupModel) copyFishConfiguration(homeDir, devexDir string) error {
	log.Info("Copying fish configuration files")

	// Create fish config directory
	fishConfigDir := homeDir + "/.config/fish"
	if err := os.MkdirAll(fishConfigDir, DirectoryPermissions); err != nil {
		return fmt.Errorf("failed to create fish config directory: %w", err)
	}

	// Copy main config.fish
	srcConfig := devexDir + "/assets/fish/config.fish"
	dstConfig := fishConfigDir + "/config.fish"

	if err := m.copyFile(srcConfig, dstConfig); err != nil {
		return fmt.Errorf("failed to copy config.fish: %w", err)
	}

	// Create a destination directory for fish config modules
	fishDefaultsDir := devexDir + "/defaults/fish"
	if err := os.MkdirAll(fishDefaultsDir, DirectoryPermissions); err != nil {
		return fmt.Errorf("failed to create fish defaults directory: %w", err)
	}

	// Copy fish configuration modules
	fishFiles := []string{"aliases", "shell", "init", "prompt"}
	for _, file := range fishFiles {
		src := devexDir + "/assets/fish/" + file
		dst := fishDefaultsDir + "/" + file
		if err := m.copyFile(src, dst); err != nil {
			log.Warn("Failed to copy fish config file", "file", file, "error", err)
		}
	}

	// Copy fish modules from the fish subdirectory if they exist
	fishSubFiles := []string{"extra", "oh-my-fish"}
	for _, file := range fishSubFiles {
		src := devexDir + "/assets/fish/fish/" + file
		dst := fishDefaultsDir + "/" + file
		if err := m.copyFile(src, dst); err != nil {
			log.Warn("Failed to copy fish config file", "file", file, "error", err)
		}
	}

	return nil
}

// switchToShell changes the user's default shell to the specified shell
func (m *SetupModel) switchToShell(ctx context.Context, shell string) error {
	shellPath, err := exec.LookPath(shell)
	if err != nil {
		return fmt.Errorf("%s not found: %w", shell, err)
	}

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		return fmt.Errorf("unable to determine current user")
	}

	// Get current shell for comparison
	currentShell, err := utils.GetUserShell(currentUser)
	if err != nil {
		log.Warn("Could not detect current shell", "error", err, "user", currentUser)
	} else {
		// Check if the current shell matches the desired shell (compare shell names, not full paths)
		currentShellName := filepath.Base(currentShell)
		selectedShellName := filepath.Base(shellPath)

		if currentShellName == selectedShellName {
			log.Info("User is already using the selected shell", "shell", shell, "currentPath", currentShell, "selectedPath", shellPath, "user", currentUser)
			m.shellSwitched = false // No switch occurred
			return nil
		}
		log.Info("Current shell differs from selected", "current", currentShell, "selected", shellPath, "user", currentUser)
	}

	log.Info("Switching to shell", "shell", shell, "path", shellPath, "user", currentUser)

	// Use secure shell change execution with proper validation
	if err := ExecuteSecureShellChange(ctx, shellPath, currentUser); err != nil {
		m.shellSwitched = false // Switch failed
		return err
	}

	m.shellSwitched = true // Switch succeeded
	log.Info("Successfully switched shell", "shell", shell)
	return nil
}

// copyFile copies a file from source to destination with security validation
func (m *SetupModel) copyFile(src, dst string) error {
	// Validate paths to prevent directory traversal
	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"

	// Validate source path is within devex assets directory
	if err := ValidatePath(src, devexDir); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	// Validate destination path is within home directory
	if err := ValidatePath(dst, homeDir); err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer func(sourceFile *os.File) {
		_ = sourceFile.Close()
	}(sourceFile)

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer func(destFile *os.File) {
		_ = destFile.Close()
	}(destFile)

	_, err = sourceFile.WriteTo(destFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// isToolAvailable checks if a tool is available in the system PATH
func isToolAvailable(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}
