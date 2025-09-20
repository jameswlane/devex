# Security Package - Pattern-Based Command Validation

This package provides a flexible, pattern-based approach to command validation that replaces the hard-coded whitelist system. It balances security with flexibility for user-defined custom configurations across Linux, Windows, and macOS.

## Problem Solved

The previous validation system used a hard-coded `allowedCommands` map that:
- Required code changes for every new tool
- Didn't scale for user-defined applications  
- Wasn't maintainable across different platforms
- Broke when users added custom applications with their own commands

## New Approach: Pattern-Based Security

### Security Levels

The new system provides three configurable security levels:

#### 1. SecurityLevelStrict
- Only allows explicitly safe platform commands
- Blocks everything except known-good tools
- Best for production environments with strict requirements

#### 2. SecurityLevelModerate (Default)
- Allows platform commands + common development tools
- Permits executable files with path validation
- Good balance of security and flexibility

#### 3. SecurityLevelPermissive  
- Only blocks obviously dangerous commands
- Allows most tools but prevents system destruction
- Best for development environments

### Key Features

1. **Platform-Aware**: Automatically recognizes Windows, macOS, and Linux commands
2. **Configuration-Driven**: Extracts commands from application YAML configs
3. **Pattern-Based Security**: Uses regex patterns instead of hard-coded lists
4. **Extensible**: Supports custom whitelist/blacklist rules
5. **Context-Aware**: Validates commands in the context of specific applications

## Usage Examples

### Basic Usage

```go
import "github.com/jameswlane/devex/pkg/security"

// Create validator with moderate security
validator := security.NewCommandValidator(security.SecurityLevelModerate)

// Validate a command
err := validator.ValidateCommand("apt install git")
if err != nil {
    // Command blocked for security reasons
    log.Error("Command validation failed", "error", err)
}
```

### Configuration-Aware Validation

```go
// Load applications from config
apps := loadApplicationsFromConfig()

// Create config-aware validator
validator := security.NewConfigBasedValidator(security.SecurityLevelModerate, apps)

// Validate command in context of specific app
err := validator.ValidateConfigCommand("custom-tool --install", "my-app")
```

### Custom Rules

```go
validator := security.NewCommandValidator(security.SecurityLevelModerate)

// Add custom commands to whitelist
validator.AddCustomWhitelist("terraform", "kubectl", "helm")

// Add commands to blacklist
validator.AddCustomBlacklist("dangerous-tool", "risky-command")
```

### Secure Command Executor

```go
// Create secure executor for TUI installer
executor := security.NewSecureCommandExecutor(
    security.SecurityLevelModerate, 
    apps, // loaded applications
)

// Use in streaming installer
installer := tui.NewStreamingInstallerWithSecureExecutor(
    program, repo, ctx, 
    security.SecurityLevelModerate, 
    apps, settings,
)
```

## Security Patterns

### Always Blocked (Dangerous Patterns)

```bash
# System destruction
rm -rf /
dd if=/dev/zero of=/dev/sda

# Sensitive files
cat /etc/shadow
echo "backdoor" >> /etc/passwd

# Network backdoors
nc -l -p 4444 -e /bin/bash

# Fork bombs
:(){ :|:& };:

# Directory traversal
cat ../../../etc/passwd
```

### Always Allowed (Safe Patterns)

```bash
# Package management
sudo apt install git
brew install node

# Version checks
git --version
node -v

# Safe file operations
mkdir ~/.config/myapp
cp config.yaml ~/.config/myapp/

# System information
uname -a
whoami
```

### Context-Dependent

Commands that are validated based on:
- Security level setting
- Platform (Linux/macOS/Windows)
- Application configuration
- Custom whitelist/blacklist

## Configuration Integration

### Application YAML Example

```yaml
applications:
  development:
    - name: custom-dev-tool
      description: 'Custom development tool'
      linux:
        install_method: 'custom'
        install_command: 'custom-installer --setup'
        pre_install:
          - command: 'setup-dependencies'
          - shell: 'export PATH=$PATH:/opt/custom'
        post_install:
          - command: 'configure-tool --init'
```

The validator automatically extracts and whitelists:
- `custom-installer` (from install_command)
- `setup-dependencies` (from pre_install)
- `configure-tool` (from post_install)

### User Configuration

Users can create `~/.devex/security.yaml`:

```yaml
security:
  level: moderate  # strict, moderate, permissive
  custom_whitelist:
    - terraform
    - kubectl  
    - helm
    - docker-compose
  custom_blacklist:
    - dangerous-tool
    - risky-command
  platform_overrides:
    linux:
      additional_commands:
        - snap
        - flatpak
```

## Migration Guide

### From Hard-coded Validation

**Before:**
```go
// Hard-coded map
allowedCommands := map[string]bool{
    "apt": true,
    "git": true,
    // ... 60+ more commands
}

if !allowedCommands[command] {
    return fmt.Errorf("command not allowed")
}
```

**After:**
```go
// Pattern-based validation
validator := security.NewCommandValidator(security.SecurityLevelModerate)
if err := validator.ValidateCommand(command); err != nil {
    return fmt.Errorf("command validation failed: %w", err)
}
```

### Adding New Tools

**Before:** Required code changes to `allowedCommands` map

**After:** Tools are automatically whitelisted when:
1. They appear in application configurations
2. They match safe patterns (e.g., package managers)
3. They're added to custom whitelist

## Testing

```go
func TestCommandValidation(t *testing.T) {
    validator := security.NewCommandValidator(security.SecurityLevelModerate)
    
    // Test safe command
    err := validator.ValidateCommand("git clone https://github.com/user/repo")
    assert.NoError(t, err)
    
    // Test dangerous command  
    err = validator.ValidateCommand("rm -rf /")
    assert.Error(t, err)
    
    // Test custom whitelist
    validator.AddCustomWhitelist("my-tool")
    err = validator.ValidateCommand("my-tool --install")
    assert.NoError(t, err)
}
```

## Benefits

1. **Scalable**: No code changes needed for new tools
2. **Flexible**: Supports user-defined applications and configurations
3. **Secure**: Maintains strong security through pattern-based validation
4. **Cross-platform**: Works across Linux, Windows, and macOS
5. **Maintainable**: Centralized security logic with clear patterns
6. **Extensible**: Easy to add custom rules and new security levels

## Security Considerations

1. **Pattern Bypass**: Regularly review and update dangerous patterns
2. **Configuration Validation**: Validate application configs for malicious commands
3. **Custom Rules**: Audit custom whitelist/blacklist rules
4. **Logging**: Log all blocked commands for security monitoring
5. **Updates**: Keep security patterns updated with new threats

This approach provides a much more maintainable and flexible solution while maintaining strong security guarantees.
