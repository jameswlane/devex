package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

// initProfileMetadataKey marks a lifecycle template that `devex init` offers
// as a profile. The profile's description, categories, languages and tools all
// come from that template, so init never keeps its own copy of the list.
const initProfileMetadataKey = "init_profile"

// availableProfiles returns the static profiles, then one profile per lifecycle
// template marked with init_profile, then "custom" last.
func availableProfiles(homeDir string) []ProfileTemplate {
	profiles := make([]ProfileTemplate, 0, len(profileTemplates)+4)
	var custom *ProfileTemplate
	for _, profile := range profileTemplates {
		if profile.Name == "custom" {
			customProfile := profile
			custom = &customProfile
			continue
		}
		profiles = append(profiles, profile)
	}
	profiles = append(profiles, lifecycleProfiles(templates.NewTemplateManager(homeDir))...)
	if custom != nil {
		profiles = append(profiles, *custom)
	}
	return profiles
}

// lifecycleProfiles builds init profiles from the lifecycle templates marked
// with init_profile metadata, sorted by template ID.
func lifecycleProfiles(manager *templates.TemplateManager) []ProfileTemplate {
	specs, err := manager.GetAvailableTemplateSpecs()
	if err != nil {
		return nil
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].ID < specs[j].ID })

	profiles := make([]ProfileTemplate, 0, len(specs))
	for _, spec := range specs {
		if enabled, _ := spec.Metadata[initProfileMetadataKey].(bool); !enabled {
			continue
		}
		profiles = append(profiles, profileFromTemplateSpec(spec))
	}
	return profiles
}

func profileFromTemplateSpec(spec templates.TemplateSpec) ProfileTemplate {
	return ProfileTemplate{
		Name:        spec.ID,
		Description: spec.Description,
		Categories:  append([]string{}, spec.Categories...),
		Languages:   metadataStrings(spec.Metadata, "languages"),
		Tools:       templateInstallTools(spec),
		Template:    spec.ID,
	}
}

// templateInstallTools lists the apps a template's install lifecycle can
// install, once each, in step order. Platform variants of one app (such as the
// Linux and macOS Android Studio steps) share a name and appear once.
func templateInstallTools(spec templates.TemplateSpec) []string {
	tools := []string{}
	seen := map[string]bool{}
	for _, step := range spec.Lifecycle[templates.LifecycleInstall] {
		if step.App == nil {
			// A `devex install <app>` shell step installs a catalog app too.
			if shell := step.Shell; shell != nil && shell.Command == "devex" && len(shell.Args) >= 2 && shell.Args[0] == "install" && !seen[shell.Args[1]] {
				seen[shell.Args[1]] = true
				tools = append(tools, shell.Args[1])
			}
			continue
		}
		name := step.App.Name
		if name == "" {
			name = step.App.Ref
		}
		if name == "" && step.App.App != nil {
			name = step.App.App.Name
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		tools = append(tools, name)
	}
	return tools
}

func metadataStrings(metadata map[string]any, key string) []string {
	values := []string{}
	switch typed := metadata[key].(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, value := range typed {
			if text, ok := value.(string); ok && text != "" {
				values = append(values, text)
			}
		}
	}
	return values
}

func findProfile(profiles []ProfileTemplate, name string) (ProfileTemplate, bool) {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return ProfileTemplate{}, false
}

func profileNames(profiles []ProfileTemplate) []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.Name)
	}
	return names
}

// printTemplateNextSteps tells the user how to install a template-backed
// profile; init writes configuration only and never runs the template.
func printTemplateNextSteps(out io.Writer, profile ProfileTemplate) {
	if profile.Template == "" {
		return
	}
	fmt.Fprintf(out, "\nThe %s profile is installed by the %q lifecycle template:\n", profile.Name, profile.Template)
	fmt.Fprintf(out, "  devex template info %s\n", profile.Template)
	fmt.Fprintf(out, "  devex template plan %s --action install\n", profile.Template)
	fmt.Fprintf(out, "  devex install --template %s\n", profile.Template)
	if len(profile.Tools) > 0 {
		fmt.Fprintf(out, "Tools it can install: %s\n", strings.Join(profile.Tools, ", "))
	}
}
