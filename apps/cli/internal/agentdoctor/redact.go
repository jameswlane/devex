package agentdoctor

import (
	"sort"
	"strings"
)

const (
	redactedPlaceholder = "[REDACTED]"
	// minRedactSecretLen avoids corrupting normal output if an unrelated test
	// or shell exports a trivial 1-5 char placeholder in an API key variable.
	minRedactSecretLen = 6
)

// AllKnownSecretEnvVars returns the deduplicated, sorted list of every API-key
// or token environment variable name across all known agents.
func AllKnownSecretEnvVars() []string {
	seen := map[string]bool{}
	for _, spec := range knownAgentDefs {
		for _, name := range spec.APIKeyEnvVars {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CollectSecretValues gathers the non-empty values of all known API-key env
// vars from one or more getenv functions so output formatters can redact them
// as defense in depth. Returned values are sorted longest-first to avoid
// partial redaction.
func CollectSecretValues(getters ...func(string) string) []string {
	seen := map[string]bool{}
	var values []string
	names := AllKnownSecretEnvVars()
	for _, getenv := range getters {
		if getenv == nil {
			continue
		}
		for _, name := range names {
			v := strings.TrimSpace(getenv(name))
			if len(v) < minRedactSecretLen || seen[v] {
				continue
			}
			seen[v] = true
			values = append(values, v)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if len(values[i]) != len(values[j]) {
			return len(values[i]) > len(values[j])
		}
		return values[i] < values[j]
	})
	return values
}

// Redact replaces every occurrence of any secret in s with [REDACTED].
func Redact(s string, secrets []string) string {
	if s == "" || len(secrets) == 0 {
		return s
	}
	for _, secret := range secrets {
		if len(secret) < minRedactSecretLen {
			continue
		}
		s = strings.ReplaceAll(s, secret, redactedPlaceholder)
	}
	return s
}
