// Package setupundo records the system changes devex setup makes outside
// DevEx's own configuration (shell rc blocks, Git identity, desktop settings)
// so `devex undo` can put each one back exactly as it was before setup ran.
//
// Changes are stored as typed records, never as command lines. Undo builds
// each restore command itself from validated fields, so a damaged or edited
// history file cannot make undo run anything other than resetting a known
// Git key, desktop setting or rc block. Rc block entries are shell code and
// cannot be authenticated; anyone able to edit the history file can already
// edit those rc files directly.
package setupundo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
)

// MetadataKey is the undo operation metadata key holding encoded changes.
const MetadataKey = "system_changes"

// Operation is the undo history operation name for setup runs.
const Operation = "setup"

// Change kinds.
const (
	KindShellRC   = "shell-rc"
	KindGitConfig = "git-config"
	KindGSettings = "gsettings"
	KindKConfig   = "kconfig"
)

// kreadUnset marks a KDE key that had no value before setup wrote it.
const kreadUnset = "__devex_unset__"

// Git exit statuses: --get finds no value, --unset finds no key.
const (
	gitGetMissing   = 1
	gitUnsetMissing = 5
)

var (
	gitKeys        = []string{"user.name", "user.email"}
	gsettingsName  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*(\.[A-Za-z0-9-]+)+$`)
	gsettingsKey   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	kconfigFile    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	kconfigSegment = regexp.MustCompile(`^[^-\x00\n][^\x00\n]*$`)
)

// Change is one undoable system change and the state to restore.
type Change struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	// Shell rc: the managed block before setup ran.
	Path    string   `json:"path,omitempty"`
	Existed bool     `json:"existed,omitempty"`
	Entries []string `json:"entries,omitempty"`
	// Settings: the previous value, or Unset when the setting had none.
	Key    string `json:"key,omitempty"`
	Schema string `json:"schema,omitempty"`
	File   string `json:"file,omitempty"`
	Group  string `json:"group,omitempty"`
	Value  string `json:"value,omitempty"`
	Unset  bool   `json:"unset,omitempty"`
}

// ShellRCPaths returns the rc files setup and its plugins may manage.
func ShellRCPaths(home string) []string {
	return []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}
}

type rcState struct {
	exists  bool
	entries []string
}

// RCSnapshot is the managed-block state of shell rc files at one moment.
type RCSnapshot map[string]rcState

// SnapshotShellRC reads the managed block of each path. Unreadable or
// malformed files are left out, so they are never reported as changed.
func SnapshotShellRC(paths []string) RCSnapshot {
	snapshot := RCSnapshot{}
	for _, path := range paths {
		exists, entries, err := managed.Read(path)
		if err != nil {
			continue
		}
		snapshot[path] = rcState{exists: exists, entries: entries}
	}
	return snapshot
}

// Changes compares the current rc files with the snapshot and returns one
// change, holding the earlier state, for each managed block that changed.
func (s RCSnapshot) Changes() []Change {
	var changes []Change
	paths := make([]string, 0, len(s))
	for path := range s {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		before := s[path]
		exists, entries, err := managed.Read(path)
		if err != nil || (exists == before.exists && slices.Equal(entries, before.entries)) {
			continue
		}
		description := "DevEx block in " + path
		if !before.exists {
			description = "Created " + path
		}
		changes = append(changes, Change{Kind: KindShellRC, Description: description, Path: path, Existed: before.exists, Entries: before.entries})
	}
	return changes
}

// Runner runs one command and returns its standard output. Tests replace it;
// errors carrying an exit status must implement ExitCode() int.
type Runner func(ctx context.Context, argv []string) (string, error)

// ExecRunner runs argv without a shell.
func ExecRunner(ctx context.Context, argv []string) (string, error) {
	output, err := exec.CommandContext(ctx, argv[0], argv[1:]...).Output() //nolint:gosec // argv is built by this package from validated fields
	return string(output), err
}

func exitCode(err error) (int, bool) {
	var coder interface{ ExitCode() int }
	if errors.As(err, &coder) {
		return coder.ExitCode(), true
	}
	return 0, false
}

// GitConfigChange reads a global Git identity key before setup changes it. Only
// Git's "no value" status counts as unset; any other failure means the earlier
// state is unknown, and the error says the key cannot be undone.
func GitConfigChange(ctx context.Context, run Runner, key string) (Change, error) {
	if !slices.Contains(gitKeys, key) {
		return Change{}, fmt.Errorf("git key %q is not undoable", key)
	}
	change := Change{Kind: KindGitConfig, Description: "Global Git " + key, Key: key}
	value, err := run(ctx, []string{"git", "config", "--global", "--get", key})
	if err == nil {
		change.Value = strings.TrimRight(value, "\n")
		return change, nil
	}
	if code, ok := exitCode(err); ok && code == gitGetMissing && ctx.Err() == nil {
		change.Unset = true
		return change, nil
	}
	return Change{}, fmt.Errorf("read global Git %s: %w", key, err)
}

// DesktopChanges reads every setting a desktop plan writes before the plan is
// applied. Commands it cannot read or restore are returned as errors so callers
// can report them; the others still undo.
func DesktopChanges(ctx context.Context, run Runner, plan [][]string) ([]Change, error) {
	var changes []Change
	var errs []error
	for _, command := range plan {
		switch {
		case len(command) == 5 && command[0] == "gsettings" && command[1] == "set":
			change := Change{Kind: KindGSettings, Description: "GNOME " + command[2] + " " + command[3], Schema: command[2], Key: command[3]}
			if err := validate(change); err != nil {
				errs = append(errs, err)
				continue
			}
			value, err := run(ctx, []string{"gsettings", "get", change.Schema, change.Key})
			if err != nil {
				errs = append(errs, fmt.Errorf("read %s %s: %w", change.Schema, change.Key, err))
				continue
			}
			change.Value = strings.TrimSpace(value)
			changes = append(changes, change)
		case len(command) == 8 && command[0] == "kwriteconfig5" && command[1] == "--file" && command[3] == "--group" && command[5] == "--key":
			change := Change{Kind: KindKConfig, Description: "KDE " + command[2] + " [" + command[4] + "] " + command[6], File: command[2], Group: command[4], Key: command[6]}
			if err := validate(change); err != nil {
				errs = append(errs, err)
				continue
			}
			value, err := run(ctx, []string{"kreadconfig5", "--file", change.File, "--group", change.Group, "--key", change.Key, "--default", kreadUnset})
			if err != nil {
				errs = append(errs, fmt.Errorf("read %s [%s] %s: %w", change.File, change.Group, change.Key, err))
				continue
			}
			change.Value = strings.TrimRight(value, "\n")
			if change.Value == kreadUnset {
				change.Value, change.Unset = "", true
			}
			changes = append(changes, change)
		default:
			data, _ := json.Marshal(command)
			errs = append(errs, fmt.Errorf("no undo for desktop command %s", data))
		}
	}
	return changes, errors.Join(errs...)
}

// validate rejects fields that could turn a restore into a different command.
func validate(change Change) error {
	switch change.Kind {
	case KindGitConfig:
		if !slices.Contains(gitKeys, change.Key) {
			return fmt.Errorf("git key %q is not undoable", change.Key)
		}
	case KindGSettings:
		if !gsettingsName.MatchString(change.Schema) || !gsettingsKey.MatchString(change.Key) {
			return fmt.Errorf("invalid GNOME setting %q %q", change.Schema, change.Key)
		}
	case KindKConfig:
		if !kconfigFile.MatchString(change.File) || !kconfigSegment.MatchString(change.Group) || !kconfigSegment.MatchString(change.Key) {
			return fmt.Errorf("invalid KDE setting %q [%q] %q", change.File, change.Group, change.Key)
		}
	default:
		return fmt.Errorf("unknown change kind %q", change.Kind)
	}
	if strings.ContainsRune(change.Value, 0) {
		return fmt.Errorf("invalid value for %s", change.Description)
	}
	return nil
}

// restoreCommand builds the fixed command that restores a setting change.
func restoreCommand(change Change) ([]string, error) {
	if err := validate(change); err != nil {
		return nil, err
	}
	switch change.Kind {
	case KindGitConfig:
		if change.Unset {
			return []string{"git", "config", "--global", "--unset", change.Key}, nil
		}
		return []string{"git", "config", "--global", "--", change.Key, change.Value}, nil
	case KindGSettings:
		return []string{"gsettings", "set", change.Schema, change.Key, change.Value}, nil
	default: // KindKConfig
		argv := []string{"kwriteconfig5", "--file", change.File, "--group", change.Group, "--key", change.Key}
		if change.Unset {
			return append(argv, "--delete"), nil
		}
		return append(argv, "--", change.Value), nil
	}
}

// Encode stores changes as a JSON string, which survives the undo history's
// YAML/JSON round trip unchanged.
func Encode(changes []Change) (string, error) {
	data, err := json.Marshal(changes)
	return string(data), err
}

// Decode reads changes stored by Encode.
func Decode(value any) ([]Change, error) {
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("setup undo metadata is %T, not a string", value)
	}
	var changes []Change
	if err := json.Unmarshal([]byte(text), &changes); err != nil {
		return nil, fmt.Errorf("decode setup undo metadata: %w", err)
	}
	return changes, nil
}

// Restore undoes changes newest first. Shell rc changes may only target the
// rc files under home. Every restore is idempotent, so a partly failed undo can
// be retried; Restore keeps going after a failure and returns every failure.
func Restore(ctx context.Context, run Runner, home string, changes []Change) error {
	allowed := ShellRCPaths(home)
	var errs []error
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]
		var err error
		if change.Kind == KindShellRC {
			if !slices.Contains(allowed, filepath.Clean(change.Path)) {
				err = fmt.Errorf("refusing to change %q: not a shell rc file in %s", change.Path, home)
			} else {
				err = restoreShellRC(change)
			}
		} else {
			err = restoreSetting(ctx, run, change)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("restore %s: %w", change.Description, err))
		}
	}
	return errors.Join(errs...)
}

func restoreSetting(ctx context.Context, run Runner, change Change) error {
	argv, err := restoreCommand(change)
	if err != nil {
		return err
	}
	_, err = run(ctx, argv)
	if code, ok := exitCode(err); ok && change.Kind == KindGitConfig && change.Unset && code == gitUnsetMissing {
		return nil // already unset
	}
	if err != nil {
		return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	return nil
}

// restoreShellRC puts the managed block back as it was. Content outside the
// block, including user edits made since setup, is left alone. A file setup
// created is deleted only while it still holds nothing but a well-formed block
// and was not replaced while undo checked it.
func restoreShellRC(change Change) error {
	if !change.Existed {
		before, err := os.Lstat(change.Path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if before.Mode().IsRegular() {
			only, err := managed.OnlyBlock(change.Path)
			if err != nil {
				return fmt.Errorf("not changed, fix the DevEx block markers first: %w", err)
			}
			if only {
				after, err := os.Lstat(change.Path)
				if err != nil {
					return err
				}
				if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
					return fmt.Errorf("%s changed while undo was checking it; run undo again", change.Path)
				}
				return os.Remove(change.Path)
			}
		}
	}
	_, err := managed.Apply(change.Path, change.Entries)
	return err
}
