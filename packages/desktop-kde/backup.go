package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BackupManager handles KDE settings backup and restore
type BackupManager struct{}

// NewBackupManager creates a new backup manager instance
func NewBackupManager() *BackupManager {
	return &BackupManager{}
}

// CreateBackup creates a backup of KDE settings
func (bm *BackupManager) CreateBackup(ctx context.Context, args []string) error {
	backupDir := bm.getBackupDirectory(args)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := bm.generateTimestamp()
	backupFile := filepath.Join(backupDir, fmt.Sprintf("kde-settings-%s.tar.gz", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Backup KDE configuration files
	if err := bm.createConfigBackup(backupFile); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Printf("✓ Backup created successfully: %s\n", backupFile)
	return nil
}

// RestoreBackup restores KDE settings from backup
func (bm *BackupManager) RestoreBackup(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current KDE settings!")

	if !bm.confirmRestore() {
		fmt.Println("Restore cancelled.")
		return nil
	}

	// Extract backup to temporary directory
	tempDir, err := bm.extractBackup(backupFile)
	if err != nil {
		return fmt.Errorf("failed to extract backup: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Printf("Warning: Failed to clean up temporary directory: %v\n", err)
		}
	}()

	// Restore configuration files
	if err := bm.restoreConfigFiles(tempDir); err != nil {
		return fmt.Errorf("failed to restore configuration: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to restart KDE Plasma or log out for all changes to take effect.")
	return nil
}

// ListBackups lists available backup files
func (bm *BackupManager) ListBackups(ctx context.Context, args []string) error {
	backupDir := bm.getBackupDirectory(args)

	fmt.Printf("Backup files in %s:\n", backupDir)

	if entries, err := os.ReadDir(backupDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), "kde-settings-") {
				info, _ := entry.Info()
				fmt.Printf("  - %s (%s)\n", entry.Name(), info.ModTime().Format("2006-01-02 15:04:05"))
			}
		}
	} else {
		fmt.Printf("No backup directory found or unable to read: %v\n", err)
	}

	return nil
}

// createConfigBackup creates a backup of KDE configuration files
func (bm *BackupManager) createConfigBackup(backupFile string) error {
	homeDir := os.Getenv("HOME")
	configDir := filepath.Join(homeDir, ".config")
	localDir := filepath.Join(homeDir, ".local/share")

	// KDE-specific config files and directories to backup
	items := []string{
		// Configuration files
		filepath.Join(configDir, "kdeglobals"),
		filepath.Join(configDir, "kwinrc"),
		filepath.Join(configDir, "plasmarc"),
		filepath.Join(configDir, "plasmashellrc"),
		filepath.Join(configDir, "kscreenlockerrc"),
		filepath.Join(configDir, "kcminputrc"),

		// Application configs
		filepath.Join(configDir, "dolphinrc"),
		filepath.Join(configDir, "konsolerc"),
		filepath.Join(configDir, "kate*"),

		// Plasma specific
		filepath.Join(configDir, "plasma*"),
		filepath.Join(localDir, "plasma*"),
		filepath.Join(localDir, "kactivitymanagerd"),
	}

	// Create tar command with existing files only
	existingItems := []string{}
	for _, item := range items {
		if _, err := os.Stat(item); err == nil {
			existingItems = append(existingItems, item)
		}
	}

	if len(existingItems) == 0 {
		return fmt.Errorf("no KDE configuration files found to backup")
	}

	// Create tar archive
	args := append([]string{"czf", backupFile}, existingItems...)
	cmd := exec.Command("tar", args...)
	return cmd.Run()
}

// extractBackup extracts a backup file to a temporary directory
func (bm *BackupManager) extractBackup(backupFile string) (string, error) {
	tempDir, err := os.MkdirTemp("", "kde-restore-*")
	if err != nil {
		return "", err
	}

	cmd := exec.Command("tar", "xzf", backupFile, "-C", tempDir)
	if err := cmd.Run(); err != nil {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Printf("Warning: Failed to clean up temporary directory: %v\n", err)
		}
		return "", err
	}

	return tempDir, nil
}

// restoreConfigFiles restores configuration files from extracted backup
func (bm *BackupManager) restoreConfigFiles(tempDir string) error {
	homeDir := os.Getenv("HOME")

	// Walk through extracted files and restore them
	return filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path from temp directory
		relPath, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}

		// Determine target path (removing the temp dir prefix)
		targetPath := filepath.Join(homeDir, relPath)

		// Ensure target directory exists
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}

		// Copy file
		return bm.copyFile(path, targetPath)
	})
}

// copyFile copies a file from src to dst
func (bm *BackupManager) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0644)
}

// getBackupDirectory determines the backup directory to use
func (bm *BackupManager) getBackupDirectory(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return filepath.Join(os.Getenv("HOME"), ".devex", "backups", "kde")
}

// generateTimestamp creates a timestamp string for backup files
func (bm *BackupManager) generateTimestamp() string {
	return strings.ReplaceAll(
		strings.ReplaceAll(
			strings.Split(time.Now().Format(time.RFC3339), "T")[0],
			":", "-"),
		" ", "_")
}

// confirmRestore prompts user for confirmation before restore
func (bm *BackupManager) confirmRestore() bool {
	fmt.Print("Continue? [y/N]: ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}

	return strings.ToLower(response) == "y"
}
