package setupundo

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
)

// OhMyZshMarker marks a framework clone created by DevEx setup.
const OhMyZshMarker = ".devex-created"

var (
	createdChecksum = regexp.MustCompile(`^[a-f0-9]{64}$`)
	ohMyZshRevision = regexp.MustCompile(`^[a-f0-9]{40}$`)
)

func OhMyZshMarkerContent(revision string) string {
	return "created by devex setup\nrevision: " + revision + "\n"
}

func restoreCreatedFile(_ string, change Change) error {
	allowed := make([]string, 0, 4)
	for _, name := range []string{"rc", "shell", "aliases", "init"} {
		allowed = append(allowed, filepath.Join(change.Root, name))
	}
	if !slices.Contains(allowed, change.Path) || !createdChecksum.MatchString(change.Checksum) {
		return fmt.Errorf("refusing to remove %q: not a recorded DevEx Zsh default", change.Path)
	}
	info, err := os.Lstat(change.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file; not removed", change.Path)
	}
	if change.Target == "" {
		return fmt.Errorf("%s has no setup-time resolved target in its legacy undo record; not removed; inspect it and remove it manually only if it is the DevEx-created default", change.Path)
	}
	current, err := filepath.EvalSymlinks(change.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if current != change.Target {
		return fmt.Errorf("%s now points to %s, not %s as after setup; not removed", change.Path, current, change.Target)
	}
	data, err := os.ReadFile(change.Path)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != change.Checksum {
		return fmt.Errorf("%s changed after setup; not removed", change.Path)
	}
	return os.Remove(change.Path)
}

func restoreOhMyZsh(home string, change Change) (string, error) {
	path := filepath.Join(home, ".oh-my-zsh")
	if change.Path != path || !ohMyZshRevision.MatchString(change.Value) {
		return "", fmt.Errorf("refusing to remove %q: not a recorded DevEx Oh My Zsh clone", change.Path)
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory; not removed", path)
	}
	marker := filepath.Join(path, OhMyZshMarker)
	markerInfo, err := os.Lstat(marker)
	if err != nil {
		return "", fmt.Errorf("read DevEx ownership marker: %w", err)
	}
	if !markerInfo.Mode().IsRegular() {
		return "", fmt.Errorf("DevEx ownership marker is not a regular file; not removed")
	}
	content, err := os.ReadFile(marker)
	if err != nil {
		return "", err
	}
	if string(content) != OhMyZshMarkerContent(change.Value) {
		return "", fmt.Errorf("DevEx ownership marker does not match; not removed")
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), filepath.Base(path)+".devex-undo-")
	if err != nil {
		return "", err
	}
	aside := filepath.Join(dir, filepath.Base(path))
	if err := os.Rename(path, aside); err != nil {
		_ = os.Remove(dir)
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return fmt.Sprintf("Moved DevEx-created Oh My Zsh to %s; its contents are preserved", aside), nil
}
