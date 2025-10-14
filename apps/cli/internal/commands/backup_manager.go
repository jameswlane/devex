package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// BackupManager handles backup and restore operations for uninstall operations
type BackupManager struct {
	backupDir string
	repo      types.Repository
}

// BackupEntry represents a backup entry
type BackupEntry struct {
	AppName      string    `json:"app_name"`
	BackupPath   string    `json:"backup_path"`
	CreatedAt    time.Time `json:"created_at"`
	PackageInfo  string    `json:"package_info"`
	ConfigFiles  []string  `json:"config_files"`
	DataFiles    []string  `json:"data_files"`
	Dependencies []string  `json:"dependencies"`
	Services     []string  `json:"services"`
}

// NewBackupManager creates a new backup manager instance
func NewBackupManager(repo types.Repository) *BackupManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to temp directory
		homeDir = "/tmp"
	}

	backupDir := filepath.Join(homeDir, ".devex", "backups")

	return &BackupManager{
		backupDir: backupDir,
		repo:      repo,
	}
}

// CreateBackup creates a backup of an application before uninstalling
func (bm *BackupManager) CreateBackup(ctx context.Context, app *types.AppConfig) (*BackupEntry, error) {
	log.Info("Creating backup for app", "app", app.Name)

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(bm.backupDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create timestamped backup subdirectory
	timestamp := time.Now().Format("20060102_150405")
	appBackupDir := filepath.Join(bm.backupDir, fmt.Sprintf("%s_%s", app.Name, timestamp))

	if err := os.MkdirAll(appBackupDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create app backup directory: %w", err)
	}

	backup := &BackupEntry{
		AppName:     app.Name,
		BackupPath:  appBackupDir,
		CreatedAt:   time.Now(),
		ConfigFiles: []string{},
		DataFiles:   []string{},
		Services:    getAppServicesForUninstall(app),
	}

	// Get package information
	packageInfo, err := bm.getPackageInfo(ctx, app)
	if err != nil {
		log.Warn("Failed to get package info for backup", "app", app.Name, "error", err)
	} else {
		backup.PackageInfo = packageInfo
	}

	// Get dependencies
	dm := NewDependencyManager(bm.repo)
	deps, err := dm.GetDependents(ctx, app.Name)
	if err != nil {
		log.Warn("Failed to get dependencies for backup", "app", app.Name, "error", err)
	} else {
		backup.Dependencies = deps
	}

	// Backup configuration files
	if err := bm.backupConfigFiles(ctx, app, appBackupDir, backup); err != nil {
		log.Warn("Failed to backup config files", "app", app.Name, "error", err)
	}

	// Backup data files
	if err := bm.backupDataFiles(ctx, app, appBackupDir, backup); err != nil {
		log.Warn("Failed to backup data files", "app", app.Name, "error", err)
	}

	// Save backup metadata
	if err := bm.saveBackupMetadata(backup); err != nil {
		return nil, fmt.Errorf("failed to save backup metadata: %w", err)
	}

	log.Info("Backup created successfully", "app", app.Name, "path", appBackupDir)
	return backup, nil
}

// getPackageInfo gets package information for backup
func (bm *BackupManager) getPackageInfo(ctx context.Context, app *types.AppConfig) (string, error) {
	// Try different package managers to get package info
	if _, err := exec.LookPath("dpkg"); err == nil {
		// Debian/Ubuntu
		cmd := exec.CommandContext(ctx, "dpkg", "-l", app.Name)
		output, err := cmd.Output()
		if err == nil {
			return string(output), nil
		}
	}

	if _, err := exec.LookPath("rpm"); err == nil {
		// Red Hat/SUSE
		cmd := exec.CommandContext(ctx, "rpm", "-qi", app.Name)
		output, err := cmd.Output()
		if err == nil {
			return string(output), nil
		}
	}

	if _, err := exec.LookPath("pacman"); err == nil {
		// Arch
		cmd := exec.CommandContext(ctx, "pacman", "-Qi", app.Name)
		output, err := cmd.Output()
		if err == nil {
			return string(output), nil
		}
	}

	return "", fmt.Errorf("could not get package info for %s", app.Name)
}

// backupConfigFiles backs up configuration files
func (bm *BackupManager) backupConfigFiles(ctx context.Context, app *types.AppConfig, backupDir string, backup *BackupEntry) error {
	configBackupDir := filepath.Join(backupDir, "config")
	if err := os.MkdirAll(configBackupDir, 0750); err != nil {
		return fmt.Errorf("failed to create config backup directory: %w", err)
	}

	for _, configFile := range app.ConfigFiles {
		srcPath := configFile.Destination

		// Expand home directory if needed
		if strings.HasPrefix(srcPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			srcPath = filepath.Join(homeDir, srcPath[2:])
		}

		// Check if file exists
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		// Create destination path
		fileName := filepath.Base(srcPath)
		dstPath := filepath.Join(configBackupDir, fileName)

		// Copy file
		if err := bm.copyFile(ctx, srcPath, dstPath); err != nil {
			log.Warn("Failed to backup config file", "src", srcPath, "dst", dstPath, "error", err)
			continue
		}

		backup.ConfigFiles = append(backup.ConfigFiles, srcPath)
		log.Debug("Backed up config file", "src", srcPath, "dst", dstPath)
	}

	return nil
}

// backupDataFiles backs up data files
func (bm *BackupManager) backupDataFiles(ctx context.Context, app *types.AppConfig, backupDir string, backup *BackupEntry) error {
	dataBackupDir := filepath.Join(backupDir, "data")
	if err := os.MkdirAll(dataBackupDir, 0750); err != nil {
		return fmt.Errorf("failed to create data backup directory: %w", err)
	}

	for _, dataFile := range app.CleanupFiles {
		srcPath := dataFile

		// Expand home directory if needed
		if strings.HasPrefix(srcPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			srcPath = filepath.Join(homeDir, srcPath[2:])
		}

		// Check if file/directory exists
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		// Create destination path
		fileName := filepath.Base(srcPath)
		dstPath := filepath.Join(dataBackupDir, fileName)

		// Copy file or directory
		if err := bm.copyFileOrDir(ctx, srcPath, dstPath); err != nil {
			log.Warn("Failed to backup data file", "src", srcPath, "dst", dstPath, "error", err)
			continue
		}

		backup.DataFiles = append(backup.DataFiles, srcPath)
		log.Debug("Backed up data file", "src", srcPath, "dst", dstPath)
	}

	return nil
}

// copyFile copies a file from src to dst
func (bm *BackupManager) copyFile(ctx context.Context, src, dst string) error {
	cmd := exec.CommandContext(ctx, "cp", "-p", src, dst)
	return cmd.Run()
}

// copyFileOrDir copies a file or directory from src to dst
func (bm *BackupManager) copyFileOrDir(ctx context.Context, src, dst string) error {
	// Check if source is a directory
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Copy directory recursively
		cmd := exec.CommandContext(ctx, "cp", "-rp", src, dst)
		return cmd.Run()
	} else {
		// Copy file
		return bm.copyFile(ctx, src, dst)
	}
}

// saveBackupMetadata saves backup metadata to a JSON file
func (bm *BackupManager) saveBackupMetadata(backup *BackupEntry) error {
	// For now, we'll save to a simple text file
	// In production, this would be saved to the database or a JSON file
	metadataPath := filepath.Join(backup.BackupPath, "backup_info.txt")

	content := fmt.Sprintf(`DevEx Backup Information
App Name: %s
Created: %s
Backup Path: %s
Package Info: %s
Dependencies: %s
Services: %s
Config Files: %d files
Data Files: %d files
`,
		backup.AppName,
		backup.CreatedAt.Format(time.RFC3339),
		backup.BackupPath,
		strings.Split(backup.PackageInfo, "\n")[0], // First line only
		strings.Join(backup.Dependencies, ", "),
		strings.Join(backup.Services, ", "),
		len(backup.ConfigFiles),
		len(backup.DataFiles),
	)

	return os.WriteFile(metadataPath, []byte(content), 0600)
}

// ListBackups lists all available backups
func (bm *BackupManager) ListBackups() ([]BackupEntry, error) {
	// This is a simplified implementation
	// In production, this would query the database
	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupEntry
	for _, entry := range entries {
		if entry.IsDir() {
			// Parse directory name to extract app name and timestamp
			parts := strings.Split(entry.Name(), "_")
			if len(parts) >= 2 {
				appName := strings.Join(parts[:len(parts)-2], "_")
				if appName == "" && len(parts) >= 3 {
					appName = parts[0]
				}

				backupPath := filepath.Join(bm.backupDir, entry.Name())
				info, err := entry.Info()
				if err != nil {
					continue
				}

				backup := BackupEntry{
					AppName:    appName,
					BackupPath: backupPath,
					CreatedAt:  info.ModTime(),
				}
				backups = append(backups, backup)
			}
		}
	}

	return backups, nil
}

// RestoreBackup restores an application from backup
func (bm *BackupManager) RestoreBackup(backupPath string) error {
	log.Info("Restoring backup from path", "path", backupPath)

	// Check if backup directory exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup directory does not exist: %s", backupPath)
	}

	// Read backup metadata
	metadataPath := filepath.Join(backupPath, "backup_info.txt")
	if _, err := os.Stat(metadataPath); err != nil {
		return fmt.Errorf("backup metadata not found: %w", err)
	}

	// Restore config files
	configDir := filepath.Join(backupPath, "config")
	if _, err := os.Stat(configDir); err == nil {
		if err := bm.restoreDirectory(configDir, "config"); err != nil {
			log.Warn("Failed to restore config files", "error", err)
		}
	}

	// Restore data files
	dataDir := filepath.Join(backupPath, "data")
	if _, err := os.Stat(dataDir); err == nil {
		if err := bm.restoreDirectory(dataDir, "data"); err != nil {
			log.Warn("Failed to restore data files", "error", err)
		}
	}

	log.Info("Backup restored successfully", "path", backupPath)
	return nil
}

// restoreDirectory restores files from a backup directory
func (bm *BackupManager) restoreDirectory(backupDir, dirType string) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(backupDir, entry.Name())

		var dstPath string
		if dirType == "config" {
			// Restore to home directory or appropriate config location
			homeDir, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			dstPath = filepath.Join(homeDir, entry.Name())
		} else {
			// For data files, we'd need more sophisticated logic
			// This is simplified for the demo
			continue
		}

		// Copy file back
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "cp", "-rp", srcPath, dstPath)
		if err := cmd.Run(); err != nil {
			log.Warn("Failed to restore file", "src", srcPath, "dst", dstPath, "error", err)
		}
	}

	return nil
}

// CleanupOldBackups removes backups older than the specified duration
func (bm *BackupManager) CleanupOldBackups(maxAge time.Duration) error {
	backups, err := bm.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, backup := range backups {
		if backup.CreatedAt.Before(cutoff) {
			if err := os.RemoveAll(backup.BackupPath); err != nil {
				log.Warn("Failed to remove old backup", "path", backup.BackupPath, "error", err)
			} else {
				log.Info("Removed old backup", "app", backup.AppName, "created", backup.CreatedAt)
				removed++
			}
		}
	}

	if removed > 0 {
		log.Info("Cleaned up old backups", "removed", removed)
	}

	return nil
}
