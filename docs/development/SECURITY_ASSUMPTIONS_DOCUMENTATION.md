# Security Assumptions Documentation

## Overview
This document outlines the security assumptions, threat model, and security measures implemented across all DevEx plugins.

## Global Security Principles

### 1. Principle of Least Privilege
- Plugins only request the minimum permissions required
- Sudo access is only used when absolutely necessary
- File system access is limited to specific directories

### 2. Input Validation
- All user inputs are validated before processing
- Dangerous characters and patterns are rejected
- URL validation for network-accessible resources

### 3. Command Injection Prevention
- All external command execution uses exec.CommandContext
- User inputs are never directly interpolated into shell commands
- Parameterized command construction where possible

## Plugin-Specific Security Assumptions

### High-Risk Plugins

#### package-manager-curlpipe ⚠️ HIGH RISK
**Security Assumptions**:
- ✅ Only executes scripts from pre-approved trusted domains
- ✅ Domain validation prevents arbitrary URL execution
- ✅ Script content validation before execution
- ✅ User confirmation required for untrusted domains (--force flag)
- ✅ Script sanitization and size limits (50KB max)
- ✅ Runtime security validation for dangerous patterns

**Threat Model**:
- **Mitigated**: Arbitrary script execution from malicious domains
- **Mitigated**: Code injection through script content
- **Mitigated**: Resource exhaustion through large scripts
- **Remaining Risk**: Malicious scripts from trusted domains (requires domain compromise)

**Security Controls**:
```go
// Trusted domains whitelist
trustedDomains := []string{
    "get.docker.com", "sh.rustup.rs", "raw.githubusercontent.com",
    "github.com", "install.python-poetry.org", "mise.jdx.dev"
}

// Runtime validation
func ValidateScriptSecurity(content string) error {
    // Size limits, binary content detection, command substitution limits
}
```

#### package-manager-docker ⚠️ MEDIUM-HIGH RISK
**Security Assumptions**:
- ⚠️ Assumes Docker daemon is properly configured with appropriate security
- ⚠️ Relies on Docker's container isolation
- ✅ Validates Docker availability before operations
- ⚠️ No additional validation of Docker images (trusts Docker Hub)

**Threat Model**:
- **Mitigated**: Plugin operation without Docker available
- **Remaining Risk**: Malicious Docker images
- **Remaining Risk**: Docker daemon privilege escalation
- **Remaining Risk**: Container escape vulnerabilities

### Package Manager Plugins

#### package-manager-apt, package-manager-dnf, etc. ⚠️ MEDIUM RISK
**Security Assumptions**:
- ✅ Uses system package manager validation
- ✅ Relies on distribution package signing
- ✅ Sudo validation before privileged operations
- ✅ Package name validation to prevent injection

**Threat Model**:
- **Mitigated**: Package name injection attacks
- **Mitigated**: Unauthorized package installation (requires sudo)
- **Remaining Risk**: Compromised packages in official repositories
- **Remaining Risk**: Sudo privilege escalation

**Security Controls**:
```go
// Package name validation
func validatePackageName(name string) error {
    if matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-\+\.]+$`, name); !matched {
        return fmt.Errorf("invalid package name")
    }
}
```

### Tool Plugins

#### tool-git ✅ LOW RISK
**Security Assumptions**:
- ✅ Only modifies Git configuration files
- ✅ Validates Git availability before operations
- ✅ No external network access
- ✅ Configuration values are validated

**Threat Model**:
- **Mitigated**: Git configuration injection
- **Mitigated**: Arbitrary file modification
- **Remaining Risk**: Git configuration tampering (requires file system access)

#### tool-shell ⚠️ MEDIUM RISK
**Security Assumptions**:
- ✅ Only modifies user shell configuration files
- ✅ Validates shell availability
- ✅ Backup creation before modifications
- ⚠️ Shell script execution for configuration

**Threat Model**:
- **Mitigated**: Shell configuration injection
- **Mitigated**: Arbitrary file modification outside user directory
- **Remaining Risk**: Shell configuration tampering leading to command execution

#### tool-stackdetector ✅ LOW RISK
**Security Assumptions**:
- ✅ Read-only filesystem operations
- ✅ No external network access
- ✅ No privilege escalation required
- ✅ Cached stat operations for performance

**Threat Model**:
- **Mitigated**: File system modification
- **Mitigated**: Information disclosure outside target directory
- **Remaining Risk**: Information gathering about project structure

### Desktop Environment Plugins

#### desktop-gnome, desktop-kde, etc. ⚠️ MEDIUM RISK
**Security Assumptions**:
- ✅ Only modifies desktop environment settings
- ✅ Validates desktop environment availability
- ✅ Uses official desktop configuration tools (gsettings, etc.)
- ⚠️ May require font and theme installation

**Threat Model**:
- **Mitigated**: Desktop environment detection and validation
- **Mitigated**: Invalid configuration settings
- **Remaining Risk**: Desktop environment privilege escalation
- **Remaining Risk**: Malicious theme/extension installation

### System Setup Plugin

#### system-setup ⚠️ HIGH RISK
**Security Assumptions**:
- ⚠️ Requires root/administrator privileges for system-level changes
- ⚠️ Modifies system configuration files
- ⚠️ May install system-level software
- ✅ Validates system compatibility before operations

**Threat Model**:
- **Remaining Risk**: System configuration corruption
- **Remaining Risk**: Privilege escalation through system modifications
- **Remaining Risk**: System security policy changes

## Environment Variable Security

### Validation Requirements
All plugins that read environment variables implement validation:

```go
func validateEnvironmentVariable(name, value string) error {
    // Validate variable name
    if !regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(name) {
        return fmt.Errorf("invalid environment variable name")
    }
    
    // Validate value for dangerous patterns
    return sdk.ValidateDangerousChars("env_var", value)
}
```

### Secure Environment Variables
- `DEVEX_*`: Validated application configuration
- `HOME`: Trusted system variable
- `PATH`: Read-only usage, not modified
- `XDG_*`: Desktop environment variables (validated)

### Dangerous Environment Variables
- Shell variables: `PS1`, `PROMPT_COMMAND` (potential injection)
- Path variables: `LD_LIBRARY_PATH`, `PYTHONPATH` (hijacking)
- Executable variables: `EDITOR`, `PAGER` (command injection)

## Command Execution Security

### Safe Execution Pattern
```go
func safeExecution(ctx context.Context, command string, args ...string) error {
    // Validate command exists
    if !sdk.CommandExists(command) {
        return fmt.Errorf("command not available: %s", command)
    }
    
    // Use context for cancellation
    cmd := exec.CommandContext(ctx, command, args...)
    
    // Set secure environment
    cmd.Env = []string{"PATH=/usr/bin:/bin"}
    
    return cmd.Run()
}
```

### Command Validation
All plugins use shared validation:
```go
func ValidateCommand(command string) error {
    // Check for dangerous patterns
    dangerousPatterns := []*regexp.Regexp{
        regexp.MustCompile(`\brm\s+.*\s+(-r|-f)/`),
        regexp.MustCompile(`\bdd\s+.*of=/dev/`),
        // ... other patterns
    }
}
```

## Network Security

### Trusted Domains (curlpipe plugin)
```go
var trustedDomains = []string{
    "get.docker.com",           // Docker official
    "sh.rustup.rs",            // Rust official
    "raw.githubusercontent.com", // GitHub raw content
    "github.com",              // GitHub releases
    "install.python-poetry.org", // Poetry official
    "mise.jdx.dev",            // Mise official
    "get.helm.sh",             // Helm official
    "install.k3s.io",          // K3s official
    "get.k3s.io",              // K3s official
    "installer.id",            // Generic installer platform
    "sh.brew.sh",              // Homebrew official
    "deno.land",               // Deno official
    "bun.sh",                  // Bun official
}
```

### Network Access Control
- **No network access**: tool-*, desktop-* (except theme downloads)
- **Restricted network access**: package-manager-curlpipe (trusted domains only)
- **System network access**: package-manager-* (uses system package managers)

## File System Security

### Safe File Operations
```go
func validatePath(path string) error {
    // Prevent directory traversal
    if strings.Contains(path, "..") {
        return fmt.Errorf("directory traversal not allowed")
    }
    
    // Restrict to safe directories
    allowedPaths := []string{
        filepath.Join(os.Getenv("HOME"), ".config"),
        filepath.Join(os.Getenv("HOME"), ".local"),
        "/tmp",
    }
    
    absPath, _ := filepath.Abs(path)
    for _, allowed := range allowedPaths {
        if strings.HasPrefix(absPath, allowed) {
            return nil
        }
    }
    
    return fmt.Errorf("path not in allowed directories")
}
```

### File Permission Model
- **User files**: 0644 (readable by owner and group)
- **Executable files**: 0755 (executable by owner, readable by all)
- **Config files**: 0600 (readable only by owner)
- **Backup files**: 0600 (readable only by owner)

## Logging and Audit

### Security Event Logging
All plugins log security-relevant events:
```go
logger.Warning("Untrusted domain access attempted: %s", domain)
logger.Info("Privilege escalation requested for: %s", operation)
logger.Debug("File modification: %s", filepath)
```

### Sensitive Data Handling
- **No sensitive data in logs**: Passwords, keys, tokens excluded
- **Path sanitization**: Home directory paths are normalized
- **Command sanitization**: Commands logged without sensitive arguments

## Compliance and Standards

### Security Standards Compliance
- **Input Validation**: OWASP Input Validation
- **Command Injection**: CWE-78 prevention
- **Path Traversal**: CWE-22 prevention
- **Privilege Escalation**: CWE-269 mitigation

### Code Quality Security
- **Static Analysis**: golangci-lint security rules
- **Dependency Scanning**: Go vulnerability database
- **Code Review**: Security-focused review process

## Risk Assessment Summary

| Plugin Type | Risk Level | Primary Concerns | Mitigation Status |
|-------------|------------|------------------|-------------------|
| tool-* | Low | Configuration modification | ✅ Mitigated |
| desktop-* | Medium | Desktop environment access | ✅ Mostly mitigated |
| package-manager-* | Medium-High | System package installation | ✅ Well mitigated |
| curlpipe | High | Script execution | ✅ Heavily mitigated |
| docker | Medium-High | Container operations | ⚠️ Relies on Docker security |
| system-setup | High | System-level changes | ⚠️ Inherently high risk |

## Security Recommendations

### For Developers
1. Always validate inputs using shared SDK functions
2. Use exec.CommandContext for all command execution
3. Implement environment-specific availability checks
4. Follow principle of least privilege
5. Log security-relevant operations

### For Users
1. Review plugin permissions before installation
2. Use trusted domains only for curlpipe operations
3. Regularly update plugins for security fixes
4. Monitor system logs for unusual activity
5. Use --dry-run flags to preview operations

### For System Administrators
1. Restrict sudo access to necessary plugins only
2. Monitor package manager operations
3. Implement network policies for plugin access
4. Regular security audits of plugin installations
5. Backup configurations before plugin operations

## Conclusion

The DevEx plugin ecosystem implements a defense-in-depth security model with multiple layers of protection:

1. **Input validation** prevents injection attacks
2. **Command validation** prevents malicious execution
3. **Environment checks** ensure appropriate context
4. **Audit logging** provides security visibility
5. **Principle of least privilege** minimizes attack surface

While some plugins inherently carry higher risks due to their functionality, appropriate mitigations are in place to manage these risks effectively.
