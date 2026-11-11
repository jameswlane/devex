// Package configfiles implements the shared install stage between package installation
// and post-install hooks. Mapping failures are warnings: remaining mappings and hooks
// continue, while Result (including warnings) is persisted for inspection and cleanup.
package configfiles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type File struct {
	Destination string `json:"destination"`
	SHA256      string `json:"sha256"`
	Backup      string `json:"backup,omitempty"`
}

type Result struct {
	Files    []File   `json:"files"`
	Warnings []string `json:"warnings,omitempty"`
}

var windowsEnv = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

// Expand supports ~/, $VAR, ${VAR}, and %VAR%. Unknown variables are errors.
// Relative sources are relative to the working directory; destinations must be
// absolute and inside HOME. No schema option currently permits external writes.
func Expand(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("empty path")
	}
	var missing string
	lookup := func(key string) string {
		value, ok := os.LookupEnv(key)
		if !ok || value == "" {
			missing = key
		}
		return value
	}
	path = os.Expand(path, lookup)
	path = windowsEnv.ReplaceAllStringFunc(path, func(match string) string { return lookup(match[1 : len(match)-1]) })
	if missing != "" {
		return "", fmt.Errorf("unset environment variable %s", missing)
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = home + path[1:]
	}
	if strings.HasPrefix(path, "~") {
		return "", errors.New("named-user home expansion is unsupported")
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", errors.New("path traversal is forbidden")
		}
	}
	return filepath.Clean(path), nil
}

func relativeDestination(home, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("destination must be absolute and inside HOME")
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", errors.New("destination outside HOME is denied")
	}
	stateDir := filepath.Join(".local", "state", "devex", "config-files")
	if rel == stateDir || strings.HasPrefix(rel, stateDir+string(filepath.Separator)) {
		return "", errors.New("destination overlaps DevEx managed-file records")
	}
	return rel, nil
}

func manifest(app string) string {
	return filepath.Join(".local", "state", "devex", "config-files", fmt.Sprintf("%x.json", sha256.Sum256([]byte(strings.ToLower(app)))))
}

func load(root *os.Root, app string) (Result, error) {
	var result Result
	data, err := root.ReadFile(manifest(app))
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

// Apply copies regular files only when their marker conditions match, backs up
// conflicts, leaves identical destinations unchanged, and records copies by
// content hash. Dry runs validate and describe the same plan without creating
// directories, backups, files, or manifests. emit receives plan/warnings.
func Apply(ctx context.Context, app string, mappings []types.ConfigFile, dryRun bool, emit func(string)) Result {
	result := Result{}
	if len(mappings) == 0 {
		return result
	}
	warn := func(mapping types.ConfigFile, err error) {
		message := fmt.Sprintf("%s: config_files %q -> %q: %v", app, mapping.Source, mapping.Destination, err)
		result.Warnings = append(result.Warnings, message)
		if emit != nil {
			emit("Warning: " + message)
		}
	}
	eligible := make([]types.ConfigFile, 0, len(mappings))
	for _, m := range mappings {
		if len(m.RequiresAny) == 0 {
			eligible = append(eligible, m)
			continue
		}
		matched, invalid := false, false
		for _, marker := range m.RequiresAny {
			path, err := Expand(marker)
			if err != nil {
				warn(m, fmt.Errorf("evaluate required marker %q: %w", marker, err))
				invalid = true
				break
			}
			info, err := os.Stat(path)
			if err == nil && info.Mode().IsRegular() {
				matched = true
				break
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				warn(m, fmt.Errorf("access required marker %q: %w", marker, err))
				invalid = true
				break
			}
		}
		if matched {
			eligible = append(eligible, m)
		} else if !invalid && emit != nil {
			emit(fmt.Sprintf("Skip %s: none of the required files exist (%s)", m.Destination, strings.Join(m.RequiresAny, ", ")))
		}
	}
	mappings = eligible
	if len(mappings) == 0 {
		return result
	}
	home, err := os.UserHomeDir()
	if err != nil {
		for _, m := range mappings {
			warn(m, err)
		}
		return result
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		for _, m := range mappings {
			warn(m, err)
		}
		return result
	}
	defer root.Close()
	previous, err := load(root, app)
	if err != nil {
		for _, m := range mappings {
			warn(m, fmt.Errorf("read managed-file record: %w", err))
		}
		return result
	}
	result.Files = previous.Files
	if !dryRun {
		if err := save(root, app, result); err != nil {
			for _, m := range mappings {
				warn(m, fmt.Errorf("prepare managed-file record: %w", err))
			}
			return result
		}
	}
	for _, m := range mappings {
		if err := ctx.Err(); err != nil {
			warn(m, err)
			break
		}
		file, err := applyOne(root, home, m, previousCopy(previous, m), dryRun, emit)
		if err != nil {
			warn(m, err)
			continue
		}
		if !dryRun {
			found := false
			for i := range result.Files {
				if result.Files[i].Destination == file.Destination {
					result.Files[i] = file
					found = true
					break
				}
			}
			if !found {
				result.Files = append(result.Files, file)
			}
			// Persist each completed mapping, even if a later mapping fails.
			if err := save(root, app, result); err != nil {
				warn(m, fmt.Errorf("record managed copy: %w", err))
			}
		}
	}
	if !dryRun {
		if err := save(root, app, result); err != nil {
			warn(types.ConfigFile{}, fmt.Errorf("record install result: %w", err))
		}
	}
	return result
}

func save(root *os.Root, app string, result Result) error {
	path := manifest(app)
	if err := root.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	// Replace via rename so a truncated manifest cannot lose ownership records.
	tmp := path + fmt.Sprintf(".%d.tmp", time.Now().UnixNano())
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(tmp) }()
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return root.Rename(tmp, path)
}

// previousCopy returns the recorded copy for m's destination, if any.
func previousCopy(previous Result, m types.ConfigFile) *File {
	destination, err := Expand(m.Destination)
	if err != nil {
		return nil
	}
	for i := range previous.Files {
		if previous.Files[i].Destination == destination {
			return &previous.Files[i]
		}
	}
	return nil
}

// applyOne copies one mapping. A destination that still holds the previously
// recorded copy is DevEx's own file: it is replaced without a new backup and
// keeps the backup of the user's original file.
func applyOne(root *os.Root, home string, m types.ConfigFile, recorded *File, dryRun bool, emit func(string)) (File, error) {
	source, err := Expand(m.Source)
	if err != nil {
		return File{}, err
	}
	destination, err := Expand(m.Destination)
	if err != nil {
		return File{}, err
	}
	rel, err := relativeDestination(home, destination)
	if err != nil {
		return File{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return File{}, fmt.Errorf("read source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return File{}, errors.New("source must be a regular file")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return File{}, fmt.Errorf("read source: %w", err)
	}
	file := File{Destination: destination, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	info, err = root.Lstat(rel)
	if err == nil {
		if !info.Mode().IsRegular() {
			return File{}, errors.New("destination must be a regular file (symlinks are denied)")
		}
		existing, err := root.ReadFile(rel)
		if err != nil {
			return File{}, fmt.Errorf("read destination: %w", err)
		}
		ownCopy := recorded != nil && fmt.Sprintf("%x", sha256.Sum256(existing)) == recorded.SHA256
		if ownCopy {
			file.Backup = recorded.Backup
		}
		if bytes.Equal(existing, data) {
			if emit != nil {
				emit(fmt.Sprintf("Unchanged %s", destination))
			}
			return file, nil
		}
		if ownCopy {
			if emit != nil {
				emit(fmt.Sprintf("Update %s (replacing DevEx's earlier copy)", destination))
			}
			if dryRun {
				return file, nil
			}
			return file, replaceFile(root, rel, data)
		}
		file.Backup = destination + "." + time.Now().UTC().Format("20060102T150405.000000000") + ".devex-bak"
	} else if !errors.Is(err, os.ErrNotExist) {
		return File{}, fmt.Errorf("access destination: %w", err)
	}
	if emit != nil {
		if file.Backup != "" {
			emit(fmt.Sprintf("Back up %s -> %s", destination, file.Backup))
		}
		emit(fmt.Sprintf("Copy %s -> %s (create parent directories)", source, destination))
	}
	if dryRun {
		return file, nil
	}
	if err := root.MkdirAll(filepath.Dir(rel), 0700); err != nil {
		return File{}, fmt.Errorf("create destination parent: %w", err)
	}
	// Write a sibling first; a failed write never displaces the user's file.
	tmp := rel + fmt.Sprintf(".%d.devex-tmp", time.Now().UnixNano())
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return File{}, fmt.Errorf("write destination: %w", err)
	}
	defer func() { _ = root.Remove(tmp) }()
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return File{}, err
	}
	if file.Backup != "" {
		backupRel, err := relativeDestination(home, file.Backup)
		if err != nil {
			return File{}, err
		}
		if err := root.Rename(rel, backupRel); err != nil {
			return File{}, fmt.Errorf("backup destination: %w", err)
		}
	}
	if err := root.Rename(tmp, rel); err != nil {
		return File{}, fmt.Errorf("replace destination (original retained in %s): %w", file.Backup, err)
	}
	return file, nil
}

// replaceFile atomically replaces rel with data.
func replaceFile(root *os.Root, rel string, data []byte) error {
	tmp := rel + fmt.Sprintf(".%d.devex-tmp", time.Now().UnixNano())
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("write destination: %w", err)
	}
	defer func() { _ = root.Remove(tmp) }()
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return root.Rename(tmp, rel)
}

// ReadResult returns persisted copies and warnings for an application's last stage.
func ReadResult(app string) (Result, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Result{}, err
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return Result{}, err
	}
	defer root.Close()
	return load(root, app)
}

// Cleanup removes only recorded files with unchanged content. User edits, untracked
// files, and timestamped backups survive uninstall. All access stays beneath HOME.
func Cleanup(app string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer root.Close()
	result, err := load(root, app)
	if err != nil {
		return err
	}
	var failures []error
	for _, file := range result.Files {
		rel, err := relativeDestination(home, file.Destination)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		info, err := root.Lstat(rel)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := root.ReadFile(rel)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != file.SHA256 {
			continue
		}
		if err := root.Remove(rel); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) == 0 {
		if err := root.Remove(manifest(app)); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// Revert undoes recorded copies: an unchanged copy is removed and the backup of
// the original file, if any, is moved back. A copy the user has changed since
// is kept, and so is its backup; each is reported. With dryRun nothing changes.
// The record is removed when every copy was reverted.
func Revert(app string, dryRun bool, emit func(string)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer root.Close()
	result, err := load(root, app)
	if err != nil {
		return err
	}
	say := func(format string, args ...any) {
		if emit != nil {
			emit(fmt.Sprintf(format, args...))
		}
	}
	var failures []error
	var kept []File
	for _, file := range result.Files {
		rel, err := relativeDestination(home, file.Destination)
		if err != nil {
			failures = append(failures, err)
			kept = append(kept, file)
			continue
		}
		state, err := copyState(root, rel, file)
		if err != nil {
			failures = append(failures, err)
			kept = append(kept, file)
			continue
		}
		if state == "modified" {
			kept = append(kept, file)
			say("Keep %s: changed since DevEx copied it", file.Destination)
			if file.Backup != "" {
				say("  The original is still at %s", file.Backup)
			}
			continue
		}
		backupRel := ""
		if file.Backup != "" {
			if backupRel, err = relativeDestination(home, file.Backup); err != nil {
				failures = append(failures, err)
				continue
			}
			if info, err := root.Lstat(backupRel); err != nil || !info.Mode().IsRegular() {
				backupRel = ""
			}
		}
		switch {
		case backupRel != "":
			say("Restore %s from %s", file.Destination, file.Backup)
		case state == "unchanged":
			say("Remove %s", file.Destination)
		default:
			continue
		}
		if dryRun {
			continue
		}
		if state == "unchanged" {
			if err := root.Remove(rel); err != nil {
				failures = append(failures, err)
				continue
			}
		}
		if backupRel != "" {
			if err := root.Rename(backupRel, rel); err != nil {
				failures = append(failures, fmt.Errorf("restore %s: %w", file.Destination, err))
				kept = append(kept, file)
			}
		}
	}
	if !dryRun {
		// Keep records only for copies that were not reverted.
		if len(kept) == 0 {
			if err := root.Remove(manifest(app)); err != nil && !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, err)
			}
		} else if err := save(root, app, Result{Files: kept}); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// CopyState reports whether a recorded copy is "unchanged", "modified" or "missing".
func CopyState(file File) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	rel, err := relativeDestination(home, file.Destination)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return "", err
	}
	defer root.Close()
	return copyState(root, rel, file)
}

func copyState(root *os.Root, rel string, file File) (string, error) {
	info, err := root.Lstat(rel)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "modified", nil
	}
	data, err := root.ReadFile(rel)
	if err != nil {
		return "", err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != file.SHA256 {
		return "modified", nil
	}
	return "unchanged", nil
}
