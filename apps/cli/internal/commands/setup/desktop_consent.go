package setup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// Preview only plugins already loaded at the plugin step; never download or
// initialize plugins while constructing a confirmation or dry-run view.
func (m *SetupModel) refreshDesktopPreview(ctx context.Context) {
	m.selections.desktopSettings = false
	m.selections.desktopPreview = ""
	m.selections.desktopPlugin = ""
	if !m.system.hasDesktop || m.pluginBootstrap == nil {
		return
	}
	desktop := strings.ToLower(m.system.detectedPlatform.DesktopEnv)
	if desktop == "plasma" {
		desktop = "kde"
	}
	if desktop != "gnome" && desktop != "kde" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	name := "desktop-" + desktop
	preview, err := describeDesktop(ctx, m.pluginBootstrap.GetManager(), name)
	if err != nil {
		return
	}
	m.selections.desktopPlugin = name
	m.selections.desktopPreview = preview
}

func describeDesktop(ctx context.Context, manager configurationPluginExecutor, name string) (string, error) {
	var output configurationOutput
	if err := manager.ExecutePluginWithIO(ctx, name, []string{"describe"}, sdk.OperationIO{Stdout: &output}); err != nil {
		return "", err
	}
	var commands [][]string
	if err := json.Unmarshal([]byte(output.String()), &commands); err != nil {
		return "", err
	}
	if len(commands) == 0 {
		return "", fmt.Errorf("empty desktop plan")
	}
	for _, command := range commands {
		if len(command) == 0 {
			return "", fmt.Errorf("empty desktop command")
		}
	}
	data, err := json.Marshal(commands)
	return string(data), err
}

func (m *SetupModel) renderDesktopPreview() string {
	if m.selections.desktopPreview == "" {
		return "  • Desktop settings: skipped (no supported preview available)\n"
	}
	state := "[ ] disabled"
	if m.selections.desktopSettings {
		state = "[x] enabled"
	}
	var commands [][]string
	if err := json.Unmarshal([]byte(m.selections.desktopPreview), &commands); err != nil {
		return "  • Desktop settings: skipped (invalid preview)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  • Desktop settings %s (press 's' to toggle):\n", state)
	for _, command := range commands {
		data, _ := json.Marshal(command)
		fmt.Fprintf(&b, "      %s\n", data)
	}
	return b.String()
}

func (m *SetupModel) applyDesktopPreview(ctx context.Context, manager configurationPluginExecutor) error {
	if !m.selections.desktopSettings || m.selections.desktopPreview == "" {
		return nil
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(m.selections.desktopPreview)))
	return runConfigurationPlugin(ctx, manager, m.selections.desktopPlugin, desktopConfigurationArgs(hash)[0])
}
