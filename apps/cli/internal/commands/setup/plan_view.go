package setup

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// renderInstallPlan lists everything setup will install or change, plus any
// excluded defaults. Included entries come from the same plan the installer runs.
func (m *SetupModel) renderInstallPlan() string {
	plan := m.installPlan()
	noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Bold(true)

	var b strings.Builder
	b.WriteString("Setup will make these changes:\n\n")

	toInstall := 0
	needsAptUpdate := false
	section := func(title string, group planGroup) {
		items := plan
		if group == planDefault && !m.selections.skipDefaultApps {
			items = make([]planItem, 0, len(m.getDefaultApps()))
			for _, app := range m.getDefaultApps() {
				items = append(items, planItem{app: app, group: planDefault})
			}
		}

		var lines []string
		count := 0
		for itemIndex, item := range items {
			if item.group != group {
				continue
			}
			count++
			excluded := group == planDefault && m.selections.excludedDefaults[item.app.Name]
			line := "  • " + item.app.Name
			if group == planDefault {
				line = "  [x] " + item.app.Name
				if excluded {
					line = "  [ ] " + item.app.Name
				}
				if m.step == StepConfirmation && itemIndex == m.cursor {
					line = cursorStyle.Render("> " + strings.TrimPrefix(line, " "))
				}
			}
			if excluded {
				lines = append(lines, line+noteStyle.Render(" — skipped"))
				continue
			}
			if item.reason != "" {
				line += noteStyle.Render(" (for " + item.reason + ")")
			}
			if m.selections.alreadyInstalled[item.app.Name] {
				line += noteStyle.Render(" — already installed, will skip")
			} else {
				toInstall++
				if item.app.GetOSConfig().InstallMethod == "apt" {
					needsAptUpdate = true
				}
			}
			lines = append(lines, line)
			if !m.selections.alreadyInstalled[item.app.Name] {
				config := item.app.GetOSConfig()
				if len(config.Packages) > 0 {
					data, _ := json.Marshal(struct {
						Packages []string `json:"packages"`
						Options  []string `json:"options,omitempty"`
					}{config.Packages, config.Options})
					lines = append(lines, fmt.Sprintf("      %s: %s", config.InstallMethod, data))
				} else if config.InstallCommand != "" {
					lines = append(lines, fmt.Sprintf("      %s: %s", config.InstallMethod, config.InstallCommand))
				}
				for _, hooks := range [][]types.InstallCommand{config.PreInstall, config.PostInstall} {
					for _, hook := range hooks {
						// Show only the selected shell, matching ApplyActions.
						if hook.ShellInit != nil {
							init := map[string]string{}
							if line := hook.ShellInit[m.getSelectedShell()]; line != "" {
								init[m.getSelectedShell()] = line
							}
							hook.ShellInit = init
						}
						data, _ := json.Marshal(hook)
						lines = append(lines, "      Hook: "+string(data))
					}
				}
				for _, file := range config.ConfigFiles {
					lines = append(lines, fmt.Sprintf("      Config file: %s → %s", file.Source, file.Destination))
				}
			}
		}
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s (%d):\n%s\n\n", title, count, strings.Join(lines, "\n"))
	}

	section("📦 Default tools", planDefault)
	section("🔗 Required by your selections", planDependency)
	section("✨ Your selections", planSelected)

	if m.selections.skipDefaultApps {
		b.WriteString(noteStyle.Render("[ ] Default tools skipped. Press 'd' to include them.") + "\n\n")
	}

	b.WriteString("⚙️  Also:\n")
	if needsAptUpdate {
		b.WriteString("  • Update package lists (apt-get update, once)\n")
	}
	fmt.Fprintf(&b, "  • Shell: %s\n", m.getSelectedShell())
	if theme := m.getSelectedTheme(); theme != "" {
		fmt.Fprintf(&b, "  • Theme preference (saved only): %s\n", theme)
	}
	b.WriteString(m.renderDesktopPreview())

	if name := strings.TrimSpace(m.git.gitFullName); name != "" {
		fmt.Fprintf(&b, "  • Git identity: %s <%s>", name, strings.TrimSpace(m.git.gitEmail))
		if m.gitIdentityUnchanged() {
			b.WriteString(" (existing, unchanged)")
		}
		b.WriteString("\n")
	}
	b.WriteString(m.renderSSHPreview())
	b.WriteString("\n")

	fmt.Fprintf(&b, "Total: %d apps to install", toInstall)
	if skipped := len(plan) - toInstall; skipped > 0 {
		fmt.Fprintf(&b, ", %d already installed", skipped)
	}
	b.WriteString("\n\n")

	toggle := "skip"
	if m.selections.skipDefaultApps {
		toggle = "include"
	}
	fmt.Fprintf(&b, "Use ↑/↓ to navigate, Space to skip/include the highlighted default, Enter to start installation, 'd' to %s all default tools, 'p' to go back, or 'q' to quit.", toggle)
	return b.String()
}
