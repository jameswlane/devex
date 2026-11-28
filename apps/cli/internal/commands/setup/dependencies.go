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
}

func (m *SetupModel) newPlanDependencies() *planDependencies {
	d := &planDependencies{
		m:          m,
		catalog:    make(map[string]types.CrossPlatformApp),
		languages:  make(map[string]bool),
		requesters: make(map[string][]string),
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

// languageEntries returns the configured programming languages, or the
// catalog's when no configuration has been written yet (a first dry run).
func (m *SetupModel) languageEntries() []types.CrossPlatformApp {
	if len(m.settings.ProgrammingLanguages) > 0 {
		return m.settings.ProgrammingLanguages
	}
	var languages []types.CrossPlatformApp
	for _, app := range m.settings.GetAllApps() {
		if strings.EqualFold(app.Category, constants.CategoryLanguages) {
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
	d.walk(requester, dependencies, map[string]bool{})
}

func (d *planDependencies) walk(requester string, dependencies []string, visiting map[string]bool) {
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
			d.walk(requester, cfg.Dependencies, visiting)
			continue
		}
		if cfg.InstallMethod != types.InstallMethodPlugin {
			continue
		}
		if d.m.system.detectedPlatform.OS != "" && !isCompatibleWithPlatform(app, d.m.system.detectedPlatform) {
			continue
		}
		// A prerequisite's own prerequisites install before it.
		d.walk(app.Name, cfg.Dependencies, visiting)
		if _, planned := d.requesters[key]; !planned {
			d.order = append(d.order, key)
		}
		if !containsFold(d.requesters[key], requester) {
			d.requesters[key] = append(d.requesters[key], requester)
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
