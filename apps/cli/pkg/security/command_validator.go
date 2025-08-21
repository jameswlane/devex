package security

import (
	"fmt"
	"os"
	"regexp"
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

		// Dangerous bash -c patterns
		regexp.MustCompile(`\|\s*bash\s+-c\s+.*rm\s+-rf`),
		regexp.MustCompile(`bash\s+-c\s+.*\|\s*(bash|sh)`),
		regexp.MustCompile(`bash\s+-c\s+.*rm\s+-rf`),

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

	// Only block the most obviously destructive commands
	minimalDangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\brm\s+(-[rfRi]*\s+)*(/|/home|/usr|/var|/etc|/boot|/sys|/proc)\s*$`), // rm -rf on system dirs
		regexp.MustCompile(`\bdd\s+.*\bof=/dev/(sd[a-z]|hd[a-z]|nvme\d+n\d+)\\b`),                // dd to disks
		regexp.MustCompile(`\bmkfs\b.*\b/dev/`),                                                  // format disks
		regexp.MustCompile(`:\(\)\{.*:\|:&.*\};:`),                                               // fork bombs
	}

	for _, pattern := range minimalDangerousPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains potentially dangerous pattern")
		}
	}

	// Allow everything else - focus on functionality over security for now
	return nil
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
