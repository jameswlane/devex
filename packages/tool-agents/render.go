package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

const (
	instructionsFileMode os.FileMode = 0o644
	settingsFileMode     os.FileMode = 0o600
	codexRulesHeader                 = "# Managed by DevEx. Changes to this file are overwritten.\n"
)

// Env injects host environment lookups so planning and rendering are testable
// inside an isolated temporary HOME.
type Env struct {
	HomeDir  func() (string, error)
	Getenv   func(string) string
	LookPath func(string) (string, error)
	StateDir func(home string) string
}

// DefaultEnv returns the host environment implementation.
func DefaultEnv() Env {
	return Env{
		HomeDir:  os.UserHomeDir,
		Getenv:   os.Getenv,
		LookPath: exec.LookPath,
		StateDir: func(home string) string {
			return sdk.PathsForHome(home).State
		},
	}
}

func (e Env) normalize() Env {
	def := DefaultEnv()
	if e.HomeDir == nil {
		e.HomeDir = def.HomeDir
	}
	if e.Getenv == nil {
		e.Getenv = def.Getenv
	}
	if e.LookPath == nil {
		e.LookPath = def.LookPath
	}
	if e.StateDir == nil {
		e.StateDir = def.StateDir
	}
	return e
}

// AgentPlan holds the planned file operations for a single agent.
type AgentPlan struct {
	Agent   string
	Skipped bool
	Reason  string
	Files   []FilePlan
}

// PlanResult contains all planned agent operations plus updated managed state.
type PlanResult struct {
	Agents       []AgentPlan
	StatePath    string
	InitialState ManagedState
	NextState    ManagedState
}

// ResolveAgentConfigDir returns the resolved user-level configuration directory
// for an agent, honouring vendor-documented environment overrides.
func ResolveAgentConfigDir(agent, home string, getenv func(string) string) string {
	switch agent {
	case AgentClaude:
		if dir := strings.TrimSpace(getenv("CLAUDE_CONFIG_DIR")); dir != "" {
			return filepath.Clean(dir)
		}
		return filepath.Join(home, ".claude")
	case AgentCodex:
		if dir := strings.TrimSpace(getenv("CODEX_HOME")); dir != "" {
			return filepath.Clean(dir)
		}
		return filepath.Join(home, ".codex")
	case AgentGemini:
		if base := strings.TrimSpace(getenv("GEMINI_CLI_HOME")); base != "" {
			return filepath.Join(filepath.Clean(base), ".gemini")
		}
		return filepath.Join(home, ".gemini")
	case AgentOpenCode:
		if dir := strings.TrimSpace(getenv("OPENCODE_CONFIG_DIR")); dir != "" {
			return filepath.Clean(dir)
		}
		if xdg := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); xdg != "" && filepath.IsAbs(xdg) {
			return filepath.Join(filepath.Clean(xdg), "opencode")
		}
		return filepath.Join(home, ".config", "opencode")
	default:
		return ""
	}
}

func agentBinaryName(agent string) string {
	switch agent {
	case AgentClaude:
		return "claude"
	case AgentCodex:
		return "codex"
	case AgentGemini:
		return "gemini"
	case AgentOpenCode:
		return "opencode"
	default:
		return ""
	}
}

func isAgentInstalled(agent, configDir string, env Env) bool {
	if bin := agentBinaryName(agent); bin != "" {
		if _, err := env.LookPath(bin); err == nil {
			return true
		}
	}
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		return true
	}
	return false
}

// BuildPlan computes all file diffs and state transitions without writing
// anything to disk.
func BuildPlan(spec Spec, env Env) (PlanResult, error) {
	env = env.normalize()
	if err := spec.Validate(); err != nil {
		return PlanResult{}, err
	}

	home, err := env.HomeDir()
	if err != nil {
		return PlanResult{}, fmt.Errorf("resolve home directory: %w", err)
	}

	statePath := filepath.Join(env.StateDir(home), "agents", "managed.json")
	initialState, err := LoadManagedState(statePath)
	if err != nil {
		return PlanResult{}, err
	}
	nextState := initialState.clone()

	result := PlanResult{
		Agents:       make([]AgentPlan, 0, len(spec.Agents)),
		StatePath:    statePath,
		InitialState: initialState,
		NextState:    nextState,
	}

	for _, agent := range spec.Agents {
		configDir := ResolveAgentConfigDir(agent.Name, home, env.Getenv)
		if !isAgentInstalled(agent.Name, configDir, env) {
			result.Agents = append(result.Agents, AgentPlan{
				Agent:   agent.Name,
				Skipped: true,
				Reason:  "not installed",
			})
			continue
		}

		prevAgentState := initialState.Agents[agent.Name]
		var (
			files     []FilePlan
			nextAgent AgentManagedState
			planErr   error
		)
		switch agent.Name {
		case AgentClaude:
			files, nextAgent, planErr = planClaude(agent, configDir, prevAgentState)
		case AgentCodex:
			files, nextAgent, planErr = planCodex(agent, configDir)
		case AgentGemini:
			files, nextAgent, planErr = planGemini(agent, configDir, prevAgentState)
		case AgentOpenCode:
			files, nextAgent, planErr = planOpenCode(agent, configDir, prevAgentState)
		default:
			planErr = fmt.Errorf("unsupported agent %q", agent.Name)
		}
		if planErr != nil {
			return PlanResult{}, fmt.Errorf("%s: %w", agent.Name, planErr)
		}

		if len(nextAgent.Allow) == 0 && len(nextAgent.Deny) == 0 && len(nextAgent.Permissions) == 0 && len(nextAgent.PresetPermissions) == 0 {
			delete(nextState.Agents, agent.Name)
		} else {
			nextState.Agents[agent.Name] = nextAgent
		}

		result.Agents = append(result.Agents, AgentPlan{
			Agent: agent.Name,
			Files: files,
		})
	}

	result.NextState = nextState
	return result, nil
}

func planInstructionsFile(agent, path, instructions string) (*FilePlan, error) {
	before, exists, mode, err := readExistingFile(path, instructionsFileMode)
	if err != nil {
		return nil, err
	}
	after, changed, err := RenderManagedBlock(before, exists, instructions)
	if err != nil {
		return nil, fmt.Errorf("instructions file %q: %w", path, err)
	}
	if !exists && !changed {
		return nil, nil
	}
	action := actionUnchanged
	if changed {
		if exists {
			action = actionUpdate
		} else {
			action = actionCreate
		}
	}
	return &FilePlan{
		Agent:  agent,
		Path:   path,
		Action: action,
		Before: before,
		After:  after,
		Mode:   mode,
		Exists: exists,
	}, nil
}

func planClaude(spec AgentSpec, configDir string, prev AgentManagedState) ([]FilePlan, AgentManagedState, error) {
	var files []FilePlan

	if fp, err := planInstructionsFile(AgentClaude, filepath.Join(configDir, "CLAUDE.md"), spec.Instructions); err != nil {
		return nil, AgentManagedState{}, err
	} else if fp != nil {
		files = append(files, *fp)
	}

	desiredAllow := make([]string, 0, len(spec.AllowedCommands))
	for _, cmd := range spec.AllowedCommands {
		desiredAllow = append(desiredAllow, formatClaudeBashRule(cmd))
	}
	desiredDeny := make([]string, 0, len(spec.DeniedCommands))
	for _, cmd := range spec.DeniedCommands {
		desiredDeny = append(desiredDeny, formatClaudeBashRule(cmd))
	}

	settingsPath := filepath.Join(configDir, "settings.json")
	before, exists, mode, err := readExistingFile(settingsPath, settingsFileMode)
	if err != nil {
		return nil, AgentManagedState{}, err
	}

	hasSettingsWork := spec.Model != "" || spec.Preset != "" ||
		len(desiredAllow) > 0 || len(desiredDeny) > 0 ||
		len(prev.Allow) > 0 || len(prev.Deny) > 0

	if !exists && !hasSettingsWork {
		return files, AgentManagedState{}, nil
	}

	var doc *jsonObject
	if exists {
		doc, err = ParseJSONObject([]byte(before), settingsPath)
		if err != nil {
			return nil, AgentManagedState{}, err
		}
	} else {
		doc = &jsonObject{}
	}
	origDoc := doc.Clone()

	if spec.Model != "" {
		doc.Set("model", newJSONString(spec.Model))
	}

	var ownedAllow, ownedDeny []string
	needPermissions := spec.Preset != "" || len(desiredAllow) > 0 || len(desiredDeny) > 0 || len(prev.Allow) > 0 || len(prev.Deny) > 0
	if needPermissions {
		perms, err := doc.EnsureObject("permissions")
		if err != nil {
			return nil, AgentManagedState{}, fmt.Errorf("%s: %w", settingsPath, err)
		}
		if spec.Preset != "" {
			defaultMode, err := claudePresetMode(spec.Preset)
			if err != nil {
				return nil, AgentManagedState{}, err
			}
			perms.Set("defaultMode", newJSONString(defaultMode))
		}
		if len(desiredAllow) > 0 || len(prev.Allow) > 0 {
			existingAllow, _, err := perms.GetStringArray("allow")
			if err != nil {
				return nil, AgentManagedState{}, fmt.Errorf("%s: permissions.%w", settingsPath, err)
			}
			ownedAllow = OwnedEntries(existingAllow, prev.Allow, desiredAllow)
			merged := MergeStringList(existingAllow, prev.Allow, desiredAllow)
			perms.SetStringArray("allow", merged)
		}
		if len(desiredDeny) > 0 || len(prev.Deny) > 0 {
			existingDeny, _, err := perms.GetStringArray("deny")
			if err != nil {
				return nil, AgentManagedState{}, fmt.Errorf("%s: permissions.%w", settingsPath, err)
			}
			ownedDeny = OwnedEntries(existingDeny, prev.Deny, desiredDeny)
			merged := MergeStringList(existingDeny, prev.Deny, desiredDeny)
			perms.SetStringArray("deny", merged)
		}
	}

	files = append(files, buildJSONFilePlan(AgentClaude, settingsPath, before, exists, mode, origDoc, doc))
	return files, AgentManagedState{
		Allow: ownedAllow,
		Deny:  ownedDeny,
	}, nil
}

func planCodex(spec AgentSpec, configDir string) ([]FilePlan, AgentManagedState, error) {
	var files []FilePlan

	if fp, err := planInstructionsFile(AgentCodex, filepath.Join(configDir, "AGENTS.md"), spec.Instructions); err != nil {
		return nil, AgentManagedState{}, err
	} else if fp != nil {
		files = append(files, *fp)
	}

	configPath := filepath.Join(configDir, "config.toml")
	updates := map[string]string{}
	orderedKeys := []string{"model", "approval_policy", "sandbox_mode"}
	if spec.Model != "" {
		updates["model"] = spec.Model
	}
	if spec.Preset != "" {
		approvalPolicy, sandboxMode, err := codexPresetModes(spec.Preset)
		if err != nil {
			return nil, AgentManagedState{}, err
		}
		updates["approval_policy"] = approvalPolicy
		updates["sandbox_mode"] = sandboxMode
	}

	beforeTOML, existsTOML, modeTOML, err := readExistingFile(configPath, settingsFileMode)
	if err != nil {
		return nil, AgentManagedState{}, err
	}
	if existsTOML || len(updates) > 0 {
		afterTOML, changedTOML, err := EditTOMLTopLevelStrings(beforeTOML, updates, orderedKeys)
		if err != nil {
			return nil, AgentManagedState{}, fmt.Errorf("%s: %w", configPath, err)
		}
		action := actionUnchanged
		if changedTOML {
			if existsTOML {
				action = actionUpdate
			} else {
				action = actionCreate
			}
		}
		files = append(files, FilePlan{
			Agent:  AgentCodex,
			Path:   configPath,
			Action: action,
			Before: beforeTOML,
			After:  afterTOML,
			Mode:   modeTOML,
			Exists: existsTOML,
		})
	}

	rulesPath := filepath.Join(configDir, "rules", "devex.rules")
	beforeRules, existsRules, modeRules, err := readExistingFile(rulesPath, settingsFileMode)
	if err != nil {
		return nil, AgentManagedState{}, err
	}
	if existsRules && !strings.HasPrefix(beforeRules, codexRulesHeader) {
		return nil, AgentManagedState{}, fmt.Errorf("%s was not written by DevEx (missing %q header); rename it so DevEx can manage this file", rulesPath, strings.TrimSpace(codexRulesHeader))
	}
	hasRules := len(spec.AllowedCommands) > 0 || len(spec.DeniedCommands) > 0
	switch {
	case hasRules:
		afterRules, err := renderCodexRules(spec.AllowedCommands, spec.DeniedCommands)
		if err != nil {
			return nil, AgentManagedState{}, err
		}
		action := actionUnchanged
		if !existsRules {
			action = actionCreate
		} else if afterRules != beforeRules {
			action = actionUpdate
		}
		files = append(files, FilePlan{
			Agent:  AgentCodex,
			Path:   rulesPath,
			Action: action,
			Before: beforeRules,
			After:  afterRules,
			Mode:   modeRules,
			Exists: existsRules,
		})
	case existsRules:
		files = append(files, FilePlan{
			Agent:  AgentCodex,
			Path:   rulesPath,
			Action: actionDelete,
			Before: beforeRules,
			After:  "",
			Mode:   modeRules,
			Exists: true,
		})
	}

	return files, AgentManagedState{}, nil
}

func planGemini(spec AgentSpec, configDir string, prev AgentManagedState) ([]FilePlan, AgentManagedState, error) {
	var files []FilePlan

	if fp, err := planInstructionsFile(AgentGemini, filepath.Join(configDir, "GEMINI.md"), spec.Instructions); err != nil {
		return nil, AgentManagedState{}, err
	} else if fp != nil {
		files = append(files, *fp)
	}

	desiredAllow := make([]string, 0, len(spec.AllowedCommands))
	for _, cmd := range spec.AllowedCommands {
		desiredAllow = append(desiredAllow, formatGeminiShellRule(cmd))
	}
	desiredDeny := make([]string, 0, len(spec.DeniedCommands))
	for _, cmd := range spec.DeniedCommands {
		desiredDeny = append(desiredDeny, formatGeminiShellRule(cmd))
	}

	settingsPath := filepath.Join(configDir, "settings.json")
	before, exists, mode, err := readExistingFile(settingsPath, settingsFileMode)
	if err != nil {
		return nil, AgentManagedState{}, err
	}

	hasSettingsWork := spec.Model != "" || spec.Preset != "" ||
		len(desiredAllow) > 0 || len(desiredDeny) > 0 ||
		len(prev.Allow) > 0 || len(prev.Deny) > 0

	if !exists && !hasSettingsWork {
		return files, AgentManagedState{}, nil
	}

	var doc *jsonObject
	if exists {
		doc, err = ParseJSONObject([]byte(before), settingsPath)
		if err != nil {
			return nil, AgentManagedState{}, err
		}
	} else {
		doc = &jsonObject{}
	}
	origDoc := doc.Clone()

	if spec.Model != "" {
		modelObj, err := doc.EnsureObject("model")
		if err != nil {
			return nil, AgentManagedState{}, fmt.Errorf("%s: %w", settingsPath, err)
		}
		modelObj.Set("name", newJSONString(spec.Model))
	}

	if spec.Preset != "" {
		approvalMode, err := geminiPresetMode(spec.Preset)
		if err != nil {
			return nil, AgentManagedState{}, err
		}
		generalObj, err := doc.EnsureObject("general")
		if err != nil {
			return nil, AgentManagedState{}, fmt.Errorf("%s: %w", settingsPath, err)
		}
		generalObj.Set("defaultApprovalMode", newJSONString(approvalMode))
	}

	var ownedAllow, ownedDeny []string
	if len(desiredAllow) > 0 || len(desiredDeny) > 0 || len(prev.Allow) > 0 || len(prev.Deny) > 0 {
		toolsObj, err := doc.EnsureObject("tools")
		if err != nil {
			return nil, AgentManagedState{}, fmt.Errorf("%s: %w", settingsPath, err)
		}
		if len(desiredAllow) > 0 || len(prev.Allow) > 0 {
			existingAllow, _, err := toolsObj.GetStringArray("allowed")
			if err != nil {
				return nil, AgentManagedState{}, fmt.Errorf("%s: tools.%w", settingsPath, err)
			}
			ownedAllow = OwnedEntries(existingAllow, prev.Allow, desiredAllow)
			merged := MergeStringList(existingAllow, prev.Allow, desiredAllow)
			toolsObj.SetStringArray("allowed", merged)
		}
		if len(desiredDeny) > 0 || len(prev.Deny) > 0 {
			existingDeny, _, err := toolsObj.GetStringArray("exclude")
			if err != nil {
				return nil, AgentManagedState{}, fmt.Errorf("%s: tools.%w", settingsPath, err)
			}
			ownedDeny = OwnedEntries(existingDeny, prev.Deny, desiredDeny)
			merged := MergeStringList(existingDeny, prev.Deny, desiredDeny)
			toolsObj.SetStringArray("exclude", merged)
		}
	}

	files = append(files, buildJSONFilePlan(AgentGemini, settingsPath, before, exists, mode, origDoc, doc))
	return files, AgentManagedState{
		Allow: ownedAllow,
		Deny:  ownedDeny,
	}, nil
}

func planOpenCode(spec AgentSpec, configDir string, prev AgentManagedState) ([]FilePlan, AgentManagedState, error) {
	var files []FilePlan

	if fp, err := planInstructionsFile(AgentOpenCode, filepath.Join(configDir, "AGENTS.md"), spec.Instructions); err != nil {
		return nil, AgentManagedState{}, err
	} else if fp != nil {
		files = append(files, *fp)
	}

	// OpenCode reads both opencode.json and opencode.jsonc, with opencode.jsonc
	// taking precedence when present. Target opencode.jsonc if it already exists
	// (rejecting comments/trailing commas rather than stripping them), otherwise
	// manage opencode.json.
	settingsPath := filepath.Join(configDir, "opencode.json")
	jsoncPath := filepath.Join(configDir, "opencode.jsonc")
	if _, err := os.Lstat(jsoncPath); err == nil {
		settingsPath = jsoncPath
	}

	presetRules, err := openCodePresetRules(spec.Preset)
	if err != nil {
		return nil, AgentManagedState{}, err
	}
	allowRules := make([]OpenCodeRule, 0, len(spec.AllowedCommands))
	for _, cmd := range spec.AllowedCommands {
		allowRules = append(allowRules, OpenCodeRule{
			Action:   "shell",
			Resource: formatOpenCodeShellResource(cmd),
			Effect:   "allow",
		})
	}
	denyRules := make([]OpenCodeRule, 0, len(spec.DeniedCommands))
	for _, cmd := range spec.DeniedCommands {
		denyRules = append(denyRules, OpenCodeRule{
			Action:   "shell",
			Resource: formatOpenCodeShellResource(cmd),
			Effect:   "deny",
		})
	}
	desiredRules := make([]OpenCodeRule, 0, len(presetRules)+len(allowRules)+len(denyRules))
	desiredRules = append(desiredRules, presetRules...)
	desiredRules = append(desiredRules, allowRules...)
	desiredRules = append(desiredRules, denyRules...)
	desiredRules = uniqueOpenCodeRules(desiredRules)

	before, exists, mode, err := readExistingFile(settingsPath, settingsFileMode)
	if err != nil {
		return nil, AgentManagedState{}, err
	}

	hasSettingsWork := spec.Model != "" || len(desiredRules) > 0 || len(prev.Permissions) > 0 || len(prev.PresetPermissions) > 0
	if !exists && !hasSettingsWork {
		return files, AgentManagedState{}, nil
	}

	var doc *jsonObject
	if exists {
		doc, err = ParseJSONObject([]byte(before), settingsPath)
		if err != nil {
			return nil, AgentManagedState{}, err
		}
	} else {
		doc = &jsonObject{}
	}
	origDoc := doc.Clone()

	if spec.Model != "" {
		doc.Set("model", newJSONString(spec.Model))
	}

	var ownedPreset, ownedExplicit []OpenCodeRule
	if len(desiredRules) > 0 || len(prev.Permissions) > 0 || len(prev.PresetPermissions) > 0 {
		var existingItems []jsonValue
		if rawPerms, ok := doc.Get("permissions"); ok {
			arr, isArr := rawPerms.(*jsonArray)
			if !isArr {
				return nil, AgentManagedState{}, fmt.Errorf("%s: key \"permissions\" must be a JSON array", settingsPath)
			}
			existingItems = arr.Items
		}
		var merged []jsonValue
		merged, ownedPreset, ownedExplicit = mergeOpenCodePermissions(existingItems, prev, presetRules, append(allowRules, denyRules...))
		doc.Set("permissions", &jsonArray{Items: merged})
	}

	files = append(files, buildJSONFilePlan(AgentOpenCode, settingsPath, before, exists, mode, origDoc, doc))
	return files, AgentManagedState{
		PresetPermissions: ownedPreset,
		Permissions:       ownedExplicit,
	}, nil
}

func buildJSONFilePlan(agent, path, before string, exists bool, mode os.FileMode, origDoc, doc *jsonObject) FilePlan {
	if exists && origDoc.jsonEqual(doc) {
		return FilePlan{
			Agent:  agent,
			Path:   path,
			Action: actionUnchanged,
			Before: before,
			After:  before,
			Mode:   mode,
			Exists: true,
		}
	}
	after := doc.MarshalIndent()
	action := actionCreate
	if exists {
		if after == before {
			action = actionUnchanged
		} else {
			action = actionUpdate
		}
	}
	return FilePlan{
		Agent:  agent,
		Path:   path,
		Action: action,
		Before: before,
		After:  after,
		Mode:   mode,
		Exists: exists,
	}
}

func claudePresetMode(preset string) (string, error) {
	switch preset {
	case PresetStrict:
		return "default", nil
	case PresetBalanced:
		return "acceptEdits", nil
	default:
		return "", fmt.Errorf("unsupported preset %q", preset)
	}
}

func codexPresetModes(preset string) (approvalPolicy, sandboxMode string, err error) {
	switch preset {
	case PresetStrict:
		return "on-request", "read-only", nil
	case PresetBalanced:
		return "on-request", "workspace-write", nil
	default:
		return "", "", fmt.Errorf("unsupported preset %q", preset)
	}
}

func geminiPresetMode(preset string) (string, error) {
	switch preset {
	case PresetStrict:
		return "default", nil
	case PresetBalanced:
		return "auto_edit", nil
	default:
		return "", fmt.Errorf("unsupported preset %q", preset)
	}
}

func openCodePresetRules(preset string) ([]OpenCodeRule, error) {
	switch preset {
	case "":
		return nil, nil
	case PresetStrict:
		return []OpenCodeRule{
			{Action: "edit", Resource: "*", Effect: "ask"},
			{Action: "shell", Resource: "*", Effect: "ask"},
		}, nil
	case PresetBalanced:
		return []OpenCodeRule{
			{Action: "edit", Resource: "*", Effect: "allow"},
			{Action: "shell", Resource: "*", Effect: "ask"},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported preset %q", preset)
	}
}

func formatClaudeBashRule(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if strings.Contains(cmd, "*") {
		return fmt.Sprintf("Bash(%s)", cmd)
	}
	return fmt.Sprintf("Bash(%s *)", cmd)
}

func formatGeminiShellRule(cmd string) string {
	return fmt.Sprintf("run_shell_command(%s)", strings.TrimSpace(cmd))
}

func formatOpenCodeShellResource(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if strings.Contains(cmd, "*") {
		return cmd
	}
	return cmd + " *"
}

func openCodeRuleFromJSON(v jsonValue) (OpenCodeRule, bool) {
	obj, ok := v.(*jsonObject)
	if !ok {
		return OpenCodeRule{}, false
	}
	actVal, ok1 := obj.Get("action")
	resVal, ok2 := obj.Get("resource")
	effVal, ok3 := obj.Get("effect")
	if !ok1 || !ok2 || !ok3 {
		return OpenCodeRule{}, false
	}
	actRaw, ok1 := actVal.(jsonRaw)
	resRaw, ok2 := resVal.(jsonRaw)
	effRaw, ok3 := effVal.(jsonRaw)
	if !ok1 || !ok2 || !ok3 {
		return OpenCodeRule{}, false
	}
	act, ok1 := actRaw.asString()
	res, ok2 := resRaw.asString()
	eff, ok3 := effRaw.asString()
	if !ok1 || !ok2 || !ok3 {
		return OpenCodeRule{}, false
	}
	return OpenCodeRule{Action: act, Resource: res, Effect: eff}, true
}

func openCodeRuleToJSON(r OpenCodeRule) *jsonObject {
	return &jsonObject{
		Entries: []jsonEntry{
			{Key: "action", Value: newJSONString(r.Action)},
			{Key: "resource", Value: newJSONString(r.Resource)},
			{Key: "effect", Value: newJSONString(r.Effect)},
		},
	}
}

// mergeOpenCodePermissions keeps every user rule object verbatim and in order,
// because OpenCode V2 uses the last matching rule. It first strips the copies
// DevEx wrote last time: preset defaults from the front (first occurrence) and
// explicit allow/deny rules from the back (last occurrence), so a user rule
// identical to a DevEx rule is never removed. Preset defaults then go first
// unless the user already has the same rule, and explicit rules always go last
// so they take effect; withdrawing one restores the user's own precedence.
func mergeOpenCodePermissions(existing []jsonValue, prev AgentManagedState, presetRules, explicitRules []OpenCodeRule) (merged []jsonValue, ownedPreset, ownedExplicit []OpenCodeRule) {
	items := slices.Clone(existing)
	removeAt := func(i int) { items = slices.Delete(items, i, i+1) }
	for _, r := range prev.PresetPermissions {
		for i := range items {
			if parsed, ok := exactOpenCodeRule(items[i]); ok && parsed == r {
				removeAt(i)
				break
			}
		}
	}
	for _, r := range prev.Permissions {
		for i := len(items) - 1; i >= 0; i-- {
			if parsed, ok := exactOpenCodeRule(items[i]); ok && parsed == r {
				removeAt(i)
				break
			}
		}
	}

	userRules := make([]OpenCodeRule, 0, len(items))
	for _, item := range items {
		if parsed, ok := openCodeRuleFromJSON(item); ok {
			userRules = append(userRules, parsed)
		}
	}
	for _, r := range uniqueOpenCodeRules(presetRules) {
		if !slices.Contains(userRules, r) {
			merged = append(merged, openCodeRuleToJSON(r))
			ownedPreset = append(ownedPreset, r)
		}
	}
	merged = append(merged, items...)
	for _, r := range uniqueOpenCodeRules(explicitRules) {
		merged = append(merged, openCodeRuleToJSON(r))
		ownedExplicit = append(ownedExplicit, r)
	}
	return merged, ownedPreset, ownedExplicit
}

// exactOpenCodeRule matches only rule objects with exactly the three keys
// DevEx writes, so a user rule carrying extra keys is never taken for DevEx's.
func exactOpenCodeRule(v jsonValue) (OpenCodeRule, bool) {
	if obj, ok := v.(*jsonObject); !ok || len(obj.Entries) != 3 {
		return OpenCodeRule{}, false
	}
	return openCodeRuleFromJSON(v)
}

func renderCodexRules(allowed, denied []string) (string, error) {
	var b strings.Builder
	b.WriteString(codexRulesHeader)

	writeRule := func(cmd, decision, justification string) error {
		tokens, err := splitCommandTokens(cmd)
		if err != nil {
			return fmt.Errorf("parse command %q for Codex prefix_rule: %w", cmd, err)
		}
		b.WriteString("\nprefix_rule(\n")
		b.WriteString("    pattern = [")
		for i, tok := range tokens {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tomlQuoteString(tok))
		}
		b.WriteString("],\n")
		fmt.Fprintf(&b, "    decision = %s,\n", tomlQuoteString(decision))
		fmt.Fprintf(&b, "    justification = %s,\n", tomlQuoteString(justification))
		b.WriteString(")\n")
		return nil
	}

	for _, cmd := range uniqueStrings(allowed) {
		if err := writeRule(cmd, "allow", "Allowed by DevEx agents configuration"); err != nil {
			return "", err
		}
	}
	for _, cmd := range uniqueStrings(denied) {
		if err := writeRule(cmd, "forbidden", "Denied by DevEx agents configuration"); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func splitCommandTokens(cmd string) ([]string, error) {
	var (
		tokens   []string
		cur      strings.Builder
		inSingle bool
		inDouble bool
		escaped  bool
		hasToken bool
	)
	for _, r := range strings.TrimSpace(cmd) {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			hasToken = true
			continue
		}
		if inSingle {
			if r == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(r)
			}
			hasToken = true
			continue
		}
		if inDouble {
			switch r {
			case '\\':
				escaped = true
			case '"':
				inDouble = false
			default:
				cur.WriteRune(r)
			}
			hasToken = true
			continue
		}
		switch r {
		case '\\':
			escaped = true
			hasToken = true
		case '\'':
			inSingle = true
			hasToken = true
		case '"':
			inDouble = true
			hasToken = true
		case ' ', '\t':
			if hasToken {
				tokens = append(tokens, cur.String())
				cur.Reset()
				hasToken = false
			}
		default:
			cur.WriteRune(r)
			hasToken = true
		}
	}
	if escaped || inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote or escape")
	}
	if hasToken {
		tokens = append(tokens, cur.String())
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return tokens, nil
}

func uniqueStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func uniqueOpenCodeRules(items []OpenCodeRule) []OpenCodeRule {
	out := make([]OpenCodeRule, 0, len(items))
	for _, r := range items {
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}
