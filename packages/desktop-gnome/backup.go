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

// BackupManager handles GNOME settings backup and restore
type BackupManager struct{}

// NewBackupManager creates a new backup manager instance
func NewBackupManager() *BackupManager {
	return &BackupManager{}
}

// CreateBackup creates a backup of GNOME settings
func (bm *BackupManager) CreateBackup(ctx context.Context, args []string) error {
	backupDir := bm.getBackupDirectory(args)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := bm.generateTimestamp()
	backupFile := filepath.Join(backupDir, fmt.Sprintf("gnome-settings-%s.conf", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Use dconf to dump settings
	cmd := exec.Command("dconf", "dump", "/")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to dump dconf settings: %w", err)
	}

	// Write to file
	if err := os.WriteFile(backupFile, output, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	fmt.Printf("✓ Backup created successfully: %s\n", backupFile)
	return nil
}

// RestoreBackup restores GNOME settings from backup
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
	fmt.Println("WARNING: This will overwrite your current GNOME settings!")

	if !bm.confirmRestore() {
		fmt.Println("Restore cancelled.")
		return nil
	}

	// Read backup file
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Use dconf to load settings
	cmd := exec.Command("dconf", "load", "/")
	cmd.Stdin = strings.NewReader(string(data))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore dconf settings: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to log out and back in for all changes to take effect.")
	return nil
}

// ListBackups lists available backup files
func (bm *BackupManager) ListBackups(ctx context.Context, args []string) error {
	backupDir := bm.getBackupDirectory(args)

	fmt.Printf("Backup files in %s:\n", backupDir)

	if entries, err := os.ReadDir(backupDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), "gnome-settings-") {
				info, _ := entry.Info()
				fmt.Printf("  - %s (%s)\n", entry.Name(), info.ModTime().Format("2006-01-02 15:04:05"))
			}
		}
	} else {
		fmt.Printf("No backup directory found or unable to read: %v\n", err)
	}

	return nil
}

// getBackupDirectory determines the backup directory to use
func (bm *BackupManager) getBackupDirectory(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return filepath.Join(os.Getenv("HOME"), ".devex", "backups", "gnome")
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
