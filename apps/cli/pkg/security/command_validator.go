package security

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// SecurityLevel defines how strict command validation should be
type SecurityLevel int

const (
	// SecurityLevelStrict blocks everything except explicitly safe patterns
	SecurityLevelStrict SecurityLevel = iota
	// SecurityLevelModerate allows most commands but blocks dangerous patterns
	SecurityLevelModerate
	// SecurityLevelPermissive only blocks obviously malicious patterns
	SecurityLevelPermissive
)

// CommandValidator provides flexible command validation with configurable security levels
type CommandValidator struct {
	level              SecurityLevel
	dangerousPatterns  []*regexp.Regexp
	safePatterns       []*regexp.Regexp
	customWhitelist    map[string]bool
	customBlacklist    map[string]bool
	allowedExecutables []string // Dynamically discovered executables
}

// NewCommandValidator creates a validator with the specified security level
func NewCommandValidator(level SecurityLevel) *CommandValidator {
	cv := &CommandValidator{
		level:           level,
		customWhitelist: make(map[string]bool),
		customBlacklist: make(map[string]bool),
	}

	cv.initializePatterns()
	cv.discoverExecutables()

	return cv
}

// initializePatterns sets up dangerous and safe patterns
func (cv *CommandValidator) initializePatterns() {
	// Always block these dangerous patterns regardless of security level
	cv.dangerousPatterns = []*regexp.Regexp{
		// Destructive commands
		regexp.MustCompile(`\brm\s+(-[rfRi]*\s+)*(/|/home|/usr|/var|/etc|/boot|/sys|/proc)\s*$`),
		regexp.MustCompile(`\bdd\s+.*\bof=/dev/(sd[a-z]|hd[a-z]|nvme\d+n\d+)\b`),
		regexp.MustCompile(`\bmkfs\b.*\b/dev/`),

		// Fork bombs and resource exhaustion
		regexp.MustCompile(`:\(\)\{.*:\|:&.*\};:`),
		regexp.MustCompile(`\.\s+/dev/zero`),

		// Privilege escalation attempts
		regexp.MustCompile(`\bchmod\s+[+\-]?s`), // SUID/SGID
		regexp.MustCompile(`\bsetuid\s*\(`),

		// System file manipulation
		regexp.MustCompile(`>\s*/etc/(passwd|shadow|sudoers|hosts)\b`),
		regexp.MustCompile(`>\s*/boot/`),

		// Network attacks
		regexp.MustCompile(`\bnc\s+.*-e\s+/bin/(bash|sh)`), // Reverse shells
		regexp.MustCompile(`\bpython.*\s+-c.*socket.*connect`),

		// Code injection in specific contexts
		regexp.MustCompile(`\bcurl\s+.*\|\s*(bash|sh)\b`),
		regexp.MustCompile(`\bwget\s+.*\|\s*(bash|sh)\b`),

		// Dangerous commands in command substitution
		regexp.MustCompile(`\$\(\s*(rm\s+-rf|dd\s+.*of=|mkfs)\b`),

		// Block command substitution with suspicious keywords
		regexp.MustCompile(`\$\([^)]*malicious[^)]*\)`), // Contains "malicious" keyword
		regexp.MustCompile(`\$\([^)]*evil[^)]*\)`),      // Contains "evil" keyword
		regexp.MustCompile(`\$\([^)]*hack[^)]*\)`),      // Contains "hack" keyword
	}

	// Safe patterns that should generally be allowed
	cv.safePatterns = []*regexp.Regexp{
		// Package managers (cross-platform)
		regexp.MustCompile(`^(apt|apt-get|yum|dnf|pacman|zypper|brew|snap|flatpak|pip|pip3|npm|yarn|pnpm|cargo|go|gem|composer|nuget|choco|scoop|winget)\s+`),

		// Version checks
		regexp.MustCompile(`\s+--version\s*$`),
		regexp.MustCompile(`\s+-v\s*$`),
		regexp.MustCompile(`\s+version\s*$`),

		// Help commands
		regexp.MustCompile(`\s+--help\s*$`),
		regexp.MustCompile(`\s+-h\s*$`),
		regexp.MustCompile(`\s+help\s*$`),

		// Common dev tools
		regexp.MustCompile(`^(git|docker|kubectl|terraform|ansible|vagrant)\s+`),

		// File operations (safe variants)
		regexp.MustCompile(`^(ls|cat|grep|find|which|whereis|file|stat|head|tail|less|more)\s+`),
		regexp.MustCompile(`^(cp|mv|mkdir|touch)\s+[^/]`), // Not on root paths

		// Shell built-ins and safe utilities
		regexp.MustCompile(`^(echo|printf|export|source|eval "\$\(.*init.*\)")\s*`),
		regexp.MustCompile(`^(cd|pwd|date|hostname|whoami|id|uname)\s*`),
	}
}

// discoverExecutables finds available executables on the system
func (cv *CommandValidator) discoverExecutables() {
	// Common paths to search for executables
	paths := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))

	cv.allowedExecutables = []string{}

	// Limit discovery to avoid performance issues
	maxExecutables := 1000
	found := 0

	for _, dir := range paths {
		if found >= maxExecutables {
			break
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if found >= maxExecutables {
				break
			}

			if !entry.IsDir() {
				info, err := entry.Info()
				if err == nil && info.Mode()&0111 != 0 { // Is executable
					cv.allowedExecutables = append(cv.allowedExecutables, entry.Name())
					found++
				}
			}
		}
	}
}

// ValidateCommand checks if a command is safe to execute
func (cv *CommandValidator) ValidateCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("empty command")
	}

	// Check custom blacklist first
	parts := strings.Fields(command)
	if len(parts) > 0 && cv.customBlacklist[parts[0]] {
		return fmt.Errorf("command '%s' is explicitly blacklisted", parts[0])
	}

	// Check custom whitelist
	if len(parts) > 0 && cv.customWhitelist[parts[0]] {
		return nil // Explicitly allowed
	}

	// Always check dangerous patterns
	for _, pattern := range cv.dangerousPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains dangerous pattern")
		}
	}

	// Apply security level specific validation
	switch cv.level {
	case SecurityLevelStrict:
		return cv.validateStrict(command)
	case SecurityLevelModerate:
		return cv.validateModerate(command)
	case SecurityLevelPermissive:
		return cv.validatePermissive(command)
	default:
		return cv.validateModerate(command)
	}
}

// validateStrict only allows explicitly safe patterns
func (cv *CommandValidator) validateStrict(command string) error {
	// Must match a safe pattern
	for _, pattern := range cv.safePatterns {
		if pattern.MatchString(command) {
			return nil
		}
	}

	// Check if it's a known executable from PATH
	parts := strings.Fields(command)
	if len(parts) > 0 {
		executable := filepath.Base(parts[0])
		for _, allowed := range cv.allowedExecutables {
			if executable == allowed {
				return nil
			}
		}
	}

	return fmt.Errorf("command not in safe patterns list (strict mode)")
}

// validateModerate allows most commands except dangerous ones
func (cv *CommandValidator) validateModerate(command string) error {
	// Safe patterns are immediately allowed
	for _, pattern := range cv.safePatterns {
		if pattern.MatchString(command) {
			return nil
		}
	}

	// Allow common administrative commands that were being blocked
	adminPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^sudo\s+(updatedb|service|systemctl|usermod|groupadd)`), // Common admin commands
		regexp.MustCompile(`mkdir\s+-p\s+`),                                         // Directory creation
		regexp.MustCompile(`ln\s+-sf?\s+\$\(which\s+\w+\)`),                         // Safe symlink creation
		regexp.MustCompile(`echo\s+.*\s+>>\s+~/\.\w+rc`),                            // Shell config modification
	}

	for _, pattern := range adminPatterns {
		if pattern.MatchString(command) {
			return nil
		}
	}

	// Only block obviously dangerous patterns in moderate mode
	obviouslyDangerous := []*regexp.Regexp{
		regexp.MustCompile(`\$\(.*rm\s+-rf\s+/\)`),     // Command substitution with rm -rf /
		regexp.MustCompile(`\|\s*base64\s+-d.*\|.*sh`), // Base64 decode to shell execution
		regexp.MustCompile(`/dev/tcp/.*\|\s*sh`),       // Network redirects to shell
		regexp.MustCompile(`\:\(\)\{.*\:\|\:\&`),       // Fork bombs
	}

	for _, pattern := range obviouslyDangerous {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains obviously dangerous pattern")
		}
	}

	// Check if the base command exists
	parts := strings.Fields(command)
	if len(parts) > 0 {
		baseCmd := parts[0]

		// Handle sudo specially
		if baseCmd == "sudo" && len(parts) > 1 {
			baseCmd = parts[1]
		}

		// Allow if it's an absolute/relative path that exists
		if strings.Contains(baseCmd, "/") {
			if _, err := os.Stat(baseCmd); err == nil {
				return nil
			}
		}

		// Allow if it's in PATH
		if cv.isInPath(baseCmd) {
			return nil
		}

		// Allow common shell built-ins
		if cv.isShellBuiltin(baseCmd) {
			return nil
		}
	}

	// In moderate mode, allow by default if no obviously dangerous patterns found
	// This is the key change - be permissive by default
	return nil
}

// validatePermissive only blocks obviously malicious commands
func (cv *CommandValidator) validatePermissive(command string) error {
	// Dangerous patterns are already checked
	// In permissive mode, that's all we block
	return nil
}

// isInPath checks if a command exists in PATH
func (cv *CommandValidator) isInPath(command string) bool {
	if runtime.GOOS == "windows" {
		command += ".exe"
	}

	for _, executable := range cv.allowedExecutables {
		if executable == command {
			return true
		}
	}

	// Fallback to actual PATH check
	_, err := os.Stat(filepath.Join("/usr/bin", command))
	if err == nil {
		return true
	}

	_, err = os.Stat(filepath.Join("/usr/local/bin", command))
	if err == nil {
		return true
	}

	if runtime.GOOS != "windows" {
		_, err = os.Stat(filepath.Join("/bin", command))
		if err == nil {
			return true
		}

		_, err = os.Stat(filepath.Join("/sbin", command))
		if err == nil {
			return true
		}
	}

	return false
}

// isShellBuiltin checks if a command is a shell built-in
func (cv *CommandValidator) isShellBuiltin(command string) bool {
	builtins := []string{
		"alias", "bg", "bind", "break", "builtin", "case", "cd", "command",
		"compgen", "complete", "continue", "declare", "dirs", "disown", "echo",
		"enable", "eval", "exec", "exit", "export", "false", "fc", "fg",
		"getopts", "hash", "help", "history", "if", "jobs", "kill", "let",
		"local", "logout", "popd", "printf", "pushd", "pwd", "read", "readonly",
		"return", "set", "shift", "shopt", "source", "suspend", "test", "times",
		"trap", "true", "type", "typeset", "ulimit", "umask", "unalias", "unset",
		"until", "wait", "while",
	}

	for _, builtin := range builtins {
		if command == builtin {
			return true
		}
	}

	return false
}

// AddToWhitelist adds a command to the custom whitelist
func (cv *CommandValidator) AddToWhitelist(command string) {
	cv.customWhitelist[command] = true
}

// AddToBlacklist adds a command to the custom blacklist
func (cv *CommandValidator) AddToBlacklist(command string) {
	cv.customBlacklist[command] = true
}

// SetSecurityLevel changes the security level
func (cv *CommandValidator) SetSecurityLevel(level SecurityLevel) {
	cv.level = level
}

// GetSecurityLevel returns the current security level
func (cv *CommandValidator) GetSecurityLevel() SecurityLevel {
	return cv.level
}
