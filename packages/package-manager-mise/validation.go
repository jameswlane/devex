package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// MaxToolNameLength defines the maximum allowed tool name length
	MaxToolNameLength = 100
)

// ValidateToolSpec validates tool specifications to prevent command injection
func (m *MisePlugin) ValidateToolSpec(toolSpec string) error {
	// Basic validation
	if toolSpec == "" {
		return fmt.Errorf("tool specification cannot be empty")
	}

	// Check length
	if len(toolSpec) > MaxToolNameLength {
		return fmt.Errorf("tool specification too long")
	}

	// Check for null bytes and control characters
	for _, r := range toolSpec {
		if r == 0 {
			return fmt.Errorf("tool specification contains null bytes")
		}
		if r < 32 && r != 9 && r != 10 && r != 13 { // Allow tab, LF, CR
			return fmt.Errorf("tool specification contains invalid control characters")
		}
	}

	// Check for dangerous characters that could be used in command injection
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "{", "}", "<", ">", "*", "?", "~", "\\n", "\\r"}
	for _, char := range dangerousChars {
		if strings.Contains(toolSpec, char) {
			return fmt.Errorf("tool specification contains potentially dangerous character: %s", char)
		}
	}

	// Validate tool specification format (tool@version or just tool)
	if err := m.ValidateToolSpecFormat(toolSpec); err != nil {
		return fmt.Errorf("invalid tool specification format: %w", err)
	}

	return nil
}

var (
	supportedMiseBackends = map[string]bool{
		"aqua":   true,
		"github": true,
		"npm":    true,
		"pipx":   true,
		"ubi":    true,
		"cargo":  true,
		"go":     true,
	}
	toolNameRegex       = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	backendSegmentRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	optionKeyRegex      = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	optionValueRegex    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	aliasRegex          = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{0,31}$`)
	versionRegex        = regexp.MustCompile(`^[~^]?v?[0-9]{1,3}(?:\.[0-9]{1,3}){0,3}(?:[a-zA-Z0-9_.-]{0,20})?$`)
)

// ValidateToolSpecFormat validates the format of tool specifications
func (m *MisePlugin) ValidateToolSpecFormat(toolSpec string) error {
	// Valid formats:
	// - tool (e.g., "node", "python")
	// - tool@version (e.g., "node@18", "python@3.11", "node@latest")
	// - <backend>:<id>[[key=value,...]][@version] (e.g., "aqua:aaif-goose/goose@latest",
	//   "npm:@google/gemini-cli@1.2.3", "pipx:aider-chat[python=3.12]@latest")
	if strings.ContainsAny(toolSpec, " \t\n\r") {
		return fmt.Errorf("tool specification cannot contain whitespace")
	}
	if strings.Contains(toolSpec, "..") {
		return fmt.Errorf("tool specification cannot contain '..'")
	}

	if backend, rest, hasBackend := strings.Cut(toolSpec, ":"); hasBackend {
		return validateBackendToolSpec(backend, rest)
	}

	// Split by @ to check format
	parts := strings.Split(toolSpec, "@")

	if len(parts) > 2 {
		return fmt.Errorf("tool specification can only contain one '@' character")
	}

	// Validate tool name (first part)
	toolName := parts[0]
	if toolName == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	// Tool name should start with a letter and contain only alphanumeric characters, hyphens, and underscores
	if !toolNameRegex.MatchString(toolName) {
		return fmt.Errorf("tool name must start with a letter and contain only alphanumeric, hyphens, and underscores")
	}

	// If version is specified, validate it
	if len(parts) == 2 {
		return validateToolVersion(parts[1])
	}

	return nil
}

func validateBackendToolSpec(backend, rest string) error {
	if !supportedMiseBackends[backend] {
		return fmt.Errorf("unsupported mise backend: %s", backend)
	}
	if rest == "" {
		return fmt.Errorf("backend tool identifier cannot be empty")
	}
	if strings.Contains(rest, ":") {
		return fmt.Errorf("backend tool specification cannot contain multiple ':' characters")
	}

	// Separate optional @version while accounting for npm scoped packages (@scope/pkg[@version]).
	var idAndOpts, version string
	hasVersion := false
	if backend == "npm" && strings.HasPrefix(rest, "@") {
		slashIdx := strings.Index(rest, "/")
		if slashIdx <= 1 || slashIdx == len(rest)-1 {
			return fmt.Errorf("scoped npm package must match @scope/name")
		}
		afterScope := rest[slashIdx+1:]
		if strings.Count(afterScope, "@") > 1 {
			return fmt.Errorf("tool specification can only contain one version '@' separator")
		}
		if nameAndOpts, ver, ok := strings.Cut(afterScope, "@"); ok {
			idAndOpts = rest[:slashIdx+1] + nameAndOpts
			version = ver
			hasVersion = true
		} else {
			idAndOpts = rest
		}
	} else {
		parts := strings.Split(rest, "@")
		if len(parts) > 2 {
			return fmt.Errorf("tool specification can only contain one '@' character")
		}
		idAndOpts = parts[0]
		if len(parts) == 2 {
			version = parts[1]
			hasVersion = true
		}
	}

	// Strip and validate optional inline tool options: id[key=value,...]
	id := idAndOpts
	if strings.ContainsAny(idAndOpts, "[]") {
		openIdx := strings.Index(idAndOpts, "[")
		if openIdx <= 0 || !strings.HasSuffix(idAndOpts, "]") || strings.Count(idAndOpts, "[") != 1 || strings.Count(idAndOpts, "]") != 1 {
			return fmt.Errorf("invalid inline tool options syntax")
		}
		id = idAndOpts[:openIdx]
		rawOpts := idAndOpts[openIdx+1 : len(idAndOpts)-1]
		if rawOpts == "" {
			return fmt.Errorf("inline tool options cannot be empty")
		}
		for _, pair := range strings.Split(rawOpts, ",") {
			key, val, ok := strings.Cut(pair, "=")
			if !ok || !optionKeyRegex.MatchString(key) || !optionValueRegex.MatchString(val) {
				return fmt.Errorf("invalid inline tool option %q", pair)
			}
		}
	}

	if err := validateBackendToolID(backend, id); err != nil {
		return err
	}
	if hasVersion {
		return validateToolVersion(version)
	}
	return nil
}

func validateBackendToolID(backend, id string) error {
	if id == "" {
		return fmt.Errorf("backend tool identifier cannot be empty")
	}
	if strings.HasPrefix(id, "-") || strings.HasPrefix(id, "/") || strings.HasSuffix(id, "/") {
		return fmt.Errorf("invalid backend tool identifier: %s", id)
	}
	if backend == "npm" && strings.HasPrefix(id, "@") {
		scope, pkg, ok := strings.Cut(id[1:], "/")
		if !ok || strings.Contains(pkg, "/") || !backendSegmentRegex.MatchString(scope) || !backendSegmentRegex.MatchString(pkg) {
			return fmt.Errorf("invalid scoped npm package identifier: %s", id)
		}
		return nil
	}
	for _, seg := range strings.Split(id, "/") {
		if !backendSegmentRegex.MatchString(seg) {
			return fmt.Errorf("invalid backend tool identifier segment %q", seg)
		}
	}
	return nil
}

func validateToolVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty when '@' is used")
	}

	// Version can be a named alias mise resolves itself, such as
	// "latest", "lts", "lts-iron", "stable" or "temurin-21".
	if aliasRegex.MatchString(version) {
		return nil
	}

	// Allow common version patterns with safer regex to prevent ReDoS
	if !versionRegex.MatchString(version) {
		return fmt.Errorf("invalid version format")
	}
	return nil
}

// ValidateShellType validates shell type to ensure it's supported
func (m *MisePlugin) ValidateShellType(shellType string) error {
	if shellType == "" {
		return fmt.Errorf("shell type cannot be empty")
	}

	// Check for dangerous characters
	if strings.ContainsAny(shellType, ";|&`$(){}[]<>\\") {
		return fmt.Errorf("shell type contains invalid characters")
	}

	// Validate against supported shells
	supportedShells := []string{"bash", "zsh", "fish"}
	for _, supported := range supportedShells {
		if shellType == supported {
			return nil
		}
	}

	return fmt.Errorf("unsupported shell type: %s (supported: bash, zsh, fish)", shellType)
}

// ValidateCommandArg validates individual command arguments
func (m *MisePlugin) ValidateCommandArg(arg string) error {
	if arg == "" {
		return fmt.Errorf("argument cannot be empty")
	}

	// Check for null bytes
	for _, r := range arg {
		if r == 0 {
			return fmt.Errorf("argument contains null bytes")
		}
	}

	// Check for shell metacharacters that could be used for command injection
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">"}
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// ValidateEnvironmentVar validates environment variable names
func (m *MisePlugin) ValidateEnvironmentVar(varName string) error {
	if varName == "" {
		return fmt.Errorf("environment variable name cannot be empty")
	}

	// Environment variable names should match the pattern [A-Z_][A-Z0-9_]*
	envVarRegex := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	if !envVarRegex.MatchString(varName) {
		return fmt.Errorf("invalid environment variable name: must start with letter or underscore and contain only letters, numbers, and underscores")
	}

	return nil
}
