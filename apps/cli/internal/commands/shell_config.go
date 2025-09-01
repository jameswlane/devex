package commands

// getSelectedShell returns the shell name corresponding to the selected index
func (m *SetupModel) getSelectedShell() string {
	if m.selectedShell >= 0 && m.selectedShell < len(m.shells) {
		return m.shells[m.selectedShell]
	}
	return "zsh" // Default fallback
}
