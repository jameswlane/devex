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

type protectedFileState struct {
	exists bool
	link   string // symlink target; empty for a regular file
	mode   fs.FileMode
	data   []byte
}

type hookScopeSnapshot struct {
	home  string
	files map[string]protectedFileState
}

func readProtectedFile(path string) (protectedFileState, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return protectedFileState{}, nil
	}
	if err != nil {
		return protectedFileState{}, err
	}
	state := protectedFileState{exists: true, mode: info.Mode()}
	if info.Mode()&fs.ModeSymlink != 0 {
		if state.link, err = os.Readlink(path); err != nil {
			return protectedFileState{}, err
		}
	}
	if info.Mode()&fs.ModeSymlink != 0 || info.Mode().IsRegular() {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return protectedFileState{}, err
		}
		state.data = data
	}
	return state, nil
}

func (s protectedFileState) equal(other protectedFileState) bool {
	return s.exists == other.exists && s.link == other.link && s.mode == other.mode && bytes.Equal(s.data, other.data)
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

func restoreProtectedFile(path string, before protectedFileState) error {
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
		// The hook may have written through the link; put the target back too.
		if data, err := os.ReadFile(path); err == nil && !bytes.Equal(data, before.data) {
			return os.WriteFile(path, before.data, 0o600) // #nosec G306 -- an existing file keeps its mode
		}
		return nil
	}
	if !before.mode.IsRegular() {
		return fmt.Errorf("cannot restore a non-regular file")
	}
	if exists && !current.Mode().IsRegular() {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, before.data, before.mode.Perm()); err != nil {
		return err
	}
	return os.Chmod(path, before.mode.Perm())
}

func readlink(path string) string {
	target, _ := os.Readlink(path)
	return target
}
