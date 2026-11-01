// Package shellrc owns the single DevEx-managed block in shell startup files.
package shellrc

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	Start   = "# >>> devex >>>"
	End     = "# <<< devex <<<"
	Comment = "# Managed by DevEx. Changes inside this block are overwritten."
)

var editMu sync.Mutex
var assignment = regexp.MustCompile(`^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=`)
var alias = regexp.MustCompile(`^alias\s+([^=\s]+)\s*(?:=|\s)`)

// Apply replaces the managed block, omitting entries already defined by the user.
func Apply(path string, entries []string) (bool, error) { return edit(path, entries, nil, false) }

// Merge adds entries without losing entries owned by other DevEx components.
func Merge(path string, entries []string) (bool, error) { return edit(path, entries, nil, true) }

// MergeLegacy migrates the old tool-shell marker and its contiguous known lines.
// Unknown lines after the marker are always retained as user content.
func MergeLegacy(path string, entries, known []string) (bool, error) {
	return edit(path, entries, known, true)
}

// Remove deletes the managed block and leaves all user content intact.
func Remove(path string) (bool, error) { return Apply(path, nil) }

func normalize(line string) string {
	line = strings.ReplaceAll(line, "${HOME}", "$HOME")
	line = strings.ReplaceAll(line, "~/", "$HOME/")
	return strings.Join(strings.Fields(line), " ")
}

func split(content string, known []string) (string, []string, error) {
	var outside strings.Builder
	var managed []string
	inBlock, legacy := false, false
	for _, line := range strings.SplitAfter(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == Start {
			if inBlock {
				return "", nil, fmt.Errorf("nested DevEx block")
			}
			inBlock = true
			legacy = false
			continue
		}
		if trimmed == End {
			if !inBlock {
				return "", nil, fmt.Errorf("unmatched DevEx end marker")
			}
			inBlock = false
			continue
		}
		if inBlock {
			if trimmed != "" && trimmed != Comment {
				managed = append(managed, strings.TrimSuffix(line, "\n"))
			}
			continue
		}
		if known != nil && trimmed == "# DevEx Shell Configuration" {
			legacy = true
			continue
		}
		if legacy {
			if slices.Contains(known, trimmed) {
				continue
			}
			legacy = false
		}
		outside.WriteString(line)
	}
	if inBlock {
		return "", nil, fmt.Errorf("unterminated DevEx block")
	}
	return outside.String(), managed, nil
}

// Conflicts reports an equivalent line or a user definition of the same setting.
func Conflicts(outside, entry string) bool {
	entry = normalize(entry)
	if after, ok := strings.CutPrefix(entry, `case ":$PATH:" in *":`); ok {
		if dir, _, found := strings.Cut(after, `:"*)`); found && HasPath(outside, dir) {
			return true
		}
	}
	if after, ok := strings.CutPrefix(entry, `if not contains -- "`); ok {
		if dir, _, found := strings.Cut(after, `" $PATH;`); found && HasPath(outside, dir) {
			return true
		}
	}
	setting, aliasName := assignment.FindStringSubmatch(entry), alias.FindStringSubmatch(entry)
	for _, line := range strings.Split(outside, "\n") {
		line = normalize(line)
		if line == entry {
			return true
		}
		if len(setting) > 1 {
			other := assignment.FindStringSubmatch(line)
			if len(other) > 1 && other[1] == setting[1] {
				return true
			}
		}
		if len(aliasName) > 1 {
			other := alias.FindStringSubmatch(line)
			if len(other) > 1 && other[1] == aliasName[1] {
				return true
			}
		}
	}
	return false
}

func edit(path string, entries, known []string, merge bool) (bool, error) {
	editMu.Lock()
	defer editMu.Unlock()
	// Follow existing symlinks so dotfile-manager links survive atomic replacement.
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}
	original, err := os.ReadFile(path)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	mode := os.FileMode(0600)
	if exists {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return false, statErr
		}
		mode = info.Mode().Perm()
	}
	outside, managed, err := split(string(original), known)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if merge {
		entries = append(managed, entries...)
	}
	unique := make([]string, 0, len(entries))
	seen := make(map[string]bool)
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		key := normalize(entry)
		if key == "" || seen[key] || Conflicts(outside, entry) {
			continue
		}
		if strings.Contains(entry, "\n") || entry == Start || entry == End {
			return false, fmt.Errorf("shell rc entries must be single lines without block markers")
		}
		seen[key] = true
		unique = append(unique, entry)
	}
	updated := outside
	if len(unique) > 0 {
		if updated != "" && !strings.HasSuffix(updated, "\n") {
			updated += "\n"
		}
		updated += Start + "\n" + Comment + "\n" + strings.Join(unique, "\n") + "\n" + End + "\n"
	}
	if updated == string(original) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return false, err
	}
	if exists {
		backups, globErr := filepath.Glob(path + ".devex-bak-*")
		if globErr != nil {
			return false, globErr
		}
		if len(backups) == 0 {
			if err := backupOriginal(path, original, mode); err != nil {
				return false, err
			}
		}
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".devex-rc-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(temp.Name())
	if err = temp.Chmod(mode); err == nil {
		_, err = temp.WriteString(updated)
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil {
		return false, err
	}
	if closeErr != nil {
		return false, closeErr
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return false, err
	}
	return true, nil
}

func backupOriginal(path string, original []byte, mode os.FileMode) error {
	prefix := filepath.Base(path) + ".devex-bak-" + time.Now().UTC().Format("20060102T150405.000000000Z") + "-"
	backup, err := os.CreateTemp(filepath.Dir(path), prefix)
	if err != nil {
		return fmt.Errorf("create rc backup: %w", err)
	}
	if err = backup.Chmod(mode); err == nil {
		_, err = backup.Write(original)
	}
	if err == nil {
		err = backup.Sync()
	}
	closeErr := backup.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(backup.Name())
		if err != nil {
			return fmt.Errorf("write rc backup: %w", err)
		}
		return closeErr
	}
	return nil
}

// PathEntry returns a shell-native guard that also works in nested shells.
func PathEntry(shell, dir string) (string, error) {
	if strings.TrimSpace(dir) == "" || strings.ContainsAny(dir, "\r\n") {
		return "", fmt.Errorf("invalid PATH directory")
	}
	if len(dir) > 1 {
		dir = strings.TrimSuffix(dir, "/")
	}
	if strings.HasPrefix(dir, "~/") {
		dir = "$HOME/" + strings.TrimPrefix(dir, "~/")
	}
	dir = strings.ReplaceAll(dir, "${HOME}", "$HOME")
	home := strings.HasPrefix(dir, "$HOME/")
	if home {
		dir = strings.TrimPrefix(dir, "$HOME")
	}
	dir = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(dir)
	if home {
		dir = "$HOME" + dir
	}
	if shell == "fish" {
		return fmt.Sprintf(`if not contains -- "%s" $PATH; set -gx PATH "%s" $PATH; end`, dir, dir), nil
	}
	return fmt.Sprintf(`case ":$PATH:" in *":%s:"*) ;; *) export PATH="%s:$PATH" ;; esac`, dir, dir), nil
}

// HasPath checks literal PATH additions outside the managed block.
func HasPath(content, dir string) bool {
	outside, _, err := split(content, nil)
	if err != nil {
		return false
	}
	dir = strings.TrimSuffix(normalize(dir), "/")
	for _, line := range strings.Split(outside, "\n") {
		line = normalize(line)
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "PATH") {
			continue
		}
		pattern := regexp.MustCompile(`(?:^|[:"'\s=])` + regexp.QuoteMeta(dir) + `/?(?:$|[:"'\s;])`)
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}
