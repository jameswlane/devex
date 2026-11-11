// Package setupundo records the system changes devex setup makes outside
// DevEx's own configuration (shell rc blocks, Git identity, desktop settings)
// so `devex undo` can put each one back exactly as it was before setup ran.
package setupundo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
)

// MetadataKey is the undo operation metadata key holding encoded changes.
const MetadataKey = "system_changes"

// Operation is the undo history operation name for setup runs.
const Operation = "setup"

const (
	KindShellRC = "shell-rc"
	KindCommand = "command"
)

// kreadUnset marks a KDE key that had no value before setup wrote it.
const kreadUnset = "__devex_unset__"

// restoreCommands are the only executables a recorded restore may run. The
// history file is user-writable, so restores never run arbitrary commands.
var restoreCommands = []string{"git", "gsettings", "kwriteconfig5"}

// Change is one undoable system change.
type Change struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	// Shell rc block state before setup ran.
	Path    string   `json:"path,omitempty"`
	Existed bool     `json:"existed,omitempty"`
	Entries []string `json:"entries,omitempty"`
	// Commands that restore the previous state, run in order.
	Restore [][]string `json:"restore,omitempty"`
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

// SnapshotShellRC reads the managed block of each path. Unreadable files are
// left out, so they are never reported as changed.
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

// Runner runs one command and returns its standard output. Tests replace it.
type Runner func(ctx context.Context, argv []string) (string, error)

// ExecRunner runs argv without a shell.
func ExecRunner(ctx context.Context, argv []string) (string, error) {
	output, err := exec.CommandContext(ctx, argv[0], argv[1:]...).Output() //nolint:gosec // argv is built from fixed tools and validated settings
	return string(output), err
}

// GitConfigChange returns how to restore a global Git setting to its current
// value, or unset it when it has none. Call it before changing the setting.
func GitConfigChange(ctx context.Context, run Runner, key string) Change {
	restore := []string{"git", "config", "--global", "--unset", key}
	if value, err := run(ctx, []string{"git", "config", "--global", "--get", key}); err == nil {
		restore = []string{"git", "config", "--global", key, strings.TrimRight(value, "\n")}
	}
	return Change{Kind: KindCommand, Description: "Global Git " + key, Restore: [][]string{restore}}
}

// DesktopChanges returns how to restore every setting a desktop plan writes,
// read before the plan is applied. Plan commands it cannot restore are
// returned as errors so callers can report them; the others still undo.
func DesktopChanges(ctx context.Context, run Runner, plan [][]string) ([]Change, error) {
	var changes []Change
	var errs []error
	for _, command := range plan {
		switch {
		case len(command) == 5 && command[0] == "gsettings" && command[1] == "set":
			schema, key := command[2], command[3]
			value, err := run(ctx, []string{"gsettings", "get", schema, key})
			if err != nil {
				errs = append(errs, fmt.Errorf("read %s %s: %w", schema, key, err))
				continue
			}
			changes = append(changes, Change{Kind: KindCommand, Description: "GNOME " + schema + " " + key,
				Restore: [][]string{{"gsettings", "set", schema, key, strings.TrimSpace(value)}}})
		case len(command) == 8 && command[0] == "kwriteconfig5" && command[1] == "--file" && command[3] == "--group" && command[5] == "--key":
			file, group, key := command[2], command[4], command[6]
			value, err := run(ctx, []string{"kreadconfig5", "--file", file, "--group", group, "--key", key, "--default", kreadUnset})
			if err != nil {
				errs = append(errs, fmt.Errorf("read %s [%s] %s: %w", file, group, key, err))
				continue
			}
			value = strings.TrimRight(value, "\n")
			restore := []string{"kwriteconfig5", "--file", file, "--group", group, "--key", key, value}
			if value == kreadUnset {
				restore = []string{"kwriteconfig5", "--file", file, "--group", group, "--key", key, "--delete"}
			}
			changes = append(changes, Change{Kind: KindCommand, Description: "KDE " + file + " [" + group + "] " + key, Restore: [][]string{restore}})
		default:
			data, _ := json.Marshal(command)
			errs = append(errs, fmt.Errorf("no undo for desktop command %s", data))
		}
	}
	return changes, errors.Join(errs...)
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

// Restore undoes changes in reverse order. It keeps going after a failure and
// returns every failure, so one broken step never blocks the others.
func Restore(ctx context.Context, run Runner, changes []Change) error {
	var errs []error
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]
		var err error
		switch change.Kind {
		case KindShellRC:
			err = restoreShellRC(change)
		case KindCommand:
			for _, argv := range change.Restore {
				if len(argv) == 0 || !slices.Contains(restoreCommands, argv[0]) {
					err = errors.Join(err, fmt.Errorf("refusing to run restore command %q", argv))
					continue
				}
				if _, runErr := run(ctx, argv); runErr != nil {
					err = errors.Join(err, fmt.Errorf("%s: %w", strings.Join(argv, " "), runErr))
				}
			}
		default:
			err = fmt.Errorf("unknown change kind %q", change.Kind)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("restore %s: %w", change.Description, err))
		}
	}
	return errors.Join(errs...)
}

// restoreShellRC puts the managed block back as it was. Content outside the
// block, including user edits made since setup, is left alone. A file setup
// created is deleted when it still holds nothing but the block.
func restoreShellRC(change Change) error {
	if change.Path == "" {
		return fmt.Errorf("missing rc path")
	}
	if !change.Existed {
		content, err := os.ReadFile(change.Path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if onlyManagedBlock(string(content)) {
			return os.Remove(change.Path)
		}
	}
	_, err := managed.Apply(change.Path, change.Entries)
	return err
}

// onlyManagedBlock reports whether content has nothing outside the DevEx block.
func onlyManagedBlock(content string) bool {
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		switch trimmed := strings.TrimSpace(line); {
		case trimmed == managed.Start:
			inBlock = true
		case trimmed == managed.End:
			inBlock = false
		case !inBlock && trimmed != "":
			return false
		}
	}
	return true
}
