package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// ValidatePath validates that targetPath is within basePath to prevent directory traversal
func ValidatePath(targetPath, basePath string) error {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve base path: %w", err)
	}

	var absTarget string
	if filepath.IsAbs(targetPath) {
		absTarget, err = filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("failed to resolve target path: %w", err)
		}
	} else {
		// For relative paths, resolve them relative to the base path
		absTarget, err = filepath.Abs(filepath.Join(absBase, targetPath))
		if err != nil {
			return fmt.Errorf("failed to resolve target path: %w", err)
		}
	}

	if !strings.HasPrefix(absTarget, absBase) {
		return fmt.Errorf("path traversal detected: %s is outside base directory %s", absTarget, absBase)
	}

	return nil
}

// copyFile copies a file from src to dst
func (m *SetupModel) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Get source file info to preserve permissions
	srcInfo, err := srcFile.Stat()
	if err == nil {
		err = dstFile.Chmod(srcInfo.Mode())
		if err != nil {
			log.Warn("Failed to preserve file permissions", "file", dst, "error", err)
		}
	}

	return nil
}

// copyThemeFiles copies theme assets including backgrounds, neovim colorschemes, and application themes
func (m *SetupModel) copyThemeFiles() error {
	log.Info("Copying theme files and configurations")

	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"
	assetsDir := m.detectAssetsDir()

	// Create necessary directories
	if err := os.MkdirAll(homeDir+"/.config", DirectoryPermissions); err != nil {
		return fmt.Errorf("failed to create .config directory: %w", err)
	}

	// Copy background images
	if err := m.copyThemeDirectory(assetsDir+"/themes/backgrounds", homeDir+"/.local/share/backgrounds"); err != nil {
		log.Warn("Failed to copy background images", "error", err)
	}

	// Copy Alacritty themes
	alacrittyConfigDir := homeDir + "/.config/alacritty"
	if err := os.MkdirAll(alacrittyConfigDir, DirectoryPermissions); err != nil {
		log.Warn("Failed to create Alacritty config directory", "error", err)
	} else {
		if err := m.copyThemeDirectory(assetsDir+"/themes/alacritty", alacrittyConfigDir+"/themes"); err != nil {
			log.Warn("Failed to copy Alacritty themes", "error", err)
		}
	}

	// Copy Neovim colorschemes
	neovimConfigDir := homeDir + "/.config/nvim"
	if err := os.MkdirAll(neovimConfigDir+"/colors", DirectoryPermissions); err != nil {
		log.Warn("Failed to create Neovim config directory", "error", err)
	} else {
		if err := m.copyThemeDirectory(assetsDir+"/themes/neovim", neovimConfigDir+"/colors"); err != nil {
			log.Warn("Failed to copy Neovim colorschemes", "error", err)
		}
	}

	// Copy Zellij themes
	zellijConfigDir := homeDir + "/.config/zellij"
	if err := os.MkdirAll(zellijConfigDir+"/themes", DirectoryPermissions); err != nil {
		log.Warn("Failed to create Zellij config directory", "error", err)
	} else {
		if err := m.copyThemeDirectory(assetsDir+"/themes/zellij", zellijConfigDir+"/themes"); err != nil {
			log.Warn("Failed to copy Zellij themes", "error", err)
		}
	}

	// Copy Oh My Posh themes
	ompConfigDir := homeDir + "/.config/oh-my-posh"
	if err := os.MkdirAll(ompConfigDir+"/themes", DirectoryPermissions); err != nil {
		log.Warn("Failed to create Oh My Posh config directory", "error", err)
	} else {
		if err := m.copyThemeDirectory(assetsDir+"/themes/oh-my-posh", ompConfigDir+"/themes"); err != nil {
			log.Warn("Failed to copy Oh My Posh themes", "error", err)
		}
	}

	// Copy Typora themes
	typoraThemeDir := homeDir + "/.config/Typora/themes"
	if err := os.MkdirAll(typoraThemeDir, DirectoryPermissions); err != nil {
		log.Warn("Failed to create Typora themes directory", "error", err)
	} else {
		if err := m.copyThemeDirectory(assetsDir+"/themes/typora", typoraThemeDir); err != nil {
			log.Warn("Failed to copy Typora themes", "error", err)
		}
	}

	// Copy GNOME theme scripts (make them executable)
	gnomeScriptDir := devexDir + "/themes/gnome"
	if err := os.MkdirAll(gnomeScriptDir, DirectoryPermissions); err != nil {
		log.Warn("Failed to create GNOME scripts directory", "error", err)
	} else {
		if err := m.copyThemeDirectory(assetsDir+"/themes/gnome", gnomeScriptDir); err != nil {
			log.Warn("Failed to copy GNOME theme scripts", "error", err)
		} else {
			// Make scripts executable
			m.makeScriptsExecutable(gnomeScriptDir)
		}
	}

	log.Info("Theme files copied successfully")
	return nil
}

// copyAppConfigFiles copies application configuration files and defaults
func (m *SetupModel) copyAppConfigFiles() error {
	log.Info("Copying application configuration files")

	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"
	assetsDir := m.detectAssetsDir()

	// Create defaults directory in devex
	defaultsDir := devexDir + "/defaults"
	if err := os.MkdirAll(defaultsDir, DirectoryPermissions); err != nil {
		return fmt.Errorf("failed to create defaults directory: %w", err)
	}

	// Copy default application configurations
	if err := m.copyThemeDirectory(assetsDir+"/defaults", defaultsDir); err != nil {
		log.Warn("Failed to copy default application configurations", "error", err)
	}

	// Copy XCompose file for special characters
	xcomposeFile := assetsDir + "/defaults/xcompose"
	if err := m.copyFile(xcomposeFile, homeDir+"/.XCompose"); err != nil {
		log.Warn("Failed to copy .XCompose file", "error", err)
	}

	log.Info("Application configuration files copied successfully")
	return nil
}

// setupGitConfiguration applies git configuration using user's name and email
func (m *SetupModel) setupGitConfiguration(ctx context.Context) error {
	log.Info("Setting up git configuration", "name", m.git.gitFullName, "email", m.git.gitEmail)

	// Set git user name
	if m.git.gitFullName != "" {
		if _, err := exec.CommandContext(ctx, "git", "config", "--global", "user.name", m.git.gitFullName).CombinedOutput(); err != nil {
			log.Warn("Failed to set git user name", "error", err)
		} else {
			log.Info("Git user name set successfully", "name", m.git.gitFullName)
		}
	}

	// Set git user email
	if m.git.gitEmail != "" {
		if _, err := exec.CommandContext(ctx, "git", "config", "--global", "user.email", m.git.gitEmail).CombinedOutput(); err != nil {
			log.Warn("Failed to set git user email", "error", err)
		} else {
			log.Info("Git user email set successfully", "email", m.git.gitEmail)
		}
	}

	// Apply additional git configuration from system.yaml
	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"
	systemConfigPath := devexDir + "/config/system.yaml"

	// TODO: Use tool-git plugin for additional git configuration
	log.Info("Additional git configuration would be handled by tool-git plugin", "configPath", systemConfigPath)

	return nil
}

// copyThemeDirectory copies all files from source directory to destination directory
func (m *SetupModel) copyThemeDirectory(srcDir, dstDir string) error {
	// Validate paths to prevent directory traversal
	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"

	// Validate source directory is within devex assets directory
	if err := ValidatePath(srcDir, devexDir); err != nil {
		return fmt.Errorf("invalid source directory: %w", err)
	}

	// Validate destination directory is within home directory
	if err := ValidatePath(dstDir, homeDir); err != nil {
		return fmt.Errorf("invalid destination directory: %w", err)
	}

	// Check if source directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", srcDir)
	}

	// Create destination directory
	if err := os.MkdirAll(dstDir, DirectoryPermissions); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each file
	for _, entry := range entries {
		if !entry.IsDir() {
			// Validate filename to prevent malicious filenames
			filename := entry.Name()
			if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
				log.Warn("Skipping file with invalid name", "filename", filename)
				continue
			}

			srcPath := filepath.Join(srcDir, filename)
			dstPath := filepath.Join(dstDir, filename)

			if err := m.copyFile(srcPath, dstPath); err != nil {
				log.Warn("Failed to copy theme file", "src", srcPath, "dst", dstPath, "error", err)
			}
		}
	}

	return nil
}

// makeScriptsExecutable makes shell scripts executable
func (m *SetupModel) makeScriptsExecutable(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Warn("Failed to read directory for making scripts executable", "dir", dir, "error", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sh") {
			scriptPath := filepath.Join(dir, entry.Name())
			if err := os.Chmod(scriptPath, ExecutablePermissions); err != nil {
				log.Warn("Failed to make script executable", "script", scriptPath, "error", err)
			}
		}
	}
}
