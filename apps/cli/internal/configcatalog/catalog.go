// Package configcatalog validates the source configuration and asset trees without
// installing software or writing user configuration.
package configcatalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"gopkg.in/yaml.v3"
)

// Validate checks the source tree rooted at apps/cli. It deliberately ignores
// user overrides: a broken default must not be hidden by a developer's config.
func Validate(ctx context.Context, root string) ([]config.Diagnostic, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	result, err := config.ValidateConfigDirs(ctx, config.CrossPlatformSettings{}, []string{filepath.Join(root, "config")}, true)
	if err != nil {
		return nil, err
	}
	diagnostics := result.Diagnostics
	apps, err := (config.FileApplicationLoader{}).LoadApplications([]string{filepath.Join(root, "config")})
	if err != nil {
		return diagnostics, err
	}
	known := map[string]bool{}
	for _, app := range apps {
		known[strings.ToLower(app.Name)] = true
	}
	sourceRoot, err := os.OpenRoot(root)
	if err != nil {
		return diagnostics, err
	}
	defer sourceRoot.Close()
	manager := templates.NewTemplateManager("")
	for _, tree := range []string{"config", "assets/templates"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") {
				return nil
			}
			add := func(field, message string) {
				diagnostics = append(diagnostics, config.Diagnostic{Severity: config.SeverityError, Category: config.CategorySchema, File: path, Field: field, Message: message})
			}
			relativePath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			data, err := sourceRoot.ReadFile(relativePath)
			if err != nil {
				return err
			}
			var node yaml.Node
			decoder := yaml.NewDecoder(bytes.NewReader(data))
			if err := decoder.Decode(&node); err != nil {
				add("", err.Error())
				return nil
			}
			var extra yaml.Node
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				add("", "configuration must contain exactly one YAML document")
				return nil
			}
			var decoded any
			if err := node.Decode(&decoded); err != nil {
				add("", err.Error())
				return nil
			}
			if decoded == nil || (node.Content[0].Kind != yaml.MappingNode && node.Content[0].Kind != yaml.SequenceNode) {
				add("", "configuration must be a non-empty mapping or sequence")
				return nil
			}
			checkReferences(node.Content[0], "", root, filepath.Dir(path), add, 0)
			if strings.Contains(filepath.ToSlash(path), "/templates/") && (filepath.Base(path) == "manifest.yaml" || filepath.Base(filepath.Dir(path)) == "templates") {
				var schema any = &templates.Template{}
				if filepath.Base(path) == "manifest.yaml" {
					schema = &templates.CustomTemplateManifest{}
				}
				var header struct {
					SchemaVersion string `yaml:"schema_version"`
				}
				if err := node.Decode(&header); err != nil {
					add("", err.Error())
					return nil
				}
				if header.SchemaVersion != "" {
					schema = &templates.TemplateSpec{}
				}
				strictDecoder := yaml.NewDecoder(bytes.NewReader(data))
				strictDecoder.KnownFields(true)
				if err := strictDecoder.Decode(schema); err != nil {
					add("", err.Error())
					return nil
				}
				spec, err := manager.LoadTemplateSpecFile(path)
				if err != nil {
					add("", err.Error())
					return nil
				}
				if err := manager.ValidateTemplateSpec(*spec); err != nil {
					add("", err.Error())
					return nil
				}
				for action, steps := range spec.Lifecycle {
					checkTemplateSteps(steps, "lifecycle."+string(action), known, add)
				}
			}
			return nil
		})
		if err != nil {
			return diagnostics, err
		}
	}
	return diagnostics, nil
}

func checkTemplateSteps(steps []templates.TemplateStepSpec, field string, known map[string]bool, add func(string, string)) {
	for i, step := range steps {
		location := fmt.Sprintf("%s[%d]", field, i)
		if step.App != nil {
			if step.App.Ref != "" && !known[strings.ToLower(step.App.Ref)] {
				add(location+".app.ref", "unknown catalog application: "+step.App.Ref)
			}
			if step.App.Installer != "" && config.ClassifyInstallMethod(step.App.Installer) == config.MethodUnknown {
				add(location+".app.installer", "unsupported install method: "+step.App.Installer)
			}
			if step.App.App != nil {
				app := config.CrossPlatformAppFromLegacy(*step.App.App)
				for _, e := range config.ValidateApplicationGraph([]types.CrossPlatformApp{app}) {
					add(location+".app."+e.Field, e.Message)
				}
			}
		}
	}
}

func checkReferences(node *yaml.Node, field, root, dir string, add func(string, string), depth int) {
	if depth > 100 {
		add(field, "YAML reference nesting exceeds maximum depth")
		return
	}
	if node.Kind == yaml.AliasNode {
		checkReferences(node.Alias, field, root, dir, add, depth+1)
		return
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i].Value, node.Content[i+1]
			location := key
			if field != "" {
				location = field + "." + key
			}
			if key == "install_method" && config.ClassifyInstallMethod(value.Value) == config.MethodUnknown {
				add(location, "unsupported install method: "+value.Value)
			}
			if key == "source" && value.Kind == yaml.ScalarNode {
				source := value.Value
				const prefix = "~/.local/share/devex/"
				if strings.HasPrefix(source, prefix) {
					relative := strings.TrimPrefix(source, prefix)
					relative = strings.TrimPrefix(relative, "assets/")
					requireFile(filepath.Join(root, "assets"), relative, location, add)
				} else if (strings.Contains(field, "config_files[") || strings.HasSuffix(field, ".file") || strings.HasSuffix(field, ".files") || strings.Contains(field, ".files[")) && source != "" && !strings.Contains(source, "://") && !strings.HasPrefix(source, "~") && !strings.Contains(source, "$") {
					requireFile(dir, source, location, add)
				}
			}
			if key == "files" && field == "" && value.Kind == yaml.SequenceNode {
				for j, file := range value.Content {
					if file.Kind == yaml.ScalarNode {
						requireFile(dir, file.Value, fmt.Sprintf("files[%d]", j), add)
					}
				}
			}
			checkReferences(value, location, root, dir, add, depth+1)
		}
	} else {
		for i, child := range node.Content {
			checkReferences(child, fmt.Sprintf("%s[%d]", field, i), root, dir, add, depth+1)
		}
	}
}

func requireFile(root, name, field string, add func(string, string)) {
	if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(filepath.Clean(name), ".."+string(filepath.Separator)) {
		add(field, "local reference escapes shipped tree: "+name)
		return
	}
	info, err := os.Stat(filepath.Join(root, name))
	if err != nil {
		add(field, "referenced local asset is missing: "+name+": "+err.Error())
	} else if !info.Mode().IsRegular() {
		add(field, "referenced local asset is not a regular file: "+name)
	}
}
