package skillinstall

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Destination identifies an agent skill directory relative to a validated root
// directory (user home, KIMI_CODE_HOME, or project directory).
type Destination struct {
	Root     string
	Relative string
}

// Path returns the absolute OS-native path of the skill directory.
func (d Destination) Path() string {
	return filepath.Join(d.Root, filepath.FromSlash(d.Relative))
}

// BundleEntry represents one directory or regular file inside a skill bundle.
type BundleEntry struct {
	Directory  bool
	Data       []byte
	Executable bool
}

// ValidateAgent verifies that agent is one of the supported coding agents.
func ValidateAgent(agent string) error {
	if agent != "claude" && agent != "codex" && agent != "kimi" {
		return fmt.Errorf("unsupported agent %q: use claude, codex or kimi", agent)
	}
	return nil
}

// ValidateScope verifies that scope is user or project and that --project-dir
// is only supplied with project scope.
func ValidateScope(scope, projectDir string) error {
	if scope != "user" && scope != "project" {
		return fmt.Errorf("unsupported scope %q: use user or project", scope)
	}
	if scope == "user" && projectDir != "" {
		return fmt.Errorf("--project-dir requires --scope project")
	}
	return nil
}

// ValidateSkillName ensures skillName is a single safe directory name with no
// path separators, parent traversal, or leading dots.
func ValidateSkillName(skillName string) error {
	if skillName == "" || skillName == "." || skillName == ".." {
		return fmt.Errorf("invalid skill name %q", skillName)
	}
	if strings.ContainsAny(skillName, `/\`) || strings.HasPrefix(skillName, ".") || strings.Contains(skillName, "\x00") {
		return fmt.Errorf("invalid skill name %q: must be a single directory name", skillName)
	}
	return nil
}

// ResolveDestination resolves the destination root and relative path for one
// skill installed for agent at scope.
func ResolveDestination(agent, scope, projectDir, skillName string) (Destination, error) {
	if err := ValidateAgent(agent); err != nil {
		return Destination{}, err
	}
	if err := ValidateScope(scope, projectDir); err != nil {
		return Destination{}, err
	}
	if err := ValidateSkillName(skillName); err != nil {
		return Destination{}, err
	}
	var root, directory string
	if scope == "project" {
		var err error
		if projectDir == "" {
			root, err = os.Getwd()
			if err != nil {
				return Destination{}, fmt.Errorf("resolve current working directory: %w", err)
			}
		} else {
			root, err = filepath.Abs(projectDir)
			if err != nil {
				return Destination{}, fmt.Errorf("resolve project directory: %w", err)
			}
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return Destination{}, fmt.Errorf("resolve home directory: %w", err)
		}
		root = home
	}
	switch agent {
	case "claude":
		directory = ".claude"
	case "codex":
		directory = ".agents"
	case "kimi":
		directory = ".kimi-code"
		if scope == "user" && os.Getenv("KIMI_CODE_HOME") != "" {
			var err error
			root, err = filepath.Abs(os.Getenv("KIMI_CODE_HOME"))
			if err != nil {
				return Destination{}, fmt.Errorf("resolve KIMI_CODE_HOME: %w", err)
			}
			directory = ""
		}
	}
	return Destination{
		Root:     filepath.Clean(root),
		Relative: filepath.ToSlash(filepath.Join(directory, "skills", skillName)),
	}, nil
}

// ResolveLegacyCodexDestination resolves the legacy ~/.codex/skills/<skill>
// directory used by older Codex guides (such as the tstack README). DevEx
// inspects this location read-only and never writes to it.
func ResolveLegacyCodexDestination(skillName string) (Destination, error) {
	if err := ValidateSkillName(skillName); err != nil {
		return Destination{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Destination{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return Destination{
		Root:     filepath.Clean(home),
		Relative: filepath.ToSlash(filepath.Join(".codex", "skills", skillName)),
	}, nil
}

// CheckCodexKimiConflict implements the shared .agents/skills conflict rules
// between Codex and Kimi Code. When installing for Kimi and a Codex copy
// already exists at .agents/skills/<skill>, skip is true with a warning. When
// installing for Codex and a Kimi copy exists, skip is false with a warning.
func CheckCodexKimiConflict(agent, scope, projectDir, skillName, label string) (skip bool, warning string, err error) {
	if agent != "codex" && agent != "kimi" {
		return false, "", nil
	}
	otherAgent := "codex"
	if agent == "codex" {
		otherAgent = "kimi"
	}
	other, err := ResolveDestination(otherAgent, scope, projectDir, skillName)
	if err != nil {
		return false, "", err
	}
	_, statErr := os.Lstat(other.Path())
	if statErr == nil {
		if agent == "kimi" {
			return true, fmt.Sprintf("Warning: %s is already installed for %s at %s; Kimi Code reads both locations, so no duplicate was created. One Codex installation in .agents/skills covers both Codex and Kimi.\n", label, otherAgent, other.Path()), nil
		}
		return false, fmt.Sprintf("Warning: %s is already installed for Kimi Code at %s; after installing for Codex, Kimi Code will load %s twice. Remove %s to avoid the duplicate; it was left unchanged.\n", label, other.Path(), label, other.Path()), nil
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return false, "", fmt.Errorf("inspect existing %s skill %q: %w", otherAgent, other.Path(), statErr)
	}
	return false, "", nil
}

// ReadBundle walks bundle starting at root and returns every directory and
// regular file with its contents and normalized executable bit. Symlinks and
// non-regular files are rejected.
func ReadBundle(bundle fs.FS, root string) (map[string]BundleEntry, error) {
	entries := map[string]BundleEntry{}
	err := fs.WalkDir(bundle, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(filepath.FromSlash(root), filepath.FromSlash(path))
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("skill bundle entry %q is a symlink", path)
		}
		if entry.IsDir() {
			entries[name] = BundleEntry{Directory: true}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("skill bundle entry %q is not a regular file", path)
		}
		data, err := fs.ReadFile(bundle, path)
		if err != nil {
			return err
		}
		entries[name] = BundleEntry{
			Data:       data,
			Executable: info.Mode()&0o111 != 0,
		}
		return nil
	})
	return entries, err
}

// ValidatePath verifies that no existing component along destination inside
// root is a symlink and that every existing parent component is a directory.
func ValidatePath(root *os.Root, destination string) error {
	current := ""
	for _, component := range strings.Split(destination, "/") {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("invalid skill path component %q in %q", component, destination)
		}
		if current == "" {
			current = component
		} else {
			current += "/" + component
		}
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect skill path %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("skill path %q contains a symlink at %q", destination, current)
		}
		if current != destination && !info.IsDir() {
			return fmt.Errorf("skill path component %q is not a directory", current)
		}
	}
	return nil
}

// OpenRoot validates destination.Root, creating it for user scope when not in
// dry-run mode, and opens an os.Root confined to destination.Root. If
// destination.Root does not exist during a dry-run, it returns (nil, false, nil)
// without writing to disk.
func OpenRoot(destination Destination, scope string, dryRun bool) (*os.Root, bool, error) {
	baseInfo, err := os.Stat(destination.Root)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if dryRun {
			return nil, false, nil
		}
		if scope == "project" {
			return nil, false, fmt.Errorf("project directory %q does not exist", destination.Root)
		}
		if err := os.MkdirAll(destination.Root, 0o755); err != nil {
			return nil, false, fmt.Errorf("create skill home %q: %w", destination.Root, err)
		}
	case err != nil:
		return nil, false, fmt.Errorf("inspect skill root %q: %w", destination.Root, err)
	case !baseInfo.IsDir():
		return nil, false, fmt.Errorf("skill root %q is not a directory", destination.Root)
	}
	root, err := os.OpenRoot(destination.Root)
	if err != nil {
		return nil, false, fmt.Errorf("open skill root %q: %w", destination.Root, err)
	}
	return root, true, nil
}

// StageAndPublish stages bundle next to destination inside root and atomically
// publishes it. When existing is true, the previous directory is renamed to a
// .devex-backup-* sibling before publishing, restored if publish fails, and
// kept (with its full path returned) when keepBackup is true.
func StageAndPublish(ctx context.Context, root *os.Root, destination Destination, bundle fs.FS, existing, keepBackup bool) (string, error) {
	parent := filepath.ToSlash(filepath.Dir(destination.Path()))
	parentRelative := filepath.ToSlash(filepath.Dir(filepath.FromSlash(destination.Relative)))
	if err := root.MkdirAll(parentRelative, 0o755); err != nil {
		return "", fmt.Errorf("create skill parent %q: %w", filepath.Dir(destination.Path()), err)
	}
	stage, err := os.MkdirTemp(parent, ".devex-install-*")
	if err != nil {
		return "", fmt.Errorf("create skill staging directory: %w", err)
	}
	stageRelative := filepath.ToSlash(filepath.Join(parentRelative, filepath.Base(stage)))
	defer func() { _ = root.RemoveAll(stageRelative) }()
	if err := StageBundle(ctx, root, stageRelative, bundle); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	backupRelative := ""
	if existing {
		var backupErr error
		backupRelative, backupErr = uniqueBackupRelative(root, parentRelative)
		if backupErr != nil {
			return "", fmt.Errorf("allocate backup path: %w", backupErr)
		}
		if err := root.Rename(destination.Relative, backupRelative); err != nil {
			return "", fmt.Errorf("back up existing skill: %w", err)
		}
	}
	if err := root.Rename(stageRelative, destination.Relative); err != nil {
		if existing {
			if restoreErr := root.Rename(backupRelative, destination.Relative); restoreErr != nil {
				return "", fmt.Errorf("publish skill: %w; restore backup %q manually: %w", err, filepath.Join(destination.Root, filepath.FromSlash(backupRelative)), restoreErr)
			}
		}
		return "", fmt.Errorf("publish skill: %w", err)
	}
	if existing {
		if !keepBackup {
			_ = root.RemoveAll(backupRelative)
			return "", nil
		}
		return filepath.Join(destination.Root, filepath.FromSlash(backupRelative)), nil
	}
	return "", nil
}

// BackupExisting moves an existing skill directory to a unique .devex-backup-*
// sibling inside root and returns the full path to the backup.
func BackupExisting(root *os.Root, destination Destination) (string, error) {
	parentRelative := filepath.ToSlash(filepath.Dir(filepath.FromSlash(destination.Relative)))
	backupRelative, err := uniqueBackupRelative(root, parentRelative)
	if err != nil {
		return "", fmt.Errorf("allocate backup path: %w", err)
	}
	if err := root.Rename(destination.Relative, backupRelative); err != nil {
		return "", fmt.Errorf("back up existing skill: %w", err)
	}
	return filepath.Join(destination.Root, filepath.FromSlash(backupRelative)), nil
}

func uniqueBackupRelative(root *os.Root, parentRelative string) (string, error) {
	base := time.Now().UnixNano()
	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf(".devex-backup-%d", base+int64(i))
		rel := filepath.ToSlash(filepath.Join(parentRelative, name))
		_, err := root.Lstat(rel)
		if errors.Is(err, os.ErrNotExist) {
			return rel, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find an unused backup directory name in %q", parentRelative)
}

// StageBundle copies bundle into destination inside root, preserving the
// executable bit (0755 when any source x bit is set, otherwise 0644) and
// rejecting symlinks or non-regular files.
func StageBundle(ctx context.Context, root *os.Root, destination string, bundle fs.FS) error {
	return fs.WalkDir(bundle, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("skill bundle entry %q is a symlink", path)
		}
		target := destination
		if path != "." {
			target = destination + "/" + path
		}
		if entry.IsDir() {
			if err := root.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create staged skill directory %q: %w", target, err)
			}
			return root.Chmod(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat skill file %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("skill bundle entry %q is not a regular file", path)
		}
		perm := os.FileMode(0o644)
		if info.Mode()&0o111 != 0 {
			perm = 0o755
		}
		data, err := fs.ReadFile(bundle, path)
		if err != nil {
			return fmt.Errorf("read skill file %q: %w", path, err)
		}
		if err := root.WriteFile(target, data, perm); err != nil {
			return fmt.Errorf("write staged skill file %q: %w", target, err)
		}
		if err := root.Chmod(target, perm); err != nil {
			return fmt.Errorf("set staged skill permissions %q: %w", target, err)
		}
		return nil
	})
}
