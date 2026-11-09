package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"gopkg.in/yaml.v3"
)

// ImportChange describes the exact semantic change and resolved conflicts.
type ImportChange struct {
	Path      string      `json:"path"`
	Before    interface{} `json:"before"`
	After     interface{} `json:"after"`
	Conflicts []string    `json:"conflicts"`
	original  []byte
	content   []byte
}
type importPlan struct {
	Mode    string         `json:"mode"`
	DryRun  bool           `json:"dry_run"`
	Applied bool           `json:"applied"`
	Changes []ImportChange `json:"changes"`
	Backups []string       `json:"backups"`
}

func importTierDirs(settings config.CrossPlatformSettings) map[string]string {
	d, t, u, e := settings.GetConfigDirsWithEnvironment()
	return map[string]string{"default": d, "team": t, "user": u, "default_env": e["default"], "team_env": e["team"], "user_env": e["user"]}
}
func importKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mergeImportValues preserves ordering, recursively merges maps and named list
// entries, and appends distinct unnamed entries in source order.
func mergeImportValues(old, incoming interface{}, path string, conflicts *[]string) interface{} {
	if reflect.DeepEqual(old, incoming) {
		return old
	}
	if a, ok := old.(map[string]interface{}); ok {
		if b, ok := incoming.(map[string]interface{}); ok {
			result := make(map[string]interface{}, len(a)+len(b))
			for k, v := range a {
				result[k] = v
			}
			for _, k := range importKeys(b) {
				if v, exists := a[k]; exists {
					result[k] = mergeImportValues(v, b[k], path+"."+k, conflicts)
				} else {
					result[k] = b[k]
				}
			}
			return result
		}
	}
	if a, ok := old.([]interface{}); ok {
		if b, ok := incoming.([]interface{}); ok {
			result := append([]interface{}{}, a...)
			for _, v := range b {
				match := -1
				for i, existing := range result {
					if reflect.DeepEqual(existing, v) {
						match = i
						break
					}
					left, lok := existing.(map[string]interface{})
					right, rok := v.(map[string]interface{})
					if lok && rok {
						name, nok := left["name"].(string)
						if nok && name != "" && name == right["name"] {
							match = i
							break
						}
					}
				}
				if match < 0 {
					result = append(result, v)
				} else {
					result[match] = mergeImportValues(result[match], v, fmt.Sprintf("%s[%d]", path, match), conflicts)
				}
			}
			return result
		}
	}
	*conflicts = append(*conflicts, path)
	return incoming
}
func planConfigurationImport(ctx context.Context, settings config.CrossPlatformSettings, data map[string]interface{}, merge bool) (*importPlan, error) {
	if err := validateImportData(data); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp("", "devex-import-plan-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root)
	p := &importPlan{Mode: "merge", Changes: []ImportChange{}, Backups: []string{}}
	if !merge {
		p.Mode = "replace"
	}
	final := map[string]interface{}{}
	tiers, ok := data["configurations"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("configurations must be a map")
	}
	dirs := importTierDirs(settings)
	for _, tier := range importKeys(tiers) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, ok := tiers[tier].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s configurations must be a map", tier)
		}
		dst := filepath.Join(root, tier)
		for _, key := range importKeys(entries) {
			if err := writeImportedConfiguration(dst, key, entries[key]); err != nil {
				return nil, err
			}
		}
		files, err := config.ConfigurationFiles(dst)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		sort.Strings(files)
		result := map[string]interface{}{}
		for _, file := range files {
			path := filepath.Join(dirs[tier], file)
			if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("import target is a symlink: %s", path)
			}
			content, err := os.ReadFile(filepath.Join(dst, file))
			if err != nil {
				return nil, err
			}
			var incoming interface{}
			if err := yaml.Unmarshal(content, &incoming); err != nil {
				return nil, err
			}
			original, err := os.ReadFile(path)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			exists := err == nil
			var before interface{}
			if exists {
				if err := yaml.Unmarshal(original, &before); err != nil {
					return nil, fmt.Errorf("read existing %s: %w", path, err)
				}
			}
			conflicts := []string{}
			after := incoming
			if exists && merge {
				after = mergeImportValues(before, incoming, "$", &conflicts)
			}
			if exists && !merge && !reflect.DeepEqual(before, after) {
				conflicts = append(conflicts, "$")
			}
			result[filepath.ToSlash(file[:len(file)-len(filepath.Ext(file))])] = after
			if !exists || !reflect.DeepEqual(before, after) || len(conflicts) > 0 {
				encoded, err := yaml.Marshal(after)
				if err != nil {
					return nil, err
				}
				p.Changes = append(p.Changes, ImportChange{Path: path, Before: before, After: after, Conflicts: conflicts, original: original, content: encoded})
			}
		}
		final[tier] = result
	}
	candidate := map[string]interface{}{"configurations": final}
	if err := validateImportCandidate(ctx, settings, candidate); err != nil {
		return nil, fmt.Errorf("import validation failed: %w", err)
	}
	return p, nil
}
func applyImportPlan(ctx context.Context, p *importPlan, force bool) error {
	for _, change := range p.Changes {
		if len(change.Conflicts) > 0 && !force {
			return fmt.Errorf("import conflicts in %s at %v; use --force to accept imported values", change.Path, change.Conflicts)
		}
	}
	// Finish validation and backups before changing any destination.
	staged := make([]string, len(p.Changes))
	defer func() {
		for _, path := range staged {
			if path != "" {
				_ = os.Remove(path)
			}
		}
	}()
	for i, change := range p.Changes {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := os.ReadFile(change.Path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if !bytes.Equal(current, change.original) {
			return fmt.Errorf("configuration changed since preview: %s", change.Path)
		}
		if err := os.MkdirAll(filepath.Dir(change.Path), 0750); err != nil {
			return err
		}
		if change.original != nil {
			f, err := os.CreateTemp(filepath.Dir(change.Path), filepath.Base(change.Path)+".backup-*")
			if err != nil {
				return err
			}
			_, writeErr := f.Write(change.original)
			closeErr := f.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
			p.Backups = append(p.Backups, f.Name())
		}
		f, err := os.CreateTemp(filepath.Dir(change.Path), ".devex-import-*")
		if err != nil {
			return err
		}
		staged[i] = f.Name()
		_, writeErr := f.Write(change.content)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	for i, change := range p.Changes {
		if err := os.Rename(staged[i], change.Path); err != nil {
			return err
		}
	}
	p.Applied = true
	return nil
}
func runConfigurationImport(ctx context.Context, out io.Writer, settings config.CrossPlatformSettings, input string, merge, force, _, _, dryRun bool, format string) error {
	if format != "table" && format != "json" {
		return fmt.Errorf("unsupported output format %q: use table or json", format)
	}
	data, err := readConfigurationImport(ctx, input)
	if err != nil {
		return err
	}
	p, err := planConfigurationImport(ctx, settings, data, merge)
	if err != nil {
		return err
	}
	p.DryRun = dryRun
	if format == "table" {
		var preview bytes.Buffer
		fmt.Fprintf(&preview, "Import mode: %s (dry run: %t)\n", p.Mode, dryRun)
		for _, change := range p.Changes {
			before, _ := yaml.Marshal(change.Before)
			after, _ := yaml.Marshal(change.After)
			fmt.Fprintf(&preview, "%s\nBefore:\n%sAfter:\n%sConflicts: %v\n", change.Path, before, after, change.Conflicts)
		}
		if _, err := out.Write(preview.Bytes()); err != nil {
			return err
		}
	}
	if !dryRun {
		err = applyImportPlan(ctx, p, force)
	}
	if format == "json" {
		if outputErr := json.NewEncoder(out).Encode(p); outputErr != nil {
			return outputErr
		}
	} else {
		for _, path := range p.Backups {
			fmt.Fprintf(out, "Backup: %s\n", path)
		}
	}
	return err
}
