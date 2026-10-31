package sdk

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// All mutations stay anchored to an open directory, including across renames of
// user-owned parent directories. O_EXCL refuses pre-existing temporary entries.
func createStorageTemp(root *os.Root, prefix string) (*os.File, error) {
	for range 10 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		name := prefix + hex.EncodeToString(random[:])
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return file, err
	}
	return nil, fmt.Errorf("cannot allocate exclusive plugin storage file")
}

func rejectStorageSymlink(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular or symlinked plugin storage path %s", name)
	}
	return nil
}

func publishStorageFile(root *os.Root, file *os.File, destination string) error {
	name := filepath.Base(file.Name())
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	current, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		return fmt.Errorf("plugin temporary file was replaced: %s", name)
	}
	if err := rejectStorageSymlink(root, destination); err != nil {
		return err
	}
	// Rename replaces a concurrently planted destination link; it never follows it.
	return root.Rename(name, destination)
}

func atomicStorageWrite(root *os.Root, name string, data []byte, mode os.FileMode, own func(*os.File) error) error {
	if err := rejectStorageSymlink(root, name); err != nil {
		return err
	}
	file, err := createStorageTemp(root, ".devex-write-")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close(); _ = root.Remove(filepath.Base(file.Name())) }()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if own != nil {
		if err := own(file); err != nil {
			return err
		}
	}
	if err := publishStorageFile(root, file, name); err != nil {
		return err
	}
	return file.Close()
}

func openStorageRoot(path string) (*os.Root, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing symlinked plugin storage directory %s", path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = root.Close()
		return nil, fmt.Errorf("plugin storage directory changed while opening %s", path)
	}
	return root, nil
}
