package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/agentapply"
	"github.com/jameswlane/devex/apps/cli/internal/agentdoctor"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/viper"
)

// AI coding agent catalog category names and setup group headings.
const (
	CategoryAICodingAgents    = "AI Coding Agents"
	CategoryAIAgentHarnesses  = "AI Agent Harnesses"
	CategoryAIEditors         = "AI Code Editors"
	CategoryLocalModels       = "Local Models"
	CategoryAIAgentConnectors = "AI Agent Connectors"

	setupAIAgentsAppsKey   = "setup.ai_agents.apps"
	setupAIAgentsPresetKey = "setup.ai_agents.preset"
)

type aiAgentGroupSpec struct {
	Heading  string
	Category string
}

// aiAgentGroups defines the display order and heading for each AI agent category.
// Groups with no platform-compatible apps in the catalog are hidden.
var aiAgentGroups = []aiAgentGroupSpec{
	{Heading: "Agent CLIs", Category: CategoryAICodingAgents},
	{Heading: "Agent harnesses", Category: CategoryAIAgentHarnesses},
	{Heading: "AI editors", Category: CategoryAIEditors},
	{Heading: "Local models", Category: CategoryLocalModels},
	{Heading: "Agent connectors", Category: CategoryAIAgentConnectors},
}

type aiAgentPresetOption struct {
	Value string
	Label string
}

var aiAgentPresetOptions = []aiAgentPresetOption{
	{Value: "", Label: "Leave permission modes unchanged"},
	{Value: config.AgentPresetBalanced, Label: "balanced"},
	{Value: config.AgentPresetStrict, Label: "strict"},
}

// Package-level hooks replaced in tests.
var (
	newAgentDoctorProbe = agentdoctor.DefaultProbe
	applyAIAgentsConfig = defaultApplyAIAgentsConfig
)

func osConfigForPlatform(app types.CrossPlatformApp, osName string) types.OSConfig {
	switch osName {
	case "linux":
		if app.Linux.InstallMethod != "" || app.Linux.InstallCommand != "" {
			return app.Linux
		}
	case "darwin":
		if app.MacOS.InstallMethod != "" || app.MacOS.InstallCommand != "" {
			return app.MacOS
		}
	case "windows":
		if app.Windows.InstallMethod != "" || app.Windows.InstallCommand != "" {
			return app.Windows
		}
	}
	return app.AllPlatforms
}

func isAIAgentCompatibleWithPlatform(app types.CrossPlatformApp, detectedPlatform platform.DetectionResult) bool {
	if detectedPlatform.OS == "" {
		return app.IsSupported()
	}
	cfg := osConfigForPlatform(app, detectedPlatform.OS)
	if cfg.InstallMethod == "" && cfg.InstallCommand == "" {
		return false
	}
	// Editors are skipped on headless hosts and on architectures their
	// repositories don't serve, as in the desktop apps step.
	if detectedPlatform.OS == "linux" && app.RequiresGraphicalDesktop() &&
		(!isCompatibleWithPlatform(app, detectedPlatform) || !isCompatibleWithDesktopEnvironment(app, detectedPlatform)) {
		return false
	}
	return true
}

func aiAgentGroupHeading(category string) (string, int) {
	for idx, g := range aiAgentGroups {
		if strings.EqualFold(category, g.Category) {
			return g.Heading, idx
		}
	}
	return "", -1
}

// getAvailableAIAgentApps returns all catalog apps belonging to the AI coding
// agent categories that can install on detectedPlatform, ordered by group and name.
func getAvailableAIAgentApps(settings config.CrossPlatformSettings, detectedPlatform platform.DetectionResult) []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp
	for _, app := range settings.GetAllApps() {
		if _, idx := aiAgentGroupHeading(app.Category); idx < 0 {
			continue
		}
		if !isAIAgentCompatibleWithPlatform(app, detectedPlatform) {
			continue
		}
		apps = append(apps, app)
	}
	sort.SliceStable(apps, func(i, j int) bool {
		_, leftGroup := aiAgentGroupHeading(apps[i].Category)
		_, rightGroup := aiAgentGroupHeading(apps[j].Category)
		if leftGroup != rightGroup {
			return leftGroup < rightGroup
		}
		leftName := strings.ToLower(apps[i].Name)
		rightName := strings.ToLower(apps[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return apps[i].Name < apps[j].Name
	})
	return apps
}

func configuredAIAgentSelection(settings config.CrossPlatformSettings, detectedPlatform platform.DetectionResult) (map[string]bool, string, error) {
	preset := strings.TrimSpace(viper.GetString(setupAIAgentsPresetKey))
	if err := config.ValidateAgentPreset(preset); err != nil {
		return nil, "", fmt.Errorf("invalid %s/--ai-agents-preset: %w", setupAIAgentsPresetKey, err)
	}

	var names []string
	if raw, ok := viper.Get(setupAIAgentsAppsKey).(string); ok {
		names = strings.Split(raw, ",")
	} else {
		names = viper.GetStringSlice(setupAIAgentsAppsKey)
	}

	available := getAvailableAIAgentApps(settings, detectedPlatform)
	canonicalNames := make(map[string]string, len(available))
	validNames := make([]string, 0, len(available))
	for _, app := range available {
		canonicalNames[strings.ToLower(app.Name)] = app.Name
		validNames = append(validNames, app.Name)
	}
	sort.Strings(validNames)

	selected := make(map[string]bool)
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		canonical, ok := canonicalNames[strings.ToLower(name)]
		if !ok {
			valid := strings.Join(validNames, ", ")
			if valid == "" {
				valid = "none"
			}
			return nil, "", fmt.Errorf("unknown AI coding agent %q in --ai-agents/%s; valid names on this platform: %s", name, setupAIAgentsAppsKey, valid)
		}
		selected[canonical] = true
	}

	return selected, preset, nil
}

// ValidateAIAgentSelection validates configured AI agent names and preset before setup runs.
func ValidateAIAgentSelection(settings config.CrossPlatformSettings, detectedPlatform platform.DetectionResult) error {
	_, _, err := configuredAIAgentSelection(settings, detectedPlatform)
	return err
}

func (m *SetupModel) aiAgentEntries() []types.CrossPlatformApp {
	return m.system.aiAgentApps
}

func (m *SetupModel) getSelectedAIAgentApps() []types.CrossPlatformApp {
	var selected []types.CrossPlatformApp
	for _, app := range m.system.aiAgentApps {
		if m.selections.selectedAIAgents[app.Name] {
			selected = append(selected, app)
		}
	}
	return selected
}

func (m *SetupModel) getSelectedHarnessIDs() []string {
	var ids []string
	for _, app := range m.getSelectedAIAgentApps() {
		if id, ok := agentapply.HarnessIDForCatalogApp(app.Name); ok {
			ids = append(ids, id)
		}
	}
	return agentapply.CanonicalHarnessIDs(ids)
}

func (m *SetupModel) hasSelectedHarnessAgent() bool {
	return len(m.getSelectedHarnessIDs()) > 0
}

func (m *SetupModel) aiAgentPresetCursorIndex() int {
	for i, opt := range aiAgentPresetOptions {
		if opt.Value == m.selections.aiAgentPreset {
			return i
		}
	}
	return 0
}

func (m *SetupModel) userConfigDir() string {
	s := m.settings
	if s.HomeDir == "" {
		s.HomeDir = m.homeDir()
	}
	return s.GetUserConfigDir()
}

// expandAIAgentPlanItems returns prerequisite planItems (the plugin-step
// prerequisites their YAML dependencies declare, and sandbox prerequisites) followed by the selected AI agent planItems.
func (m *SetupModel) expandAIAgentPlanItems(existingNames map[string]bool) ([]planItem, []planItem) {
	selected := m.getSelectedAIAgentApps()
	if len(selected) == 0 {
		return nil, nil
	}

	catalog := make(map[string]types.CrossPlatformApp)
	for _, app := range m.settings.GetAllApps() {
		catalog[strings.ToLower(strings.TrimSpace(app.Name))] = app
	}

	var deps []planItem
	dependencies := m.newPlanDependencies()
	prereqOrder := make([]string, 0)
	prereqApps := make(map[string]types.CrossPlatformApp)
	prereqErrs := make(map[string]error)
	prereqRequesters := make(map[string][]string)

	type resolvedAgent struct {
		app types.CrossPlatformApp
		err error
	}
	resolvedSelected := make([]resolvedAgent, 0, len(selected))
	for _, app := range selected {
		resolved, chErr := m.resolvePlanApp(app, false)
		resolvedSelected = append(resolvedSelected, resolvedAgent{app: resolved, err: chErr})
		cfg := osConfigForPlatform(resolved, m.system.detectedPlatform.OS)
		dependencies.add(resolved.Name, cfg.Dependencies)
		appKey := strings.ToLower(strings.TrimSpace(resolved.Name))
		for _, prereqName := range cfg.SandboxPrerequisites {
			key := strings.ToLower(strings.TrimSpace(prereqName))
			if key == "" || key == appKey || existingNames[key] {
				continue
			}
			prereqApp, ok := catalog[key]
			if !ok || !isAIAgentCompatibleWithPlatform(prereqApp, m.system.detectedPlatform) {
				continue
			}
			if _, seen := prereqApps[key]; !seen {
				prereqOrder = append(prereqOrder, key)
				resolvedPrereq, prereqErr := m.resolvePlanApp(prereqApp, true)
				prereqApps[key] = resolvedPrereq
				prereqErrs[key] = prereqErr
				// A prerequisite's own declared prerequisites are planned first too.
				if prereqErr == nil {
					dependencies.add(resolvedPrereq.Name, osConfigForPlatform(resolvedPrereq, m.system.detectedPlatform.OS).Dependencies)
				}
			}
			prereqRequesters[key] = append(prereqRequesters[key], resolved.Name)
		}
	}

	deps = append(deps, dependencies.items("AI agents: ", existingNames)...)

	// A selected agent that is also another agent's prerequisite keeps the
	// channel chosen for it, in the prerequisite's earlier slot.
	selectedByKey := make(map[string]resolvedAgent, len(resolvedSelected))
	for _, entry := range resolvedSelected {
		selectedByKey[strings.ToLower(strings.TrimSpace(entry.app.Name))] = entry
	}
	for _, key := range prereqOrder {
		if existingNames[key] {
			continue
		}
		app, prereqErr := prereqApps[key], prereqErrs[key]
		if entry, ok := selectedByKey[key]; ok {
			app, prereqErr = entry.app, entry.err
		}
		deps = append(deps, planItem{
			app:        app,
			group:      planDependency,
			reason:     "AI agents: " + strings.Join(prereqRequesters[key], ", "),
			channelErr: prereqErr,
		})
		existingNames[key] = true
	}

	selectedItems := make([]planItem, 0, len(resolvedSelected))
	for _, entry := range resolvedSelected {
		key := strings.ToLower(strings.TrimSpace(entry.app.Name))
		if existingNames[key] {
			continue
		}
		existingNames[key] = true
		selectedItems = append(selectedItems, planItem{app: entry.app, group: planSelected, channelErr: entry.err})
	}

	return deps, selectedItems
}

func (m *SetupModel) succeededHarnessIDs() []string {
	recorded := map[string]tui.AppOutcome{}
	for _, res := range m.installation.outcomeRecorder().Results() {
		recorded[res.Name] = res.Outcome
	}

	var ids []string
	for _, app := range m.getSelectedAIAgentApps() {
		id, ok := agentapply.HarnessIDForCatalogApp(app.Name)
		if !ok {
			continue
		}
		// Only an agent this run installed or found installed gets configured.
		if outcome := recorded[app.Name]; outcome != tui.OutcomeInstalled && outcome != tui.OutcomeSkipped {
			continue
		}
		ids = append(ids, id)
	}
	return agentapply.CanonicalHarnessIDs(ids)
}

func defaultApplyAIAgentsConfig(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap, cfg config.AgentsConfig) error {
	if errs := config.ValidateAgentsConfig(cfg, "agents.yaml"); len(errs) > 0 {
		return fmt.Errorf("invalid agents configuration: %s", errs[0].Error())
	}
	if len(cfg.Enabled) == 0 {
		return nil
	}
	spec := agentapply.BuildResolvedSpec(cfg, cfg.Enabled, "", nil, nil)
	executor := agentapply.BootstrapExecutor{
		GetBootstrap: func() *bootstrap.PluginBootstrap { return pluginBootstrap },
	}
	var out, errBuf bytes.Buffer
	streams := sdk.OperationIO{
		Stdout: &out,
		Stderr: &errBuf,
	}
	if err := agentapply.Execute(ctx, executor, spec, streams, false); err != nil {
		return err
	}
	if skipped := agentapply.SkippedAgents(out.String()); len(skipped) > 0 {
		return fmt.Errorf("tool-agents did not configure every agent: %s", strings.Join(skipped, "; "))
	}
	return nil
}

// handleAIAgentsConfiguration writes a minimal agents.yaml when absent (leaving
// any existing file untouched), applies harness defaults via tool-agents (recording
// failures as warnings rather than setup errors), and runs agent doctor for the
// selected agents to populate the final summary.
func (m *SetupModel) handleAIAgentsConfiguration(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap) {
	selectedApps := m.getSelectedAIAgentApps()
	if len(selectedApps) == 0 {
		return
	}
	// Agents installed to user-scope directories this run are not on PATH yet.
	agentapply.EnsureUserBinDirsOnPath(m.homeDir())

	if harnessIDs := m.succeededHarnessIDs(); len(harnessIDs) > 0 {
		m.writeAndApplyDefaultAgentsConfig(ctx, pluginBootstrap, harnessIDs)
	}

	m.collectAIAgentsDoctorSummary(ctx, selectedApps)
}

// keptAgentsConfigNote tells the user setup left their agents configuration alone.
const keptAgentsConfigNote = "Kept your existing agents.yaml; review it with 'devex agent diff' and apply it with 'devex agent apply'"

// hasExistingAgentsConfig reports whether agents are already configured, either
// in any loaded configuration tier or in the user's agents.yaml.
func (m *SetupModel) hasExistingAgentsConfig() (bool, error) {
	if m.settings.Agents.IsConfigured() {
		return true, nil
	}
	agentsPath := filepath.Join(m.userConfigDir(), "agents.yaml")
	if _, err := os.Stat(agentsPath); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect %s: %w", agentsPath, err)
	}
	return false, nil
}

// writeAndApplyDefaultAgentsConfig writes and applies a minimal agents.yaml.
// Existing agents configuration is never modified or applied by setup.
func (m *SetupModel) writeAndApplyDefaultAgentsConfig(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap, harnessIDs []string) {
	existing, err := m.hasExistingAgentsConfig()
	if err != nil {
		m.installation.addAIAgentWarning(err.Error())
		return
	}
	if existing {
		m.installation.addChange(keptAgentsConfigNote)
		return
	}
	agentsPath, created, err := agentapply.WriteDefaultAgentsYAMLIfMissing(m.userConfigDir(), harnessIDs, m.selections.aiAgentPreset)
	if err != nil {
		log.Warn("Could not write agents.yaml", "path", agentsPath, "error", err)
		m.installation.addAIAgentWarning(fmt.Sprintf("Could not write %s: %v", agentsPath, err))
		return
	}
	if !created {
		m.installation.addChange(keptAgentsConfigNote)
		return
	}
	change := fmt.Sprintf("Wrote %s (enabled: %s)", agentsPath, strings.Join(harnessIDs, ", "))
	if m.selections.aiAgentPreset != "" {
		change += fmt.Sprintf(", preset: %s", m.selections.aiAgentPreset)
	}
	m.installation.addChange(change)
	cfg := config.AgentsConfig{Enabled: harnessIDs, Preset: m.selections.aiAgentPreset}
	if applyErr := applyAIAgentsConfig(ctx, pluginBootstrap, cfg); applyErr != nil {
		log.Warn("Could not apply agents.yaml", "error", applyErr)
		m.installation.addAIAgentWarning(fmt.Sprintf("Could not apply agents.yaml: %v (run 'devex agent apply' later)", applyErr))
		return
	}
	m.installation.addChange("Applied agents.yaml via tool-agents")
}

func (m *SetupModel) collectAIAgentsDoctorSummary(ctx context.Context, selectedApps []types.CrossPlatformApp) {
	known := agentdoctor.KnownAgents()
	byName := make(map[string]agentdoctor.AgentSpec, len(known))
	for _, spec := range known {
		byName[strings.ToLower(strings.TrimSpace(spec.Name))] = spec
	}

	specs := make([]agentdoctor.AgentSpec, 0, len(selectedApps))
	seen := make(map[string]bool, len(selectedApps))
	for _, app := range selectedApps {
		spec, ok := byName[strings.ToLower(strings.TrimSpace(app.Name))]
		if !ok || seen[spec.ID] {
			continue
		}
		seen[spec.ID] = true
		specs = append(specs, spec)
	}
	if len(specs) == 0 {
		return
	}

	probe := newAgentDoctorProbe()
	report := agentdoctor.Check(ctx, probe, specs)
	secrets := agentdoctor.CollectSecretValues(probe.Getenv, os.Getenv)

	var b strings.Builder
	b.WriteString(agentdoctor.FormatTable(report, secrets))
	b.WriteString("\nNext steps (log in to your selected agents):\n")
	for _, spec := range specs {
		switch {
		case len(spec.LoginArgs) > 0:
			fmt.Fprintf(&b, "  • %s: devex agent login %s\n", spec.Name, spec.ID)
		case spec.NoLoginGuidance != "":
			fmt.Fprintf(&b, "  • %s: %s\n", spec.Name, spec.NoLoginGuidance)
		default:
			fmt.Fprintf(&b, "  • %s: authenticate with %s's own CLI or secret manager\n", spec.Name, spec.Name)
		}
	}

	m.installation.setAIAgentSummary(agentdoctor.Redact(b.String(), secrets))
}

func (is *InstallationState) addAIAgentWarning(warning string) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.aiAgentWarnings = append(is.aiAgentWarnings, warning)
}

func (is *InstallationState) getAIAgentWarnings() []string {
	is.mu.RLock()
	defer is.mu.RUnlock()
	return append([]string(nil), is.aiAgentWarnings...)
}

func (is *InstallationState) setAIAgentSummary(summary string) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.aiAgentSummary = summary
}

func (is *InstallationState) getAIAgentSummary() string {
	is.mu.RLock()
	defer is.mu.RUnlock()
	return is.aiAgentSummary
}

func (m *SetupModel) renderAIAgentsPreview() string {
	harnessIDs := m.getSelectedHarnessIDs()
	if len(harnessIDs) == 0 {
		return ""
	}
	agentsPath := filepath.Join(m.userConfigDir(), "agents.yaml")
	var b strings.Builder
	existing, err := m.hasExistingAgentsConfig()
	if err != nil {
		fmt.Fprintf(&b, "  • %v\n", err)
		return b.String()
	}
	if existing {
		fmt.Fprintf(&b, "  • Keep existing agents configuration unchanged; review it with 'devex agent diff' and apply it with 'devex agent apply'\n")
		return b.String()
	}

	data, err := agentapply.RenderMinimalAgentsYAML(harnessIDs, m.selections.aiAgentPreset)
	if err != nil {
		return ""
	}
	fmt.Fprintf(&b, "  • Write %s:\n", agentsPath)
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		fmt.Fprintf(&b, "      %s\n", line)
	}
	b.WriteString("  • Apply agents.yaml via tool-agents\n")
	return b.String()
}

// AIAgentsSummary returns the AI coding agent warnings, doctor results and
// login steps, so they can be printed after the interactive wizard closes.
func (m *SetupModel) AIAgentsSummary() string {
	return m.renderAIAgentsSummary()
}

func (m *SetupModel) renderAIAgentsSummary() string {
	warnings := m.installation.getAIAgentWarnings()
	summary := m.installation.getAIAgentSummary()
	if len(warnings) == 0 && summary == "" {
		return ""
	}
	var b strings.Builder
	if len(warnings) > 0 {
		b.WriteString("AI coding agent warnings:\n")
		for _, w := range warnings {
			fmt.Fprintf(&b, "  • %s\n", w)
		}
		b.WriteString("\n")
	}
	if summary != "" {
		b.WriteString(summary)
		if !strings.HasSuffix(summary, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}
