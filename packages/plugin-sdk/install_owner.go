package sdk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SetInstallOwner accepts a resolved Unix account for elevated plugin storage.
func (d *Downloader) SetInstallOwner(identity UserIdentity) error {
	if _, _, err := installOwnerIDs(&identity); err != nil {
		return err
	}
	if filepath.Clean(d.pluginDir) != PathsForHome(identity.HomeDir).Plugins() {
		return fmt.Errorf("elevated plugin storage must be at the resolved invoking user data directory")
	}
	d.installOwner = &identity
	d.cacheDir = filepath.Join(PathsForHome(identity.HomeDir).Cache, "plugins")
	return nil
}

func installOwnerIDs(identity *UserIdentity) (int, int, error) {
	uid, err := strconv.Atoi(identity.UID)
	if err != nil || uid < 0 {
		return 0, 0, fmt.Errorf("invalid plugin owner UID %q", identity.UID)
	}
	gid, err := strconv.Atoi(identity.GID)
	if err != nil || gid < 0 {
		return 0, 0, fmt.Errorf("invalid plugin owner GID %q", identity.GID)
	}
	return uid, gid, nil
}

// Ownership and mode changes use the already open inode, never a user-replaceable path.
func (d *Downloader) ownInstallFile(file *os.File, logicalPath string) error {
	if d.installOwner == nil {
		return nil
	}
	uid, gid, err := installOwnerIDs(d.installOwner)
	if err != nil {
		return err
	}
	if d.chown != nil {
		err = d.chown(logicalPath, uid, gid)
	} else {
		err = file.Chown(uid, gid)
	}
	if err != nil {
		return fmt.Errorf("set plugin owner for %s: %w", logicalPath, err)
	}
	return nil
}

func (d *Downloader) ownExistingInstallFile(root *os.Root, name string) error {
	if d.installOwner == nil {
		return nil
	}
	if err := rejectStorageSymlink(root, name); err != nil {
		return err
	}
	uid, gid, err := installOwnerIDs(d.installOwner)
	if err != nil {
		return err
	}
	if d.chown != nil {
		return d.chown(filepath.Join(d.pluginDir, name), uid, gid)
	}
	// Lchown is anchored and cannot change an outside target if a link is planted.
	return root.Lchown(name, uid, gid)
}

func (d *Downloader) openOwnedDirectory(parent *os.Root, name, logicalPath string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	created := false
	if errors.Is(err, os.ErrNotExist) {
		if err := parent.Mkdir(name, 0755); err != nil {
			if errors.Is(err, os.ErrExist) {
				return d.openOwnedDirectory(parent, name, logicalPath)
			}
			return nil, err
		}
		created = true
		before, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing symlinked plugin storage directory %s", logicalPath)
	}
	uid, _, err := installOwnerIDs(d.installOwner)
	if err != nil {
		return nil, err
	}
	if !created {
		if err := checkDirectoryOwner(before, uid); err != nil {
			return nil, fmt.Errorf("%s: %w", logicalPath, err)
		}
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = child.Close()
		return nil, fmt.Errorf("plugin directory changed while opening %s", logicalPath)
	}
	if created {
		file, err := child.Open(".")
		if err == nil {
			err = d.ownInstallFile(file, logicalPath)
			_ = file.Close()
		}
		if err != nil {
			_ = child.Close()
			return nil, err
		}
		after, err = child.Stat(".")
		if err == nil {
			err = checkDirectoryOwner(after, uid)
		}
		if err != nil {
			_ = child.Close()
			return nil, err
		}
	}
	return child, nil
}

func (d *Downloader) openInstallStorage(cache bool) (*os.Root, error) {
	path := d.pluginDir
	if cache {
		path = d.cacheDir
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("plugin storage path must be absolute; check DevEx directory configuration: %s", path)
	}
	if d.installOwner == nil {
		return openStorageRoot(path)
	}
	rel, err := filepath.Rel(d.installOwner.HomeDir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("elevated storage must remain beneath invoking user's home: %s", path)
	}
	home, err := openPathRoot(d.installOwner.HomeDir, false, nil)
	if err != nil {
		return nil, err
	}
	logical := d.installOwner.HomeDir
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		logical = filepath.Join(logical, part)
		next, err := d.openOwnedDirectory(home, part, logical)
		_ = home.Close()
		if err != nil {
			return nil, err
		}
		home = next
	}
	return home, nil
}

// PrepareInstallDirectory refuses linked/foreign-owned storage before elevated writes.
func (d *Downloader) PrepareInstallDirectory() error {
	root, err := d.openInstallStorage(false)
	if err != nil {
		return err
	}
	return root.Close()
}
