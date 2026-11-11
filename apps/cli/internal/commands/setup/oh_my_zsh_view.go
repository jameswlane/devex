package setup

import (
	"fmt"
	"strings"
)

func (m *SetupModel) renderOhMyZshPreview() string {
	plan, err := m.getOhMyZshPlan()
	if err != nil {
		return "  • Oh My Zsh configuration error: " + err.Error() + "\n"
	}
	if plan == nil {
		return ""
	}
	var b strings.Builder
	if plan.alreadyLoads {
		b.WriteString("  • your ~/.zshrc already loads Oh My Zsh; it is left unchanged\n")
		b.WriteString("  • Login shell remains unchanged\n")
		return b.String()
	}
	if plan.clone {
		fmt.Fprintf(&b, "  • Install Oh My Zsh in %s\n", plan.frameworkDir)
		fmt.Fprintf(&b, "      Repository: %s\n", ohMyZshRepository)
		fmt.Fprintf(&b, "      Pinned commit: %s; git fetch --depth 1, then detached checkout (no install script)\n", ohMyZshRevision)
	} else {
		fmt.Fprintf(&b, "  • Reuse Oh My Zsh at %s; do not modify it\n", plan.frameworkDir)
	}
	b.WriteString("  • Bundled Zsh defaults:\n")
	b.WriteString("      History: keep 5,000 entries in memory and in ~/.zsh_history.\n")
	b.WriteString("      PATH: preserve existing entries; add ~/.local/bin and the DevEx bin directory only when missing.\n")
	b.WriteString("      Aliases: add shortcuts only when their external tools are installed.\n")
	b.WriteString("      Init: activate mise once; initialize zoxide and fzf only when installed.\n")
	for _, asset := range plan.assets {
		action := "keep existing, unchanged"
		if asset.create {
			action = "create"
		}
		fmt.Fprintf(&b, "      %s: %s\n", action, asset.path)
	}
	fmt.Fprintf(&b, "  • Merge into the DevEx block in %s:\n", plan.rcFile)
	for _, entry := range plan.rcEntries() {
		fmt.Fprintf(&b, "      %s\n", entry)
	}
	fmt.Fprintf(&b, "  • Before the first rc write, back up an existing file as %s.devex-bak-*\n", plan.rcFile)
	b.WriteString("  • Login shell remains unchanged\n")
	return b.String()
}
