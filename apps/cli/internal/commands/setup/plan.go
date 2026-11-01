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
	planDependency                  // pulled in by a selection (mise, docker)
	planSelected                    // chosen by the user
)

// planItem is one app in the install plan.
type planItem struct {
	app    types.CrossPlatformApp
	group  planGroup
	reason string // for dependencies: what needs it
}

// installPlan returns every app setup will install, in install order. Both
// the Confirmation screen and the installer use it, so what the user confirms
// is exactly what runs.
func (m *SetupModel) installPlan() []planItem {
	var plan []planItem

	if !m.selections.skipDefaultApps {
		for _, app := range m.getDefaultApps() {
			plan = append(plan, planItem{app: app, group: planDefault})
		}
	}

	if languages := m.getSelectedLanguages(); len(languages) > 0 {
		if miseApp := m.getMiseApp(); miseApp != nil {
			plan = append(plan, planItem{app: *miseApp, group: planDependency, reason: "languages: " + strings.Join(languages, ", ")})
		}
		for _, app := range m.getLanguageApps() {
			plan = append(plan, planItem{app: app, group: planSelected})
		}
	}

	if databases := m.getSelectedDatabases(); len(databases) > 0 {
		if dockerApp := m.getDockerApp(); dockerApp != nil {
			plan = append(plan, planItem{app: *dockerApp, group: planDependency, reason: "databases: " + strings.Join(databases, ", ")})
		}
		for _, app := range m.getDatabaseApps() {
			plan = append(plan, planItem{app: app, group: planSelected})
		}
	}

	if m.system.hasDesktop {
		for _, name := range m.getSelectedDesktopApps() {
			if app := m.getDesktopAppByName(name); app != nil {
				plan = append(plan, planItem{app: *app, group: planSelected})
			}
		}
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
var installedAppsCheck = tui.InstalledAptApps

// refreshInstalledApps records which planned apps are already installed so the
// Confirmation screen can show them as skipped, matching what the installer does.
func (m *SetupModel) refreshInstalledApps() {
	m.selections.alreadyInstalled = installedAppsCheck(context.Background(), planApps(m.installPlan()))
}
