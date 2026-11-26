package agentdoctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
)

const (
	// commandTimeout bounds each agent version or status subprocess invocation.
	commandTimeout   = 5 * time.Second
	maxVersionLength = 120
)

// LoginState summarizes an agent's observed authentication state.
type LoginState string

const (
	LoginStateLoggedIn    LoginState = "logged_in"
	LoginStateNotLoggedIn LoginState = "not_logged_in"
	LoginStateUnknown     LoginState = "unknown"
)

// Probe injects every host-inspecting operation so Check stays pure and
// testable without reading real credentials or executing real host binaries.
type Probe struct {
	Getenv       func(key string) string
	Stat         func(path string) (os.FileInfo, error)
	ReadFile     func(path string) ([]byte, error)
	Run          func(ctx context.Context, name string, args ...string) ([]byte, error)
	EvalSymlinks func(path string) (string, error)
	UserHomeDir  func() (string, error)
	GOOS         string
	Now          func() time.Time
	SandboxProbe agentsandbox.Probe
}

// DefaultProbe returns a Probe wired to the local operating system.
func DefaultProbe() Probe {
	return Probe{
		Getenv: os.Getenv,
		Stat:   os.Stat,
		// ReadFile is only ever invoked on MCP config files, never credential files.
		ReadFile: os.ReadFile,
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
			defer cancel()
			cmd := exec.CommandContext(cmdCtx, name, args...)
			// Stop waiting for output pipes held open by grandchildren after kill.
			cmd.WaitDelay = time.Second
			return cmd.CombinedOutput()
		},
		EvalSymlinks: filepath.EvalSymlinks,
		UserHomeDir:  os.UserHomeDir,
		GOOS:         runtime.GOOS,
		Now:          time.Now,
		SandboxProbe: agentsandbox.DefaultProbe(),
	}
}

func (p Probe) withDefaults() Probe {
	if p.GOOS == "" {
		p.GOOS = runtime.GOOS
	}
	if p.Now == nil {
		p.Now = time.Now
	}
	if p.Getenv == nil {
		p.Getenv = func(string) string { return "" }
	}
	if p.Stat == nil {
		p.Stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	}
	if p.ReadFile == nil {
		p.ReadFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	}
	if p.Run == nil {
		p.Run = func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("probe runner not configured")
		}
	}
	if p.EvalSymlinks == nil {
		p.EvalSymlinks = func(path string) (string, error) { return path, nil }
	}
	return p
}

func (p Probe) homeDir() string {
	if h := strings.TrimSpace(p.Getenv("HOME")); h != "" {
		return h
	}
	if p.GOOS == "windows" {
		if h := strings.TrimSpace(p.Getenv("USERPROFILE")); h != "" {
			return h
		}
	}
	if p.UserHomeDir != nil {
		if h, err := p.UserHomeDir(); err == nil && strings.TrimSpace(h) != "" {
			return h
		}
	}
	return ""
}

// LoginRunner executes an agent's interactive login command with terminal I/O attached.
type LoginRunner func(ctx context.Context, binaryPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) error

// DefaultLoginRunner hands off execution directly to the agent binary,
// inheriting the current environment and attaching stdin/stdout/stderr.
func DefaultLoginRunner() LoginRunner {
	return func(ctx context.Context, binaryPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		cmd := exec.CommandContext(ctx, binaryPath, args...)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	}
}

// SandboxSummary captures sandbox prerequisite status for an agent that
// declares linux.sandbox_prerequisites in the DevEx catalog.
type SandboxSummary struct {
	Status        agentsandbox.Status `json:"status"`
	Prerequisites []string            `json:"prerequisites"`
	BwrapPath     string              `json:"bwrap_path,omitempty"`
	SocatPath     string              `json:"socat_path,omitempty"`
	BwrapWorks    bool                `json:"bwrap_works"`
}

// Problem describes a detected issue with an actionable fix command or step.
type Problem struct {
	Code    string `json:"code"`
	Agent   string `json:"agent"`
	Summary string `json:"summary"`
	Fix     string `json:"fix"`
}

// AgentReport holds the diagnostic findings for a single known agent.
type AgentReport struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	CatalogFile   string            `json:"catalog_file"`
	Binary        string            `json:"binary"`
	Installed     bool              `json:"installed"`
	Version       string            `json:"version,omitempty"`
	ResolvedPath  string            `json:"resolved_path,omitempty"`
	PathMatches   []PathMatch       `json:"path_matches,omitempty"`
	Shadowed      bool              `json:"shadowed,omitempty"`
	ShadowedByNPM bool              `json:"shadowed_by_npm,omitempty"`
	LoginState    LoginState        `json:"login_state,omitempty"`
	LoginDetail   string            `json:"login_detail,omitempty"`
	EnvVarsSet    []string          `json:"env_vars_set,omitempty"`
	Sandbox       *SandboxSummary   `json:"sandbox,omitempty"`
	MCPConfigs    []MCPConfigResult `json:"mcp_configs,omitempty"`
	Node          *NodeCheck        `json:"node,omitempty"`
	Problems      []Problem         `json:"problems,omitempty"`
}

// Report is the complete diagnostic output of devex agent doctor.
type Report struct {
	CheckedAt time.Time     `json:"checked_at"`
	GOOS      string        `json:"goos"`
	Agents    []AgentReport `json:"agents"`
	Problems  []Problem     `json:"problems"`
}

// Check runs diagnostics for agents (or all KnownAgents when agents is empty)
// using probe. Sandbox checks and Node version checks run at most once per call.
func Check(ctx context.Context, probe Probe, agents []AgentSpec) Report {
	probe = probe.withDefaults()
	if len(agents) == 0 {
		agents = KnownAgents()
	}
	secrets := CollectSecretValues(probe.Getenv)

	report := Report{
		CheckedAt: probe.Now().UTC(),
		GOOS:      probe.GOOS,
		Agents:    make([]AgentReport, 0, len(agents)),
		Problems:  []Problem{},
	}

	var (
		sandboxChecked bool
		sandboxReport  agentsandbox.Report
		nodeCache      nodeProbeCache
	)
	needsSocat := anyAgentRequiresSocat(agents)

	for _, spec := range agents {
		ar := checkAgent(ctx, probe, spec, secrets, &nodeCache, func() agentsandbox.Report {
			if !sandboxChecked {
				sp := probe.SandboxProbe
				if sp.GOOS == "" {
					sp.GOOS = probe.GOOS
				}
				if !needsSocat {
					sp.SocatNotRequired = true
				}
				sandboxReport = agentsandbox.Check(ctx, sp)
				sandboxChecked = true
			}
			return sandboxReport
		})
		report.Agents = append(report.Agents, ar)
		report.Problems = append(report.Problems, ar.Problems...)
	}

	return report
}

func anyAgentRequiresSocat(agents []AgentSpec) bool {
	for _, spec := range agents {
		for _, p := range spec.SandboxPrerequisites {
			if strings.EqualFold(p, "socat") {
				return true
			}
		}
	}
	return false
}

func agentRequiresSocat(spec AgentSpec) bool {
	for _, p := range spec.SandboxPrerequisites {
		if strings.EqualFold(p, "socat") {
			return true
		}
	}
	return false
}

func checkAgent(
	ctx context.Context,
	probe Probe,
	spec AgentSpec,
	secrets []string,
	nodeCache *nodeProbeCache,
	getSandbox func() agentsandbox.Report,
) AgentReport {
	ar := AgentReport{
		ID:          spec.ID,
		Name:        spec.Name,
		CatalogFile: spec.CatalogFile,
		Binary:      spec.Binary,
	}

	matches := FindBinariesOnPATH(probe, spec.Binary)
	if len(matches) == 0 {
		return ar
	}

	ar.Installed = true
	ar.ResolvedPath = matches[0].Path
	ar.PathMatches = matches

	// 1. Version check (first line, trimmed, length-capped, redacted).
	if len(spec.VersionArgs) > 0 {
		cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
		out, err := probe.Run(cmdCtx, ar.ResolvedPath, spec.VersionArgs...)
		cancel()
		if err == nil {
			ar.Version = Redact(firstLineTrimmed(string(out), maxVersionLength), secrets)
		}
	}

	// 2. Duplicate / PATH shadowing analysis.
	if len(matches) > 1 {
		ar.Shadowed = true
		ar.Problems = append(ar.Problems, analyzePathShadowing(spec, matches, &ar.ShadowedByNPM)...)
	}

	// 3. Relevant API-key env var NAMES (never read or record values).
	for _, envName := range spec.APIKeyEnvVars {
		if strings.TrimSpace(probe.Getenv(envName)) != "" {
			ar.EnvVarsSet = append(ar.EnvVarsSet, envName)
		}
	}

	// 4. Login state (never read credential files; never keep login-status output).
	checkLogin(ctx, probe, spec, &ar)

	// 5. Node.js version check where relevant.
	if spec.MinNodeVersion != "" && (spec.NodeRequired || matches[0].IsNPMGlobal) {
		nc := evaluateNodeCheck(ctx, probe, nodeCache, spec.MinNodeVersion)
		ar.Node = &nc
		if !nc.OK {
			rec := recommendedMiseNodeSpec(spec.MinNodeVersion)
			summary := fmt.Sprintf("Node.js >= %s is required but node was not found on PATH", spec.MinNodeVersion)
			if nc.Detected != "" {
				summary = fmt.Sprintf("Node.js %s is older than the required >= %s", nc.Detected, spec.MinNodeVersion)
			}
			ar.Problems = append(ar.Problems, Problem{
				Code:    "node_version",
				Agent:   spec.ID,
				Summary: summary,
				Fix:     fmt.Sprintf("mise use -g node@%s", rec),
			})
		}
	}

	// 6. Sandbox prerequisites for agents that declare them in the catalog.
	if len(spec.SandboxPrerequisites) > 0 {
		sb := getSandbox()
		status := sb.Status
		// If this agent does not require socat (e.g. codex, copilot) and bwrap
		// itself works, a missing socat is not a problem for this agent.
		if !agentRequiresSocat(spec) && status == agentsandbox.StatusMissing && sb.BwrapWorks && sb.SocatPath == "" {
			status = agentsandbox.StatusOK
		}
		ar.Sandbox = &SandboxSummary{
			Status:        status,
			Prerequisites: append([]string(nil), spec.SandboxPrerequisites...),
			BwrapPath:     sb.BwrapPath,
			SocatPath:     sb.SocatPath,
			BwrapWorks:    sb.BwrapWorks,
		}
		if status != agentsandbox.StatusOK && status != agentsandbox.StatusUnsupported {
			ar.Problems = append(ar.Problems, Problem{
				Code:    "sandbox",
				Agent:   spec.ID,
				Summary: fmt.Sprintf("Linux sandbox status is %s", status),
				Fix:     `Run "devex agent sandbox" for manual remediation commands`,
			})
		}
	}

	// 7. MCP configuration validation (only for existing config files).
	for _, mcpSpec := range spec.MCPConfigs {
		resolved := mcpSpec.Path.Resolve(probe)
		if resolved == "" {
			continue
		}
		info, err := probe.Stat(resolved)
		if err != nil || (info != nil && info.IsDir()) {
			continue
		}
		res := validateMCPConfig(probe, mcpSpec, resolved)
		ar.MCPConfigs = append(ar.MCPConfigs, res)
		if res.Status == MCPStatusInvalid {
			ar.Problems = append(ar.Problems, Problem{
				Code:    "mcp_config",
				Agent:   spec.ID,
				Summary: fmt.Sprintf("MCP config %s is %s", res.Path, res.Error),
				Fix:     fmt.Sprintf("Fix syntax errors in %s", res.Path),
			})
		}
	}

	return ar
}

func analyzePathShadowing(spec AgentSpec, matches []PathMatch, shadowedByNPM *bool) []Problem {
	first := matches[0]
	var problems []Problem

	if first.IsNPMGlobal {
		var native *PathMatch
		for i := 1; i < len(matches); i++ {
			if !matches[i].IsNPMGlobal {
				native = &matches[i]
				break
			}
		}
		if native != nil {
			*shadowedByNPM = true
			nativeDir := filepath.Dir(native.Path)
			npmDir := filepath.Dir(first.Path)
			fix := fmt.Sprintf("Move %s ahead of %s in PATH", nativeDir, npmDir)
			if spec.NPMPackage != "" {
				fix = fmt.Sprintf("npm uninstall -g %s (or move %s ahead of %s in PATH)", spec.NPMPackage, nativeDir, npmDir)
			}
			problems = append(problems, Problem{
				Code:    "npm_shadowing",
				Agent:   spec.ID,
				Summary: fmt.Sprintf("npm global install at %s shadows %s", first.Path, native.Path),
				Fix:     fix,
			})
			return problems
		}
	}

	// Non-npm shadowing or duplicate installs.
	second := matches[1]
	firstDir := filepath.Dir(first.Path)
	secondDir := filepath.Dir(second.Path)
	if second.IsNPMGlobal && spec.NPMPackage != "" {
		problems = append(problems, Problem{
			Code:    "duplicate_install",
			Agent:   spec.ID,
			Summary: fmt.Sprintf("duplicate %s binaries found (%s and npm global %s)", spec.Binary, first.Path, second.Path),
			Fix:     fmt.Sprintf("npm uninstall -g %s", spec.NPMPackage),
		})
		return problems
	}
	problems = append(problems, Problem{
		Code:    "path_order",
		Agent:   spec.ID,
		Summary: fmt.Sprintf("multiple %s binaries on PATH: %s shadows %s", spec.Binary, first.Path, second.Path),
		Fix:     fmt.Sprintf("Move %s ahead of %s in PATH (or remove the unused copy)", secondDir, firstDir),
	})
	return problems
}

func checkLogin(ctx context.Context, probe Probe, spec AgentSpec, ar *AgentReport) {
	if len(spec.LoginStatusArgs) > 0 {
		cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
		// Deliberately discard stdout/stderr — only inspect exit status.
		_, err := probe.Run(cmdCtx, ar.ResolvedPath, spec.LoginStatusArgs...)
		// A killed subprocess reports "signal: killed", not DeadlineExceeded,
		// so the context decides whether the status command timed out.
		timedOut := cmdCtx.Err() != nil
		cancel()
		cmdDesc := spec.Binary + " " + strings.Join(spec.LoginStatusArgs, " ")
		switch {
		case err == nil:
			ar.LoginState = LoginStateLoggedIn
			ar.LoginDetail = cmdDesc + " exited 0"
		case timedOut || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
			ar.LoginState = LoginStateUnknown
			ar.LoginDetail = cmdDesc + " timed out"
		default:
			ar.LoginState = LoginStateNotLoggedIn
			ar.LoginDetail = cmdDesc + " exited non-zero"
			ar.Problems = append(ar.Problems, notLoggedInProblem(spec))
		}
		return
	}

	// Check documented credential files via Stat only (NEVER ReadFile).
	for _, tpl := range spec.CredentialPaths {
		resolved := tpl.Resolve(probe)
		if resolved == "" {
			continue
		}
		info, err := probe.Stat(resolved)
		if err == nil && (info == nil || !info.IsDir()) {
			ar.LoginState = LoginStateLoggedIn
			ar.LoginDetail = fmt.Sprintf("credential file present (%s)", resolved)
			return
		}
	}

	// Check whether any known API-key env var NAME is set.
	if len(ar.EnvVarsSet) > 0 {
		ar.LoginState = LoginStateLoggedIn
		ar.LoginDetail = fmt.Sprintf("env var %s set", strings.Join(ar.EnvVarsSet, ", "))
		return
	}

	// File-backed agents without OS keyring fallback can be reported not_logged_in.
	if len(spec.CredentialPaths) > 0 && !spec.UsesKeyring {
		ar.LoginState = LoginStateNotLoggedIn
		ar.LoginDetail = "no credential file or API key env var found"
		ar.Problems = append(ar.Problems, notLoggedInProblem(spec))
		return
	}

	ar.LoginState = LoginStateUnknown
	if spec.UsesKeyring {
		ar.LoginDetail = "credentials may be in OS keyring; no token env var or file found"
	} else {
		ar.LoginDetail = "no status command or credential signal"
	}
}

func notLoggedInProblem(spec AgentSpec) Problem {
	fix := spec.NoLoginGuidance
	if len(spec.LoginArgs) > 0 {
		fix = fmt.Sprintf("devex agent login %s", spec.ID)
	} else if fix == "" {
		fix = fmt.Sprintf("Authenticate %s using the vendor CLI or export a provider API key", spec.Name)
	}
	return Problem{
		Code:    "not_logged_in",
		Agent:   spec.ID,
		Summary: fmt.Sprintf("%s is not logged in (or session expired)", spec.Name),
		Fix:     fix,
	}
}

func firstLineTrimmed(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if maxLen > 0 && len(trimmed) > maxLen {
			return trimmed[:maxLen]
		}
		return trimmed
	}
	return ""
}

// FormatJSON renders Report as indented JSON with defense-in-depth secret redaction.
func FormatJSON(r Report, secrets []string) ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal agent doctor report: %w", err)
	}
	redacted := Redact(string(data), secrets) + "\n"
	return []byte(redacted), nil
}

// FormatTable renders Report as a compact human-readable diagnostic summary
// with defense-in-depth secret redaction.
func FormatTable(r Report, secrets []string) string {
	var b strings.Builder
	b.WriteString("Agent Doctor\n")
	for _, ag := range r.Agents {
		if !ag.Installed {
			fmt.Fprintf(&b, "  %-10s not installed\n", ag.ID)
			continue
		}
		ver := ag.Version
		if ver == "" {
			ver = "unknown version"
		}
		fmt.Fprintf(&b, "  %-10s installed (%s)\n", ag.ID, ver)
		fmt.Fprintf(&b, "    %-14s %s\n", "path:", ag.ResolvedPath)
		if len(ag.PathMatches) > 1 {
			extras := make([]string, 0, len(ag.PathMatches))
			for _, m := range ag.PathMatches {
				label := m.Path
				if m.IsNPMGlobal {
					label += " [npm-global]"
				}
				extras = append(extras, label)
			}
			fmt.Fprintf(&b, "    %-14s %s\n", "all matches:", strings.Join(extras, ", "))
		}
		if ag.LoginDetail != "" {
			fmt.Fprintf(&b, "    %-14s %s (%s)\n", "login:", ag.LoginState, ag.LoginDetail)
		} else {
			fmt.Fprintf(&b, "    %-14s %s\n", "login:", ag.LoginState)
		}
		if len(ag.EnvVarsSet) > 0 {
			fmt.Fprintf(&b, "    %-14s %s\n", "env vars set:", strings.Join(ag.EnvVarsSet, ", "))
		}
		if ag.Node != nil {
			nodeState := "ok"
			if !ag.Node.OK {
				nodeState = "missing or too old"
			}
			detected := ag.Node.Detected
			if detected == "" {
				detected = "not found"
			}
			fmt.Fprintf(&b, "    %-14s %s (detected %s, requires >= %s)\n", "node:", nodeState, detected, ag.Node.Required)
		}
		if ag.Sandbox != nil {
			fmt.Fprintf(&b, "    %-14s %s\n", "sandbox:", ag.Sandbox.Status)
		}
		for _, mcp := range ag.MCPConfigs {
			if mcp.Status == MCPStatusOK {
				fmt.Fprintf(&b, "    %-14s %s (ok)\n", "mcp config:", mcp.Path)
			} else {
				fmt.Fprintf(&b, "    %-14s %s (%s)\n", "mcp config:", mcp.Path, mcp.Error)
			}
		}
	}

	if len(r.Problems) == 0 {
		b.WriteString("\nNo problems detected.\n")
	} else {
		fmt.Fprintf(&b, "\nProblems (%d):\n", len(r.Problems))
		for i, p := range r.Problems {
			fmt.Fprintf(&b, "  %d. [%s] %s\n     Fix: %s\n", i+1, p.Agent, p.Summary, p.Fix)
		}
	}

	return Redact(b.String(), secrets)
}
