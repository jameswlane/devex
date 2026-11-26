package agentapply

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	// ToolAgentsPluginName is the plugin that renders agents.yaml into native harness files.
	ToolAgentsPluginName = "tool-agents"
)

// PluginExecutor executes the tool-agents plugin.
type PluginExecutor interface {
	ExecuteAgentPlugin(ctx context.Context, pluginName string, args []string, streams sdk.OperationIO, allowDownload bool) error
}

// BootstrapProvider returns an already-initialized PluginBootstrap when available.
type BootstrapProvider func() *bootstrap.PluginBootstrap

// BootstrapExecutor runs tool-agents via DevEx's plugin bootstrap.
type BootstrapExecutor struct {
	GetBootstrap BootstrapProvider
}

// ExecuteAgentPlugin invokes pluginName via the plugin bootstrap, ensuring it is
// downloaded when allowDownload is true and offline/skip-plugin-download are not set.
func (e BootstrapExecutor) ExecuteAgentPlugin(ctx context.Context, pluginName string, args []string, streams sdk.OperationIO, allowDownload bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var b *bootstrap.PluginBootstrap
	if e.GetBootstrap != nil {
		b = e.GetBootstrap()
	}
	if b == nil {
		var err error
		b, err = bootstrap.NewPluginBootstrap(!allowDownload || viper.GetBool("skip-plugin-download") || viper.GetBool("offline"))
		if err != nil {
			return fmt.Errorf("initialize plugin bootstrap: %w", err)
		}
		if err := b.Initialize(ctx); err != nil {
			return fmt.Errorf("discover plugins: %w", err)
		}
	}
	if allowDownload && !viper.GetBool("skip-plugin-download") && !viper.GetBool("offline") {
		if _, err := b.EnsurePluginWithContext(ctx, pluginName); err != nil {
			return fmt.Errorf("ensure %s plugin: %w", pluginName, err)
		}
	}
	if err := b.ExecutePluginWithIO(ctx, pluginName, args, streams); err != nil {
		if !allowDownload && strings.Contains(strings.ToLower(err.Error()), "not found") {
			return fmt.Errorf("%s plugin is not installed; install it with 'devex plugin install %s' or run 'devex agent apply' without --dry-run: %w", pluginName, pluginName, err)
		}
		return err
	}
	return nil
}

// ResolvedPluginSpec is the JSON contract passed to tool-agents apply/diff.
type ResolvedPluginSpec struct {
	Agents []ResolvedAgentSpec `json:"agents"`
}

// ResolvedAgentSpec describes one fully resolved agent configuration for tool-agents.
type ResolvedAgentSpec struct {
	Name            string                  `json:"name"`
	Preset          string                  `json:"preset,omitempty"`
	Instructions    string                  `json:"instructions,omitempty"`
	Model           string                  `json:"model,omitempty"`
	AllowedCommands []string                `json:"allowed_commands,omitempty"`
	DeniedCommands  []string                `json:"denied_commands,omitempty"`
	MCPServers      []ResolvedMCPServerSpec `json:"mcp_servers,omitempty"`
}

// ResolvedMCPServerSpec describes one MCP server entry for tool-agents.
type ResolvedMCPServerSpec struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// catalogAppHarnessIDs is the single explicit mapping from catalog application
// names (lowercased) to declarative harness IDs supported by tool-agents.
var catalogAppHarnessIDs = map[string]string{
	"claude code": config.AgentClaude,
	"codex cli":   config.AgentCodex,
	"gemini cli":  config.AgentGemini,
	"opencode":    config.AgentOpenCode,
}

// templateSlugHarnessIDs maps template/CLI slugs to declarative harness IDs.
var templateSlugHarnessIDs = map[string]string{
	"claude":      config.AgentClaude,
	"claude-code": config.AgentClaude,
	"codex":       config.AgentCodex,
	"codex-cli":   config.AgentCodex,
	"gemini":      config.AgentGemini,
	"gemini-cli":  config.AgentGemini,
	"opencode":    config.AgentOpenCode,
}

// HarnessIDForCatalogApp maps a catalog application name (case-insensitive) to
// its declarative harness ID in agents.yaml.
func HarnessIDForCatalogApp(appName string) (string, bool) {
	id, ok := catalogAppHarnessIDs[strings.ToLower(strings.TrimSpace(appName))]
	return id, ok
}

// HarnessIDForNameOrSlug maps a catalog name, template slug, or harness ID
// (case-insensitive) to its declarative harness ID in agents.yaml.
func HarnessIDForNameOrSlug(value string) (string, bool) {
	needle := strings.ToLower(strings.TrimSpace(value))
	if id, ok := catalogAppHarnessIDs[needle]; ok {
		return id, true
	}
	id, ok := templateSlugHarnessIDs[needle]
	return id, ok
}

// CanonicalHarnessIDs deduplicates ids and orders them in config.SupportedAgents() order.
func CanonicalHarnessIDs(ids []string) []string {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		if config.IsSupportedAgent(id) {
			want[id] = true
		}
	}
	ordered := make([]string, 0, len(want))
	for _, id := range config.SupportedAgents() {
		if want[id] {
			ordered = append(ordered, id)
		}
	}
	return ordered
}

// RenderMinimalAgentsYAML encodes a minimal agents.yaml containing only enabled
// harness IDs and, when non-empty, preset.
func RenderMinimalAgentsYAML(enabled []string, preset string) ([]byte, error) {
	canonical := CanonicalHarnessIDs(enabled)
	if len(canonical) == 0 {
		return nil, fmt.Errorf("at least one supported harness agent is required")
	}
	preset = strings.TrimSpace(preset)
	if err := config.ValidateAgentPreset(preset); err != nil {
		return nil, err
	}
	type minimalAgents struct {
		Enabled []string `yaml:"enabled"`
		Preset  string   `yaml:"preset,omitempty"`
	}
	doc := struct {
		Agents minimalAgents `yaml:"agents"`
	}{
		Agents: minimalAgents{
			Enabled: canonical,
			Preset:  preset,
		},
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal agents.yaml: %w", err)
	}
	return data, nil
}

// WriteDefaultAgentsYAMLIfMissing writes a minimal agents.yaml in userConfigDir
// only when the file does not already exist. When the file already exists, it is
// left untouched and created is false.
func WriteDefaultAgentsYAMLIfMissing(userConfigDir string, enabled []string, preset string) (string, bool, error) {
	agentsPath := filepath.Join(userConfigDir, "agents.yaml")
	if _, err := os.Stat(agentsPath); err == nil {
		return agentsPath, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return agentsPath, false, fmt.Errorf("inspect %s: %w", agentsPath, err)
	}

	data, err := RenderMinimalAgentsYAML(enabled, preset)
	if err != nil {
		return agentsPath, false, err
	}
	if err := os.MkdirAll(userConfigDir, 0o750); err != nil {
		return agentsPath, false, fmt.Errorf("create config directory %s: %w", userConfigDir, err)
	}
	file, err := os.OpenFile(agentsPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return agentsPath, false, nil
	}
	if err != nil {
		return agentsPath, false, fmt.Errorf("create %s: %w", agentsPath, err)
	}
	written := false
	defer func() {
		if !written {
			_ = os.Remove(agentsPath)
		}
	}()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return agentsPath, false, fmt.Errorf("write %s: %w", agentsPath, err)
	}
	if err := file.Close(); err != nil {
		return agentsPath, false, fmt.Errorf("close %s: %w", agentsPath, err)
	}
	written = true
	return agentsPath, true, nil
}

// BuildResolvedSpec expands declarative AgentsConfig into the per-agent JSON
// spec consumed by the tool-agents plugin.
func BuildResolvedSpec(
	cfg config.AgentsConfig,
	targetAgents []string,
	presetOverride string,
	lookPath func(string) (string, error),
	executable func() (string, error),
) ResolvedPluginSpec {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if executable == nil {
		executable = os.Executable
	}
	out := ResolvedPluginSpec{
		Agents: make([]ResolvedAgentSpec, 0, len(targetAgents)),
	}
	devexCmd := ""
	if cfg.DevExMCP {
		devexCmd = ResolveDevExMCPCommand(lookPath, executable)
	}
	for _, name := range targetAgents {
		override := cfg.Overrides[name]
		preset := cfg.Preset
		if override.Preset != "" {
			preset = override.Preset
		}
		if presetOverride != "" {
			preset = presetOverride
		}
		var mcpServers []ResolvedMCPServerSpec
		for _, srv := range cfg.MCPServers {
			if len(srv.Agents) > 0 && !slices.Contains(srv.Agents, name) {
				continue
			}
			mcpServers = append(mcpServers, ResolvedMCPServerSpec{
				Name:      srv.Name,
				Transport: srv.Transport,
				Command:   srv.Command,
				Args:      slices.Clone(srv.Args),
				Env:       maps.Clone(srv.Env),
				URL:       srv.URL,
				Headers:   maps.Clone(srv.Headers),
			})
		}
		if cfg.DevExMCP {
			mcpServers = append(mcpServers, ResolvedMCPServerSpec{
				Name:      "devex",
				Transport: config.MCPTransportStdio,
				Command:   devexCmd,
				Args:      []string{"mcp", "serve"},
			})
		}
		out.Agents = append(out.Agents, ResolvedAgentSpec{
			Name:            name,
			Preset:          preset,
			Instructions:    cfg.Instructions,
			Model:           override.Model,
			AllowedCommands: config.MergeAgentCommands(cfg.AllowedCommands, override.AllowedCommands),
			DeniedCommands:  config.MergeAgentCommands(cfg.DeniedCommands, override.DeniedCommands),
			MCPServers:      mcpServers,
		})
	}
	return out
}

// Execute serializes spec to a temporary 0600 JSON file and invokes tool-agents
// apply (or diff when diffOnly is true).
func Execute(ctx context.Context, executor PluginExecutor, spec ResolvedPluginSpec, streams sdk.OperationIO, diffOnly bool) error {
	specBytes, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("encode resolved agents spec: %w", err)
	}
	specBytes = append(specBytes, '\n')

	tmpFile, err := os.CreateTemp("", "devex-agents-spec-*.json")
	if err != nil {
		return fmt.Errorf("create temporary agents spec: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("protect temporary agents spec: %w", err)
	}
	if _, err := tmpFile.Write(specBytes); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temporary agents spec: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temporary agents spec: %w", err)
	}

	pluginArgs := []string{"apply", "--config", tmpPath}
	allowDownload := true
	if diffOnly {
		pluginArgs = []string{"diff", "--config", tmpPath}
		allowDownload = false
	}

	var stderrBuf bytes.Buffer
	stderrWriter := streams.Stderr
	if stderrWriter == nil {
		stderrWriter = io.Discard
	}
	streams.Stderr = io.MultiWriter(stderrWriter, &stderrBuf)

	if err := executor.ExecuteAgentPlugin(ctx, ToolAgentsPluginName, pluginArgs, streams, allowDownload); err != nil {
		if hasMCPServersConfigured(spec) && isUnknownMCPFieldError(err, stderrBuf.String()) {
			return fmt.Errorf("installed %s plugin does not support mcp_servers; update it with 'devex plugin update %s': %w", ToolAgentsPluginName, ToolAgentsPluginName, err)
		}
		return err
	}
	return nil
}

// ResolveDevExMCPCommand returns "devex" when PATH resolves to the current binary,
// or the absolute path of the current binary otherwise.
func ResolveDevExMCPCommand(lookPath func(string) (string, error), executable func() (string, error)) string {
	exePath, err := executable()
	if err != nil || strings.TrimSpace(exePath) == "" {
		return "devex"
	}
	if abs, absErr := filepath.Abs(exePath); absErr == nil {
		exePath = abs
	}
	if lookedUp, err := lookPath("devex"); err == nil && sameExecutableFile(lookedUp, exePath) {
		return "devex"
	}
	return exePath
}

func sameExecutableFile(a, b string) bool {
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	if a == b {
		return true
	}
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

func hasMCPServersConfigured(spec ResolvedPluginSpec) bool {
	for _, a := range spec.Agents {
		if len(a.MCPServers) > 0 {
			return true
		}
	}
	return false
}

func isUnknownMCPFieldError(err error, stderr string) bool {
	combined := strings.ToLower(err.Error() + "\n" + stderr)
	return strings.Contains(combined, "unknown field") && strings.Contains(combined, "mcp_servers")
}

// EnsureUserBinDirsOnPath appends the user-scope binary directories that agent
// installers write to (~/.local/bin and mise shims) to PATH when they exist and
// are missing, so a just-installed agent is found before the shell reloads.
func EnsureUserBinDirsOnPath(home string) {
	if strings.TrimSpace(home) == "" {
		return
	}
	miseData := os.Getenv("MISE_DATA_DIR")
	if miseData == "" {
		miseData = filepath.Join(home, ".local", "share", "mise")
	}
	path := os.Getenv("PATH")
	entries := filepath.SplitList(path)
	for _, dir := range []string{filepath.Join(home, ".local", "bin"), filepath.Join(miseData, "shims")} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() || slices.Contains(entries, dir) { //nolint:gosec // only checks that a PATH candidate directory exists
			continue
		}
		entries = append(entries, dir)
	}
	_ = os.Setenv("PATH", strings.Join(entries, string(os.PathListSeparator)))
}

// SkippedAgents returns the "skipped (<reason>): <agent>" lines tool-agents
// prints for agents it did not configure.
func SkippedAgents(output string) []string {
	var skipped []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "skipped (") {
			skipped = append(skipped, line)
		}
	}
	return skipped
}
