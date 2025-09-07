package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// handleBackup creates timestamped backups of shell configuration files
func (p *ShellPlugin) handleBackup(ctx context.Context, args []string) error {
	fmt.Println("Backing up shell configuration...")

	currentShell := p.DetectCurrentShell()
	if currentShell == "unknown" {
		return fmt.Errorf("could not detect current shell")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine files to backup
	filesToBackup := p.getFilesToBackup(currentShell, homeDir)
	if len(filesToBackup) == 0 {
		return fmt.Errorf("unsupported shell: %s", currentShell)
	}

	// Create a backup directory
	backupPath, err := p.createBackupDirectory(homeDir)
	if err != nil {
		return err
	}

	// Back up each file
	backedUpCount, err := p.backupFiles(filesToBackup, backupPath)
	if err != nil {
		return err
	}

	// Report results
	p.reportBackupResults(backedUpCount, backupPath)

	return nil
}

// getFilesToBackup returns the list of files to back up for the given shell
func (p *ShellPlugin) getFilesToBackup(shell, homeDir string) []string {
	switch shell {
	case "bash":
		return []string{
			filepath.Join(homeDir, ".bashrc"),
			filepath.Join(homeDir, ".bash_profile"),
			filepath.Join(homeDir, ".profile"),
		}
	case "zsh":
		return []string{
			filepath.Join(homeDir, ".zshrc"),
			filepath.Join(homeDir, ".zprofile"),
			filepath.Join(homeDir, ".zshenv"),
		}
	case "fish":
		return []string{
			filepath.Join(homeDir, ".config", "fish", "config.fish"),
			filepath.Join(homeDir, ".config", "fish", "functions"),
		}
	default:
		return []string{}
	}
}

// createBackupDirectory creates a timestamped backup directory
func (p *ShellPlugin) createBackupDirectory(homeDir string) (string, error) {
	backupDir := filepath.Join(homeDir, ".devex", "backups", "shell")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, timestamp)

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	return backupPath, nil
}

// backupFiles backs up each file to the backup directory
func (p *ShellPlugin) backupFiles(filesToBackup []string, backupPath string) (int, error) {
	backedUpCount := 0

	for _, file := range filesToBackup {
		if err := p.backupSingleFile(file, backupPath); err != nil {
			fmt.Printf("Warning: %v\n", err)
			continue
		}
		backedUpCount++
	}

	return backedUpCount, nil
}

// backupSingleFile backs up a single file if it exists
func (p *ShellPlugin) backupSingleFile(file, backupPath string) error {
	// Check if file exists
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil // Skip non-existent files silently
	}

	// Read file content
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	// Write to backup location
	backupFile := filepath.Join(backupPath, filepath.Base(file))
	if err := os.WriteFile(backupFile, content, 0644); err != nil {
		return fmt.Errorf("failed to backup %s: %w", file, err)
	}

	fmt.Printf("✅ Backed up %s\n", file)
	return nil
}

// reportBackupResults reports the backup operation results
func (p *ShellPlugin) reportBackupResults(backedUpCount int, backupPath string) {
	if backedUpCount > 0 {
		fmt.Printf("\n🎉 Backup completed successfully!\n")
		fmt.Printf("📁 %d files backed up to: %s\n", backedUpCount, backupPath)
		fmt.Println("💡 You can restore these files manually if needed")
	} else {
		fmt.Println("\n⚠️  No configuration files found to backup")
		fmt.Println("💡 Run 'shell setup' to create configuration files first")
	}
}
