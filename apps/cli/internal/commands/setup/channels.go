package setup

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/channels"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// resolvePlanApp resolves app's install channel for setup planning and refuses
// switches away from a different recorded channel. When isDependency is true,
// the run-level --channel / DEVEX_CHANNEL override is omitted so auto-expanded
// prerequisites keep their own preferences or recorded channel.
func (m *SetupModel) resolvePlanApp(app types.CrossPlatformApp, isDependency bool) (types.CrossPlatformApp, error) {
	var resolved types.CrossPlatformApp
	var err error
	if m.selections.selectedChannels != nil {
		if ch := strings.TrimSpace(m.selections.selectedChannels[app.Name]); ch != "" {
			resolved, err = app.WithChannel(ch)
			if err != nil {
				return app, err
			}
		}
	}
	if resolved.Name == "" {
		prefs := channels.PreferencesFromViper()
		if isDependency {
			prefs = prefs.WithoutExplicitChannel()
		}
		recorded := channels.InstalledChannelForApp(m.repo, app)
		resolved, _, err = channels.ResolveForInstall(app, prefs, recorded)
		if err != nil {
			return app, err
		}
	}
	if switchErr := channels.CheckRefusedChannelSwitch(m.repo, resolved); switchErr != nil {
		return resolved, switchErr
	}
	return resolved, nil
}

// appWithSelectedChannel applies any interactive TUI channel choice for app,
// falling back to configured channel preferences or recorded channel for display.
func (m *SetupModel) appWithSelectedChannel(app types.CrossPlatformApp) types.CrossPlatformApp {
	if m.selections.selectedChannels != nil {
		if ch := strings.TrimSpace(m.selections.selectedChannels[app.Name]); ch != "" {
			if switched, err := app.WithChannel(ch); err == nil {
				return switched
			}
		}
	}
	prefs := channels.PreferencesFromViper()
	recorded := channels.InstalledChannelForApp(m.repo, app)
	if chosen, _, err := channels.ResolveForInstall(app, prefs, recorded); err == nil {
		return chosen
	}
	if chosen, _, err := channels.ResolveForInstall(app, prefs.WithoutExplicitChannel(), recorded); err == nil {
		return chosen
	}
	return app
}

func planChannelErrors(plan []planItem) []error {
	var errs []error
	for _, item := range plan {
		if item.channelErr != nil {
			errs = append(errs, item.channelErr)
		}
	}
	return errs
}

func joinPlanChannelErrors(plan []planItem) error {
	errs := planChannelErrors(plan)
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.Join(errs...)
}

func (m *SetupModel) validatePlanChannels() error {
	return joinPlanChannelErrors(m.installPlan())
}

// cycleAppChannel switches the highlighted application to its next available
// install channel, returning true when the selection changed.
func (m *SetupModel) cycleAppChannel(app types.CrossPlatformApp) bool {
	all := app.Channels()
	available := make([]types.ChannelInfo, 0, len(all))
	for _, ch := range all {
		if ch.Available {
			available = append(available, ch)
		}
	}
	if len(available) <= 1 {
		return false
	}
	current := m.appWithSelectedChannel(app).SelectedChannel()
	nextIdx := 0
	for i, ch := range available {
		if strings.EqualFold(ch.Name, current) {
			nextIdx = (i + 1) % len(available)
			break
		}
	}
	if m.selections.selectedChannels == nil {
		m.selections.selectedChannels = make(map[string]string)
	}
	m.selections.selectedChannels[app.Name] = available[nextIdx].Name
	return true
}

// formatSetupChannelTag renders a compact channel summary beside an app name in
// the setup TUI, marking the official channel and listing alternatives when present.
func formatSetupChannelTag(app types.CrossPlatformApp) string {
	all := app.Channels()
	if len(all) == 0 {
		return ""
	}
	selected := app.SelectedChannel()
	var available []types.ChannelInfo
	selectedOfficial := false
	for _, ch := range all {
		if !ch.Available {
			continue
		}
		available = append(available, ch)
		if strings.EqualFold(ch.Name, selected) {
			selectedOfficial = ch.Official
		}
	}
	if len(available) == 0 {
		return ""
	}
	formatOne := func(name string, official bool) string {
		if official {
			return name + " (official)"
		}
		return name
	}
	if len(available) == 1 {
		return "[" + formatOne(available[0].Name, available[0].Official) + "]"
	}
	others := make([]string, 0, len(available)-1)
	for _, ch := range available {
		if strings.EqualFold(ch.Name, selected) {
			continue
		}
		others = append(others, formatOne(ch.Name, ch.Official))
	}
	return fmt.Sprintf("[%s | alt: %s]", formatOne(selected, selectedOfficial), strings.Join(others, ", "))
}
