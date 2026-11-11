package setupundo

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
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

func restoreCreatedFile(home string, change Change) error {
	allowed := make([]string, 0, 4)
	for _, name := range []string{"rc", "shell", "aliases", "init"} {
		allowed = append(allowed, filepath.Join(devexpaths.ForHome(home).Data, "defaults", "zsh", name))
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
	data, err := os.ReadFile(change.Path)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != change.Checksum {
		return fmt.Errorf("%s changed after setup; not removed", change.Path)
	}
	return os.Remove(change.Path)
}

func restoreOhMyZsh(home string, change Change) error {
	path := filepath.Join(home, ".oh-my-zsh")
	if change.Path != path || !ohMyZshRevision.MatchString(change.Value) {
		return fmt.Errorf("refusing to remove %q: not a recorded DevEx Oh My Zsh clone", change.Path)
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory; not removed", path)
	}
	marker := filepath.Join(path, OhMyZshMarker)
	markerInfo, err := os.Lstat(marker)
	if err != nil {
		return fmt.Errorf("read DevEx ownership marker: %w", err)
	}
	if !markerInfo.Mode().IsRegular() {
		return fmt.Errorf("DevEx ownership marker is not a regular file; not removed")
	}
	content, err := os.ReadFile(marker)
	if err != nil {
		return err
	}
	if string(content) != OhMyZshMarkerContent(change.Value) {
		return fmt.Errorf("DevEx ownership marker does not match; not removed")
	}
	return os.RemoveAll(path)
}
