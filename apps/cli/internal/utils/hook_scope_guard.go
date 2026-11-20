package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HookProtectedPaths are the user files, relative to the home directory, that a
// raw hook must never change. Static validation rejects hooks that name them;
// the guard also catches computed paths and external scripts at run time.
var HookProtectedPaths = []string{
	".bashrc", ".bash_profile", ".bash_login", ".bash_logout", ".profile",
	".zshrc", ".zshenv", ".zprofile", ".zlogin", ".zlogout",
	".config/fish/config.fish",
	".gitconfig", ".config/git/config", ".git-credentials",
	".ssh/config", ".ssh/authorized_keys",
}

// HookScopeError reports protected user files a hook changed. DevEx restores
// them before returning the error, unless Restore lists a failure.
type HookScopeError struct {
	AppName string
	Changed []string
	Restore error
}

func (e *HookScopeError) Error() string {
	app := ""
	if e.AppName != "" {
		app = fmt.Sprintf(" for app %q", e.AppName)
	}
	msg := fmt.Sprintf("install hook%s changed user configuration outside its scope (%s)", app, strings.Join(e.Changed, ", "))
	if e.Restore != nil {
		return msg + fmt.Sprintf("; restoring the original files failed: %v; check these files by hand", e.Restore)
	}
	return msg + "; DevEx restored the original files. Use declarative shell_init, path or config_files instead"
}

func (e *HookScopeError) Unwrap() error { return e.Restore }

// protectedFileState is one protected path: the entry itself (Lstat) and, for a
// regular file or a symlink, the content it resolves to (Stat).
type protectedFileState struct {
	exists     bool
	link       string // symlink target; empty for a regular file
	mode       fs.FileMode
	owner      fileOwnership
	dataExists bool
	dataMode   fs.FileMode
	dataOwner  fileOwnership
	data       []byte
	parent     string // resolved parent directory, to detect a swapped parent
}

type hookScopeSnapshot struct {
	home  string
	files map[string]protectedFileState
}

// resolvedDir resolves the deepest existing ancestor of dir and appends the
// missing components, so a parent the hook creates or swaps can be compared.
func resolvedDir(dir string) (string, error) {
	var missing []string
	for current := dir; ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) || filepath.Dir(current) == current {
			return "", err
		}
		missing = append([]string{filepath.Base(current)}, missing...)
	}
}

func readProtectedFile(path string) (protectedFileState, error) {
	parent, err := resolvedDir(filepath.Dir(path))
	if err != nil {
		return protectedFileState{}, err
	}
	state := protectedFileState{parent: parent}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return protectedFileState{}, err
	}
	state.exists, state.mode, state.owner = true, info.Mode(), ownershipOf(info)
	if info.Mode()&fs.ModeSymlink != 0 {
		if state.link, err = os.Readlink(path); err != nil {
			return protectedFileState{}, err
		}
	} else if !info.Mode().IsRegular() {
		return state, nil
	}
	target, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return protectedFileState{}, err
	}
	state.dataExists, state.dataMode, state.dataOwner = true, target.Mode(), ownershipOf(target)
	if target.Mode().IsRegular() {
		if state.data, err = os.ReadFile(path); err != nil {
			return protectedFileState{}, err
		}
	}
	return state, nil
}

func (s protectedFileState) equal(other protectedFileState) bool {
	return s.exists == other.exists && s.link == other.link && s.mode == other.mode && s.owner == other.owner &&
		s.dataExists == other.dataExists && s.dataMode == other.dataMode && s.dataOwner == other.dataOwner &&
		bytes.Equal(s.data, other.data) && s.parent == other.parent
}

// snapshotHookScope records protected files under home before a hook runs.
func snapshotHookScope(home string) (*hookScopeSnapshot, error) {
	if home == "" {
		return nil, fmt.Errorf("install hook scope guard needs the user's home directory")
	}
	snapshot := &hookScopeSnapshot{home: home, files: map[string]protectedFileState{}}
	for _, rel := range HookProtectedPaths {
		state, err := readProtectedFile(filepath.Join(home, rel))
		if err != nil {
			return nil, fmt.Errorf("install hook scope guard cannot read %s: %w", filepath.Join("~", rel), err)
		}
		snapshot.files[rel] = state
	}
	return snapshot, nil
}

// verify restores every protected file the hook changed and reports them.
func (s *hookScopeSnapshot) verify(appName string) error {
	var changed []string
	var restoreErrs []error
	for rel, before := range s.files {
		path := filepath.Join(s.home, rel)
		after, err := readProtectedFile(path)
		if err == nil && before.equal(after) {
			continue
		}
		changed = append(changed, filepath.Join("~", rel))
		if err := restoreProtectedFile(path, before); err != nil {
			restoreErrs = append(restoreErrs, fmt.Errorf("%s: %w", filepath.Join("~", rel), err))
		}
	}
	if len(changed) == 0 {
		return nil
	}
	sort.Strings(changed)
	return &HookScopeError{AppName: appName, Changed: changed, Restore: errors.Join(restoreErrs...)}
}

// guardHookScope runs fn and then verifies and restores the protected files.
// A scope violation is reported even when fn fails, since a hook may write and
// then exit non-zero.
func guardHookScope(home, appName string, fn func() error) error {
	snapshot, err := snapshotHookScope(home)
	if err != nil {
		return err
	}
	runErr := fn()
	if scopeErr := snapshot.verify(appName); scopeErr != nil {
		return errors.Join(scopeErr, runErr)
	}
	return runErr
}

func restoreProtectedFile(path string, before protectedFileState) error {
	// Never write through a parent the hook replaced: it may point elsewhere.
	if parent, err := resolvedDir(filepath.Dir(path)); err != nil || parent != before.parent {
		return fmt.Errorf("its parent directory changed; refusing to restore through it")
	}
	current, err := os.Lstat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	exists := err == nil
	if !before.exists {
		if exists {
			return os.Remove(path)
		}
		return nil
	}
	if before.link != "" {
		if !exists || current.Mode()&fs.ModeSymlink == 0 || readlink(path) != before.link {
			if exists {
				if err := os.Remove(path); err != nil {
					return err
				}
			}
			if err := os.Symlink(before.link, path); err != nil {
				return err
			}
		}
		if err := restoreOwnership(path, before.owner, true); err != nil {
			return err
		}
		if !before.dataExists {
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("the hook created the symlink's target; remove it by hand")
			}
			return nil
		}
		// The hook may have written through the link, or removed its target.
		return restoreContent(path, before.data, before.dataMode, before.dataOwner)
	}
	if !before.mode.IsRegular() {
		return fmt.Errorf("cannot restore a non-regular file")
	}
	if exists && !current.Mode().IsRegular() {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return restoreContent(path, before.data, before.mode, before.owner)
}

// restoreContent writes data to path (following a symlink), then restores the
// mode and owner. A hook may have made the file read-only, so make it writable
// first.
func restoreContent(path string, data []byte, mode fs.FileMode, owner fileOwnership) error {
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		if err := os.Chmod(path, 0o600); err != nil {
			return err
		}
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(path, mode.Perm()); err != nil {
		return err
	}
	return restoreOwnership(path, owner, false)
}

func readlink(path string) string {
	target, _ := os.Readlink(path)
	return target
}
