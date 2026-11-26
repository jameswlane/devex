package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type fileAction string

const (
	actionCreate    fileAction = "create"
	actionUpdate    fileAction = "update"
	actionDelete    fileAction = "delete"
	actionUnchanged fileAction = "unchanged"
)

// FilePlan describes the planned change to one native agent file.
type FilePlan struct {
	Agent           string
	Path            string
	Action          fileAction
	Before          string
	After           string
	Mode            os.FileMode
	Exists          bool
	OwnsPermissions bool
	OwnsMCP         bool
}

// resolveWriteTarget follows existing symlinks so dotfile-manager links
// survive atomic replacement.
func resolveWriteTarget(path string) (string, error) {
	if target, err := filepath.EvalSymlinks(path); err == nil {
		return target, nil
	}
	// Handle dangling symlink: write to the symlink's target path.
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		link, readErr := os.Readlink(path)
		if readErr != nil {
			return "", fmt.Errorf("read symlink %q: %w", path, readErr)
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		return filepath.Clean(link), nil
	}
	return path, nil
}

func readExistingFile(path string, defaultMode os.FileMode) (content string, exists bool, mode os.FileMode, err error) {
	resolved, err := resolveWriteTarget(path)
	if err != nil {
		return "", false, defaultMode, err
	}
	data, err := os.ReadFile(resolved)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, defaultMode, nil
	}
	if err != nil {
		return "", false, defaultMode, fmt.Errorf("read %q: %w", path, err)
	}
	info, statErr := os.Stat(resolved)
	if statErr != nil {
		return "", false, defaultMode, fmt.Errorf("stat %q: %w", path, statErr)
	}
	return string(data), true, info.Mode().Perm(), nil
}

// ApplyFilePlan applies one planned file operation, creating a one-time sibling
// backup before the first modification of an existing file and writing
// atomically through symlinks.
func ApplyFilePlan(plan FilePlan) (backupPath string, err error) {
	if plan.Action == actionUnchanged {
		return "", nil
	}

	resolved, err := resolveWriteTarget(plan.Path)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return "", fmt.Errorf("create directory %q: %w", filepath.Dir(resolved), err)
	}

	if plan.Exists {
		backupPath, err = ensureOneTimeBackup(resolved, []byte(plan.Before), plan.Mode)
		if err != nil {
			return "", err
		}
	}

	if plan.Action == actionDelete {
		if err := os.Remove(resolved); err != nil && !errors.Is(err, os.ErrNotExist) {
			return backupPath, fmt.Errorf("remove %q: %w", resolved, err)
		}
		return backupPath, nil
	}

	if err := writeFileAtomic(resolved, []byte(plan.After), plan.Mode); err != nil {
		return backupPath, err
	}
	return backupPath, nil
}

func ensureOneTimeBackup(resolvedPath string, original []byte, mode os.FileMode) (string, error) {
	backups, err := filepath.Glob(resolvedPath + ".devex-bak-*")
	if err != nil {
		return "", fmt.Errorf("check existing backups for %q: %w", resolvedPath, err)
	}
	if len(backups) > 0 {
		return "", nil
	}
	prefix := filepath.Base(resolvedPath) + ".devex-bak-" + time.Now().UTC().Format("20060102T150405.000000000Z") + "-"
	backup, err := os.CreateTemp(filepath.Dir(resolvedPath), prefix)
	if err != nil {
		return "", fmt.Errorf("create backup for %q: %w", resolvedPath, err)
	}
	name := backup.Name()
	if err = backup.Chmod(mode); err == nil {
		_, err = backup.Write(original)
	}
	if err == nil {
		err = backup.Sync()
	}
	closeErr := backup.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(name)
		if err != nil {
			return "", fmt.Errorf("write backup %q: %w", name, err)
		}
		return "", closeErr
	}
	return name, nil
}

func writeFileAtomic(resolvedPath string, data []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(resolvedPath), ".devex-agent-*")
	if err != nil {
		return fmt.Errorf("create temp file for %q: %w", resolvedPath, err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()

	if err = temp.Chmod(mode); err == nil {
		_, err = temp.Write(data)
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil {
		return fmt.Errorf("write temp file for %q: %w", resolvedPath, err)
	}
	if closeErr != nil {
		return fmt.Errorf("close temp file for %q: %w", resolvedPath, closeErr)
	}
	if err := os.Rename(tempName, resolvedPath); err != nil {
		return fmt.Errorf("replace %q: %w", resolvedPath, err)
	}
	return nil
}
