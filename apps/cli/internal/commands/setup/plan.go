package setup

import (
	"context"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// planGroup says why an app is in the install plan.
type planGroup int

const (
	planDefault    planGroup = iota // installed by default (default: true)
	planDependency                  // a prerequisite a selection declares in its YAML dependencies
	planSelected                    // chosen by the user
)

// planItem is one app in the install plan.
type planItem struct {
	app        types.CrossPlatformApp
	group      planGroup
	reason     string // for dependencies: what needs it
	channelErr error  // invalid channel selection or refused channel switch
	// pluginStep, when set, installs the app by running a plugin command; the
	// Confirmation screen shows the plugin's own description of it.
	pluginStep *pluginStep
}

// installPlan returns every app setup will install, in install order. Both
// the Confirmation screen and the installer use it, so what the user confirms
// is exactly what runs.
func (m *SetupModel) installPlan() []planItem {
	var plan []planItem

	if !m.selections.skipDefaultApps {
		for _, app := range m.getDefaultApps() {
			if m.selections.excludedDefaults[app.Name] {
				continue
			}
			resolved, chErr := m.resolvePlanApp(app, false)
			plan = append(plan, planItem{app: resolved, group: planDefault, channelErr: chErr})
		}
	}

	existing := make(map[string]bool, len(plan))
	for _, item := range plan {
		existing[dependencyKey(item.app.Name)] = true
	}

	if languages := m.getSelectedLanguages(); len(languages) > 0 {
		dependencies := m.newPlanDependencies()
		for _, language := range languages {
			dependencies.add(language, m.languageDependencies(language))
		}
		plan = append(plan, dependencies.items("languages: ", existing)...)
		for _, app := range m.getLanguageApps() {
			resolved, chErr := m.resolvePlanApp(app, false)
			plan = append(plan, planItem{app: resolved, group: planSelected, channelErr: chErr})
		}
	}

	if databases := m.getSelectedDatabases(); len(databases) > 0 {
		if dockerApp := m.getDockerApp(); dockerApp != nil {
			resolved, chErr := m.resolvePlanApp(*dockerApp, true)
			plan = append(plan, planItem{app: resolved, group: planDependency, reason: "databases: " + strings.Join(databases, ", "), channelErr: chErr})
		}
		for _, app := range m.getDatabaseApps() {
			resolved, chErr := m.resolvePlanApp(app, false)
			plan = append(plan, planItem{app: resolved, group: planSelected, channelErr: chErr})
		}
	}

	if m.system.hasDesktop {
		for _, name := range m.getSelectedDesktopApps() {
			if app := m.getDesktopAppByName(name); app != nil {
				resolved, chErr := m.resolvePlanApp(*app, false)
				plan = append(plan, planItem{app: resolved, group: planSelected, channelErr: chErr})
			}
		}
	}

	for _, item := range plan {
		existing[dependencyKey(item.app.Name)] = true
	}
	agentDeps, agentItems := m.expandAIAgentPlanItems(existing)
	plan = append(plan, agentDeps...)
	plan = append(plan, agentItems...)

	for i := range plan {
		plan[i] = m.withPluginStep(plan[i])
	}
	return plan
}

// planApps returns the apps of a plan in install order.
func planApps(plan []planItem) []types.CrossPlatformApp {
	apps := make([]types.CrossPlatformApp, 0, len(plan))
	for _, item := range plan {
		apps = append(apps, item.app)
	}
	return apps
}

// installedAppsCheck is replaced in tests.
var installedAppsCheck = tui.InstalledApps

// preparePackageBatch shares the TUI's batched package transactions; tests replace it.
var preparePackageBatch = tui.PreparePackageBatch

// prepareAptSources writes the APT sources of an app the batch did not install; tests replace it.
var prepareAptSources = tui.PrepareAptSources

// refreshInstalledApps records which planned apps are already installed so the
// Confirmation screen can show them as skipped, matching what the installer does.
func (m *SetupModel) refreshInstalledApps() {
	if m.step == StepConfirmation {
		maxCursor := len(m.getDefaultApps()) - 1
		switch {
		case m.selections.skipDefaultApps || maxCursor < 0:
			m.cursor = 0
		case m.cursor > maxCursor:
			m.cursor = maxCursor
		case m.cursor < 0:
			m.cursor = 0
		}
	}

	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	m.refreshDesktopPreview(ctx)
	plan := m.installPlan()
	checkable := make([]types.CrossPlatformApp, 0, len(plan))
	for _, item := range plan {
		if item.channelErr == nil {
			checkable = append(checkable, item.app)
		}
	}
	m.selections.alreadyInstalled = tui.SkippableInstalledApps(m.repo, installedAppsCheck(ctx, checkable))
	m.refreshPluginPreviews(ctx, plan)
}
