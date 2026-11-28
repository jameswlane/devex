package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type aiAgentViewRow struct {
	appIndex int
	heading  string
}

func (m *SetupModel) renderAIAgents() string {
	apps := m.aiAgentEntries()
	rows := make([]aiAgentViewRow, 0, len(apps)+len(aiAgentGroups))
	lastHeading := ""
	for i, app := range apps {
		heading, _ := aiAgentGroupHeading(app.Category)
		if heading == "" {
			heading = app.Category
		}
		if heading != lastHeading {
			rows = append(rows, aiAgentViewRow{appIndex: -1, heading: heading})
			lastHeading = heading
		}
		rows = append(rows, aiAgentViewRow{appIndex: i})
	}

	height := m.height
	if height <= 0 {
		height = defaultDesktopAppsHeight
	}
	visibleCount := height - desktopAppsViewOverhead
	if visibleCount < 1 {
		visibleCount = 1
	}
	if visibleCount > len(rows) {
		visibleCount = len(rows)
	}

	cursorRow := 0
	for i, row := range rows {
		if row.appIndex == m.cursor {
			cursorRow = i
			break
		}
	}
	maxOffset := len(rows) - visibleCount
	if m.aiAgentsOffset > maxOffset {
		m.aiAgentsOffset = maxOffset
	}
	if m.aiAgentsOffset < 0 {
		m.aiAgentsOffset = 0
	}
	if cursorRow < m.aiAgentsOffset {
		m.aiAgentsOffset = cursorRow
	}
	if cursorRow >= m.aiAgentsOffset+visibleCount {
		m.aiAgentsOffset = cursorRow - visibleCount + 1
	}

	end := m.aiAgentsOffset + visibleCount
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Bold(true)
	groupStyle := lipgloss.NewStyle().Bold(true)

	lines := []string{
		titleStyle.Render("🤖 AI Coding Agents (Optional)"),
		"",
		subtitleStyle.Render("Select optional AI coding agents to install, or press Enter to skip."),
		"",
	}
	for _, row := range rows[m.aiAgentsOffset:end] {
		if row.appIndex == -1 {
			lines = append(lines, groupStyle.Render(row.heading))
			continue
		}
		app := apps[row.appIndex]
		cursor := " "
		if m.cursor == row.appIndex {
			cursor = cursorStyle.Render(">")
		}
		selected := " "
		if m.selections.selectedAIAgents[app.Name] {
			selected = selectedStyle.Render("✓")
		}
		label := app.Name
		if tag := formatSetupChannelTag(m.appWithSelectedChannel(app)); tag != "" {
			label += " " + subtitleStyle.Render(tag)
		}
		lines = append(lines, fmt.Sprintf("%s [%s] %s", cursor, selected, label))
	}

	var scrollIndicators []string
	if m.aiAgentsOffset > 0 {
		scrollIndicators = append(scrollIndicators, "↑ more")
	}
	if end < len(rows) {
		scrollIndicators = append(scrollIndicators, "↓ more")
	}
	if len(scrollIndicators) > 0 {
		lines = append(lines, strings.Join(scrollIndicators, " "))
	}

	description := ""
	if m.cursor >= 0 && m.cursor < len(apps) {
		description = apps[m.cursor].Description
	}
	lines = append(lines,
		"",
		description,
		"",
		"Use ↑/↓ to navigate, Space to select/deselect, 'c' to switch channel, Enter to continue (or skip if none selected)",
	)
	return strings.Join(lines, "\n")
}

func (m *SetupModel) renderAIAgentPreset() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true).
		Margin(1, 0)
	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Margin(0, 0, 1, 0)
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Bold(true)
	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("🔐 AI Coding Agent Permission Preset"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Choose the default permission preset written to agents.yaml (only if agents.yaml does not already exist):"))
	b.WriteString("\n\n")

	for i, opt := range aiAgentPresetOptions {
		cursor := " "
		if m.cursor == i {
			cursor = cursorStyle.Render(">")
		}
		selected := " "
		if m.selections.aiAgentPreset == opt.Value {
			selected = selectedStyle.Render("●")
		}
		fmt.Fprintf(&b, "%s [%s] %s\n", cursor, selected, opt.Label)
	}

	b.WriteString("\n\nUse ↑/↓ to navigate, Space to select, Enter to continue")
	return b.String()
}
