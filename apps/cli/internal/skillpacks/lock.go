package skillpacks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

const (
	// LockFileName is the filename of the skill pack lock file.
	LockFileName = "agent-skills.lock.json"
	// LockFileVersion is the current lock file schema version.
	LockFileVersion = 1
)

// FileRecord stores the locked relative path, SHA-256 hash, and normalized mode
// of one file inside an installed skill.
type FileRecord struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode"`
}

// InstalledSkill records one DevEx-managed skill installation keyed by its
// destination directory path.
type InstalledSkill struct {
	Pack        string       `json:"pack"`
	Skill       string       `json:"skill"`
	Agent       string       `json:"agent"`
	Scope       string       `json:"scope"`
	Destination string       `json:"destination"`
	Source      string       `json:"source"`
	SkillsPath  string       `json:"skills_path"`
	Ref         string       `json:"ref"`
	SHA         string       `json:"sha"`
	InstalledAt time.Time    `json:"installed_at"`
	Files       []FileRecord `json:"files"`
	Unreviewed  bool         `json:"unreviewed,omitempty"`
}

// LockFile tracks every DevEx-managed skill installation for a given scope.
type LockFile struct {
	Version int                       `json:"version"`
	Skills  map[string]InstalledSkill `json:"skills"`
}

// NewLockFile initializes an empty LockFile.
func NewLockFile() *LockFile {
	return &LockFile{
		Version: LockFileVersion,
		Skills:  map[string]InstalledSkill{},
	}
}

// UserLockPath returns the lock file path for user-scope installations in the
// DevEx state directory.
func UserLockPath() (string, error) {
	dirs, err := devexpaths.Resolve()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.State, LockFileName), nil
}

// ProjectLockPath returns PROJECT/.devex/agent-skills.lock.json for projectDir
// (or the current working directory when projectDir is empty).
func ProjectLockPath(projectDir string) (string, error) {
	var root string
	var err error
	if projectDir == "" {
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve current working directory: %w", err)
		}
	} else {
		root, err = filepath.Abs(projectDir)
		if err != nil {
			return "", fmt.Errorf("resolve project directory: %w", err)
		}
	}
	return filepath.Join(filepath.Clean(root), ".devex", LockFileName), nil
}

// LockPath returns the lock file path for the given scope and optional projectDir.
func LockPath(scope, projectDir string) (string, error) {
	if scope == "project" {
		return ProjectLockPath(projectDir)
	}
	return UserLockPath()
}

// LoadLock reads and decodes a lock file from path. If the file does not exist,
// an empty LockFile is returned with a nil error.
func LoadLock(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewLockFile(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skill pack lock file %q: %w", path, err)
	}
	return ParseLock(data, path)
}

// ParseLock decodes raw lock file JSON bytes.
func ParseLock(data []byte, path string) (*LockFile, error) {
	var lock LockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("decode skill pack lock file %q: %w", path, err)
	}
	if lock.Version == 0 {
		lock.Version = LockFileVersion
	}
	if lock.Skills == nil {
		lock.Skills = map[string]InstalledSkill{}
	}
	return &lock, nil
}

// SaveLock writes lock atomically with mode 0600 to path.
func SaveLock(path string, lock *LockFile) error {
	if lock == nil {
		lock = NewLockFile()
	}
	if lock.Version == 0 {
		lock.Version = LockFileVersion
	}
	if lock.Skills == nil {
		lock.Skills = map[string]InstalledSkill{}
	}
	for k, skill := range lock.Skills {
		sort.Slice(skill.Files, func(i, j int) bool {
			return skill.Files[i].Path < skill.Files[j].Path
		})
		lock.Skills[k] = skill
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create lock file directory %q: %w", dir, err)
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skill pack lock file: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".agent-skills-lock-*.json")
	if err != nil {
		return fmt.Errorf("create temporary lock file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write lock file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod lock file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync lock file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("publish lock file: %w", err)
	}
	cleanup = false
	return nil
}

// SortedEntries returns all InstalledSkill entries sorted deterministically by
// pack, scope, agent, skill, and destination.
func (l *LockFile) SortedEntries() []InstalledSkill {
	if l == nil || len(l.Skills) == 0 {
		return nil
	}
	entries := make([]InstalledSkill, 0, len(l.Skills))
	for _, s := range l.Skills {
		entries = append(entries, s)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Pack != entries[j].Pack {
			return entries[i].Pack < entries[j].Pack
		}
		if entries[i].Scope != entries[j].Scope {
			return entries[i].Scope < entries[j].Scope
		}
		if entries[i].Agent != entries[j].Agent {
			return entries[i].Agent < entries[j].Agent
		}
		if entries[i].Skill != entries[j].Skill {
			return entries[i].Skill < entries[j].Skill
		}
		return entries[i].Destination < entries[j].Destination
	})
	return entries
}
