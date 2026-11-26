package skillpacks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	skillinstall "github.com/jameswlane/devex/apps/cli/internal/agentskills/install"
)

// SnapshotFile holds one validated regular file read from a skill snapshot.
type SnapshotFile struct {
	Path   string
	Data   []byte
	SHA256 string
	Mode   fs.FileMode // normalized: 0o755 if executable, else 0o644
}

// SkillSnapshot is an immutable, symlink-free in-memory snapshot of one skill directory.
type SkillSnapshot struct {
	Name  string
	Files map[string]SnapshotFile
}

// NormalizedMode normalizes a file's permissions to 0755 when any executable
// bit is set and 0644 otherwise.
func NormalizedMode(mode fs.FileMode) fs.FileMode {
	if mode&0o111 != 0 {
		return 0o755
	}
	return 0o644
}

// FormatFileMode formats a normalized file mode as a 4-digit octal string.
func FormatFileMode(mode fs.FileMode) string {
	return fmt.Sprintf("%04o", NormalizedMode(mode).Perm())
}

// FileRecords returns deterministic sorted FileRecords for locking.
func (s *SkillSnapshot) FileRecords() []FileRecord {
	paths := make([]string, 0, len(s.Files))
	for p := range s.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	records := make([]FileRecord, 0, len(paths))
	for _, p := range paths {
		f := s.Files[p]
		records = append(records, FileRecord{
			Path:   f.Path,
			SHA256: f.SHA256,
			Mode:   FormatFileMode(f.Mode),
		})
	}
	return records
}

// FS returns a read-only fs.FS backed by this snapshot with normalized modes.
func (s *SkillSnapshot) FS() fs.FS {
	return &snapshotFS{files: s.Files}
}

// MatchesRecords reports whether s has the exact same set of files, SHA-256
// hashes, and normalized executable bits as records.
func (s *SkillSnapshot) MatchesRecords(records []FileRecord) bool {
	if len(s.Files) != len(records) {
		return false
	}
	for _, rec := range records {
		f, ok := s.Files[rec.Path]
		if !ok {
			return false
		}
		if f.SHA256 != rec.SHA256 || FormatFileMode(f.Mode) != rec.Mode {
			return false
		}
	}
	return true
}

// Equals reports whether two snapshots have identical file sets, hashes, and modes.
func (s *SkillSnapshot) Equals(other *SkillSnapshot) bool {
	if s == nil || other == nil {
		return false
	}
	return s.MatchesRecords(other.FileRecords())
}

// ReadSkillSnapshot reads <checkoutDir>/<skillsPath>/<skillName>/ through a
// confined os.Root, rejecting symlinks, non-regular files, path traversal, and
// skill directories missing SKILL.md.
func ReadSkillSnapshot(checkoutDir, skillsPath, skillName string) (*SkillSnapshot, error) {
	if err := validateRelativeSkillsPath(skillsPath); err != nil {
		return nil, fmt.Errorf("invalid skills path %q: %w", skillsPath, err)
	}
	if err := skillinstall.ValidateSkillName(skillName); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(checkoutDir)
	if err != nil {
		return nil, fmt.Errorf("open checkout root %q: %w", checkoutDir, err)
	}
	defer func() { _ = root.Close() }()

	relSkillDir := skillName
	if skillsPath != "" && skillsPath != "." {
		relSkillDir = path.Clean(skillsPath) + "/" + skillName
	}
	if err := skillinstall.ValidatePath(root, relSkillDir); err != nil {
		return nil, err
	}
	info, err := root.Lstat(relSkillDir)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found at %q: %w", skillName, relSkillDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("skill %q at %q is not a regular directory", skillName, relSkillDir)
	}
	snap, err := readSnapshotFromRoot(root, relSkillDir, skillName)
	if err != nil {
		return nil, err
	}
	if _, ok := snap.Files["SKILL.md"]; !ok {
		return nil, fmt.Errorf("skill %q is missing required SKILL.md", skillName)
	}
	return snap, nil
}

// DiscoverSkills lists all valid skill directories (containing SKILL.md) under
// <checkoutDir>/<skillsPath> without following symlinks.
func DiscoverSkills(checkoutDir, skillsPath string) ([]string, error) {
	if err := validateRelativeSkillsPath(skillsPath); err != nil {
		return nil, fmt.Errorf("invalid skills path %q: %w", skillsPath, err)
	}
	root, err := os.OpenRoot(checkoutDir)
	if err != nil {
		return nil, fmt.Errorf("open checkout root %q: %w", checkoutDir, err)
	}
	defer func() { _ = root.Close() }()

	base := "."
	if skillsPath != "" && skillsPath != "." {
		base = path.Clean(skillsPath)
		if err := skillinstall.ValidatePath(root, base); err != nil {
			return nil, err
		}
	}
	f, err := root.Open(base)
	if err != nil {
		return nil, fmt.Errorf("open skills directory %q: %w", base, err)
	}
	entries, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("read skills directory %q: %w", base, err)
	}
	var skills []string
	for _, entry := range entries {
		name := entry.Name()
		if skillinstall.ValidateSkillName(name) != nil {
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		skillMD := name + "/SKILL.md"
		if base != "." {
			skillMD = base + "/" + skillMD
		}
		info, statErr := root.Lstat(skillMD)
		if statErr == nil && info.Mode().IsRegular() {
			skills = append(skills, name)
		}
	}
	sort.Strings(skills)
	if len(skills) == 0 {
		return nil, fmt.Errorf("no skills containing SKILL.md found under %q", skillsPath)
	}
	return skills, nil
}

// SnapshotFromDir reads an on-disk skill directory using a confined os.Root,
// rejecting symlinks and non-regular files.
func SnapshotFromDir(dir, skillName string) (*SkillSnapshot, error) {
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("skill directory %q is not a regular directory", dir)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open skill directory %q: %w", dir, err)
	}
	defer func() { _ = root.Close() }()
	return readSnapshotInsideRoot(root, skillName)
}

func readSnapshotFromRoot(parent *os.Root, relSkillDir, skillName string) (*SkillSnapshot, error) {
	skillRoot, err := parent.OpenRoot(relSkillDir)
	if err != nil {
		return nil, fmt.Errorf("open skill directory %q: %w", relSkillDir, err)
	}
	defer func() { _ = skillRoot.Close() }()
	return readSnapshotInsideRoot(skillRoot, skillName)
}

func readSnapshotInsideRoot(skillRoot *os.Root, skillName string) (*SkillSnapshot, error) {
	files := map[string]SnapshotFile{}
	var walk func(rel string) error
	walk = func(rel string) error {
		f, err := skillRoot.Open(rel)
		if err != nil {
			return fmt.Errorf("open %q in skill %q: %w", rel, skillName, err)
		}
		entries, err := f.ReadDir(-1)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("read directory %q in skill %q: %w", rel, skillName, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
				return fmt.Errorf("skill %q contains invalid path component %q", skillName, name)
			}
			child := name
			if rel != "." {
				child = rel + "/" + name
			}
			info, err := skillRoot.Lstat(child)
			if err != nil {
				return fmt.Errorf("stat %q in skill %q: %w", child, skillName, err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("skill %q contains symlink %q (symlinks are rejected)", skillName, child)
			}
			if info.IsDir() {
				if err := walk(child); err != nil {
					return err
				}
				continue
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("skill %q contains non-regular file %q", skillName, child)
			}
			data, err := skillRoot.ReadFile(child)
			if err != nil {
				return fmt.Errorf("read %q in skill %q: %w", child, skillName, err)
			}
			sum := sha256.Sum256(data)
			files[child] = SnapshotFile{
				Path:   child,
				Data:   data,
				SHA256: hex.EncodeToString(sum[:]),
				Mode:   NormalizedMode(info.Mode()),
			}
		}
		return nil
	}
	if err := walk("."); err != nil {
		return nil, err
	}
	return &SkillSnapshot{Name: skillName, Files: files}, nil
}

// CheckLocalEdits inspects an installed skill on disk against its locked file
// records. It returns (true, nil) if files were added, removed, modified, had
// their executable bit changed, or if a symlink/non-regular file was added. If
// the destination directory does not exist, it returns (false, os.ErrNotExist).
func CheckLocalEdits(installed InstalledSkill) (bool, error) {
	if _, err := os.Lstat(installed.Destination); err != nil {
		return false, err
	}
	snap, err := SnapshotFromDir(installed.Destination, installed.Skill)
	if err != nil {
		// Symlink or non-regular file introduced locally counts as a local edit.
		return true, nil
	}
	return !snap.MatchesRecords(installed.Files), nil
}

// DirMatchesSnapshot checks whether an existing on-disk directory exists as a
// regular directory and matches snap byte-for-byte (including file set and
// normalized executable bits).
func DirMatchesSnapshot(dir string, snap *SkillSnapshot) (exists bool, matches bool, err error) {
	_, statErr := os.Lstat(dir)
	if errors.Is(statErr, os.ErrNotExist) {
		return false, false, nil
	}
	if statErr != nil {
		return false, false, statErr
	}
	diskSnap, readErr := SnapshotFromDir(dir, snap.Name)
	if readErr != nil {
		// Existing directory contains symlinks or is not a regular dir; treat as non-matching local copy.
		return true, false, nil
	}
	return true, diskSnap.Equals(snap), nil
}

// snapshotFS is a minimal in-memory fs.FS implementation for SkillSnapshot.
type snapshotFS struct {
	files map[string]SnapshotFile
}

var (
	_ fs.FS         = (*snapshotFS)(nil)
	_ fs.ReadFileFS = (*snapshotFS)(nil)
	_ fs.ReadDirFS  = (*snapshotFS)(nil)
	_ fs.StatFS     = (*snapshotFS)(nil)
)

func (s *snapshotFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if f, ok := s.files[name]; ok {
		return &snapshotMemFile{file: f, reader: bytes.NewReader(f.Data)}, nil
	}
	entries, ok := s.dirEntries(name)
	if ok {
		return &snapshotMemDir{name: name, entries: entries}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (s *snapshotFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}
	f, ok := s.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	out := make([]byte, len(f.Data))
	copy(out, f.Data)
	return out, nil
}

func (s *snapshotFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	entries, ok := s.dirEntries(name)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	return entries, nil
}

func (s *snapshotFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if f, ok := s.files[name]; ok {
		return snapshotFileInfo{name: path.Base(name), size: int64(len(f.Data)), mode: f.Mode}, nil
	}
	if _, ok := s.dirEntries(name); ok {
		base := "."
		if name != "." {
			base = path.Base(name)
		}
		return snapshotFileInfo{name: base, mode: fs.ModeDir | 0o755, isDir: true}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (s *snapshotFS) dirEntries(dir string) ([]fs.DirEntry, bool) {
	found := dir == "."
	prefix := ""
	if dir != "." {
		prefix = dir + "/"
	}
	children := map[string]fs.DirEntry{}
	for p, f := range s.files {
		if prefix != "" {
			if !strings.HasPrefix(p, prefix) {
				continue
			}
			found = true
		}
		rest := strings.TrimPrefix(p, prefix)
		if idx := strings.IndexByte(rest, '/'); idx >= 0 {
			sub := rest[:idx]
			if _, exists := children[sub]; !exists {
				children[sub] = snapshotDirEntry{info: snapshotFileInfo{name: sub, mode: fs.ModeDir | 0o755, isDir: true}}
			}
		} else {
			children[rest] = snapshotDirEntry{info: snapshotFileInfo{name: rest, size: int64(len(f.Data)), mode: f.Mode}}
		}
	}
	if !found {
		return nil, false
	}
	names := make([]string, 0, len(children))
	for n := range children {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]fs.DirEntry, 0, len(names))
	for _, n := range names {
		out = append(out, children[n])
	}
	return out, true
}

type snapshotFileInfo struct {
	name  string
	size  int64
	mode  fs.FileMode
	isDir bool
}

func (fi snapshotFileInfo) Name() string       { return fi.name }
func (fi snapshotFileInfo) Size() int64        { return fi.size }
func (fi snapshotFileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi snapshotFileInfo) ModTime() time.Time { return time.Time{} }
func (fi snapshotFileInfo) IsDir() bool        { return fi.isDir }
func (fi snapshotFileInfo) Sys() any           { return nil }

type snapshotDirEntry struct {
	info snapshotFileInfo
}

func (de snapshotDirEntry) Name() string               { return de.info.name }
func (de snapshotDirEntry) IsDir() bool                { return de.info.isDir }
func (de snapshotDirEntry) Type() fs.FileMode          { return de.info.mode.Type() }
func (de snapshotDirEntry) Info() (fs.FileInfo, error) { return de.info, nil }

type snapshotMemFile struct {
	file   SnapshotFile
	reader *bytes.Reader
}

func (f *snapshotMemFile) Stat() (fs.FileInfo, error) {
	return snapshotFileInfo{name: path.Base(f.file.Path), size: int64(len(f.file.Data)), mode: f.file.Mode}, nil
}
func (f *snapshotMemFile) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f *snapshotMemFile) Close() error               { return nil }

type snapshotMemDir struct {
	name    string
	entries []fs.DirEntry
	offset  int
}

func (d *snapshotMemDir) Stat() (fs.FileInfo, error) {
	base := "."
	if d.name != "." {
		base = path.Base(d.name)
	}
	return snapshotFileInfo{name: base, mode: fs.ModeDir | 0o755, isDir: true}, nil
}

func (d *snapshotMemDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}

func (d *snapshotMemDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		out := d.entries[d.offset:]
		d.offset = len(d.entries)
		return out, nil
	}
	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}
	end := d.offset + n
	if end > len(d.entries) {
		end = len(d.entries)
	}
	out := d.entries[d.offset:end]
	d.offset = end
	return out, nil
}

func (d *snapshotMemDir) Close() error { return nil }

// DestinationKey normalizes a destination path for lock file lookup.
func DestinationKey(dest string) string {
	return filepath.Clean(dest)
}
