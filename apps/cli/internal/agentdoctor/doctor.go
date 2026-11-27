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
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	"github.com/jameswlane/devex/apps/cli/internal/localmodels"
	"github.com/jameswlane/devex/apps/cli/internal/skillpacks"
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
	Getenv                   func(key string) string
	Stat                     func(path string) (os.FileInfo, error)
	ReadFile                 func(path string) ([]byte, error)
	Run                      func(ctx context.Context, name string, args ...string) ([]byte, error)
	EvalSymlinks             func(path string) (string, error)
	UserHomeDir              func() (string, error)
	Getwd                    func() (string, error)
	GOOS                     string
	Now                      func() time.Time
	SandboxProbe             agentsandbox.Probe
	LocalModelsProbe         *localmodels.Probe
	SkillPackUserLockPath    string
	SkillPackProjectLockPath string
	SkillPackCatalog         *skillpacks.Catalog
}

// DefaultProbe returns a Probe wired to the local operating system.
func DefaultProbe() Probe {
	lmProbe := localmodels.DefaultProbe()
	return Probe{
		Getenv: os.Getenv,
		Stat:   os.Stat,
		// ReadFile is only ever invoked on MCP config files and skill pack lock files, never credential files.
		ReadFile: os.ReadFile,
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
			defer cancel()
			cmd := exec.CommandContext(cmdCtx, name, args...)
			// Stop waiting for output pipes held open by grandchildren after kill.
			cmd.WaitDelay = time.Second
			// Stdout only: stderr is discarded so diagnostics never echo it.
			return cmd.Output()
		},
		EvalSymlinks:     filepath.EvalSymlinks,
		UserHomeDir:      os.UserHomeDir,
		Getwd:            os.Getwd,
		GOOS:             runtime.GOOS,
		Now:              time.Now,
		SandboxProbe:     agentsandbox.DefaultProbe(),
		LocalModelsProbe: &lmProbe,
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
	CheckedAt         time.Time                  `json:"checked_at"`
	GOOS              string                     `json:"goos"`
	Agents            []AgentReport              `json:"agents"`
	LocalModelRuntime *localmodels.RuntimeStatus `json:"local_model_runtime,omitempty"`
	SkillPacks        []SkillPackReport          `json:"skill_packs,omitempty"`
	Problems          []Problem                  `json:"problems"`
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

	var lmProbe localmodels.Probe
	if probe.LocalModelsProbe != nil {
		lmProbe = *probe.LocalModelsProbe
	}
	if lmProbe.GOOS == "" {
		lmProbe.GOOS = probe.GOOS
	}
	rt := localmodels.InspectRuntime(ctx, lmProbe)
	report.LocalModelRuntime = &rt

	report.SkillPacks = checkSkillPacks(probe, agents)

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
			ar.Version = versionToken(Redact(string(out), secrets))
		}
	}

	// 2. Duplicate / PATH shadowing analysis.
	if len(matches) > 1 {
		ar.Shadowed = true
		ar.Problems = append(ar.Problems, analyzePathShadowing(spec, matches, &ar.ShadowedByNPM)...)
	}

	// 3. Relevant API-key env var NAMES (never read or record values).
	for _, envName := range append(append([]string(nil), spec.APIKeyEnvVars...), spec.RedactOnlyEnvVars...) {
		if strings.TrimSpace(probe.Getenv(envName)) != "" {
			ar.EnvVarsSet = append(ar.EnvVarsSet, envName)
		}
	}

	// 4. Login state (never read credential files; never keep login-status output).
	checkLogin(ctx, probe, spec, &ar)

	// 5. Node.js version check where relevant.
	if spec.MinNodeVersion != "" && (spec.NodeRequired || matches[0].IsNPMGlobal) {
		nodeCache.secrets = secrets
		nc := evaluateNodeCheck(ctx, probe, nodeCache, spec.MinNodeVersion)
		ar.Node = &nc
		if !nc.OK {
			rec := recommendedMiseNodeSpec(spec.MinNodeVersion)
			summary := fmt.Sprintf("Node.js >= %s is required but node was not found on PATH", spec.MinNodeVersion)
			fix := fmt.Sprintf("mise use -g node@%s", rec)
			switch {
			case nc.Detected != "":
				summary = fmt.Sprintf("Node.js %s is older than the required >= %s", nc.Detected, spec.MinNodeVersion)
			case nc.Path != "":
				summary = fmt.Sprintf("could not determine the version of %s (Node.js >= %s is required)", nc.Path, spec.MinNodeVersion)
				fix = fmt.Sprintf("Check that %s --version runs, or run: mise use -g node@%s", nc.Path, rec)
			}
			ar.Problems = append(ar.Problems, Problem{
				Code:    "node_version",
				Agent:   spec.ID,
				Summary: summary,
				Fix:     fix,
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
		if errors.Is(err, os.ErrNotExist) || (err == nil && info != nil && info.IsDir()) {
			continue
		}
		var res MCPConfigResult
		switch {
		case err != nil:
			res = MCPConfigResult{Path: resolved, Format: mcpSpec.Format, Status: MCPStatusInvalid, Error: "invalid: unreadable file"}
		case info != nil && !info.Mode().IsRegular():
			// Reading a FIFO or device could block the doctor.
			res = MCPConfigResult{Path: resolved, Format: mcpSpec.Format, Status: MCPStatusInvalid, Error: "invalid: not a regular file"}
		default:
			res = validateMCPConfig(probe, mcpSpec, resolved)
		}
		ar.MCPConfigs = append(ar.MCPConfigs, res)
		if res.Status == MCPStatusInvalid {
			ar.Problems = append(ar.Problems, Problem{
				Code:    "mcp_config",
				Agent:   spec.ID,
				Summary: fmt.Sprintf("MCP config %s is %s", res.Path, res.Error),
				Fix:     mcpFix(res),
			})
		}
	}

	return ar
}

func mcpFix(res MCPConfigResult) string {
	switch res.Error {
	case "invalid: unreadable file":
		return fmt.Sprintf("Check that %s exists and is readable by your user", res.Path)
	case "invalid: not a regular file":
		return fmt.Sprintf("Replace %s with a regular file", res.Path)
	}
	return fmt.Sprintf("Fix syntax errors in %s", res.Path)
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
		case !isExitError(err):
			// The command never ran to completion, so it said nothing about login.
			ar.LoginState = LoginStateUnknown
			ar.LoginDetail = cmdDesc + " could not run"
		default:
			ar.LoginState = LoginStateNotLoggedIn
			ar.LoginDetail = cmdDesc + " exited non-zero"
			ar.Problems = append(ar.Problems, notLoggedInProblem(spec))
		}
		return
	}

	// Check documented credential files via Stat only (NEVER ReadFile).
	credentialUninspectable := false
	for _, tpl := range spec.CredentialPaths {
		resolved := tpl.Resolve(probe)
		if resolved == "" {
			continue
		}
		info, err := probe.Stat(resolved)
		if err == nil && (info == nil || info.Mode().IsRegular()) {
			ar.LoginState = LoginStateLoggedIn
			ar.LoginDetail = fmt.Sprintf("credential file present (%s)", resolved)
			return
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			credentialUninspectable = true
		}
	}

	// Check whether any known API-key env var NAME is set.
	var loginEnvSet []string
	for _, name := range ar.EnvVarsSet {
		if slices.Contains(spec.APIKeyEnvVars, name) {
			loginEnvSet = append(loginEnvSet, name)
		}
	}
	if len(loginEnvSet) > 0 && !spec.EnvVarsDoNotAuthenticate {
		ar.LoginState = LoginStateLoggedIn
		ar.LoginDetail = fmt.Sprintf("env var %s set", strings.Join(loginEnvSet, ", "))
		return
	}

	// File-backed agents without OS keyring fallback can be reported not_logged_in.
	if len(spec.CredentialPaths) > 0 && !spec.UsesKeyring && spec.OtherAuthStore == "" && !credentialUninspectable && len(ar.EnvVarsSet) == 0 {
		ar.LoginState = LoginStateNotLoggedIn
		ar.LoginDetail = "no credential file or API key env var found"
		ar.Problems = append(ar.Problems, notLoggedInProblem(spec))
		return
	}

	ar.LoginState = LoginStateUnknown
	switch {
	case credentialUninspectable:
		ar.LoginDetail = "credential file could not be inspected"
	case spec.UsesKeyring:
		ar.LoginDetail = "credentials may be in OS keyring; no token env var or file found"
	case spec.OtherAuthStore != "":
		ar.LoginDetail = fmt.Sprintf("credentials may be in %s; no credential file found", spec.OtherAuthStore)
	case len(ar.EnvVarsSet) > 0:
		ar.LoginDetail = fmt.Sprintf("env var %s set; DevEx cannot confirm %s uses it", strings.Join(ar.EnvVarsSet, ", "), spec.Name)
	default:
		ar.LoginDetail = "no status command or credential signal"
	}
}

func isExitError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.Exited()
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

var versionTokenRegexp = regexp.MustCompile(`\bv?\d+(?:\.\d+)+(?:-[0-9A-Za-z.]+)?`)

// versionToken reports only a recognized version number from command output,
// never free-form text that could carry a credential.
func versionToken(out string) string {
	token := versionTokenRegexp.FindString(out)
	if len(token) > maxVersionLength {
		return ""
	}
	return token
}

// FormatJSON renders Report as indented JSON with defense-in-depth secret redaction.
func FormatJSON(r Report, secrets []string) ([]byte, error) {
	data, err := json.MarshalIndent(redactReport(r, secrets), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal agent doctor report: %w", err)
	}
	return append(data, '\n'), nil
}

// redactReport redacts every free-form string before marshaling, so JSON
// escaping can neither hide a secret nor be broken by the replacement.
func redactReport(r Report, secrets []string) Report {
	red := func(s string) string { return Redact(s, secrets) }
	problems := func(in []Problem) []Problem {
		out := make([]Problem, len(in))
		for i, p := range in {
			p.Summary, p.Fix = red(p.Summary), red(p.Fix)
			out[i] = p
		}
		return out
	}
	r.Problems = problems(r.Problems)
	agents := make([]AgentReport, len(r.Agents))
	for i, a := range r.Agents {
		a.Version, a.ResolvedPath, a.LoginDetail = red(a.Version), red(a.ResolvedPath), red(a.LoginDetail)
		matches := make([]PathMatch, len(a.PathMatches))
		for j, m := range a.PathMatches {
			m.Path, m.RealPath = red(m.Path), red(m.RealPath)
			matches[j] = m
		}
		a.PathMatches = matches
		configs := make([]MCPConfigResult, len(a.MCPConfigs))
		for j, c := range a.MCPConfigs {
			c.Path, c.Error = red(c.Path), red(c.Error)
			configs[j] = c
		}
		a.MCPConfigs = configs
		if a.Node != nil {
			n := *a.Node
			n.Detected, n.Path = red(n.Detected), red(n.Path)
			a.Node = &n
		}
		if a.Sandbox != nil {
			sb := *a.Sandbox
			sb.BwrapPath, sb.SocatPath = red(sb.BwrapPath), red(sb.SocatPath)
			a.Sandbox = &sb
		}
		a.Problems = problems(a.Problems)
		agents[i] = a
	}
	r.Agents = agents
	if r.LocalModelRuntime != nil {
		rt := *r.LocalModelRuntime
		rt.BinaryPath = red(rt.BinaryPath)
		rt.Version = red(rt.Version)
		rt.ServiceState = red(rt.ServiceState)
		rt.BoundAddress = red(rt.BoundAddress)
		rt.GPU.Runtime = red(rt.GPU.Runtime)
		if len(rt.GPU.Hints) > 0 {
			hints := make([]string, len(rt.GPU.Hints))
			for i, h := range rt.GPU.Hints {
				hints[i] = red(h)
			}
			rt.GPU.Hints = hints
		}
		if len(rt.GPU.GPUs) > 0 {
			gpus := make([]localmodels.GPUDevice, len(rt.GPU.GPUs))
			for i, g := range rt.GPU.GPUs {
				g.Name = red(g.Name)
				g.DriverVersion = red(g.DriverVersion)
				g.Memory = red(g.Memory)
				g.PCIAddress = red(g.PCIAddress)
				g.DeviceID = red(g.DeviceID)
				gpus[i] = g
			}
			rt.GPU.GPUs = gpus
		}
		r.LocalModelRuntime = &rt
	}
	if len(r.SkillPacks) > 0 {
		packs := make([]SkillPackReport, len(r.SkillPacks))
		for i, sp := range r.SkillPacks {
			sp.Pack = red(sp.Pack)
			packs[i] = sp
		}
		r.SkillPacks = packs
	}
	return r
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
			nodeState, detected := "ok", ag.Node.Detected
			switch {
			case ag.Node.OK:
			case detected != "":
				nodeState = "too old"
			case ag.Node.Path != "":
				nodeState, detected = "version unknown", ag.Node.Path
			default:
				nodeState, detected = "missing", "not found"
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

	if r.LocalModelRuntime != nil {
		rt := r.LocalModelRuntime
		b.WriteString("\nLocal Model Runtime:\n")
		if rt.Installed {
			ver := rt.Version
			if ver == "" {
				ver = "unknown version"
			}
			fmt.Fprintf(&b, "  %-10s installed (%s)\n", "ollama", ver)
			if rt.BinaryPath != "" {
				fmt.Fprintf(&b, "    %-14s %s\n", "path:", rt.BinaryPath)
			}
		} else {
			fmt.Fprintf(&b, "  %-10s not installed\n", "ollama")
		}
		if rt.ServiceState != "" {
			fmt.Fprintf(&b, "    %-14s %s\n", "service:", rt.ServiceState)
		}
		if rt.BoundAddress != "" {
			if rt.Installed || rt.ServiceActive || rt.APIReachable {
				reach := "unreachable"
				if rt.APIReachable {
					reach = "reachable"
				}
				fmt.Fprintf(&b, "    %-14s %s (%s)\n", "address:", rt.BoundAddress, reach)
			} else {
				fmt.Fprintf(&b, "    %-14s %s\n", "address:", rt.BoundAddress)
			}
		}
		gpuRuntime := rt.GPU.Runtime
		if gpuRuntime == "" {
			gpuRuntime = "CPU only"
		}
		fmt.Fprintf(&b, "    %-14s %s\n", "gpu:", gpuRuntime)
		for _, hint := range rt.GPU.Hints {
			fmt.Fprintf(&b, "    %-14s %s\n", "hint:", hint)
		}
	}

	if len(r.SkillPacks) > 0 {
		b.WriteString("\nSkill Packs:\n")
		for _, sp := range r.SkillPacks {
			pins := make([]string, 0, len(sp.PinnedSHAs))
			for _, sha := range sp.PinnedSHAs {
				pins = append(pins, skillpacks.ShortSHA(sha))
			}
			status := "pinned " + strings.Join(pins, ", ")
			if sp.UpdateAvailable {
				status += fmt.Sprintf(" (update available: %s)", skillpacks.ShortSHA(sp.CatalogSHA))
			}
			if sp.Unreviewed {
				status += " [unreviewed]"
			}
			fmt.Fprintf(&b, "  %-16s %-8s %-8s %s (%d skills)\n", sp.Pack, sp.Agent, sp.Scope, status, len(sp.Skills))
			if len(sp.LocallyEdited) > 0 {
				fmt.Fprintf(&b, "    %-14s %s\n", "local edits:", strings.Join(sp.LocallyEdited, ", "))
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
