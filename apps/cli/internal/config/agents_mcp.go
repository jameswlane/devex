package config

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	mcpServerNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]{0,63}$`)
	mcpEnvVarNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	mcpEnvRefRe     = regexp.MustCompile(`^\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}$`)
	mcpHeaderNameRe = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`)

	mcpSecretArgFlags = map[string]bool{
		"--token":        true,
		"--api-key":      true,
		"--apikey":       true,
		"--password":     true,
		"--secret":       true,
		"--auth-token":   true,
		"--access-token": true,
	}

	mcpSecretPATRe    = regexp.MustCompile(`(^|[^A-Z0-9])PAT([^A-Z0-9]|$)`)
	mcpSecretValueRes = []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])github_pat_[A-Za-z0-9_]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])glpat-[A-Za-z0-9_-]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])sk-ant-[A-Za-z0-9_-]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])sk-[A-Za-z0-9_-]{12,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])xox[abposr]-[A-Za-z0-9-]{6,}`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])AKIA[0-9A-Z]{16}(?:[^0-9A-Z]|$)`),
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])AIza[0-9A-Za-z_-]{16,}`),
	}
	mcpBearerTokenRe = regexp.MustCompile(`(?i)(?:^|\s)Bearer\s+(\S+)`)
)

func validateAgentMCPServers(
	cfg AgentsConfig,
	seenEnabled map[string]bool,
	capsMap map[string]AgentCapabilities,
	add func(field, msg string),
) {
	seenNames := make(map[string]bool, len(cfg.MCPServers))
	for i, srv := range cfg.MCPServers {
		prefix := fmt.Sprintf("agents.mcp_servers[%d]", i)
		label := srv.Name
		if label == "" {
			label = fmt.Sprintf("#%d", i)
		}

		if !mcpServerNameRe.MatchString(srv.Name) {
			add(prefix+".name", fmt.Sprintf("MCP server name %q must match ^[A-Za-z0-9_][A-Za-z0-9_-]{0,63}$", srv.Name))
		} else {
			if seenNames[srv.Name] {
				add(prefix+".name", fmt.Sprintf("duplicate MCP server name %q", srv.Name))
			}
			seenNames[srv.Name] = true
			if cfg.DevExMCP && srv.Name == "devex" {
				add(prefix+".name", `MCP server name "devex" is reserved when agents.devex_mcp is true`)
			}
		}

		switch srv.Transport {
		case MCPTransportStdio:
			validateStdioMCPServer(srv, prefix, label, add)
		case MCPTransportHTTP:
			validateHTTPMCPServer(srv, prefix, label, add)
		default:
			add(prefix+".transport", fmt.Sprintf("MCP server %q has unsupported transport %q (supported: %s, %s)", label, srv.Transport, MCPTransportStdio, MCPTransportHTTP))
		}

		seenTargets := make(map[string]bool, len(srv.Agents))
		for j, target := range srv.Agents {
			field := fmt.Sprintf("%s.agents[%d]", prefix, j)
			caps, supported := capsMap[target]
			if !supported {
				add(field, fmt.Sprintf("MCP server %q references unsupported agent %q (supported: %s)", label, target, strings.Join(SupportedAgents(), ", ")))
				continue
			}
			if seenTargets[target] {
				add(field, fmt.Sprintf("MCP server %q lists duplicate agent %q", label, target))
				continue
			}
			seenTargets[target] = true
			if !seenEnabled[target] {
				add(field, fmt.Sprintf("MCP server %q targets agent %q which is not in agents.enabled", label, target))
			}
			if !caps.MCPServers {
				add(field, fmt.Sprintf("agent %q has no native equivalent for mcp_servers", target))
			}
		}
		if len(srv.Agents) == 0 {
			for _, name := range cfg.Enabled {
				if caps, ok := capsMap[name]; ok && !caps.MCPServers {
					add(prefix, fmt.Sprintf("agent %q has no native equivalent for mcp_servers", name))
				}
			}
		}
	}

	if cfg.DevExMCP {
		for _, name := range cfg.Enabled {
			if caps, ok := capsMap[name]; ok && !caps.MCPServers {
				add("agents.devex_mcp", fmt.Sprintf("agent %q has no native equivalent for mcp_servers", name))
			}
		}
	}
}

func validateStdioMCPServer(srv AgentMCPServerConfig, prefix, label string, add func(field, msg string)) {
	if strings.TrimSpace(srv.Command) == "" || hasAgentControlChars(srv.Command) {
		add(prefix+".command", fmt.Sprintf("MCP server %q requires a non-empty single-line command without control characters", label))
	}
	if srv.URL != "" {
		add(prefix+".url", fmt.Sprintf("MCP server %q uses transport %q and must not set url", label, MCPTransportStdio))
	}
	if len(srv.Headers) > 0 {
		add(prefix+".headers", fmt.Sprintf("MCP server %q uses transport %q and must not set headers", label, MCPTransportStdio))
	}
	for i := 0; i < len(srv.Args); i++ {
		arg := srv.Args[i]
		field := fmt.Sprintf("%s.args[%d]", prefix, i)
		if hasAgentControlChars(arg) {
			add(field, fmt.Sprintf("MCP server %q args[%d] must not contain control characters", label, i))
			continue
		}
		if flagPart, valPart, hasEq := strings.Cut(arg, "="); hasEq && mcpSecretArgFlags[strings.ToLower(flagPart)] && strings.TrimSpace(valPart) != "" {
			add(field, fmt.Sprintf("MCP server %q args[%d] passes a secret via %s; pass secrets via env references (${env:VAR_NAME}) in env instead", label, i, flagPart))
			continue
		}
		if mcpSecretArgFlags[strings.ToLower(arg)] && i+1 < len(srv.Args) && strings.TrimSpace(srv.Args[i+1]) != "" {
			nextIdx := i + 1
			nextField := fmt.Sprintf("%s.args[%d]", prefix, nextIdx)
			if hasAgentControlChars(srv.Args[nextIdx]) {
				add(nextField, fmt.Sprintf("MCP server %q args[%d] must not contain control characters", label, nextIdx))
			} else {
				add(nextField, fmt.Sprintf("MCP server %q args[%d] passes a secret via %s; pass secrets via env references (${env:VAR_NAME}) in env instead", label, nextIdx, arg))
			}
			i++
			continue
		}
		if LooksLikeMCPSecretValue(arg) {
			add(field, fmt.Sprintf("MCP server %q args[%d] appears to contain a literal secret; pass secrets via env references (${env:VAR_NAME}) instead", label, i))
		}
	}
	for _, k := range sortedStringKeys(srv.Env) {
		field := prefix + ".env." + k
		v := srv.Env[k]
		if !mcpEnvVarNameRe.MatchString(k) || hasAgentControlChars(k) {
			add(field, fmt.Sprintf("MCP server %q env key %q must match ^[A-Za-z_][A-Za-z0-9_]*$", label, k))
			continue
		}
		if hasAgentControlChars(v) {
			add(field, fmt.Sprintf("MCP server %q env.%s must not contain control characters", label, k))
			continue
		}
		if errMsg := validateMCPSecretPolicyValue(label, "env."+k, k, v, false); errMsg != "" {
			add(field, errMsg)
		}
	}
}

func validateHTTPMCPServer(srv AgentMCPServerConfig, prefix, label string, add func(field, msg string)) {
	if srv.Command != "" {
		add(prefix+".command", fmt.Sprintf("MCP server %q uses transport %q and must not set command", label, MCPTransportHTTP))
	}
	if len(srv.Args) > 0 {
		add(prefix+".args", fmt.Sprintf("MCP server %q uses transport %q and must not set args", label, MCPTransportHTTP))
	}
	if len(srv.Env) > 0 {
		add(prefix+".env", fmt.Sprintf("MCP server %q uses transport %q and must not set env", label, MCPTransportHTTP))
	}
	if strings.TrimSpace(srv.URL) == "" || hasAgentControlChars(srv.URL) {
		add(prefix+".url", fmt.Sprintf("MCP server %q requires a valid http(s) url without control characters", label))
	} else {
		parsed, err := url.Parse(srv.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			add(prefix+".url", fmt.Sprintf("MCP server %q url must be a valid http or https URL with a host", label))
		} else {
			if parsed.User != nil {
				add(prefix+".url", fmt.Sprintf("MCP server %q url must not contain embedded credentials; use headers with ${env:VAR_NAME} instead", label))
			}
			query := parsed.Query()
			for _, qk := range sortedQueryKeys(query) {
				for _, qv := range query[qk] {
					if (IsMCPSecretKey(qk) && strings.TrimSpace(qv) != "") || LooksLikeMCPSecretValue(qv) {
						add(prefix+".url", fmt.Sprintf("MCP server %q url query parameter %q appears to contain a literal secret; use headers with ${env:VAR_NAME} instead", label, qk))
						break
					}
				}
			}
		}
	}
	for _, k := range sortedStringKeys(srv.Headers) {
		field := prefix + ".headers." + k
		v := srv.Headers[k]
		if !mcpHeaderNameRe.MatchString(k) || hasAgentControlChars(k) {
			add(field, fmt.Sprintf("MCP server %q header name %q is not a valid HTTP header name", label, k))
			continue
		}
		if hasAgentControlChars(v) {
			add(field, fmt.Sprintf("MCP server %q headers.%s must not contain control characters", label, k))
			continue
		}
		if errMsg := validateMCPSecretPolicyValue(label, "headers."+k, k, v, strings.EqualFold(k, "Authorization")); errMsg != "" {
			add(field, errMsg)
		}
	}
}

func validateMCPSecretPolicyValue(serverName, fieldName, key, val string, allowBearerRef bool) string {
	if strings.Contains(val, "${keyring:") {
		return fmt.Sprintf("MCP server %q %s uses a keyring reference, which is not supported in v1; use ${env:VAR_NAME} instead", serverName, fieldName)
	}
	if mcpEnvRefRe.MatchString(val) {
		return ""
	}
	if allowBearerRef && strings.HasPrefix(val, "Bearer ") && mcpEnvRefRe.MatchString(strings.TrimPrefix(val, "Bearer ")) {
		return ""
	}
	if strings.Contains(val, "${env:") {
		return fmt.Sprintf("MCP server %q %s contains an invalid environment variable reference; use ${env:VAR_NAME} (or \"Bearer ${env:VAR_NAME}\" for Authorization)", serverName, fieldName)
	}
	if IsMCPSecretKey(key) {
		return fmt.Sprintf("MCP server %q %s looks like a secret and must use an environment variable reference (${env:VAR_NAME}), not a literal value", serverName, fieldName)
	}
	if LooksLikeMCPSecretValue(val) {
		return fmt.Sprintf("MCP server %q %s appears to contain a literal secret; use an environment variable reference (${env:VAR_NAME}) instead", serverName, fieldName)
	}
	return ""
}

// IsMCPSecretKey reports whether an env var, header, or query parameter name
// looks like it holds a secret credential.
func IsMCPSecretKey(key string) bool {
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
	if mcpSecretPATRe.MatchString(upper) {
		return true
	}
	return hasMCPSecretAuthToken(upper)
}

func hasMCPSecretAuthToken(upper string) bool {
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

// LooksLikeMCPSecretValue reports whether val matches a known literal secret or
// token shape (never returning or echoing the matched secret).
func LooksLikeMCPSecretValue(val string) bool {
	for _, re := range mcpSecretValueRes {
		if re.MatchString(val) {
			return true
		}
	}
	for _, m := range mcpBearerTokenRe.FindAllStringSubmatch(val, -1) {
		if len(m) >= 2 && !mcpEnvRefRe.MatchString(m[1]) {
			return true
		}
	}
	return false
}

func sortedStringKeys(m map[string]string) []string {
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
