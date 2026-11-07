package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
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
	if err := bm.createConfigBackup(ctx, backupFile); err != nil {
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

	// Every member is validated before anything is written beneath HOME.
	members, err := readBackupMembers(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	restored, err := restoreBackupMembers(backupFile, os.Getenv("HOME"), members)
	if err != nil {
		return fmt.Errorf("failed to restore configuration: %w", err)
	}

	fmt.Printf("✓ Restored %d files. Settings restored successfully!\n", restored)
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

// backupPatterns are the KDE configuration files and directories backed up,
// relative to HOME. Patterns use filepath.Match syntax.
var backupPatterns = []string{
	// Configuration files
	".config/kdeglobals",
	".config/kwinrc",
	".config/plasmarc",
	".config/plasmashellrc",
	".config/kscreenlockerrc",
	".config/kcminputrc",

	// Application configs
	".config/dolphinrc",
	".config/konsolerc",
	".config/kate*",

	// Plasma specific
	".config/plasma*",
	".local/share/plasma*",
	".local/share/kactivitymanagerd",
}

// backupRoots are the HOME-relative trees a backup may contain and a restore
// may write; backupPatterns must stay beneath them.
var backupRoots = []string{".config", ".local/share"}

// createConfigBackup archives the existing KDE configuration relative to HOME,
// so restore can return each member to the same HOME-relative path.
func (bm *BackupManager) createConfigBackup(ctx context.Context, backupFile string) error {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return fmt.Errorf("HOME is not set")
	}

	members, err := backupMembers(homeDir)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return fmt.Errorf("no KDE configuration files found to backup")
	}

	args := append([]string{"czf", backupFile, "-C", homeDir}, members...)
	cmd := exec.CommandContext(ctx, "tar", args...)
	return cmd.Run()
}

// backupMembers expands backupPatterns beneath homeDir and returns the existing
// matches as sorted, de-duplicated, slash-separated HOME-relative paths.
func backupMembers(homeDir string) ([]string, error) {
	seen := map[string]bool{}
	var members []string
	for _, pattern := range backupPatterns {
		matches, err := filepath.Glob(filepath.Join(homeDir, filepath.FromSlash(pattern)))
		if err != nil {
			return nil, fmt.Errorf("invalid backup pattern %q: %w", pattern, err)
		}
		for _, match := range matches {
			relative, err := filepath.Rel(homeDir, match)
			if err != nil {
				return nil, err
			}
			relative = filepath.ToSlash(relative)
			if !seen[relative] {
				seen[relative] = true
				members = append(members, relative)
			}
		}
	}
	sort.Strings(members)
	return members, nil
}

// backupMember is a validated archive entry; name is a clean HOME-relative path.
type backupMember struct {
	name string
	dir  bool
	mode os.FileMode
}

// readBackupMembers reads every archive header and rejects the archive before
// any write when a member is absolute, traverses with "..", leaves the KDE
// backup roots, or is not a regular file or directory (links could redirect
// later writes outside HOME).
func readBackupMembers(backupFile string) ([]backupMember, error) {
	var members []backupMember
	files := 0
	err := walkBackup(backupFile, func(header *tar.Header, _ io.Reader) error {
		member, err := validateBackupMember(header)
		if err != nil {
			return err
		}
		if !member.dir {
			files++
		}
		members = append(members, member)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if files == 0 {
		return nil, fmt.Errorf("backup contains no KDE configuration files")
	}
	return members, nil
}

func validateBackupMember(header *tar.Header) (backupMember, error) {
	name := header.Name
	if name == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return backupMember{}, fmt.Errorf("archive member %q is an absolute path", name)
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return backupMember{}, fmt.Errorf("archive member %q traverses outside HOME", name)
		}
	}
	clean := path.Clean(strings.ReplaceAll(name, `\`, "/"))
	if !withinBackupRoots(clean) && (header.Typeflag != tar.TypeDir || !ancestorOfBackupRoot(clean)) {
		return backupMember{}, fmt.Errorf("archive member %q is outside the KDE configuration directories (%s)", name, strings.Join(backupRoots, ", "))
	}
	switch header.Typeflag {
	case tar.TypeDir:
		return backupMember{name: clean, dir: true}, nil
	case tar.TypeReg:
		return backupMember{name: clean, mode: header.FileInfo().Mode().Perm() | 0o600}, nil
	default:
		return backupMember{}, fmt.Errorf("archive member %q has unsupported type %q; only regular files and directories are restored", name, string(header.Typeflag))
	}
}

// ancestorOfBackupRoot allows parent directory entries such as ".local".
func ancestorOfBackupRoot(clean string) bool {
	for _, root := range backupRoots {
		if strings.HasPrefix(root, clean+"/") {
			return true
		}
	}
	return false
}

func withinBackupRoots(clean string) bool {
	for _, root := range backupRoots {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			return true
		}
	}
	return false
}

// restoreBackupMembers writes the validated members beneath homeDir. It reads
// the archive a second time and requires the same member sequence, so content
// is only taken from entries that were validated.
func restoreBackupMembers(backupFile, homeDir string, members []backupMember) (int, error) {
	if homeDir == "" {
		return 0, fmt.Errorf("HOME is not set")
	}
	root, err := os.OpenRoot(homeDir)
	if err != nil {
		return 0, err
	}
	defer func() { _ = root.Close() }()
	index, restored := 0, 0
	err = walkBackup(backupFile, func(header *tar.Header, content io.Reader) error {
		if index >= len(members) {
			return fmt.Errorf("archive changed during restore")
		}
		member := members[index]
		index++
		current, err := validateBackupMember(header)
		if err != nil || current.name != member.name || current.dir != member.dir {
			return fmt.Errorf("archive changed during restore")
		}
		target := filepath.FromSlash(member.name)
		if member.dir {
			return root.MkdirAll(target, 0o755)
		}
		if err := root.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeBackupFile(root, target, content, member.mode); err != nil {
			return fmt.Errorf("restore %s: %w", member.name, err)
		}
		restored++
		return nil
	})
	if err == nil && index != len(members) {
		err = fmt.Errorf("archive changed during restore")
	}
	return restored, err
}

func writeBackupFile(root *os.Root, target string, content io.Reader, mode os.FileMode) error {
	file, err := root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, content); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// walkBackup visits each member of a gzip-compressed tar archive in order.
func walkBackup(backupFile string, visit func(*tar.Header, io.Reader) error) error {
	file, err := os.Open(backupFile)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("not a gzip-compressed tar archive: %w", err)
	}
	defer func() { _ = compressed.Close() }()
	archive := tar.NewReader(compressed)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %w", err)
		}
		if err := visit(header, archive); err != nil {
			return err
		}
	}
}

// getBackupDirectory determines the backup directory to use
func (bm *BackupManager) getBackupDirectory(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return filepath.Join(sdk.PathsForHome("").Backups(), "kde")
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
