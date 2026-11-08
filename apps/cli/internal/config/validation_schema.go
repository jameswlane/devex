package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	"gopkg.in/yaml.v3"
)

// validateEffectiveDirectory follows the same directories and depth limits as
// directory_loader.go. Optional absent tiers do not require a system directory.
func (v *ConfigValidator) validateEffectiveDirectory(dir string, sources map[string]string) error {
	for _, name := range CrossPlatformFiles {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		schema := reflect.TypeOf(CrossPlatformSettings{})
		if name == "security.yaml" {
			schema = reflect.TypeOf(SecurityConfigField{})
		}
		node := v.validateSchema(path, schema)
		if node != nil && name == "applications.yaml" {
			v.checkLegacyApplications(node, path, sources)
		}
	}
	groups := []struct {
		name   string
		depth  int
		schema reflect.Type
	}{
		{"applications", 4, reflect.TypeOf(types.CrossPlatformApp{})},
		{"environments/programming-languages", 1, reflect.TypeOf(types.AppConfig{})},
		{"environments/shell", 1, reflect.TypeOf(types.AppConfig{})},
		{"environments/fonts", 1, reflect.TypeOf(types.Font{})},
		{"system", 3, nil}, {"desktop", 4, reflect.TypeOf(DesktopEnvConfig{})},
	}
	for _, group := range groups {
		root := filepath.Join(dir, group.name)
		files, err := configFilesIfPresent(root, group.depth)
		if err != nil {
			v.addError(root, "", "Unable to read configuration directory", err)
			continue
		}
		for _, name := range files {
			path := filepath.Join(root, name)
			schema := group.schema
			if group.name == "desktop" && len(strings.Split(filepath.ToSlash(name), "/")) < 2 {
				continue
			}
			if group.name == "system" {
				switch name {
				case "git.yaml":
					schema = reflect.TypeOf(types.GitConfig{})
				case "global.yaml":
					schema = reflect.TypeOf(struct {
						GlobalTheme string `mapstructure:"global_theme"`
					}{})
					// SSH and terminal settings are intentionally open maps in the loader.
				}
			}
			node := v.validateSchema(path, schema)
			if node == nil {
				continue
			}
			if strings.HasPrefix(group.name, "applications") || group.name == "environments/shell" || group.name == "environments/programming-languages" {
				var app types.CrossPlatformApp
				if err := node.Decode(&app); err != nil {
					v.addError(path, "", "Invalid application structure", err)
					continue
				}
				sources[strings.ToLower(app.Name)] = path
				if group.name != "applications" && len(platformConfigs(app)) == 0 {
					var legacy types.AppConfig
					if err := node.Decode(&legacy); err != nil {
						v.addError(path, "", "Invalid application structure", err)
						continue
					}
					app = CrossPlatformAppFromLegacy(legacy)
				}
				if strings.TrimSpace(app.Name) == "" {
					v.addError(path, "name", "Application name cannot be empty", nil)
				}
				v.validateCrossPlatformApp(app, path)
			}
		}
	}
	return nil
}

// validateSchema checks YAML keys against the actual decoded type rather than
// maintaining a second hand-written list of allowed fields.
func (v *ConfigValidator) validateSchema(path string, schema reflect.Type) *yaml.Node {
	info, err := os.Stat(path)
	if err != nil {
		v.addError(path, "", "Cannot inspect configuration", err)
		return nil
	}
	if info.Size() > maxFileSize {
		v.addError(path, "", "Configuration exceeds maximum size", nil)
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		v.addError(path, "", "Cannot read configuration", err)
		return nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		v.addError(path, "", "Invalid YAML", err)
		return nil
	}
	if len(doc.Content) == 0 {
		return nil
	}
	node := doc.Content[0]
	if schema != nil {
		v.checkSchema(node, schema, path, "")
	}
	return node
}

func schemaFields(t reflect.Type) map[string]reflect.Type {
	fields := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("mapstructure")
		if tag == "" {
			tag = f.Tag.Get("yaml")
		}
		parts := strings.Split(tag, ",")
		if parts[0] == "-" {
			continue
		}
		if parts[0] == "" && len(parts) > 1 && f.Type.Kind() == reflect.Struct {
			for key, value := range schemaFields(f.Type) {
				fields[key] = value
			}
			continue
		}
		key := parts[0]
		if key == "" {
			key = strings.ToLower(f.Name)
		}
		fields[key] = f.Type
	}
	return fields
}

func (v *ConfigValidator) checkSchema(node *yaml.Node, t reflect.Type, path, field string) {
	v.checkSchemaAt(node, t, path, field, 0)
}

func (v *ConfigValidator) checkSchemaAt(node *yaml.Node, t reflect.Type, path, field string, depth int) {
	if depth > 100 {
		v.addError(path, field, "YAML schema nesting exceeds maximum depth", nil)
		return
	}

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if node.Kind == yaml.AliasNode {
		v.checkSchemaAt(node.Alias, t, path, field, depth+1)
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		// Git configurations accept both one object and a list.
		if node.Kind == yaml.SequenceNode {
			for i, child := range node.Content {
				v.checkSchemaAt(child, t, path, fmt.Sprintf("%s[%d]", field, i), depth+1)
			}
			return
		}
		if node.Kind != yaml.MappingNode {
			return
		}
		fields := schemaFields(t)
		if t == reflect.TypeOf(CrossPlatformSettings{}) {
			fields["applications"] = reflect.TypeOf(map[string]interface{}{})
			fields["languages"] = reflect.TypeOf([]string{})
			fields["editor"] = reflect.TypeOf("")
			fields["variables"] = reflect.TypeOf(map[string]interface{}{})
		}
		// Environment entries accept either legacy or cross-platform definitions.
		if t == reflect.TypeOf(types.AppConfig{}) {
			for k, value := range schemaFields(reflect.TypeOf(types.CrossPlatformApp{})) {
				fields[k] = value
			}
		}
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Tag == "!!merge" {
				v.checkSchemaAt(value, t, path, field, depth+1)
				continue
			}
			location := key.Value
			if field != "" {
				location = field + "." + key.Value
			}
			childType, ok := fields[key.Value]
			if !ok {
				message := fmt.Sprintf("Unknown field (line %d, column %d)", key.Line, key.Column)
				if v.strict {
					v.addError(path, location, message, nil)
				} else {
					v.addWarning(path, location, message, nil)
				}
				continue
			}
			v.checkSchemaAt(value, childType, path, location, depth+1)
		}
	case reflect.Slice, reflect.Array:
		if node.Kind == yaml.SequenceNode {
			for i, child := range node.Content {
				v.checkSchemaAt(child, t.Elem(), path, fmt.Sprintf("%s[%d]", field, i), depth+1)
			}
		}
	case reflect.Map:
		if node.Kind == yaml.MappingNode {
			for i := 0; i < len(node.Content); i += 2 {
				v.checkSchemaAt(node.Content[i+1], t.Elem(), path, field+"."+node.Content[i].Value, depth+1)
			}
		}
	}
}

func (v *ConfigValidator) checkLegacyApplications(node *yaml.Node, path string, sources map[string]string) {
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == "applications" {
				v.checkLegacyApplications(node.Content[i+1], path, sources)
				return
			}
		}
		for i := 1; i < len(node.Content); i += 2 {
			v.checkLegacyApplications(node.Content[i], path, sources)
		}
	} else if node.Kind == yaml.SequenceNode {
		for i, child := range node.Content {
			v.checkSchema(child, reflect.TypeOf(types.AppConfig{}), path, fmt.Sprintf("applications[%d]", i))
			app, err := decodeLegacyApplication(child)
			if err != nil {
				v.addError(path, "applications", "Invalid application structure", err)
				continue
			}
			sources[strings.ToLower(app.Name)] = path
			v.validateCrossPlatformApp(app, path)
		}
	}
}
