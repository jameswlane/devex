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
	"strconv"
	"strings"
)

var (
	mcpServerNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]{0,63}$`)
	mcpEnvVarNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	mcpEnvRefRe     = regexp.MustCompile(`^\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}$`)
	mcpHeaderNameRe = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`)

	claudeNativeRefRe   = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*(?::-[^}]*)?\}$`)
	geminiNativeRefRe   = regexp.MustCompile(`^(?:\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*|%[A-Za-z_][A-Za-z0-9_]*%)$`)
	opencodeNativeRefRe = regexp.MustCompile(`^\{env:[A-Za-z_][A-Za-z0-9_]*\}$`)

	secretArgFlags = map[string]bool{
		"--token":        true,
		"--api-key":      true,
		"--apikey":       true,
		"--password":     true,
		"--secret":       true,
		"--auth-token":   true,
		"--access-token": true,
	}

	secretPATRe    = regexp.MustCompile(`(^|[^A-Z0-9])PAT([^A-Z0-9]|$)`)
	secretValueRes = []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])((?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{6,})`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(github_pat_[A-Za-z0-9_]{6,})`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(glpat-[A-Za-z0-9_-]{6,})`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(sk-ant-[A-Za-z0-9_-]{6,})`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(sk-[A-Za-z0-9_-]{12,})`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(xox[abposr]-[A-Za-z0-9-]{6,})`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(AKIA[0-9A-Z]{16})(?:[^0-9A-Z]|$)`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(AIza[0-9A-Za-z_-]{16,})`),
	}
	bearerTokenRe         = regexp.MustCompile(`(?i)(?:^|\s)Bearer\s+(\S+)`)
	diffBearerTokenRe     = regexp.MustCompile(`(?i)(?:^|[\s"'])Bearer\s+(\$\{[^}\s"']+\}|\{env:[^}\s"']+\}|[^\s"',}\]]+)`)
	diffJSONKVRe          = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"(\s*:\s*)"((?:[^"\\]|\\.)*)"`)
	diffTOMLKVRe          = regexp.MustCompile(`(?:"((?:[^"\\]|\\.)*)"|'([^']*)'|([A-Za-z0-9_-]+))(\s*=\s*)(?:"((?:[^"\\]|\\.)*)"|'([^']*)')`)
	diffURLUserinfoRe     = regexp.MustCompile(`(https?://)([^/\s"'@]+)(@)`)
	diffURLQueryParamRe   = regexp.MustCompile(`([?&])([A-Za-z0-9_.-]+)=([^&\s"']+)`)
	diffSecretArgFlagEqRe = regexp.MustCompile(`(?i)(--(?:token|api-key|apikey|password|secret|auth-token|access-token)=)([^\s"',}\]]+)`)
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
			return fmt.Errorf("mcp_servers[%d]: name %q must match ^[A-Za-z0-9_][A-Za-z0-9_-]{0,63}$", i, srv.Name)
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
	for i := 0; i < len(srv.Args); i++ {
		arg := srv.Args[i]
		if hasControlChars(arg) {
			return fmt.Errorf("mcp_servers[%q].args[%d]: must not contain control characters", label, i)
		}
		if flagPart, valPart, hasEq := strings.Cut(arg, "="); hasEq && secretArgFlags[strings.ToLower(flagPart)] && strings.TrimSpace(valPart) != "" {
			return fmt.Errorf("mcp_servers[%q].args[%d]: passes a secret via %s; pass secrets via env references (${env:VAR_NAME}) in env instead", label, i, flagPart)
		}
		if secretArgFlags[strings.ToLower(arg)] && i+1 < len(srv.Args) && strings.TrimSpace(srv.Args[i+1]) != "" {
			if hasControlChars(srv.Args[i+1]) {
				return fmt.Errorf("mcp_servers[%q].args[%d]: must not contain control characters", label, i+1)
			}
			return fmt.Errorf("mcp_servers[%q].args[%d]: passes a secret via %s; pass secrets via env references (${env:VAR_NAME}) in env instead", label, i+1, arg)
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
		"AUTHORIZATION",
		"AUTH_TOKEN",
		"COOKIE",
	} {
		if strings.Contains(upper, needle) {
			return true
		}
	}
	if secretPATRe.MatchString(upper) {
		return true
	}
	return hasSecretAuthToken(upper)
}

func hasSecretAuthToken(upper string) bool {
	tokens := strings.Split(upper, "_")
	for i, tok := range tokens {
		if tok != "AUTH" {
			continue
		}
		if i+1 < len(tokens) {
			switch tokens[i+1] {
			case "MODE", "TYPE", "METHOD", "PROVIDER", "URL", "ENABLED", "DISABLED":
				continue
			}
		}
		return true
	}
	return false
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
					argStrings := make([]string, len(arr.Items))
					isStr := make([]bool, len(arr.Items))
					for i, item := range arr.Items {
						if raw, isRaw := item.(jsonRaw); isRaw {
							if s, ok := raw.asString(); ok {
								argStrings[i] = s
								isStr[i] = true
							}
						}
					}
					for _, idx := range scanArgSliceForSecrets(argStrings, isStr, isNativeRef) {
						warnings = append(warnings, formatSecretWarning(agent, srvName, fmt.Sprintf("%s[%d]", argField, idx), path))
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
			argStrings := make([]string, len(rawArgs))
			isStr := make([]bool, len(rawArgs))
			for i, item := range rawArgs {
				if s, ok := item.(string); ok {
					argStrings[i] = s
					isStr[i] = true
				}
			}
			for _, idx := range scanArgSliceForSecrets(argStrings, isStr, nil) {
				warnings = append(warnings, formatSecretWarning(AgentCodex, srvName, fmt.Sprintf("args[%d]", idx), path))
			}
		}
		if rawURL, ok := tbl["url"].(string); ok {
			warnings = append(warnings, scanURLForSecretWarnings(AgentCodex, srvName, "url", rawURL, path, nil)...)
		}
	}
	return warnings
}

func scanArgSliceForSecrets(args []string, isStr []bool, isNativeRef func(string) bool) []int {
	var indices []int
	for i := 0; i < len(args); i++ {
		if !isStr[i] {
			continue
		}
		s := args[i]
		if flagPart, valPart, hasEq := strings.Cut(s, "="); hasEq && secretArgFlags[strings.ToLower(flagPart)] && strings.TrimSpace(valPart) != "" {
			if !isSafeNativeValue(valPart, false, isNativeRef) {
				indices = append(indices, i)
			}
			continue
		}
		if secretArgFlags[strings.ToLower(s)] && i+1 < len(args) && isStr[i+1] && strings.TrimSpace(args[i+1]) != "" {
			if !isSafeNativeValue(args[i+1], false, isNativeRef) {
				indices = append(indices, i+1)
			}
			i++
			continue
		}
		if !isSafeNativeValue(s, false, isNativeRef) && looksLikeSecretValue(s, isNativeRef) {
			indices = append(indices, i)
		}
	}
	return indices
}

func isAnySafeEnvRef(val string, allowBearer bool) bool {
	isRef := func(s string) bool {
		if _, ok := parseEnvRef(s); ok {
			return true
		}
		return claudeNativeRefRe.MatchString(s) ||
			geminiNativeRefRe.MatchString(s) ||
			opencodeNativeRefRe.MatchString(s)
	}
	if isRef(val) {
		return true
	}
	if allowBearer && len(val) > len("Bearer ") && strings.EqualFold(val[:len("Bearer ")], "Bearer ") {
		return isRef(val[len("Bearer "):])
	}
	return false
}

// redactDiffContent replaces detected credentials in diff display content with
// "***", including context lines, while leaving safe environment variable
// references intact. It is only used for display diffs, never for written files.
func redactDiffContent(content string) string {
	if content == "" {
		return ""
	}
	content = redactDiffSecretArgArrays(content)
	lines := splitLinesPreserve(content)
	for i, line := range lines {
		lines[i] = redactDiffLine(line)
	}
	return strings.Join(lines, "")
}

type diffStringEdit struct {
	start int
	end   int
	repl  string
}

type diffArrayElem struct {
	isStr      bool
	val        string
	innerStart int
	innerEnd   int
}

// redactDiffSecretArgArrays scans JSON and TOML arrays (single-line or
// multi-line) and reuses scanArgSliceForSecrets to redact both `--flag=literal`
// and separate `["--flag", "literal"]` argument elements while preserving safe
// environment variable references.
func redactDiffSecretArgArrays(content string) string {
	var edits []diffStringEdit
	isSafeRef := func(s string) bool { return isAnySafeEnvRef(s, false) }

	pos := 0
	for pos < len(content) {
		switch content[pos] {
		case '#':
			pos = skipToEndOfLine(content, pos)
		case '"', '\'':
			if _, _, _, next, ok := scanQuotedString(content, pos); ok {
				pos = next
			} else {
				pos++
			}
		case '[':
			pos = scanDiffArray(content, pos, isSafeRef, &edits)
		default:
			pos++
		}
	}

	if len(edits) == 0 {
		return content
	}
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].start < edits[j].start
	})
	var b strings.Builder
	last := 0
	for _, e := range edits {
		if e.start < last || e.start > e.end {
			continue
		}
		b.WriteString(content[last:e.start])
		b.WriteString(e.repl)
		last = e.end
	}
	b.WriteString(content[last:])
	return b.String()
}

func skipToEndOfLine(s string, pos int) int {
	for pos < len(s) && s[pos] != '\n' {
		pos++
	}
	return pos
}

func skipDiffWSAndComments(s string, pos int) int {
	for pos < len(s) {
		switch s[pos] {
		case ' ', '\t', '\r', '\n':
			pos++
		case '#':
			pos = skipToEndOfLine(s, pos)
		default:
			return pos
		}
	}
	return pos
}

func scanQuotedString(s string, pos int) (val string, innerStart, innerEnd, next int, ok bool) {
	if pos >= len(s) {
		return "", 0, 0, pos, false
	}
	if strings.HasPrefix(s[pos:], `"""`) {
		innerStart = pos + 3
		for i := innerStart; i < len(s); {
			if s[i] == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if strings.HasPrefix(s[i:], `"""`) {
				innerEnd = i
				raw := strings.TrimPrefix(s[innerStart:innerEnd], "\r\n")
				raw = strings.TrimPrefix(raw, "\n")
				return raw, innerStart, innerEnd, i + 3, true
			}
			i++
		}
		return "", 0, 0, pos, false
	}
	if strings.HasPrefix(s[pos:], `'''`) {
		innerStart = pos + 3
		endRel := strings.Index(s[innerStart:], `'''`)
		if endRel < 0 {
			return "", 0, 0, pos, false
		}
		innerEnd = innerStart + endRel
		raw := strings.TrimPrefix(s[innerStart:innerEnd], "\r\n")
		raw = strings.TrimPrefix(raw, "\n")
		return raw, innerStart, innerEnd, innerEnd + 3, true
	}
	if s[pos] == '"' {
		innerStart = pos + 1
		for i := innerStart; i < len(s) && s[i] != '\n'; {
			if s[i] == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if s[i] == '"' {
				innerEnd = i
				unq, err := strconv.Unquote(s[pos : i+1])
				if err != nil {
					unq = s[innerStart:innerEnd]
				}
				return unq, innerStart, innerEnd, i + 1, true
			}
			i++
		}
		return "", 0, 0, pos, false
	}
	if s[pos] == '\'' {
		innerStart = pos + 1
		for i := innerStart; i < len(s) && s[i] != '\n'; i++ {
			if s[i] == '\'' {
				innerEnd = i
				return s[innerStart:innerEnd], innerStart, innerEnd, i + 1, true
			}
		}
		return "", 0, 0, pos, false
	}
	return "", 0, 0, pos, false
}

func scanDiffArray(s string, openBracket int, isSafeRef func(string) bool, edits *[]diffStringEdit) int {
	pos := openBracket + 1
	var elems []diffArrayElem
	closed := false

	for pos < len(s) {
		pos = skipDiffWSAndComments(s, pos)
		if pos >= len(s) {
			break
		}
		if s[pos] == ']' {
			pos++
			closed = true
			break
		}
		if s[pos] == ',' {
			pos++
			continue
		}

		if s[pos] == '"' || s[pos] == '\'' {
			if val, innerStart, innerEnd, next, ok := scanQuotedString(s, pos); ok {
				afterStr := skipDiffWSAndComments(s, next)
				if afterStr < len(s) && (s[afterStr] == ',' || s[afterStr] == ']') {
					elems = append(elems, diffArrayElem{
						isStr:      true,
						val:        val,
						innerStart: innerStart,
						innerEnd:   innerEnd,
					})
					pos = afterStr
					if s[pos] == ',' {
						pos++
					}
					continue
				}
			}
		}

		elems = append(elems, diffArrayElem{isStr: false})
		braceDepth := 0
		for pos < len(s) {
			ch := s[pos]
			if ch == '#' {
				pos = skipToEndOfLine(s, pos)
				continue
			}
			if ch == '"' || ch == '\'' {
				if _, _, _, next, ok := scanQuotedString(s, pos); ok {
					pos = next
					continue
				}
				pos++
				continue
			}
			if ch == '[' {
				pos = scanDiffArray(s, pos, isSafeRef, edits)
				continue
			}
			if ch == '{' {
				braceDepth++
				pos++
				continue
			}
			if ch == '}' {
				if braceDepth > 0 {
					braceDepth--
				}
				pos++
				continue
			}
			if braceDepth == 0 && (ch == ',' || ch == ']') {
				if ch == ',' {
					pos++
				}
				break
			}
			pos++
		}
	}

	if !closed || len(elems) == 0 {
		return pos
	}

	argStrings := make([]string, len(elems))
	isStr := make([]bool, len(elems))
	for i, el := range elems {
		argStrings[i] = el.val
		isStr[i] = el.isStr
	}

	for _, idx := range scanArgSliceForSecrets(argStrings, isStr, isSafeRef) {
		el := elems[idx]
		if !el.isStr {
			continue
		}
		if flagPart, _, hasEq := strings.Cut(el.val, "="); hasEq && secretArgFlags[strings.ToLower(flagPart)] {
			if eqOffset := strings.IndexByte(s[el.innerStart:el.innerEnd], '='); eqOffset >= 0 {
				*edits = append(*edits, diffStringEdit{
					start: el.innerStart + eqOffset + 1,
					end:   el.innerEnd,
					repl:  "***",
				})
				continue
			}
		}
		if idx > 0 && elems[idx-1].isStr && secretArgFlags[strings.ToLower(elems[idx-1].val)] {
			*edits = append(*edits, diffStringEdit{
				start: el.innerStart,
				end:   el.innerEnd,
				repl:  "***",
			})
		}
	}
	return pos
}

func redactDiffLine(line string) string {
	eol := lineEnding(line)
	body := strings.TrimSuffix(line, eol)
	trimmed := strings.TrimSpace(body)

	// Codex env-var-name-only fields hold env var names, not secret values.
	skipKeyRedaction := strings.HasPrefix(trimmed, "bearer_token_env_var") ||
		strings.HasPrefix(trimmed, "env_http_headers") ||
		strings.HasPrefix(trimmed, "env_vars")

	if !skipKeyRedaction {
		// 1. JSON "key": "value"
		body = replaceSubmatchIndexes(body, diffJSONKVRe, func(src string, idx []int) (int, int, string, bool) {
			key := src[idx[2]:idx[3]]
			val := src[idx[6]:idx[7]]
			if val == "" || !isSecretKey(key) || isAnySafeEnvRef(val, strings.EqualFold(key, "Authorization")) {
				return 0, 0, "", false
			}
			return idx[6], idx[7], "***", true
		})

		// 2. TOML key = "value" or key = 'value' (including inside inline tables)
		body = replaceSubmatchIndexes(body, diffTOMLKVRe, func(src string, idx []int) (int, int, string, bool) {
			key := firstMatchedGroup(src, idx, 1, 2, 3)
			valStart, valEnd := firstMatchedSpan(idx, 5, 6)
			if valStart < 0 {
				return 0, 0, "", false
			}
			val := src[valStart:valEnd]
			if val == "" || !isSecretKey(key) || isAnySafeEnvRef(val, strings.EqualFold(key, "Authorization")) {
				return 0, 0, "", false
			}
			return valStart, valEnd, "***", true
		})
	}

	// 3. URL embedded credentials and secret query parameters.
	body = diffURLUserinfoRe.ReplaceAllString(body, "${1}***${3}")
	body = replaceSubmatchIndexes(body, diffURLQueryParamRe, func(src string, idx []int) (int, int, string, bool) {
		key := src[idx[4]:idx[5]]
		val := src[idx[6]:idx[7]]
		if val == "" || !isSecretKey(key) || isAnySafeEnvRef(val, false) {
			return 0, 0, "", false
		}
		return idx[6], idx[7], "***", true
	})

	// 4. Secret CLI flags --flag=value.
	body = replaceSubmatchIndexes(body, diffSecretArgFlagEqRe, func(src string, idx []int) (int, int, string, bool) {
		val := src[idx[4]:idx[5]]
		if val == "" || isAnySafeEnvRef(val, false) {
			return 0, 0, "", false
		}
		return idx[4], idx[5], "***", true
	})

	// 5. Bearer <literal> tokens.
	body = replaceSubmatchIndexes(body, diffBearerTokenRe, func(src string, idx []int) (int, int, string, bool) {
		tok := src[idx[2]:idx[3]]
		if tok == "" || tok == "***" || isAnySafeEnvRef(tok, false) {
			return 0, 0, "", false
		}
		return idx[2], idx[3], "***", true
	})

	// 6. Known token-shape regexes (reusing secretValueRes).
	for _, re := range secretValueRes {
		body = replaceSubmatchIndexes(body, re, func(_ string, idx []int) (int, int, string, bool) {
			if len(idx) < 4 || idx[2] < 0 {
				return 0, 0, "", false
			}
			return idx[2], idx[3], "***", true
		})
	}

	return body + eol
}

func replaceSubmatchIndexes(s string, re *regexp.Regexp, fn func(src string, idx []int) (start, end int, repl string, ok bool)) string {
	matches := re.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	changed := false
	for _, idx := range matches {
		start, end, repl, ok := fn(s, idx)
		if !ok || start < last {
			continue
		}
		b.WriteString(s[last:start])
		b.WriteString(repl)
		last = end
		changed = true
	}
	if !changed {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

func firstMatchedGroup(s string, idx []int, groups ...int) string {
	start, end := firstMatchedSpan(idx, groups...)
	if start < 0 {
		return ""
	}
	return s[start:end]
}

func firstMatchedSpan(idx []int, groups ...int) (int, int) {
	for _, g := range groups {
		if 2*g+1 < len(idx) && idx[2*g] >= 0 {
			return idx[2*g], idx[2*g+1]
		}
	}
	return -1, -1
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
) (diffPlan *MCPDiffPlan, actions []MCPCommandAction, notice string, ownedMCP []string, resolvedPath string, skipped bool, warnings []string, err error) {
	claudeJSONPath := resolveClaudeMCPPath(home, env.Getenv)
	resolvedPath, err = resolveCanonicalWriteTarget(claudeJSONPath)
	if err != nil {
		return nil, nil, "", nil, "", false, nil, err
	}
	prevMCP := prevMCPServersForPath(prev, resolvedPath)

	beforeRaw, exists, mode, readErr := readExistingFile(claudeJSONPath, settingsFileMode)
	if readErr != nil {
		return nil, nil, "", nil, resolvedPath, false, nil, readErr
	}

	var existingMCP *jsonObject
	if exists && strings.TrimSpace(beforeRaw) != "" {
		doc, parseErr := ParseJSONObject([]byte(beforeRaw), claudeJSONPath)
		if parseErr != nil {
			if len(servers) == 0 && len(prevMCP) == 0 {
				return nil, nil, "", nil, resolvedPath, false, nil, nil
			}
			return nil, nil, "", nil, resolvedPath, false, nil, parseErr
		}
		if rawMCP, ok := doc.Get("mcpServers"); ok {
			obj, isObj := rawMCP.(*jsonObject)
			if !isObj {
				return nil, nil, "", nil, resolvedPath, false, nil, fmt.Errorf("%s: key \"mcpServers\" must be a JSON object", claudeJSONPath)
			}
			existingMCP = obj
		}
	}

	warnings = scanJSONMCPServers(AgentClaude, claudeJSONPath, existingMCP, claudeNativeRefRe.MatchString)

	if len(servers) == 0 && len(prevMCP) == 0 {
		return nil, nil, "", nil, resolvedPath, false, warnings, nil
	}

	claudeBin, lookErr := env.LookPath("claude")
	if lookErr != nil {
		notice = "skipped (claude binary not found on PATH; install Claude Code and re-run 'devex agent apply' to manage Claude MCP servers): claude mcp"
		return nil, nil, notice, nil, resolvedPath, true, warnings, nil
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
	for _, prevName := range prevMCP {
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
		wasManaged := slices.Contains(prevMCP, name)

		if present {
			if claudeServerSemanticEqual(existingVal, desiredObj) {
				// Semantically identical: adopt if unmanaged, no command needed.
				ownedMCP = append(ownedMCP, name)
				continue
			}
			if !wasManaged {
				return nil, nil, "", nil, resolvedPath, false, warnings, fmt.Errorf("MCP server %q already exists in %s and is not managed by DevEx (remove it or match its configuration)", name, claudeJSONPath)
			}
			afterMCP.Set(name, desiredObj)
			compact, compactErr := marshalCompactJSON(desiredObj)
			if compactErr != nil {
				return nil, nil, "", nil, resolvedPath, false, warnings, fmt.Errorf("encode Claude MCP server %q: %w", name, compactErr)
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
			return nil, nil, "", nil, resolvedPath, false, warnings, fmt.Errorf("encode Claude MCP server %q: %w", name, compactErr)
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
	return diffPlan, actions, "", ownedMCP, resolvedPath, false, warnings, nil
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
	return renderOpenCodeMCPServerObjectWithMode(srv, false)
}

func renderOpenCodeV2MCPServerObject(srv MCPServerSpec) *jsonObject {
	return renderOpenCodeMCPServerObjectWithMode(srv, true)
}

func renderOpenCodeMCPServerObjectWithMode(srv MCPServerSpec, v2 bool) *jsonObject {
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
		if v2 {
			obj.Set("disabled", newJSONBool(false))
		} else {
			obj.Set("enabled", newJSONBool(true))
		}
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
		if v2 {
			obj.Set("disabled", newJSONBool(false))
		} else {
			obj.Set("enabled", newJSONBool(true))
		}
	}
	return obj
}

// mergeOpenCodeMCPServers reconciles DevEx-managed MCP servers for OpenCode,
// supporting both the legacy `mcp.<name>` shape (with `enabled: true`) and the
// OpenCode v2 `mcp.servers.<name>` shape (with `disabled: false`). When both
// opencode.json and opencode.jsonc exist, it reports a conflict if a desired
// server name already exists in the non-target file.
func mergeOpenCodeMCPServers(
	settingsPath, otherPath string,
	doc *jsonObject,
	servers []MCPServerSpec,
	prevManaged []string,
) (owned []string, warnings []string, err error) {
	if otherPath != "" && len(servers) > 0 {
		if err := checkOpenCodeNonTargetConflicts(settingsPath, otherPath, servers); err != nil {
			return nil, nil, err
		}
	}

	var mcpObj *jsonObject
	if raw, ok := doc.Get("mcp"); ok {
		obj, isObj := raw.(*jsonObject)
		if !isObj {
			return nil, nil, fmt.Errorf("%s: key %q must be a JSON object", settingsPath, "mcp")
		}
		mcpObj = obj
	}

	// OpenCode v2 layout: an object at `mcp.servers` that does not itself look
	// like a legacy server definition (e.g. a server literally named "servers").
	if serversObj, isV2 := openCodeV2ServersContainer(mcpObj); isV2 {
		owned, warnings, err = mergeJSONMCPServerContainer(
			AgentOpenCode,
			settingsPath,
			serversObj,
			servers,
			prevManaged,
			renderOpenCodeV2MCPServerObject,
			opencodeNativeRefRe.MatchString,
		)
		if err != nil {
			return nil, warnings, err
		}
		if len(serversObj.Entries) == 0 && len(servers) == 0 && len(prevManaged) > 0 {
			mcpObj.Delete("servers")
			if len(mcpObj.Entries) == 0 {
				doc.Delete("mcp")
			}
		}
		return owned, warnings, nil
	}

	return mergeJSONMCPServers(
		AgentOpenCode,
		settingsPath,
		"mcp",
		doc,
		servers,
		prevManaged,
		renderOpenCodeMCPServerObject,
		opencodeNativeRefRe.MatchString,
	)
}

func isOpenCodeServerDefKey(key string) bool {
	switch key {
	case "type", "command", "url", "environment", "headers", "enabled", "disabled":
		return true
	default:
		return false
	}
}

func openCodeV2ServersContainer(mcpObj *jsonObject) (*jsonObject, bool) {
	if mcpObj == nil {
		return nil, false
	}
	rawServers, hasServers := mcpObj.Get("servers")
	if !hasServers {
		return nil, false
	}
	serversObj, isObj := rawServers.(*jsonObject)
	if !isObj {
		return nil, false
	}
	for _, entry := range serversObj.Entries {
		if isOpenCodeServerDefKey(entry.Key) {
			return nil, false
		}
	}
	return serversObj, true
}

func checkOpenCodeNonTargetConflicts(settingsPath, otherPath string, servers []MCPServerSpec) error {
	rawContent, exists, _, err := readExistingFile(otherPath, settingsFileMode)
	if err != nil {
		return err
	}
	if !exists || strings.TrimSpace(rawContent) == "" {
		return nil
	}
	otherDoc, err := ParseJSONObject([]byte(rawContent), otherPath)
	if err != nil {
		return err
	}
	rawMCP, ok := otherDoc.Get("mcp")
	if !ok {
		return nil
	}
	mcpObj, isObj := rawMCP.(*jsonObject)
	if !isObj {
		return nil
	}

	v2Servers, _ := openCodeV2ServersContainer(mcpObj)

	for _, srv := range servers {
		if v2Servers != nil {
			if _, present := v2Servers.Get(srv.Name); present {
				return fmt.Errorf("MCP server %q already exists in %s (non-target OpenCode config) and is not managed by DevEx; remove it there so DevEx can manage it in %s", srv.Name, otherPath, settingsPath)
			}
			continue
		}
		if _, present := mcpObj.Get(srv.Name); present {
			return fmt.Errorf("MCP server %q already exists in %s (non-target OpenCode config) and is not managed by DevEx; remove it there so DevEx can manage it in %s", srv.Name, otherPath, settingsPath)
		}
	}
	return nil
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
	}

	if existingMap == nil && len(servers) > 0 {
		existingMap, err = doc.EnsureObject(rootKey)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", settingsPath, err)
		}
	}

	owned, warnings, err = mergeJSONMCPServerContainer(agent, settingsPath, existingMap, servers, prevManaged, renderServer, isNativeRef)
	if err != nil {
		return nil, warnings, err
	}

	// Clean up empty container key if we removed the last managed server and no
	// unmanaged servers remain.
	if existingMap != nil && len(existingMap.Entries) == 0 && len(servers) == 0 && len(prevManaged) > 0 {
		doc.Delete(rootKey)
	}

	return owned, warnings, nil
}

func mergeJSONMCPServerContainer(
	agent, settingsPath string,
	existingMap *jsonObject,
	servers []MCPServerSpec,
	prevManaged []string,
	renderServer func(MCPServerSpec) *jsonObject,
	isNativeRef func(string) bool,
) (owned []string, warnings []string, err error) {
	if existingMap != nil {
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
