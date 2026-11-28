package setup

import (
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/constants"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// planDependencies collects the prerequisite tools that selected items declare
// in their YAML dependencies. A prerequisite is a catalog app that installs
// through a plugin step (install_method: plugin), such as the mise entry.
// Programming languages are walked through but never added: a language is the
// user's own choice on the Languages screen, so an AI agent that depends on
// Node.js pulls in Node.js's prerequisites, not Node.js itself.
type planDependencies struct {
	m          *SetupModel
	catalog    map[string]types.CrossPlatformApp
	languages  map[string]bool
	order      []string
	requesters map[string][]string
	// needs maps each item that reaches a prerequisite, including languages
	// walked through on the way, to the prerequisites it must wait for.
	needs map[string][]string
}

func (m *SetupModel) newPlanDependencies() *planDependencies {
	d := &planDependencies{
		m:          m,
		catalog:    make(map[string]types.CrossPlatformApp),
		languages:  make(map[string]bool),
		requesters: make(map[string][]string),
		needs:      make(map[string][]string),
	}
	for _, app := range m.settings.GetAllApps() {
		key := dependencyKey(app.Name)
		if _, seen := d.catalog[key]; !seen {
			d.catalog[key] = app
		}
	}
	for _, language := range m.languageEntries() {
		key := dependencyKey(language.Name)
		d.languages[key] = true
		d.catalog[key] = language
	}
	return d
}

// languageEntries returns the configured programming languages, followed by
// the catalog's languages that configuration doesn't define (all of them
// before a first run writes the configuration).
func (m *SetupModel) languageEntries() []types.CrossPlatformApp {
	languages := append([]types.CrossPlatformApp(nil), m.settings.ProgrammingLanguages...)
	configured := make(map[string]bool, len(languages))
	for _, language := range languages {
		configured[dependencyKey(language.Name)] = true
	}
	for _, app := range m.settings.GetAllApps() {
		if strings.EqualFold(app.Category, constants.CategoryLanguages) && !configured[dependencyKey(app.Name)] {
			configured[dependencyKey(app.Name)] = true
			languages = append(languages, app)
		}
	}
	return languages
}

func dependencyKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// languageDependencies returns a configured language's declared dependencies.
func (m *SetupModel) languageDependencies(name string) []string {
	for _, language := range m.languageEntries() {
		if strings.EqualFold(language.Name, name) {
			return language.GetOSConfig().Dependencies
		}
	}
	return nil
}

// platformConfig is app's configuration for the platform setup runs on.
func (m *SetupModel) platformConfig(app types.CrossPlatformApp) types.OSConfig {
	if m.system.detectedPlatform.OS == "" {
		return app.GetOSConfig()
	}
	return osConfigForPlatform(app, m.system.detectedPlatform.OS)
}

// add records the prerequisites that requester's dependencies reach.
func (d *planDependencies) add(requester string, dependencies []string) {
	d.walk(requester, []string{requester}, dependencies, map[string]bool{})
}

// walk records the prerequisites dependencies reach. requester is named in
// the plan's reason; every item in chain must wait for what is found.
func (d *planDependencies) walk(requester string, chain []string, dependencies []string, visiting map[string]bool) {
	for _, name := range dependencies {
		key := dependencyKey(name)
		if key == "" || visiting[key] {
			continue
		}
		app, ok := d.catalog[key]
		if !ok {
			continue
		}
		visiting[key] = true
		cfg := d.m.platformConfig(app)
		if d.languages[key] {
			d.walk(requester, append(chain[:len(chain):len(chain)], app.Name), cfg.Dependencies, visiting)
			continue
		}
		if cfg.InstallMethod != types.InstallMethodPlugin {
			continue
		}
		if d.m.system.detectedPlatform.OS != "" && !isCompatibleWithPlatform(app, d.m.system.detectedPlatform) {
			continue
		}
		// A prerequisite's own prerequisites install before it.
		d.walk(app.Name, []string{app.Name}, cfg.Dependencies, visiting)
		if _, planned := d.requesters[key]; !planned {
			d.order = append(d.order, key)
		}
		if !containsFold(d.requesters[key], requester) {
			d.requesters[key] = append(d.requesters[key], requester)
		}
		for _, item := range chain {
			itemKey := dependencyKey(item)
			if !containsFold(d.needs[itemKey], app.Name) {
				d.needs[itemKey] = append(d.needs[itemKey], app.Name)
			}
		}
	}
}

// items returns a plan item for every collected prerequisite that existing
// does not already hold, and records each in existing.
func (d *planDependencies) items(reasonPrefix string, existing map[string]bool) []planItem {
	items := make([]planItem, 0, len(d.order))
	for _, key := range d.order {
		if existing[key] {
			continue
		}
		existing[key] = true
		resolved, err := d.m.resolvePlanApp(d.catalog[key], true)
		items = append(items, planItem{
			app:        resolved,
			group:      planDependency,
			reason:     reasonPrefix + strings.Join(d.requesters[key], ", "),
			channelErr: err,
		})
	}
	return items
}

// prerequisitesOf returns the prerequisites the named item must wait for.
func (d *planDependencies) prerequisitesOf(name string) []string {
	return d.needs[dependencyKey(name)]
}

// withDependsOn returns app with names added to every configuration's
// dependencies, so the install scheduler starts app only after them.
func withDependsOn(app types.CrossPlatformApp, names []string) types.CrossPlatformApp {
	if len(names) == 0 {
		return app
	}
	add := func(cfg types.OSConfig) types.OSConfig {
		if cfg.InstallMethod == "" {
			return cfg
		}
		deps := append([]string(nil), cfg.Dependencies...)
		for _, name := range names {
			if !containsFold(deps, name) {
				deps = append(deps, name)
			}
		}
		cfg.Dependencies = deps
		return cfg
	}
	addAll := func(cfg types.OSConfig) types.OSConfig {
		cfg = add(cfg)
		alternatives := make([]types.OSConfig, len(cfg.Alternatives))
		for i, alternative := range cfg.Alternatives {
			alternatives[i] = add(alternative)
		}
		if len(alternatives) > 0 {
			cfg.Alternatives = alternatives
		}
		return cfg
	}
	app.Linux = addAll(app.Linux)
	app.MacOS = addAll(app.MacOS)
	app.Windows = addAll(app.Windows)
	app.AllPlatforms = addAll(app.AllPlatforms)
	return app
}

func containsFold(values []string, value string) bool {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return true
		}
	}
	return false
}

// withPluginStep marks a plan item whose app installs through a plugin step,
// so the Confirmation screen previews that step.
func (m *SetupModel) withPluginStep(item planItem) planItem {
	if item.channelErr != nil {
		return item
	}
	cfg := m.platformConfig(item.app)
	if cfg.InstallMethod != types.InstallMethodPlugin {
		return item
	}
	plugin, args, err := types.ParsePluginCommand(cfg.InstallCommand)
	if err != nil || len(args) == 0 {
		return item
	}
	item.pluginStep = &pluginStep{plugin: plugin, command: args[0], args: args[1:]}
	return item
}
