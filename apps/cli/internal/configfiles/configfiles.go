// Package configfiles implements the shared install stage between package installation
// and post-install hooks. Mapping failures are warnings unless an approved readable
// source changed; those changes fail the install before post-install hooks. Result
// (including warnings) is persisted for inspection and cleanup.
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

	"github.com/jameswlane/devex/apps/cli/internal/execinputs"
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
	// Changed is set when a source or marker differs from a confirmed plan;
	// unlike other warnings it must fail the install.
	Changed error `json:"-"`
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

// recordsDir holds the managed-file records, relative to HOME.
var recordsDir = filepath.Join(".local", "state", "devex", "config-files")

func relativeDestination(home, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("destination must be absolute and inside HOME")
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", errors.New("destination outside HOME is denied")
	}
	if within(recordsDir, rel) {
		return "", errors.New("destination overlaps DevEx managed-file records")
	}
	return rel, nil
}

// within reports whether rel is dir or inside it; both are relative to HOME.
func within(dir, rel string) bool {
	return rel == dir || strings.HasPrefix(rel, dir+string(filepath.Separator))
}

// maxSymlinks bounds symlink resolution, as the kernel does.
const maxSymlinks = 40

// resolve returns rel with every existing symlink along it followed, relative
// to HOME. Components that do not exist yet are kept as written.
func resolve(root *os.Root, home, rel string) (string, error) {
	pending := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	resolved, hops := "", 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if resolved == "" {
				return "", fmt.Errorf("%s resolves outside HOME", filepath.Join(home, rel))
			}
			if resolved = filepath.Dir(resolved); resolved == "." {
				resolved = ""
			}
			continue
		}
		next := filepath.Join(resolved, part)
		info, err := root.Lstat(next)
		if errors.Is(err, os.ErrNotExist) {
			return filepath.Join(append([]string{next}, pending...)...), nil
		}
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", filepath.Join(home, next), err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			resolved = next
			continue
		}
		if hops++; hops > maxSymlinks {
			return "", fmt.Errorf("resolve %s: too many symlinks", filepath.Join(home, rel))
		}
		target, err := root.Readlink(next)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", filepath.Join(home, next), err)
		}
		if filepath.IsAbs(target) {
			inHome, err := filepath.Rel(home, target)
			if err != nil || (inHome != "." && !filepath.IsLocal(inHome)) {
				return "", fmt.Errorf("%s links outside HOME (%s)", filepath.Join(home, next), target)
			}
			resolved, target = "", inHome
		}
		pending = append(strings.Split(target, string(filepath.Separator)), pending...)
	}
	return resolved, nil
}

// guarded is a directory no destination may resolve into, relative to HOME.
type guarded struct {
	rel, name, reason string
}

// guards lists the managed-file records and the protected directories inside
// HOME, both as written and with symlinks followed.
func guards(root *os.Root, home string, protected []string) []guarded {
	var out []guarded
	add := func(rel, reason string) {
		out = append(out, guarded{rel: rel, name: filepath.Join(home, rel), reason: reason})
		if real, err := resolve(root, home, rel); err == nil && real != rel {
			out = append(out, guarded{rel: real, name: filepath.Join(home, rel), reason: reason})
		}
	}
	add(recordsDir, "DevEx managed-file records")
	for _, dir := range protected {
		rel, err := filepath.Rel(home, filepath.Clean(dir))
		if err != nil || !filepath.IsLocal(rel) {
			continue // outside HOME, which no destination can reach
		}
		add(rel, "DevEx's own directory")
	}
	return out
}

// checkResolved refuses rel when, with symlinks in its path followed, it lands
// inside a guarded directory.
func checkResolved(root *os.Root, home, rel string, guarded []guarded) error {
	real, err := resolve(root, home, rel)
	if err != nil {
		return err
	}
	for _, g := range guarded {
		if within(g.rel, rel) || within(g.rel, real) {
			return fmt.Errorf("destination resolves to %s, inside %s %s; choose a destination outside it", filepath.Join(home, real), g.reason, g.name)
		}
	}
	return nil
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

// Options adjusts ApplyWithOptions.
type Options struct {
	// Protected lists directories no destination may be inside, also when a
	// symlink in the destination's path leads there.
	Protected []string
}

// Apply copies regular files only when their marker conditions match, backs up
// conflicts, leaves identical destinations unchanged, and records copies by
// content hash. A destination that already matched before DevEx wrote it is
// not recorded, so cleanup and revert never remove it. Dry runs validate and
// describe the same plan without creating directories, backups, files, or
// manifests. emit receives plan/warnings.
func Apply(ctx context.Context, app string, mappings []types.ConfigFile, dryRun bool, emit func(string)) Result {
	return ApplyWithOptions(ctx, app, mappings, dryRun, emit, Options{})
}

// ApplyWithOptions is Apply with options.
func ApplyWithOptions(ctx context.Context, app string, mappings []types.ConfigFile, dryRun bool, emit func(string), opts Options) Result {
	result := Result{}
	if len(mappings) == 0 {
		return result
	}
	warn := func(mapping types.ConfigFile, err error) {
		if result.Changed == nil && errors.Is(err, execinputs.ErrChanged) {
			result.Changed = err
		}
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
			present := err == nil && info.Mode().IsRegular()
			if verifyErr := execinputs.VerifyMarker(ctx, path, present); verifyErr != nil {
				warn(m, verifyErr)
				invalid = true
				break
			}
			if present {
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
	guarded := guards(root, home, opts.Protected)
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
		file, err := applyOne(ctx, root, home, m, previousCopy(previous, m), guarded, dryRun, emit)
		if err != nil {
			warn(m, err)
			continue
		}
		if file.Destination == "" {
			continue // already matched and not DevEx's copy: not taken over
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

// sourceAccessError keeps optional-source failures as warnings, but a source
// approved as readable must remain readable and regular during execution.
// Preserve the underlying I/O error alongside ErrChanged for diagnostics.
func sourceAccessError(ctx context.Context, source string, cause error) error {
	if inputs, pinned := execinputs.Pinned(ctx); pinned {
		if approved, known := inputs.Files[source]; known && approved != execinputs.Missing {
			return fmt.Errorf("source %s %w; cannot read the approved file: %w", source, execinputs.ErrChanged, cause)
		}
	}
	return cause
}

// applyOne copies one mapping. A destination that still holds the previously
// recorded copy is DevEx's own file: it is replaced without a new backup and
// keeps the backup of the user's original file. A destination that already
// matches but is not DevEx's copy returns an empty File: it is left alone and
// not recorded.
func applyOne(ctx context.Context, root *os.Root, home string, m types.ConfigFile, recorded *File, guarded []guarded, dryRun bool, emit func(string)) (File, error) {
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
	if err := checkResolved(root, home, rel, guarded); err != nil {
		return File{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return File{}, sourceAccessError(ctx, source, fmt.Errorf("read source: %w", err))
	}
	if !info.Mode().IsRegular() {
		return File{}, sourceAccessError(ctx, source, errors.New("source must be a regular file"))
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return File{}, sourceAccessError(ctx, source, fmt.Errorf("read source: %w", err))
	}
	// A confirmed install copies only the content that was confirmed.
	if err := execinputs.VerifyFile(ctx, source, data); err != nil {
		return File{}, err
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
			if !ownCopy {
				return File{}, nil
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
	return cleanup(app, nil)
}

// ErrRecordChanged reports that an application's managed-file record differs
// from the one a caller approved.
var ErrRecordChanged = errors.New("the managed configuration-file record changed since it was approved")

// CleanupRecorded is Cleanup bound to an approved record: it removes nothing
// and returns ErrRecordChanged unless the current record lists exactly the
// approved files.
func CleanupRecorded(app string, approved []File) error {
	return cleanup(app, &approved)
}

func sameFiles(a, b []File) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cleanup(app string, approved *[]File) error {
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
	if approved != nil && !sameFiles(result.Files, *approved) {
		return fmt.Errorf("%s: %w; prepare the uninstall again", app, ErrRecordChanged)
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

// Revert undoes recorded copies: an unchanged copy is replaced by the backup of
// the original file, or removed when there was none. A copy the user has
// changed since is kept, and so is its backup; each is reported. A copy whose
// recorded backup is missing, inaccessible or not a regular file is kept, as
// is any copy that could not be reverted, and an error says what to do. With
// dryRun nothing changes. Only reverted copies leave the record, which is
// removed once none remain.
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
				failures = append(failures, fmt.Errorf("restore %s from %s: %w", file.Destination, file.Backup, err))
				kept = append(kept, file)
				continue
			}
			info, err := root.Lstat(backupRel)
			switch {
			case errors.Is(err, os.ErrNotExist) && state == "missing":
				// Nothing is left to restore or remove.
				say("Forget %s: neither it nor its backup %s exists", file.Destination, file.Backup)
				continue
			case errors.Is(err, os.ErrNotExist):
				failures = append(failures, fmt.Errorf("cannot restore %s: its backup %s is missing; %s and its record were kept; put the original back at %s, or delete %s if you no longer need it, then run the rollback again", file.Destination, file.Backup, file.Destination, file.Backup, file.Destination))
				kept = append(kept, file)
				continue
			case err != nil:
				failures = append(failures, fmt.Errorf("cannot restore %s: its backup %s is not accessible (%w); %s and its record were kept; fix access to the backup, then run the rollback again", file.Destination, file.Backup, err, file.Destination))
				kept = append(kept, file)
				continue
			case !info.Mode().IsRegular():
				failures = append(failures, fmt.Errorf("cannot restore %s: its backup %s is not a regular file; %s and its record were kept; put the original back at %s, then run the rollback again", file.Destination, file.Backup, file.Destination, file.Backup))
				kept = append(kept, file)
				continue
			}
		}
		switch {
		case backupRel != "":
			say("Restore %s from %s", file.Destination, file.Backup)
		case state == "unchanged":
			say("Remove %s", file.Destination)
		default:
			continue // missing and nothing to restore
		}
		if dryRun {
			continue
		}
		if backupRel != "" {
			// Rename over the deployed copy: if it fails, that copy stays.
			if err := root.Rename(backupRel, rel); err != nil {
				failures = append(failures, fmt.Errorf("restore %s from %s: %w", file.Destination, file.Backup, err))
				kept = append(kept, file)
			}
			continue
		}
		if err := root.Remove(rel); err != nil {
			failures = append(failures, fmt.Errorf("remove %s: %w", file.Destination, err))
			kept = append(kept, file)
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
