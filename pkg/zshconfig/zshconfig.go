package zshconfig

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/fileutils"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallZSH installs zsh and zplug
func InstallZSH() error {
	// Install zsh and zplug
	cmd := exec.Command("sudo", "apt", "install", "-y", "zsh", "zplug")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install zsh and zplug: %v", err)
	}
	fmt.Println("zsh and zplug installed successfully")
	return nil
}

func InstallZSHConfig() error {
	zshrcPath, err := config.LoadCustomOrDefaultFile(".zshrc", "zsh")
	if err != nil {
		return fmt.Errorf("failed to load zsh config: %v", err)
	}

	err = fileutils.CopyFile(zshrcPath, filepath.Join(os.Getenv("HOME"), ".zshrc"))
	if err != nil {
		return fmt.Errorf("failed to copy zsh config: %v", err)
	}

	fmt.Println(".zshrc installed successfully")
	return nil
}

// BackupAndCopyZSHConfig backs up existing .zshrc and .inputrc files, then copies the new ones
func BackupAndCopyZSHConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %v", err)
	}

	// Backup and copy .zshrc
	if err := backupAndCopyFile(homeDir, ".zshrc", "~/.local/share/devex/configs/zshrc"); err != nil {
		return err
	}

	// Backup and copy .inputrc
	if err := backupAndCopyFile(homeDir, ".inputrc", "~/.local/share/devex/configs/inputrc"); err != nil {
		return err
	}

	// Source the new shell configuration
	cmd := exec.Command("source", "~/.local/share/devex/defaults/zsh/shell")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to source the new shell configuration: %v", err)
	}

	fmt.Println("Shell configuration sourced successfully")
	return nil
}

// backupAndCopyFile backs up the original file and copies the new one
func backupAndCopyFile(homeDir, filename, sourcePath string) error {
	destFilePath := filepath.Join(homeDir, filename)

	// Check if the destination file exists and back it up
	if _, err := os.Stat(destFilePath); err == nil {
		backupFilePath := destFilePath + ".bak"
		if err := os.Rename(destFilePath, backupFilePath); err != nil {
			return fmt.Errorf("failed to backup %s: %v", filename, err)
		}
		fmt.Printf("Backed up %s to %s\n", filename, backupFilePath)
	}

	// Copy the new config file
	cmd := exec.Command("cp", sourcePath, destFilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy %s: %v", filename, err)
	}

	fmt.Printf("Copied new %s to %s\n", filename, destFilePath)
	return nil
}
