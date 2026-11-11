package setup

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/tui"
)

// outcomeRecorder returns the recorder the installers report to, creating it on
// first use so the completion screen always has one to read.
func (is *InstallationState) outcomeRecorder() *tui.OutcomeRecorder {
	is.mu.Lock()
	defer is.mu.Unlock()
	if is.outcomes == nil {
		is.outcomes = tui.NewOutcomeRecorder()
	}
	return is.outcomes
}

// addChange records a configuration change setup made outside app installs.
func (is *InstallationState) addChange(change string) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.changes = append(is.changes, change)
}

func (is *InstallationState) getChanges() []string {
	is.mu.RLock()
	defer is.mu.RUnlock()
	return append([]string(nil), is.changes...)
}

// renderChangeInventory lists what setup actually did: each planned app's
// outcome and each configuration change, so the result is never summarized as
// a generic "development tools" line.
func (m *SetupModel) renderChangeInventory() string {
	recorded := map[string]tui.AppResult{}
	for _, result := range m.installation.outcomeRecorder().Results() {
		recorded[result.Name] = result
	}
	groups := map[tui.AppOutcome][]string{}
	for _, app := range m.buildAppList() {
		result, ok := recorded[app.Name]
		switch {
		case ok:
			groups[result.Outcome] = append(groups[result.Outcome], app.Name)
		case m.selections.alreadyInstalled[app.Name]:
			groups[tui.OutcomeSkipped] = append(groups[tui.OutcomeSkipped], app.Name)
		default:
			groups[tui.OutcomeNotRun] = append(groups[tui.OutcomeNotRun], app.Name)
		}
	}

	var b strings.Builder
	b.WriteString("What changed:\n")
	for _, group := range []struct {
		outcome tui.AppOutcome
		title   string
	}{
		{tui.OutcomeInstalled, "Installed"},
		{tui.OutcomeSkipped, "Already installed, skipped"},
		{tui.OutcomeFailed, "Failed"},
		{tui.OutcomeCanceled, "Canceled"},
		{tui.OutcomeNotRun, "Not run"},
	} {
		if names := groups[group.outcome]; len(names) > 0 {
			fmt.Fprintf(&b, "  • %s (%d): %s\n", group.title, len(names), strings.Join(names, ", "))
		}
	}
	var declined []string
	for _, app := range m.getDefaultApps() {
		if m.selections.skipDefaultApps || m.selections.excludedDefaults[app.Name] {
			declined = append(declined, app.Name)
		}
	}
	if len(declined) > 0 {
		fmt.Fprintf(&b, "  • Skipped by your choice (%d): %s\n", len(declined), strings.Join(declined, ", "))
	}
	if len(groups) == 0 && len(declined) == 0 {
		b.WriteString("  • No applications were planned\n")
	}
	changes := m.installation.getChanges()
	if len(changes) == 0 {
		b.WriteString("  • No configuration changes\n")
	}
	for _, change := range changes {
		fmt.Fprintf(&b, "  • %s\n", change)
	}
	return b.String()
}
