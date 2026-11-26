package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	mcpServerNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	mcpEnvVarNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	mcpEnvRefRe     = regexp.MustCompile(`^\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}$`)
	mcpHeaderNameRe = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`)

	claudeNativeRefRe   = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*(?::-[^}]*)?\}$`)
	geminiNativeRefRe   = regexp.MustCompile(`^(?:\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*|%[A-Za-z_][A-Za-z0-9_]*%)$`)
	opencodeNativeRefRe = regexp.MustCompile(`^\{env:[A-Za-z_][A-Za-z0-9_]*\}$`)

	secretPATRe    = regexp.MustCompile(`(^|[^A-Z0-9])PAT([^A-Z0-9]|$)`)
	secretValueRes = []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])github_pat_[A-Za-z0-9_]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])glpat-[A-Za-z0-9_-]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])sk-ant-[A-Za-z0-9_-]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])sk-[A-Za-z0-9_-]{12,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])xox[abposr]-[A-Za-z0-9-]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])AKIA[0-9A-Z]{16}(?:[^0-9A-Z]|$)`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])AIza[0-9A-Za-z_-]{16,}`),
	}
	bearerTokenRe = regexp.MustCompile(`(?i)(?:^|\s)Bearer\s+(\S+)`)
)

// MCPDiffPlan describes a diff preview for an agent's MCP configuration managed
// via CLI (currently Claude Code's ~/.claude.json mcpServers section).
type MCPDiffPlan struct {
	Agent  string
	Path   string
	Action fileAction
	Before string
	After  string
	Exists bool
	Mode   uint32
}

// MCPCommandAction represents one CLI command to run during apply (e.g.
// `claude mcp remove` or `claude mcp add-json`).
type MCPCommandAction struct {
	ServerName string
	Kind       string // "remove" or "add"
	Binary     string
	Args       []string
}

func validateMCPServers(servers []MCPServerSpec) error {
	seen := make(map[string]bool, len(servers))
	for i, srv := range servers {
		label := srv.Name
		if label == "" {
			label = fmt.Sprintf("#%d", i)
		}
		if !mcpServerNameRe.MatchString(srv.Name) {
			return fmt.Errorf("mcp_servers[%d]: name %q must match ^[A-Za-z0-9_-]{1,64}$", i, srv.Name)
		}
		if seen[srv.Name] {
			return fmt.Errorf("mcp_servers[%d]: duplicate MCP server name %q", i, srv.Name)
		}
		seen[srv.Name] = true

		switch srv.Transport {
		case MCPTransportStdio:
			if err := validateStdioServer(srv, label); err != nil {
				return err
			}
		case MCPTransportHTTP:
			if err := validateHTTPServer(srv, label); err != nil {
				return err
			}
		default:
			return fmt.Errorf("mcp_servers[%q]: unsupported transport %q (supported: %s, %s)", label, srv.Transport, MCPTransportStdio, MCPTransportHTTP)
		}
	}
	return nil
}

func validateStdioServer(srv MCPServerSpec, label string) error {
	if strings.TrimSpace(srv.Command) == "" || hasControlChars(srv.Command) {
		return fmt.Errorf("mcp_servers[%q].command: must be a non-empty single-line string without control characters", label)
	}
	if srv.URL != "" {
		return fmt.Errorf("mcp_servers[%q].url: must not be set for stdio transport", label)
	}
	if len(srv.Headers) > 0 {
		return fmt.Errorf("mcp_servers[%q].headers: must not be set for stdio transport", label)
	}
	for i, arg := range srv.Args {
		if hasControlChars(arg) {
			return fmt.Errorf("mcp_servers[%q].args[%d]: must not contain control characters", label, i)
		}
		if looksLikeSecretValue(arg, nil) {
			return fmt.Errorf("mcp_servers[%q].args[%d]: appears to contain a literal secret; pass secrets via env references (${env:VAR_NAME}) instead", label, i)
		}
	}
	for _, k := range sortedMapKeys(srv.Env) {
		v := srv.Env[k]
		if !mcpEnvVarNameRe.MatchString(k) || hasControlChars(k) {
			return fmt.Errorf("mcp_servers[%q].env.%s: invalid environment variable name", label, k)
		}
		if hasControlChars(v) {
			return fmt.Errorf("mcp_servers[%q].env.%s: must not contain control characters", label, k)
		}
		if err := validateSecretPolicyValue(label, "env."+k, k, v, false); err != nil {
			return err
		}
	}
	return nil
}

func validateHTTPServer(srv MCPServerSpec, label string) error {
	if srv.Command != "" {
		return fmt.Errorf("mcp_servers[%q].command: must not be set for http transport", label)
	}
	if len(srv.Args) > 0 {
		return fmt.Errorf("mcp_servers[%q].args: must not be set for http transport", label)
	}
	if len(srv.Env) > 0 {
		return fmt.Errorf("mcp_servers[%q].env: must not be set for http transport", label)
	}
	if strings.TrimSpace(srv.URL) == "" || hasControlChars(srv.URL) {
		return fmt.Errorf("mcp_servers[%q].url: requires a valid http(s) URL without control characters", label)
	}
	parsed, err := url.Parse(srv.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("mcp_servers[%q].url: must be a valid http or https URL with a host", label)
	}
	if parsed.User != nil {
		return fmt.Errorf("mcp_servers[%q].url: must not contain embedded credentials; use headers with ${env:VAR_NAME} instead", label)
	}
	query := parsed.Query()
	for _, qk := range sortedQueryKeys(query) {
		for _, qv := range query[qk] {
			if (isSecretKey(qk) && strings.TrimSpace(qv) != "") || looksLikeSecretValue(qv, nil) {
				return fmt.Errorf("mcp_servers[%q].url: query parameter %q appears to contain a literal secret; use headers with ${env:VAR_NAME} instead", label, qk)
			}
		}
	}
	for _, k := range sortedMapKeys(srv.Headers) {
		v := srv.Headers[k]
		if !mcpHeaderNameRe.MatchString(k) || hasControlChars(k) {
			return fmt.Errorf("mcp_servers[%q].headers.%s: invalid HTTP header name", label, k)
		}
		if hasControlChars(v) {
			return fmt.Errorf("mcp_servers[%q].headers.%s: must not contain control characters", label, k)
		}
		if err := validateSecretPolicyValue(label, "headers."+k, k, v, strings.EqualFold(k, "Authorization")); err != nil {
			return err
		}
	}
	return nil
}

func validateSecretPolicyValue(serverName, fieldName, key, val string, allowBearerRef bool) error {
	if strings.Contains(val, "${keyring:") {
		return fmt.Errorf("mcp_servers[%q].%s: keyring references are not supported in v1; use ${env:VAR_NAME} instead", serverName, fieldName)
	}
	if _, ok := parseEnvRef(val); ok {
		return nil
	}
	if allowBearerRef {
		if _, ok := parseBearerEnvRef(val); ok {
			return nil
		}
	}
	if strings.Contains(val, "${env:") {
		return fmt.Errorf("mcp_servers[%q].%s: invalid environment variable reference; use ${env:VAR_NAME} (or \"Bearer ${env:VAR_NAME}\" for Authorization)", serverName, fieldName)
	}
	if isSecretKey(key) {
		return fmt.Errorf("mcp_servers[%q].%s: looks like a secret and must use an environment variable reference (${env:VAR_NAME}), not a literal value", serverName, fieldName)
	}
	if looksLikeSecretValue(val, nil) {
		return fmt.Errorf("mcp_servers[%q].%s: appears to contain a literal secret; use an environment variable reference (${env:VAR_NAME}) instead", serverName, fieldName)
	}
	return nil
}

func parseEnvRef(val string) (string, bool) {
	m := mcpEnvRefRe.FindStringSubmatch(val)
	if len(m) != 2 {
		return "", false
	}
	return m[1], true
}

func parseBearerEnvRef(val string) (string, bool) {
	if !strings.HasPrefix(val, "Bearer ") {
		return "", false
	}
	return parseEnvRef(strings.TrimPrefix(val, "Bearer "))
}

func isSecretKey(key string) bool {
	upper := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(key)), "-", "_")
	if upper == "" {
		return false
	}
	for _, needle := range []string{
		"TOKEN",
		"SECRET",
		"PASSWORD",
		"PASSWD",
		"API_KEY",
		"APIKEY",
		"ACCESS_KEY",
		"PRIVATE_KEY",
		"CREDENTIAL",
		"AUTH",
	} {
		if strings.Contains(upper, needle) {
			return true
		}
	}
	return secretPATRe.MatchString(upper)
}

func looksLikeSecretValue(val string, isNativeRef func(string) bool) bool {
	for _, re := range secretValueRes {
		if re.MatchString(val) {
			return true
		}
	}
	for _, m := range bearerTokenRe.FindAllStringSubmatch(val, -1) {
		if len(m) >= 2 {
			tok := m[1]
			if _, ok := parseEnvRef(tok); ok {
				continue
			}
			if isNativeRef != nil && isNativeRef(tok) {
				continue
			}
			return true
		}
	}
	return false
}

func isSafeNativeValue(val string, allowBearer bool, isNativeRef func(string) bool) bool {
	if isNativeRef == nil {
		return false
	}
	if isNativeRef(val) {
		return true
	}
	if allowBearer && strings.HasPrefix(val, "Bearer ") && isNativeRef(strings.TrimPrefix(val, "Bearer ")) {
		return true
	}
	return false
}

func formatSecretWarning(agent, serverName, field, path string) string {
	return fmt.Sprintf("warning: %s MCP server %q field %s in %s appears to contain a literal secret; replace it with an environment variable reference", agent, serverName, field, path)
}

func scanURLForSecretWarnings(agent, serverName, fieldName, rawURL, path string, isNativeRef func(string) bool) []string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	var warnings []string
	if parsed.User != nil {
		warnings = append(warnings, formatSecretWarning(agent, serverName, fieldName, path))
	}
	query := parsed.Query()
	for _, qk := range sortedQueryKeys(query) {
		for _, qv := range query[qk] {
			if isSafeNativeValue(qv, false, isNativeRef) {
				continue
			}
			if (isSecretKey(qk) && strings.TrimSpace(qv) != "") || looksLikeSecretValue(qv, isNativeRef) {
				warnings = append(warnings, formatSecretWarning(agent, serverName, fieldName+".query."+qk, path))
				break
			}
		}
	}
	return warnings
}

func scanJSONMCPServers(agent, path string, serversObj *jsonObject, isNativeRef func(string) bool) []string {
	if serversObj == nil {
		return nil
	}
	var warnings []string
	names := make([]string, 0, len(serversObj.Entries))
	byName := make(map[string]*jsonObject, len(serversObj.Entries))
	for _, entry := range serversObj.Entries {
		if obj, ok := entry.Value.(*jsonObject); ok {
			names = append(names, entry.Key)
			byName[entry.Key] = obj
		}
	}
	sort.Strings(names)
	for _, srvName := range names {
		obj := byName[srvName]
		for _, envKey := range []string{"env", "environment"} {
			if rawEnv, ok := obj.Get(envKey); ok {
				if envObj, isObj := rawEnv.(*jsonObject); isObj {
					warnings = append(warnings, scanJSONKVMap(agent, srvName, envKey, path, envObj, false, isNativeRef)...)
				}
			}
		}
		if rawHdrs, ok := obj.Get("headers"); ok {
			if hdrObj, isObj := rawHdrs.(*jsonObject); isObj {
				warnings = append(warnings, scanJSONKVMap(agent, srvName, "headers", path, hdrObj, true, isNativeRef)...)
			}
		}
		for _, argField := range []string{"args", "command"} {
			if rawArr, ok := obj.Get(argField); ok {
				if arr, isArr := rawArr.(*jsonArray); isArr {
					for i, item := range arr.Items {
						if raw, isRaw := item.(jsonRaw); isRaw {
							if s, isStr := raw.asString(); isStr && !isSafeNativeValue(s, false, isNativeRef) && looksLikeSecretValue(s, isNativeRef) {
								warnings = append(warnings, formatSecretWarning(agent, srvName, fmt.Sprintf("%s[%d]", argField, i), path))
							}
						}
					}
				}
			}
		}
		for _, urlField := range []string{"url", "httpUrl"} {
			if rawURL, ok := obj.Get(urlField); ok {
				if raw, isRaw := rawURL.(jsonRaw); isRaw {
					if s, isStr := raw.asString(); isStr {
						warnings = append(warnings, scanURLForSecretWarnings(agent, srvName, urlField, s, path, isNativeRef)...)
					}
				}
			}
		}
	}
	return warnings
}

func scanJSONKVMap(agent, srvName, section, path string, kv *jsonObject, isHeaders bool, isNativeRef func(string) bool) []string {
	keys := make([]string, 0, len(kv.Entries))
	vals := make(map[string]string, len(kv.Entries))
	for _, e := range kv.Entries {
		if raw, ok := e.Value.(jsonRaw); ok {
			if s, isStr := raw.asString(); isStr {
				keys = append(keys, e.Key)
				vals[e.Key] = s
			}
		}
	}
	sort.Strings(keys)
	var warnings []string
	for _, k := range keys {
		v := vals[k]
		allowBearer := isHeaders && strings.EqualFold(k, "Authorization")
		if isSafeNativeValue(v, allowBearer, isNativeRef) {
			continue
		}
		if (isSecretKey(k) && strings.TrimSpace(v) != "") || looksLikeSecretValue(v, isNativeRef) {
			warnings = append(warnings, formatSecretWarning(agent, srvName, section+"."+k, path))
		}
	}
	return warnings
}

func scanCodexMCPServers(path string, parsed map[string]any) []string {
	rawServers, ok := parsed["mcp_servers"]
	if !ok {
		return nil
	}
	serversMap, ok := rawServers.(map[string]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(serversMap))
	for k := range serversMap {
		names = append(names, k)
	}
	sort.Strings(names)

	var warnings []string
	for _, srvName := range names {
		tbl, ok := serversMap[srvName].(map[string]any)
		if !ok {
			continue
		}
		// Codex does not expand env references inside `env` or `http_headers`;
		// any secret-looking key or secret-shaped value there is a literal secret.
		for _, section := range []string{"env", "http_headers"} {
			rawMap, ok := tbl[section].(map[string]any)
			if !ok {
				continue
			}
			keys := make([]string, 0, len(rawMap))
			for k := range rawMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v, isStr := rawMap[k].(string)
				if !isStr {
					continue
				}
				if (isSecretKey(k) && strings.TrimSpace(v) != "") || looksLikeSecretValue(v, nil) {
					warnings = append(warnings, formatSecretWarning(AgentCodex, srvName, section+"."+k, path))
				}
			}
		}
		if rawArgs, ok := tbl["args"].([]any); ok {
			for i, item := range rawArgs {
				if s, isStr := item.(string); isStr && looksLikeSecretValue(s, nil) {
					warnings = append(warnings, formatSecretWarning(AgentCodex, srvName, fmt.Sprintf("args[%d]", i), path))
				}
			}
		}
		if rawURL, ok := tbl["url"].(string); ok {
			warnings = append(warnings, scanURLForSecretWarnings(AgentCodex, srvName, "url", rawURL, path, nil)...)
		}
	}
	return warnings
}

func renderRefValue(val string, formatVar func(name string) string) string {
	if name, ok := parseEnvRef(val); ok {
		return formatVar(name)
	}
	if name, ok := parseBearerEnvRef(val); ok {
		return "Bearer " + formatVar(name)
	}
	return val
}

func renderDollarBraceVar(name string) string { return "${" + name + "}" }
func renderOpenCodeVar(name string) string    { return "{env:" + name + "}" }

// --- Claude Code MCP ---

func resolveClaudeMCPPath(home string, getenv func(string) string) string {
	if dir := strings.TrimSpace(getenv("CLAUDE_CONFIG_DIR")); dir != "" {
		return filepath.Join(filepath.Clean(dir), ".claude.json")
	}
	return filepath.Join(home, ".claude.json")
}

func renderClaudeMCPServerObject(srv MCPServerSpec) *jsonObject {
	obj := &jsonObject{}
	switch srv.Transport {
	case MCPTransportStdio:
		obj.Set("type", newJSONString("stdio"))
		obj.Set("command", newJSONString(srv.Command))
		args := srv.Args
		if args == nil {
			args = []string{}
		}
		obj.SetStringArray("args", args)
		if len(srv.Env) > 0 {
			envObj := &jsonObject{}
			for _, k := range sortedMapKeys(srv.Env) {
				envObj.Set(k, newJSONString(renderRefValue(srv.Env[k], renderDollarBraceVar)))
			}
			obj.Set("env", envObj)
		}
	case MCPTransportHTTP:
		obj.Set("type", newJSONString("http"))
		obj.Set("url", newJSONString(srv.URL))
		if len(srv.Headers) > 0 {
			hdrObj := &jsonObject{}
			for _, k := range sortedMapKeys(srv.Headers) {
				hdrObj.Set(k, newJSONString(renderRefValue(srv.Headers[k], renderDollarBraceVar)))
			}
			obj.Set("headers", hdrObj)
		}
	}
	return obj
}

func marshalCompactJSON(obj *jsonObject) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(obj.MarshalIndent())); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// claudeServerSemanticEqual compares an existing Claude mcpServers entry with a
// DevEx-rendered entry, treating omitted default fields (type:"stdio" when
// command is set, empty args/env/headers) as equivalent.
func claudeServerSemanticEqual(existing jsonValue, desired *jsonObject) bool {
	exObj, ok := existing.(*jsonObject)
	if !ok {
		return false
	}
	return jsonSemanticEqual(normalizeClaudeServerObj(exObj), normalizeClaudeServerObj(desired))
}

func normalizeClaudeServerObj(in *jsonObject) *jsonObject {
	out := in.Clone()
	if _, hasType := out.Get("type"); !hasType {
		if _, hasCmd := out.Get("command"); hasCmd {
			out.Set("type", newJSONString("stdio"))
		}
	}
	if v, ok := out.Get("args"); ok {
		if arr, isArr := v.(*jsonArray); isArr && len(arr.Items) == 0 {
			out.Delete("args")
		}
	}
	for _, mapKey := range []string{"env", "headers"} {
		if v, ok := out.Get(mapKey); ok {
			if sub, isObj := v.(*jsonObject); isObj && len(sub.Entries) == 0 {
				out.Delete(mapKey)
			}
		}
	}
	return out
}

func planClaudeMCP(
	servers []MCPServerSpec,
	home string,
	env Env,
	prev AgentManagedState,
) (diffPlan *MCPDiffPlan, actions []MCPCommandAction, notice string, ownedMCP []string, warnings []string, err error) {
	claudeJSONPath := resolveClaudeMCPPath(home, env.Getenv)
	beforeRaw, exists, mode, readErr := readExistingFile(claudeJSONPath, settingsFileMode)
	if readErr != nil {
		return nil, nil, "", nil, nil, readErr
	}

	var existingMCP *jsonObject
	if exists && strings.TrimSpace(beforeRaw) != "" {
		doc, parseErr := ParseJSONObject([]byte(beforeRaw), claudeJSONPath)
		if parseErr != nil {
			if len(servers) == 0 && len(prev.MCPServers) == 0 {
				return nil, nil, "", nil, nil, nil
			}
			return nil, nil, "", nil, nil, parseErr
		}
		if rawMCP, ok := doc.Get("mcpServers"); ok {
			obj, isObj := rawMCP.(*jsonObject)
			if !isObj {
				return nil, nil, "", nil, nil, fmt.Errorf("%s: key \"mcpServers\" must be a JSON object", claudeJSONPath)
			}
			existingMCP = obj
		}
	}

	warnings = scanJSONMCPServers(AgentClaude, claudeJSONPath, existingMCP, claudeNativeRefRe.MatchString)

	if len(servers) == 0 && len(prev.MCPServers) == 0 {
		return nil, nil, "", nil, warnings, nil
	}

	claudeBin, lookErr := env.LookPath("claude")
	if lookErr != nil {
		notice = "skipped (claude binary not found on PATH; install Claude Code and re-run 'devex agent apply' to manage Claude MCP servers): claude mcp"
		return nil, nil, notice, slices.Clone(prev.MCPServers), warnings, nil
	}

	if existingMCP == nil {
		existingMCP = &jsonObject{}
	}
	beforeMCP := existingMCP.Clone()
	afterMCP := existingMCP.Clone()

	desiredByName := make(map[string]MCPServerSpec, len(servers))
	desiredOrder := make([]string, 0, len(servers))
	for _, srv := range servers {
		desiredByName[srv.Name] = srv
		desiredOrder = append(desiredOrder, srv.Name)
	}

	// 1. Plan removals for previously managed servers no longer declared.
	for _, prevName := range prev.MCPServers {
		if _, stillWanted := desiredByName[prevName]; stillWanted {
			continue
		}
		if _, present := afterMCP.Get(prevName); present {
			afterMCP.Delete(prevName)
			actions = append(actions, MCPCommandAction{
				ServerName: prevName,
				Kind:       "remove",
				Binary:     claudeBin,
				Args:       []string{"mcp", "remove", "--scope", "user", prevName},
			})
		}
	}

	// 2. Plan additions and updates for desired servers.
	ownedMCP = make([]string, 0, len(desiredOrder))
	for _, name := range desiredOrder {
		srv := desiredByName[name]
		desiredObj := renderClaudeMCPServerObject(srv)
		existingVal, present := beforeMCP.Get(name)
		wasManaged := slices.Contains(prev.MCPServers, name)

		if present {
			if claudeServerSemanticEqual(existingVal, desiredObj) {
				// Semantically identical: adopt if unmanaged, no command needed.
				ownedMCP = append(ownedMCP, name)
				continue
			}
			if !wasManaged {
				return nil, nil, "", nil, warnings, fmt.Errorf("MCP server %q already exists in %s and is not managed by DevEx (remove it or match its configuration)", name, claudeJSONPath)
			}
			afterMCP.Set(name, desiredObj)
			compact, compactErr := marshalCompactJSON(desiredObj)
			if compactErr != nil {
				return nil, nil, "", nil, warnings, fmt.Errorf("encode Claude MCP server %q: %w", name, compactErr)
			}
			actions = append(actions,
				MCPCommandAction{
					ServerName: name,
					Kind:       "remove",
					Binary:     claudeBin,
					Args:       []string{"mcp", "remove", "--scope", "user", name},
				},
				MCPCommandAction{
					ServerName: name,
					Kind:       "add",
					Binary:     claudeBin,
					Args:       []string{"mcp", "add-json", "--scope", "user", name, compact},
				},
			)
			ownedMCP = append(ownedMCP, name)
			continue
		}

		afterMCP.Set(name, desiredObj)
		compact, compactErr := marshalCompactJSON(desiredObj)
		if compactErr != nil {
			return nil, nil, "", nil, warnings, fmt.Errorf("encode Claude MCP server %q: %w", name, compactErr)
		}
		actions = append(actions, MCPCommandAction{
			ServerName: name,
			Kind:       "add",
			Binary:     claudeBin,
			Args:       []string{"mcp", "add-json", "--scope", "user", name, compact},
		})
		ownedMCP = append(ownedMCP, name)
	}

	action := actionUnchanged
	if len(actions) > 0 {
		if exists {
			action = actionUpdate
		} else {
			action = actionCreate
		}
	}
	diffPlan = &MCPDiffPlan{
		Agent:  AgentClaude,
		Path:   claudeJSONPath,
		Action: action,
		Before: beforeMCP.MarshalIndent(),
		After:  afterMCP.MarshalIndent(),
		Exists: exists,
		Mode:   uint32(mode),
	}
	return diffPlan, actions, "", ownedMCP, warnings, nil
}

// --- Codex MCP ---

// CodexMCPTable holds the structured Codex [mcp_servers.<name>] configuration.
type CodexMCPTable struct {
	Command           string
	Args              []string
	EnvVars           []string
	Env               map[string]string
	URL               string
	BearerTokenEnvVar string
	EnvHTTPHeaders    map[string]string
	HTTPHeaders       map[string]string
}

func buildCodexMCPTable(srv MCPServerSpec) (CodexMCPTable, error) {
	var tbl CodexMCPTable
	switch srv.Transport {
	case MCPTransportStdio:
		tbl.Command = srv.Command
		tbl.Args = slices.Clone(srv.Args)
		for _, k := range sortedMapKeys(srv.Env) {
			v := srv.Env[k]
			if refVar, isRef := parseEnvRef(v); isRef {
				if refVar != k {
					return CodexMCPTable{}, fmt.Errorf("MCP server %q env.%s references ${env:%s}, which cannot be expressed safely in Codex (Codex env_vars only forwards an environment variable under its own name)", srv.Name, k, refVar)
				}
				tbl.EnvVars = append(tbl.EnvVars, refVar)
				continue
			}
			if tbl.Env == nil {
				tbl.Env = map[string]string{}
			}
			tbl.Env[k] = v
		}
	case MCPTransportHTTP:
		tbl.URL = srv.URL
		for _, k := range sortedMapKeys(srv.Headers) {
			v := srv.Headers[k]
			if strings.EqualFold(k, "Authorization") {
				if refVar, isBearer := parseBearerEnvRef(v); isBearer {
					tbl.BearerTokenEnvVar = refVar
					continue
				}
			}
			if refVar, isRef := parseEnvRef(v); isRef {
				if tbl.EnvHTTPHeaders == nil {
					tbl.EnvHTTPHeaders = map[string]string{}
				}
				tbl.EnvHTTPHeaders[k] = refVar
				continue
			}
			if tbl.HTTPHeaders == nil {
				tbl.HTTPHeaders = map[string]string{}
			}
			tbl.HTTPHeaders[k] = v
		}
	default:
		return CodexMCPTable{}, fmt.Errorf("MCP server %q: unsupported transport %q", srv.Name, srv.Transport)
	}
	return tbl, nil
}

// --- Gemini CLI MCP ---

func renderGeminiMCPServerObject(srv MCPServerSpec) *jsonObject {
	obj := &jsonObject{}
	switch srv.Transport {
	case MCPTransportStdio:
		obj.Set("command", newJSONString(srv.Command))
		if len(srv.Args) > 0 {
			obj.SetStringArray("args", srv.Args)
		}
		if len(srv.Env) > 0 {
			envObj := &jsonObject{}
			for _, k := range sortedMapKeys(srv.Env) {
				envObj.Set(k, newJSONString(renderRefValue(srv.Env[k], renderDollarBraceVar)))
			}
			obj.Set("env", envObj)
		}
	case MCPTransportHTTP:
		obj.Set("httpUrl", newJSONString(srv.URL))
		if len(srv.Headers) > 0 {
			hdrObj := &jsonObject{}
			for _, k := range sortedMapKeys(srv.Headers) {
				hdrObj.Set(k, newJSONString(renderRefValue(srv.Headers[k], renderDollarBraceVar)))
			}
			obj.Set("headers", hdrObj)
		}
	}
	return obj
}

// --- OpenCode MCP ---

func renderOpenCodeMCPServerObject(srv MCPServerSpec) *jsonObject {
	obj := &jsonObject{}
	switch srv.Transport {
	case MCPTransportStdio:
		obj.Set("type", newJSONString("local"))
		cmdSlice := make([]string, 0, 1+len(srv.Args))
		cmdSlice = append(cmdSlice, srv.Command)
		cmdSlice = append(cmdSlice, srv.Args...)
		obj.SetStringArray("command", cmdSlice)
		if len(srv.Env) > 0 {
			envObj := &jsonObject{}
			for _, k := range sortedMapKeys(srv.Env) {
				envObj.Set(k, newJSONString(renderRefValue(srv.Env[k], renderOpenCodeVar)))
			}
			obj.Set("environment", envObj)
		}
		obj.Set("enabled", newJSONBool(true))
	case MCPTransportHTTP:
		obj.Set("type", newJSONString("remote"))
		obj.Set("url", newJSONString(srv.URL))
		if len(srv.Headers) > 0 {
			hdrObj := &jsonObject{}
			for _, k := range sortedMapKeys(srv.Headers) {
				hdrObj.Set(k, newJSONString(renderRefValue(srv.Headers[k], renderOpenCodeVar)))
			}
			obj.Set("headers", hdrObj)
		}
		obj.Set("enabled", newJSONBool(true))
	}
	return obj
}

// mergeJSONMCPServers reconciles DevEx-managed MCP server entries inside a JSON
// object key (e.g. "mcpServers" for Gemini or "mcp" for OpenCode) while
// preserving unmanaged entries, checking conflicts, adopting identical entries,
// and scanning existing entries for literal secrets.
func mergeJSONMCPServers(
	agent, settingsPath, rootKey string,
	doc *jsonObject,
	servers []MCPServerSpec,
	prevManaged []string,
	renderServer func(MCPServerSpec) *jsonObject,
	isNativeRef func(string) bool,
) (owned []string, warnings []string, err error) {
	var existingMap *jsonObject
	if raw, ok := doc.Get(rootKey); ok {
		obj, isObj := raw.(*jsonObject)
		if !isObj {
			return nil, nil, fmt.Errorf("%s: key %q must be a JSON object", settingsPath, rootKey)
		}
		existingMap = obj
		warnings = scanJSONMCPServers(agent, settingsPath, existingMap, isNativeRef)
	}

	if len(servers) == 0 && len(prevManaged) == 0 {
		return nil, warnings, nil
	}

	desiredByName := make(map[string]MCPServerSpec, len(servers))
	desiredOrder := make([]string, 0, len(servers))
	for _, srv := range servers {
		desiredByName[srv.Name] = srv
		desiredOrder = append(desiredOrder, srv.Name)
	}

	if existingMap == nil && len(desiredOrder) > 0 {
		existingMap, err = doc.EnsureObject(rootKey)
		if err != nil {
			return nil, warnings, fmt.Errorf("%s: %w", settingsPath, err)
		}
	}

	if existingMap != nil {
		for _, prevName := range prevManaged {
			if _, want := desiredByName[prevName]; !want {
				existingMap.Delete(prevName)
			}
		}
	}

	owned = make([]string, 0, len(desiredOrder))
	for _, name := range desiredOrder {
		desiredObj := renderServer(desiredByName[name])
		existingVal, present := existingMap.Get(name)
		wasManaged := slices.Contains(prevManaged, name)

		if present {
			if jsonSemanticEqual(existingVal, desiredObj) {
				owned = append(owned, name)
				continue
			}
			if !wasManaged {
				return nil, warnings, fmt.Errorf("MCP server %q already exists in %s and is not managed by DevEx (remove it or match its configuration)", name, settingsPath)
			}
		}
		existingMap.Set(name, desiredObj)
		owned = append(owned, name)
	}

	// Clean up empty container key if we removed the last managed server and no
	// unmanaged servers remain.
	if existingMap != nil && len(existingMap.Entries) == 0 && len(desiredOrder) == 0 && len(prevManaged) > 0 {
		doc.Delete(rootKey)
	}

	return owned, warnings, nil
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedQueryKeys(q url.Values) []string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
