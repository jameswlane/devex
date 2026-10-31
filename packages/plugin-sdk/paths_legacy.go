package sdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type legacyMove struct{ source, destination string }

// Keep legacy knowledge here for the one-release compatibility window.
func (p Paths) legacyMoves() []legacyMove {
	old := filepath.Join(p.Home, ".devex")
	data := filepath.Join(p.Home, ".local", "share", "devex")
	moves := []legacyMove{}
	for _, name := range []string{"config.yaml", "config", "environments", "templates", "team", "plugins.yaml", "security.yaml"} {
		moves = append(moves, legacyMove{filepath.Join(old, name), filepath.Join(p.Config, name)})
	}
	for _, name := range []string{"datastore.db", "datastore.db-wal", "datastore.db-shm", "state.db", "state.db-wal", "state.db-shm", "undo", "recovery"} {
		moves = append(moves, legacyMove{filepath.Join(old, name), filepath.Join(p.State, name)})
	}
	for _, name := range []string{"plugins", "backups"} {
		moves = append(moves, legacyMove{filepath.Join(old, name), filepath.Join(p.Data, name)})
	}
	moves = append(moves, legacyMove{filepath.Join(data, "logs"), p.Logs()})
	for _, name := range []string{"config", "assets", "help", "migrations"} {
		if data != p.Data {
			moves = append(moves, legacyMove{filepath.Join(data, name), filepath.Join(p.Data, name)})
		}
	}
	return moves
}

// ReadPath prefers the resolved location, falling back for one release only.
// Linked legacy trees are never used. Writers should use the resolved path directly.
func (p Paths) ReadPath(path string) string {
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return path
	}
	for _, move := range p.legacyMoves() {
		rel, err := filepath.Rel(move.destination, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		candidate := filepath.Join(move.source, rel)
		parent, err := openPathRoot(filepath.Dir(candidate), false, nil)
		if err != nil {
			continue
		}
		info, err := parent.Lstat(filepath.Base(candidate))
		_ = parent.Close()
		if err == nil && info.Mode()&os.ModeSymlink == 0 {
			return candidate
		}
	}
	return path
}

// openPathRoot walks from the filesystem root without following symlinks. Every
// component is checked against the opened inode, as with plugin install storage.
func openPathRoot(path string, create bool, owner *Downloader) (*os.Root, error) {
	path = filepath.Clean(path)
	if owner != nil {
		rel, err := filepath.Rel(owner.installOwner.HomeDir, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("elevated storage must remain beneath invoking user's home: %s", path)
		}
		root, err := openPathRoot(owner.installOwner.HomeDir, false, nil)
		if err != nil {
			return nil, err
		}
		logical := owner.installOwner.HomeDir
		if rel == "." {
			return root, nil
		}
		for _, part := range strings.Split(rel, string(os.PathSeparator)) {
			logical = filepath.Join(logical, part)
			next, err := owner.openOwnedDirectory(root, part, logical)
			_ = root.Close()
			if err != nil {
				return nil, err
			}
			root = next
		}
		return root, nil
	}

	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("storage path must be absolute: %s", path)
	}
	volume := filepath.VolumeName(path) + string(os.PathSeparator)
	root, err := os.OpenRoot(volume)
	if err != nil {
		return nil, err
	}
	rest := strings.TrimPrefix(path, volume)
	logical := volume
	for _, part := range strings.Split(rest, string(os.PathSeparator)) {
		if part == "" {
			continue
		}
		logical = filepath.Join(logical, part)
		var next *os.Root
		if owner != nil {
			next, err = owner.openOwnedDirectory(root, part, logical)
		} else {
			var before os.FileInfo
			before, err = root.Lstat(part)
			if errors.Is(err, os.ErrNotExist) && create {
				err = root.Mkdir(part, 0700)
				if err == nil || errors.Is(err, os.ErrExist) {
					before, err = root.Lstat(part)
				}
			}
			if err == nil {
				if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
					err = fmt.Errorf("refusing symlinked storage directory %s", logical)
				} else {
					next, err = root.OpenRoot(part)
					if err == nil {
						after, e := next.Stat(".")
						if e != nil || !os.SameFile(before, after) {
							_ = next.Close()
							next = nil
							err = fmt.Errorf("storage directory changed while opening %s", logical)
						}
					}
				}
			}
		}
		_ = root.Close()
		if err != nil {
			return nil, err
		}
		root = next
	}
	return root, nil
}

// Migrate moves legacy files individually, publishing complete files before
// removing sources. Existing destinations always win, so retries and partially
// completed migrations preserve user changes. Dry runs perform validation only.
func (p Paths) Migrate(ctx context.Context, dryRun bool, out io.Writer) error {
	var owner *Downloader
	resolver := NewPrivilegeResolver()
	if resolver.Elevated {
		identity, err := resolver.ResolveUser(nil)
		if err != nil {
			return err
		}
		if identity.UID != "0" {
			if identity.HomeDir != p.Home {
				return fmt.Errorf("migration home does not match invoking account")
			}
			owner = &Downloader{installOwner: identity}
		}
	}
	moves := p.legacyMoves()
	for _, name := range []string{"cache", "plugin-cache", "setup-cache"} {
		if err := validateLegacy(ctx, filepath.Join(p.Home, ".devex", name)); err != nil {
			return err
		}
	}
	for _, move := range moves {
		if err := validateLegacy(ctx, move.source); err != nil {
			return err
		}
	}
	for _, move := range moves {
		if err := moveLegacy(ctx, move.source, move.destination, dryRun, out, owner); err != nil {
			return fmt.Errorf("migrate %s to %s: %w", move.source, move.destination, err)
		}
	}
	old := filepath.Join(p.Home, ".devex")
	if _, err := os.Lstat(old); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	root, err := openPathRoot(old, false, nil)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	for _, cache := range []string{"cache", "plugin-cache", "setup-cache"} {
		if err := validateLegacy(ctx, filepath.Join(old, cache)); err != nil {
			return err
		}
		if _, err := root.Lstat(cache); err == nil {
			if dryRun {
				_, _ = fmt.Fprintf(out, "Would discard legacy cache %s\n", filepath.Join(old, cache))
			} else {
				if err := root.RemoveAll(cache); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(out, "Discarded legacy cache %s\n", filepath.Join(old, cache))
			}
		}
	}
	if dryRun {
		_, _ = fmt.Fprintf(out, "Would write migration README in %s\n", old)
		return nil
	}
	text := fmt.Sprintf("DevEx files have moved to the XDG directories.\nConfig: %s\nData: %s\nState: %s\nCache: %s\nRun devex paths to inspect the current layout.\nThis legacy directory is retained for one release; unresolved conflicts remain here.\n", p.Config, p.Data, p.State, p.Cache)
	if existing, err := root.ReadFile("README"); err == nil && string(existing) == text {
		return nil
	}
	var own func(*os.File) error
	if owner != nil {
		own = func(file *os.File) error { return owner.ownInstallFile(file, filepath.Join(old, "README")) }
	}
	return atomicStorageWrite(root, "README", []byte(text), 0600, own)
}

func validateLegacy(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := openPathRoot(filepath.Dir(path), false, nil)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	info, err := root.Lstat(filepath.Base(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return fmt.Errorf("refusing symlinked or special migration source %s", path)
	}
	if info.IsDir() {
		child, err := root.OpenRoot(filepath.Base(path))
		if err != nil {
			return err
		}
		defer func() { _ = child.Close() }()
		entries, err := child.Open(".")
		if err != nil {
			return err
		}
		names, err := entries.Readdirnames(-1)
		_ = entries.Close()
		if err != nil {
			return err
		}
		for _, name := range names {
			if err := validateLegacy(ctx, filepath.Join(path, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func moveLegacy(ctx context.Context, source, destination string, dry bool, out io.Writer, owner *Downloader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.HasPrefix(filepath.Clean(destination), filepath.Clean(source)+string(os.PathSeparator)) {
		return fmt.Errorf("migration destination must not be inside source: %s", destination)
	}
	if filepath.Clean(source) == filepath.Clean(destination) {
		return nil
	}
	src, err := openPathRoot(filepath.Dir(source), false, nil)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	name := filepath.Base(source)
	info, err := src.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlinked source %s", source)
	}
	if info.IsDir() {
		if dry {
			dest, err := openPathRoot(destination, false, nil)
			if err == nil {
				_ = dest.Close()
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if !dry {
			dest, err := openPathRoot(destination, true, owner)
			if err != nil {
				return err
			}
			_ = dest.Close()
		}
		dir, err := openPathRoot(source, false, nil)
		if err != nil {
			return err
		}
		file, err := dir.Open(".")
		if err != nil {
			_ = dir.Close()
			return err
		}
		names, err := file.Readdirnames(-1)
		_ = file.Close()
		_ = dir.Close()
		if err != nil {
			return err
		}
		for _, name := range names {
			if err := moveLegacy(ctx, filepath.Join(source, name), filepath.Join(destination, name), dry, out, owner); err != nil {
				return err
			}
		}
		if !dry { // Only remove an empty source; conflicts remain for manual review.
			err := src.Remove(name)
			if err != nil {
				if _, e := src.Stat(name); e != nil {
					return err
				}
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing special source %s", source)
	}
	parent, e := openPathRoot(filepath.Dir(destination), false, nil)
	if e == nil {
		_ = parent.Close()
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if target, err := os.Lstat(destination); err == nil {
		if target.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlinked migration destination %s", destination)
		}
		_, _ = fmt.Fprintf(out, "Keeping existing %s (legacy source retained: %s)\n", destination, source)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if dry {
		_, _ = fmt.Fprintf(out, "Would move %s -> %s\n", source, destination)
		return nil
	}
	dst, err := openPathRoot(filepath.Dir(destination), true, owner)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()
	input, err := src.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	opened, err := input.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return fmt.Errorf("source changed while opening %s", source)
	}
	temp, err := createStorageTemp(dst, ".migration-")
	if err != nil {
		return err
	}
	tempName := filepath.Base(temp.Name())
	defer func() { _ = temp.Close(); _ = dst.Remove(tempName) }()
	if _, err := io.Copy(temp, input); err != nil {
		return err
	}
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if owner != nil {
		if err := owner.ownInstallFile(temp, destination); err != nil {
			return err
		}
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	// Link publishes without overwriting a destination created by another process.
	if err := dst.Link(tempName, filepath.Base(destination)); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	directory, err := dst.Open(".")
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	_ = directory.Close()
	if syncErr != nil && runtime.GOOS != "windows" {
		return syncErr
	}
	current, err := src.Lstat(name)
	if err != nil {
		return err
	}
	if !os.SameFile(info, current) {
		return fmt.Errorf("migration source changed: %s", source)
	}
	if err := src.Remove(name); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Moved %s -> %s\n", source, destination)
	return nil
}
