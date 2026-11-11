// Package dotfiles deploys files from a user's dotfiles Git repository.
//
// A sync fetches the repository at a ref into a temporary directory, reads the
// devex-dotfiles.yaml manifest at its root, and deploys the listed files with
// internal/configfiles: destinations stay inside HOME, differing files are
// backed up first, and every copy is recorded so it can be reverted. No clone
// is kept between syncs.
package dotfiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

const (
	// ManifestFile is the deployment manifest at the repository root.
	ManifestFile = "devex-dotfiles.yaml"
	// RecordKey is the configfiles record that owns deployed dotfiles.
	RecordKey = "dotfiles"
	// MaxFiles bounds the manifest.
	MaxFiles = 500
)

// File maps a repository file to a destination.
type File struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
}

// Manifest is devex-dotfiles.yaml.
type Manifest struct {
	Files []File `yaml:"files"`
}

// State records the last successful sync.
type State struct {
	Repository string    `json:"repository"`
	Ref        string    `json:"ref,omitempty"`
	Commit     string    `json:"commit"`
	SyncedAt   time.Time `json:"synced_at"`
}

var (
	scpLike = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[^/].*$`)
	safeRef = regexp.MustCompile(`^[A-Za-z0-9._/@+-]+$`)
)

// ValidateRepository accepts https://, ssh:// and git@host:path URLs and
// absolute local paths. Credentials in the URL are refused: they would be
// stored in configuration in plain text.
func ValidateRepository(repository string) error {
	repository = strings.TrimSpace(repository)
	switch {
	case repository == "":
		return errors.New("no dotfiles repository: pass --repo or set dotfiles.repository")
	case strings.HasPrefix(repository, "-"):
		return fmt.Errorf("repository %q must not start with '-'", repository)
	case scpLike.MatchString(repository):
		return nil
	case filepath.IsAbs(repository):
		return nil
	}
	parsed, err := url.Parse(repository)
	if err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}
	switch parsed.Scheme {
	case "https", "ssh", "file":
	default:
		return fmt.Errorf("unsupported repository %q: use https://, ssh://, git@host:path or an absolute path", Redact(repository))
	}
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword || parsed.Scheme == "https" {
			return errors.New("repository URL must not contain credentials; use an SSH key or a git credential helper for private repositories")
		}
	}
	return nil
}

// ValidateRef accepts branch, tag and commit names.
func ValidateRef(ref string) error {
	if ref == "" {
		return nil
	}
	if strings.HasPrefix(ref, "-") || !safeRef.MatchString(ref) || strings.Contains(ref, "..") {
		return fmt.Errorf("invalid ref %q", ref)
	}
	return nil
}

// Redact removes any user information from a repository URL for messages.
func Redact(repository string) string {
	parsed, err := url.Parse(repository)
	if err != nil || parsed.User == nil || parsed.Scheme == "" {
		return repository
	}
	parsed.User = nil
	return parsed.String()
}

// Git runs git with args in dir. Tests replace it.
var Git = func(ctx context.Context, dir string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	// Never wait for a password prompt; private repositories use SSH keys or
	// credential helpers.
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// Checkout is a fetched repository in a temporary directory.
type Checkout struct {
	Dir    string
	Commit string
}

// Remove deletes the temporary checkout.
func (c Checkout) Remove() error { return os.RemoveAll(c.Dir) }

// Fetch fetches repository at ref (the default branch when empty) into a new
// temporary directory, with history depth 1 and without submodules.
func Fetch(ctx context.Context, repository, ref string) (Checkout, error) {
	if err := ValidateRepository(repository); err != nil {
		return Checkout{}, err
	}
	if err := ValidateRef(ref); err != nil {
		return Checkout{}, err
	}
	dir, err := os.MkdirTemp("", "devex-dotfiles-*")
	if err != nil {
		return Checkout{}, err
	}
	checkout := Checkout{Dir: dir}
	target := ref
	if target == "" {
		target = "HEAD"
	}
	steps := [][]string{
		{"init", "--quiet"},
		{"remote", "add", "origin", "--", repository},
		{"fetch", "--quiet", "--depth", "1", "--no-recurse-submodules", "origin", "--", target},
		{"-c", "advice.detachedHead=false", "checkout", "--quiet", "--detach", "FETCH_HEAD"},
	}
	for _, args := range steps {
		if _, err := Git(ctx, dir, args...); err != nil {
			_ = checkout.Remove()
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Checkout{}, ctxErr
			}
			return Checkout{}, fmt.Errorf("fetch %s at %s: %w", Redact(repository), target, redactError(err, repository))
		}
	}
	commit, err := Git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		_ = checkout.Remove()
		return Checkout{}, err
	}
	checkout.Commit = commit
	return checkout, nil
}

func redactError(err error, repository string) error {
	redacted := Redact(repository)
	if redacted == repository {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), repository, redacted))
}

// LoadManifest reads the manifest from a checkout and turns it into config
// file mappings. Sources must be regular files inside the checkout (no
// symlinks, no ".."). Destinations must expand inside HOME and outside
// DevEx's own directories; configfiles enforces HOME again when writing.
func LoadManifest(checkout Checkout, home string) ([]types.ConfigFile, error) {
	root, err := os.OpenRoot(checkout.Dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := root.ReadFile(ManifestFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("the repository has no %s at its root; add one listing the files to deploy", ManifestFile)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ManifestFile, err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ManifestFile, err)
	}
	if len(manifest.Files) == 0 {
		return nil, fmt.Errorf("%s lists no files", ManifestFile)
	}
	if len(manifest.Files) > MaxFiles {
		return nil, fmt.Errorf("%s lists %d files; at most %d are allowed", ManifestFile, len(manifest.Files), MaxFiles)
	}
	dirs := devexpaths.ForHome(home)
	protected := []string{dirs.Config, dirs.Data, dirs.State, dirs.Cache}
	destinations := map[string]bool{}
	mappings := make([]types.ConfigFile, 0, len(manifest.Files))
	for i, file := range manifest.Files {
		where := fmt.Sprintf("%s files[%d]", ManifestFile, i)
		source := filepath.ToSlash(filepath.Clean(strings.TrimSpace(file.Source)))
		if source == "." || !fs.ValidPath(source) || source == ManifestFile {
			return nil, fmt.Errorf("%s: source %q must be a file path inside the repository", where, file.Source)
		}
		if strings.HasPrefix(source, ".git/") || source == ".git" {
			return nil, fmt.Errorf("%s: source %q is inside .git", where, file.Source)
		}
		info, err := root.Lstat(source)
		if err != nil {
			return nil, fmt.Errorf("%s: source %q: %w", where, file.Source, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: source %q must be a regular file (symlinks are not deployed)", where, file.Source)
		}
		destination, err := configfiles.Expand(file.Destination)
		if err != nil {
			return nil, fmt.Errorf("%s: destination %q: %w", where, file.Destination, err)
		}
		if !inside(home, destination) {
			return nil, fmt.Errorf("%s: destination %q must be inside your home directory", where, file.Destination)
		}
		for _, dir := range protected {
			if inside(dir, destination) || destination == dir {
				return nil, fmt.Errorf("%s: destination %q is inside DevEx's own directory %s", where, file.Destination, dir)
			}
		}
		if destinations[destination] {
			return nil, fmt.Errorf("%s: destination %q is listed twice", where, file.Destination)
		}
		destinations[destination] = true
		mappings = append(mappings, types.ConfigFile{Source: filepath.Join(checkout.Dir, filepath.FromSlash(source)), Destination: destination})
	}
	return mappings, nil
}

func inside(parent, path string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != "." && filepath.IsLocal(rel)
}

func statePath(home string) string {
	return filepath.Join(devexpaths.ForHome(home).State, "dotfiles.json")
}

// ReadState returns the last sync, or false when none is recorded.
func ReadState(home string) (State, bool, error) {
	data, err := os.ReadFile(statePath(home))
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, false, fmt.Errorf("read dotfiles state: %w", err)
	}
	return state, true, nil
}

// WriteState records a successful sync.
func WriteState(home string, state State) error {
	path := statePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ClearState removes the sync record.
func ClearState(home string) error {
	err := os.Remove(statePath(home))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
