// Package shellrc applies declarative shell actions to the selected shell only.
package shellrc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
)

type shellKey struct{}

// WithShell carries setup's explicit shell choice through installer calls.
func WithShell(ctx context.Context, shell string) context.Context {
	return context.WithValue(ctx, shellKey{}, shell)
}

// SelectedShell returns the explicit choice, falling back to the current shell.
func SelectedShell(ctx context.Context) string {
	if shell, ok := ctx.Value(shellKey{}).(string); ok {
		return shell
	}
	return filepath.Base(os.Getenv("SHELL"))
}

// Apply replaces the single DevEx block in path.
func Apply(path string, entries []string) (bool, error) { return managed.Apply(path, entries) }

// Remove removes the single DevEx block in path.
func Remove(path string) (bool, error) { return managed.Remove(path) }

// ApplyActions merges an app's actions with other managed shell entries.
func ApplyActions(ctx context.Context, init map[string]string, dirs []string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	shell := SelectedShell(ctx)
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	var path string
	switch shell {
	case "bash":
		path = filepath.Join(home, ".bashrc")
	case "zsh":
		path = filepath.Join(home, ".zshrc")
	case "fish":
		path = filepath.Join(home, ".config", "fish", "config.fish")
	default:
		return false, fmt.Errorf("cannot apply shell actions: unsupported or unset shell %q", shell)
	}
	var entries []string
	if line := init[shell]; line != "" {
		entries = append(entries, line)
	}
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	for _, dir := range dirs {
		expanded := strings.ReplaceAll(strings.ReplaceAll(dir, "${HOME}", home), "$HOME", home)
		if strings.HasPrefix(expanded, "~/") {
			expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
		}
		if managed.HasPath(string(content), dir) {
			continue
		}
		entry, err := managed.PathEntry(shell, dir)
		if err != nil {
			return false, err
		}
		// Retain an existing guard after this shell has put its directory on PATH.
		if !strings.Contains(string(content), entry) {
			present := false
			for _, current := range filepath.SplitList(os.Getenv("PATH")) {
				if current == expanded {
					present = true
					break
				}
			}
			if present {
				continue
			}
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return false, nil
	}
	return managed.Merge(path, entries)
}
