package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	BackupsDir       = "backups"
	MaxBackups       = 50
	BackupTimeFormat = "20060102-150405"
	MetadataFile     = ".backup-metadata.json"
	// Security limits for tar extraction
	MaxFileSize  = 100 * 1024 * 1024  // 100MB per file
	MaxTotalSize = 1024 * 1024 * 1024 // 1GB total
	MaxFiles     = 10000              // Maximum number of files
)

type BackupManager struct {
	baseDir    string
	backupsDir string
}

type BackupMetadata struct {
	ID          string            `json:"id" yaml:"id"`
	Timestamp   time.Time         `json:"timestamp" yaml:"timestamp"`
	Description string            `json:"description" yaml:"description"`
	Type        string            `json:"type" yaml:"type"`
	Files       []string          `json:"files" yaml:"files"`
	Size        int64             `json:"size" yaml:"size"`
	Version     string            `json:"version" yaml:"version"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Changes     map[string]string `json:"changes,omitempty" yaml:"changes,omitempty"`
}

type BackupOptions struct {
	Description string
	Type        string
	Tags        []string
	MaxBackups  int
	Compress    bool
	Include     []string
	Exclude     []string
}

func NewBackupManager(baseDir string) *BackupManager {
	return &BackupManager{
		baseDir:    baseDir,
		backupsDir: filepath.Join(baseDir, BackupsDir),
	}
}

func (bm *BackupManager) CreateBackup(options BackupOptions) (*BackupMetadata, error) {
	if err := bm.ensureBackupsDir(); err != nil {
		return nil, fmt.Errorf("failed to create backups directory: %w", err)
	}

	backupID := bm.generateBackupID()
	backupDir := filepath.Join(bm.backupsDir, backupID)

	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	configDir := filepath.Join(bm.baseDir, "config")
	files, err := bm.collectFiles(configDir, options.Include, options.Exclude)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files from %s (include: %v, exclude: %v): %w", configDir, options.Include, options.Exclude, err)
	}

	var totalSize int64
	var backedUpFiles []string

	if options.Compress {
		archivePath := filepath.Join(backupDir, "config.tar.gz")
		size, err := bm.createCompressedBackup(configDir, archivePath, files)
		if err != nil {
			if removeErr := os.RemoveAll(backupDir); removeErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to clean up backup directory: %v\n", removeErr)
			}
			return nil, fmt.Errorf("failed to create compressed backup: %w", err)
		}
		totalSize = size
		backedUpFiles = []string{"config.tar.gz"}
	} else {
		for _, file := range files {
			src := filepath.Join(configDir, file)
			dst := filepath.Join(backupDir, "config", file)

			if err := bm.copyFile(src, dst); err != nil {
				if removeErr := os.RemoveAll(backupDir); removeErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to clean up backup directory: %v\n", removeErr)
				}
				return nil, fmt.Errorf("failed to copy file %s: %w", file, err)
			}

			info, _ := os.Stat(src)
			if info != nil {
				totalSize += info.Size()
			}
			backedUpFiles = append(backedUpFiles, filepath.Join("config", file))
		}
	}

	metadata := &BackupMetadata{
		ID:          backupID,
		Timestamp:   time.Now(),
		Description: options.Description,
		Type:        options.Type,
		Files:       backedUpFiles,
		Size:        totalSize,
		Version:     "1.0.0",
		Tags:        options.Tags,
	}

	if err := bm.saveMetadata(backupDir, metadata); err != nil {
		if removeErr := os.RemoveAll(backupDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up backup directory %s: %v\n", backupDir, removeErr)
		}
		return nil, fmt.Errorf("failed to save metadata to %s for backup %s: %w", backupDir, backupID, err)
	}

	if err := bm.updateGlobalMetadata(metadata); err != nil {
		return metadata, err
	}

	if err := bm.pruneOldBackups(options.MaxBackups); err != nil {
		return metadata, err
	}

	return metadata, nil
}

func (bm *BackupManager) RestoreBackup(backupID string, targetDir string) error {
	backupDir := filepath.Join(bm.backupsDir, backupID)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup %s not found in directory %s", backupID, bm.backupsDir)
	}

	_, err := bm.loadMetadata(backupDir)
	if err != nil {
		return fmt.Errorf("failed to load backup metadata for %s from %s: %w", backupID, backupDir, err)
	}

	if targetDir == "" {
		targetDir = filepath.Join(bm.baseDir, "config")
	}

	currentBackup, err := bm.CreateBackup(BackupOptions{
		Description: fmt.Sprintf("Pre-restore backup (restoring from %s)", backupID),
		Type:        "pre-restore",
		Tags:        []string{"auto", "pre-restore"},
	})
	if err != nil {
		return fmt.Errorf("failed to create pre-restore backup before restoring %s to %s: %w", backupID, targetDir, err)
	}

	compressedPath := filepath.Join(backupDir, "config.tar.gz")
	if _, err := os.Stat(compressedPath); err == nil {
		if err := bm.extractCompressedBackup(compressedPath, targetDir); err != nil {
			if currentBackup != nil {
				if restoreErr := bm.RestoreBackup(currentBackup.ID, targetDir); restoreErr != nil {
					// Log the restore error but don't fail the original error
					fmt.Fprintf(os.Stderr, "Warning: failed to restore backup %s: %v\n", currentBackup.ID, restoreErr)
				}
			}
			return fmt.Errorf("failed to extract backup: %w", err)
		}
	} else {
		configBackupDir := filepath.Join(backupDir, "config")
		if err := bm.copyDirectory(configBackupDir, targetDir); err != nil {
			if currentBackup != nil {
				if restoreErr := bm.RestoreBackup(currentBackup.ID, targetDir); restoreErr != nil {
					// Log the restore error but don't fail the original error
					fmt.Fprintf(os.Stderr, "Warning: failed to restore backup %s: %v\n", currentBackup.ID, restoreErr)
				}
			}
			return fmt.Errorf("failed to restore files: %w", err)
		}
	}

	return nil
}

func (bm *BackupManager) ListBackups(filter string, limit int) ([]*BackupMetadata, error) {
	metadataPath := filepath.Join(bm.backupsDir, MetadataFile)
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return []*BackupMetadata{}, nil
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var allBackups []*BackupMetadata
	if err := json.Unmarshal(data, &allBackups); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	var filtered []*BackupMetadata
	for _, backup := range allBackups {
		if filter == "" || bm.matchesFilter(backup, filter) {
			filtered = append(filtered, backup)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

func (bm *BackupManager) GetBackup(backupID string) (*BackupMetadata, error) {
	backupDir := filepath.Join(bm.backupsDir, backupID)
	return bm.loadMetadata(backupDir)
}

func (bm *BackupManager) DeleteBackup(backupID string) error {
	backupDir := filepath.Join(bm.backupsDir, backupID)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup %s not found in directory %s", backupID, bm.backupsDir)
	}

	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("failed to delete backup %s from %s: %w", backupID, backupDir, err)
	}

	return bm.removeFromGlobalMetadata(backupID)
}

func (bm *BackupManager) CompareBackups(id1, id2 string) (*BackupComparison, error) {
	backup1, err := bm.GetBackup(id1)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup %s: %w", id1, err)
	}

	backup2, err := bm.GetBackup(id2)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup %s: %w", id2, err)
	}

	comparison := &BackupComparison{
		Backup1:       backup1,
		Backup2:       backup2,
		AddedFiles:    []string{},
		RemovedFiles:  []string{},
		ModifiedFiles: []string{},
	}

	files1 := make(map[string]bool)
	for _, f := range backup1.Files {
		files1[f] = true
	}

	files2 := make(map[string]bool)
	for _, f := range backup2.Files {
		files2[f] = true
	}

	for f := range files1 {
		if !files2[f] {
			comparison.RemovedFiles = append(comparison.RemovedFiles, f)
		}
	}

	for f := range files2 {
		if !files1[f] {
			comparison.AddedFiles = append(comparison.AddedFiles, f)
		} else if bm.filesDiffer(id1, id2, f) {
			comparison.ModifiedFiles = append(comparison.ModifiedFiles, f)
		}
	}

	return comparison, nil
}

func (bm *BackupManager) AutoBackup(settings config.CrossPlatformSettings) (*BackupMetadata, error) {
	return bm.CreateBackup(BackupOptions{
		Description: "Automatic backup before configuration change",
		Type:        "auto",
		Tags:        []string{"auto"},
		MaxBackups:  MaxBackups,
		Compress:    true,
	})
}

func (bm *BackupManager) ensureBackupsDir() error {
	return os.MkdirAll(bm.backupsDir, 0750)
}

func (bm *BackupManager) generateBackupID() string {
	return fmt.Sprintf("backup-%s", time.Now().Format(BackupTimeFormat))
}

func (bm *BackupManager) collectFiles(dir string, include, exclude []string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if bm.shouldInclude(relPath, include, exclude) {
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

func (bm *BackupManager) shouldInclude(path string, include, exclude []string) bool {
	if len(include) > 0 {
		included := false
		for _, pattern := range include {
			if matched, _ := filepath.Match(pattern, path); matched {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	for _, pattern := range exclude {
		if matched, _ := filepath.Match(pattern, path); matched {
			return false
		}
	}

	return true
}

func (bm *BackupManager) createCompressedBackup(sourceDir, archivePath string, files []string) (int64, error) {
	file, err := os.Create(archivePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	var totalSize int64

	for _, filePath := range files {
		fullPath := filepath.Join(sourceDir, filePath)
		if err := bm.addToTar(tarWriter, fullPath, filePath); err != nil {
			return 0, err
		}

		info, _ := os.Stat(fullPath)
		if info != nil {
			totalSize += info.Size()
		}
	}

	return totalSize, nil
}

func (bm *BackupManager) addToTar(tw *tar.Writer, sourcePath, archivePath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}

	header.Name = archivePath

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	return err
}

func (bm *BackupManager) extractCompressedBackup(archivePath, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	var totalSize int64
	var fileCount int

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Security check: prevent file traversal attacks
		if err := bm.validatePath(header.Name, targetDir); err != nil {
			return fmt.Errorf("security violation: %w", err)
		}

		// Security check: prevent decompression bomb
		if header.Size > MaxFileSize {
			return fmt.Errorf("file %s exceeds maximum size limit (%d bytes)", header.Name, MaxFileSize)
		}

		totalSize += header.Size
		if totalSize > MaxTotalSize {
			return fmt.Errorf("total extraction size exceeds limit (%d bytes)", MaxTotalSize)
		}

		fileCount++
		if fileCount > MaxFiles {
			return fmt.Errorf("too many files in archive (limit: %d)", MaxFiles)
		}

		// Use validated clean path for security
		cleanPath := filepath.Clean(header.Name)
		targetPath := filepath.Join(targetDir, cleanPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
				return err
			}

			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}

			// Use LimitReader to prevent reading more than expected
			limitedReader := io.LimitReader(tarReader, header.Size)
			if _, err := io.Copy(outFile, limitedReader); err != nil {
				if closeErr := outFile.Close(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", closeErr)
				}
				return err
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close extracted file: %w", err)
			}

			// Apply safe file permissions (use a fixed safe mode to avoid conversion issues)
			// Instead of using the potentially unsafe header mode, use a safe default
			var safeMode os.FileMode
			if header.Mode&0200 != 0 { // If original had write permission
				safeMode = os.FileMode(0644)
			} else {
				safeMode = os.FileMode(0444) // Read-only
			}

			if err := os.Chmod(targetPath, safeMode); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePath ensures the path is safe for extraction
func (bm *BackupManager) validatePath(path, targetDir string) error {
	// Clean the path to remove any .. components
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal attempt detected: %s", path)
	}

	// Check for absolute paths
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("absolute paths not allowed: %s", path)
	}

	// Ensure the final path is within the target directory
	finalPath := filepath.Join(targetDir, cleanPath)
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute target directory: %w", err)
	}

	absFinalPath, err := filepath.Abs(finalPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute final path: %w", err)
	}

	if !strings.HasPrefix(absFinalPath, absTargetDir) {
		return fmt.Errorf("path outside target directory: %s", path)
	}

	return nil
}

func (bm *BackupManager) copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

func (bm *BackupManager) copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return bm.copyFile(path, dstPath)
	})
}

func (bm *BackupManager) saveMetadata(backupDir string, metadata *BackupMetadata) error {
	metadataPath := filepath.Join(backupDir, "metadata.json")
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0600)
}

func (bm *BackupManager) loadMetadata(backupDir string) (*BackupMetadata, error) {
	metadataPath := filepath.Join(backupDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	var metadata BackupMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (bm *BackupManager) updateGlobalMetadata(metadata *BackupMetadata) error {
	metadataPath := filepath.Join(bm.backupsDir, MetadataFile)

	var allBackups []*BackupMetadata

	if data, err := os.ReadFile(metadataPath); err == nil {
		if err := json.Unmarshal(data, &allBackups); err != nil {
			// Log the unmarshal error but continue with empty slice
			fmt.Fprintf(os.Stderr, "Warning: failed to unmarshal existing metadata: %v\n", err)
		}
	}

	allBackups = append(allBackups, metadata)

	data, err := json.MarshalIndent(allBackups, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0600)
}

func (bm *BackupManager) removeFromGlobalMetadata(backupID string) error {
	metadataPath := filepath.Join(bm.backupsDir, MetadataFile)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil
	}

	var allBackups []*BackupMetadata
	if err := json.Unmarshal(data, &allBackups); err != nil {
		return err
	}

	var filtered []*BackupMetadata
	for _, backup := range allBackups {
		if backup.ID != backupID {
			filtered = append(filtered, backup)
		}
	}

	data, err = json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0600)
}

func (bm *BackupManager) pruneOldBackups(maxBackups int) error {
	if maxBackups <= 0 {
		maxBackups = MaxBackups
	}

	backups, err := bm.ListBackups("", 0)
	if err != nil {
		return err
	}

	autoBackups := []*BackupMetadata{}
	for _, backup := range backups {
		if backup.Type == "auto" {
			autoBackups = append(autoBackups, backup)
		}
	}

	if len(autoBackups) > maxBackups {
		sort.Slice(autoBackups, func(i, j int) bool {
			return autoBackups[i].Timestamp.Before(autoBackups[j].Timestamp)
		})

		toDelete := len(autoBackups) - maxBackups
		for i := 0; i < toDelete; i++ {
			if err := bm.DeleteBackup(autoBackups[i].ID); err != nil {
				return err
			}
		}
	}

	return nil
}

func (bm *BackupManager) matchesFilter(backup *BackupMetadata, filter string) bool {
	filter = strings.ToLower(filter)

	if strings.Contains(strings.ToLower(backup.ID), filter) {
		return true
	}

	if strings.Contains(strings.ToLower(backup.Description), filter) {
		return true
	}

	if strings.Contains(strings.ToLower(backup.Type), filter) {
		return true
	}

	for _, tag := range backup.Tags {
		if strings.Contains(strings.ToLower(tag), filter) {
			return true
		}
	}

	return false
}

func (bm *BackupManager) filesDiffer(backupID1, backupID2, file string) bool {
	path1 := filepath.Join(bm.backupsDir, backupID1, file)
	path2 := filepath.Join(bm.backupsDir, backupID2, file)

	data1, err1 := os.ReadFile(path1)
	data2, err2 := os.ReadFile(path2)

	if err1 != nil || err2 != nil {
		return true
	}

	return string(data1) != string(data2)
}

type BackupComparison struct {
	Backup1       *BackupMetadata
	Backup2       *BackupMetadata
	AddedFiles    []string
	RemovedFiles  []string
	ModifiedFiles []string
}

func (bm *BackupManager) ExportBackup(backupID string, outputPath string, format string) error {
	backup, err := bm.GetBackup(backupID)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(backup, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(outputPath, data, 0600)
	case "yaml":
		data, err := yaml.Marshal(backup)
		if err != nil {
			return err
		}
		return os.WriteFile(outputPath, data, 0600)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func BackupBeforeOperation(baseDir string, operation string) (*BackupMetadata, error) {
	manager := NewBackupManager(baseDir)
	return manager.CreateBackup(BackupOptions{
		Description: fmt.Sprintf("Auto-backup before %s", operation),
		Type:        "auto",
		Tags:        []string{"auto", operation},
		Compress:    true,
		MaxBackups:  MaxBackups,
	})
}
