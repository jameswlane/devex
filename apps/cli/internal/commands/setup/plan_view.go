package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderInstallPlan lists everything setup will install or change, built from
// the same plan the installer runs.
func (m *SetupModel) renderInstallPlan() string {
	plan := m.installPlan()
	noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	var b strings.Builder
	b.WriteString("Setup will make these changes:\n\n")

	toInstall := 0
	needsAptUpdate := false
	section := func(title string, group planGroup) {
		var lines []string
		for _, item := range plan {
			if item.group != group {
				continue
			}
			line := "  • " + item.app.Name
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
		}
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s (%d):\n%s\n\n", title, len(lines), strings.Join(lines, "\n"))
	}

	section("📦 Default tools", planDefault)
	section("🔗 Required by your selections", planDependency)
	section("✨ Your selections", planSelected)

	if m.selections.skipDefaultApps {
		b.WriteString(noteStyle.Render("Default tools skipped. Press 'd' to include them.") + "\n\n")
	}

	b.WriteString("⚙️  Also:\n")
	if needsAptUpdate {
		b.WriteString("  • Update package lists (apt-get update, once)\n")
	}
	fmt.Fprintf(&b, "  • Shell: %s\n", m.getSelectedShell())
	if theme := m.getSelectedTheme(); theme != "" {
		fmt.Fprintf(&b, "  • Theme: %s\n", theme)
	}
	if desktop := m.system.detectedPlatform.DesktopEnv; m.system.hasDesktop && desktop != "" && desktop != "unknown" {
		fmt.Fprintf(&b, "  • %s desktop settings from the desktop-%s plugin\n", desktop, desktop)
	}
	if name := strings.TrimSpace(m.git.gitFullName); name != "" {
		fmt.Fprintf(&b, "  • Git identity: %s <%s>", name, strings.TrimSpace(m.git.gitEmail))
		if m.gitIdentityUnchanged() {
			b.WriteString(" (existing, unchanged)")
		}
		b.WriteString("\n")
	}
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
	fmt.Fprintf(&b, "Press Enter to start installation, 'd' to %s default tools, 'p' to go back, or 'q' to quit.", toggle)
	return b.String()
}
