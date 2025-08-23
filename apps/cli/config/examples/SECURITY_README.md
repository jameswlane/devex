# DevEx Security Configuration System

This directory contains example security configurations for the DevEx CLI tool. The security system provides configurable command validation with override capabilities, similar to lint-ignore systems.

## Overview

The DevEx security system balances functionality with safety by:
- Providing multiple security levels (Strict, Moderate, Permissive, Enterprise)
- Allowing specific security rule overrides with transparency
- Warning users when potentially dangerous operations are bypassed
- Maintaining protection against critical system-damaging commands

## Security Levels

### 0 - Strict
- Only explicitly safe patterns are allowed
- Unknown executables are blocked
- All network and file system operations require explicit approval
- Suitable for: High-security environments, shared systems

### 1 - Moderate (Default)
- Most common development operations are allowed
- Dangerous patterns are blocked
- Unknown executables are allowed if in system PATH
- Suitable for: Most development environments

### 2 - Permissive
- Only critical dangerous patterns are blocked
- Trusts user judgment for most operations
- Provides warnings but minimal restrictions
- Suitable for: Personal development machines, experienced developers

### 3 - Enterprise
- Maximum flexibility with warning transparency
- Assumes enterprise security controls are in place
- All operations logged for audit purposes
- Suitable for: Enterprise environments with additional security layers

## Configuration Files

### Default Configuration
- **Location**: `~/.local/share/devex/config/security.yaml`
- **Purpose**: System-wide default security settings
- **Override**: Can be overridden by user-specific config

### User Override Configuration
- **Location**: `~/.devex/security.yaml`
- **Purpose**: User-specific security overrides
- **Priority**: Takes precedence over system defaults

## Security Rule Types

| Rule Type | Description | Examples |
|-----------|-------------|----------|
| `dangerous-command` | Commands that could damage the system | `rm -rf /`, `dd to disks` |
| `unknown-executable` | Executables not in system PATH | Custom scripts, downloaded binaries |
| `command-injection` | Shell injection patterns | Pipes to shell, command substitution |
| `privilege-escalation` | Commands requiring elevated privileges | `sudo` operations |
| `network-access` | Commands accessing network resources | `curl`, `wget`, `ssh` |
| `filesystem-access` | Commands modifying system files | Writing to `/etc`, `/usr`, `/var` |

## Override Configuration Structure

```yaml
# Security level (0=Strict, 1=Moderate, 2=Permissive, 3=Enterprise)
level: 1

# Enterprise mode settings
enterprise_mode: false
warn_on_overrides: true

# Global overrides (apply to all applications)
global_overrides:
  - rule_type: "command-injection"
    pattern: "curl.*github\\.com.*\\| bash"
    reason: "Allow GitHub installation scripts"
    warn_user: true

# Application-specific overrides
app_overrides:
  docker:
    - rule_type: "privilege-escalation"
      pattern: "sudo docker.*"
      reason: "Docker requires sudo for installation"
      warn_user: false
```

## Example Configurations

### security-strict.yaml
For high-security environments where only explicitly approved operations are allowed.

**Use Cases:**
- Shared development servers
- Production-like environments
- Security-sensitive projects
- Learning environments

**Characteristics:**
- Minimal overrides
- Only verified safe operations
- Explicit approval required for network access
- No shell injection patterns allowed

### security.yaml (Default/Moderate)
Balanced configuration suitable for most development environments.

**Use Cases:**
- Individual development machines
- Team development environments
- Standard software projects
- CI/CD environments

**Characteristics:**
- Common development tools allowed
- Package managers trusted
- Some shell operations permitted
- Reasonable security boundaries

### security-permissive.yaml
For experienced developers who need maximum flexibility.

**Use Cases:**
- Personal development machines
- Rapid prototyping environments
- Experienced developer workstations
- Research and experimentation

**Characteristics:**
- Most operations allowed
- Only critical dangers blocked
- Warnings for transparency
- Trust-based approach

### security-enterprise.yaml
For enterprise environments with additional security controls.

**Use Cases:**
- Enterprise development teams
- Organizations with security frameworks
- Environments with monitoring/auditing
- Large-scale development operations

**Characteristics:**
- Maximum flexibility
- Comprehensive logging
- Assumes external security controls
- Administrative override capabilities

## Usage Examples

### Using a Custom Security Configuration

```bash
# Set security config via environment variable
export DEVEX_SECURITY_CONFIG="~/.devex/security-custom.yaml"
devex install

# Or specify during installation
devex install --security-config /path/to/security.yaml
```

### Creating Custom Overrides

1. Copy an example configuration:
   ```bash
   cp ~/.local/share/devex/config/examples/security-permissive.yaml ~/.devex/security.yaml
   ```

2. Edit to add your specific overrides:
   ```yaml
   app_overrides:
     my-custom-tool:
       - rule_type: "unknown-executable"
         pattern: "/opt/custom/bin/.*"
         reason: "Allow custom enterprise tools"
         warn_user: true
   ```

3. Validate your configuration:
   ```bash
   devex security validate ~/.devex/security.yaml
   ```

## Best Practices

### For Organizations
- Start with `security-strict.yaml` and add overrides as needed
- Document all overrides with clear reasons
- Regular security configuration reviews
- Use enterprise mode for maximum visibility

### For Individual Developers
- Start with default moderate level
- Add specific overrides for your development tools
- Enable warnings to stay aware of security implications
- Regularly review and clean up unnecessary overrides

### For Security-Sensitive Projects
- Use strict mode with minimal overrides
- Require approval for any new overrides
- Log all security warnings
- Regular audit of allowed operations

## Security Override Patterns

### Common Patterns

```yaml
# Allow specific GitHub repositories
- pattern: "git clone https://github\\.com/(company|user)/[a-zA-Z0-9\\-_]+\\.git"

# Allow trusted installation sources
- pattern: "curl -fsSL https://(cli\\.github\\.com|mise\\.jdx\\.dev)/.*\\| bash"

# Allow development tool setup
- pattern: "bash -c 'export PATH=.*\\$HOME/.local/bin.*'"

# Allow temporary directory cleanup
- pattern: "rm -rf /tmp/[a-zA-Z0-9\\-_]+"
```

### Pattern Guidelines
- Use specific patterns rather than broad wildcards
- Include domain/path restrictions for network operations
- Limit file system access to specific directories
- Escape regex special characters properly

## Troubleshooting

### Command Blocked Unexpectedly
1. Check current security level: `devex security status`
2. Review active overrides: `devex security list-overrides`
3. Test pattern matching: `devex security test-pattern "your-command"`
4. Add override if appropriate

### Override Not Working
1. Validate configuration syntax: `devex security validate`
2. Check pattern regex syntax
3. Verify rule type matches the command being blocked
4. Ensure configuration file is in correct location

### Too Many Warnings
1. Adjust `warn_on_overrides` setting
2. Set `warn_user: false` for trusted overrides
3. Consider moving to more permissive security level

## Security Considerations

### What This System Protects Against
- Accidental system damage from typos
- Obviously malicious command patterns
- Unintended privilege escalation
- Injection attacks through configuration

### What This System Does NOT Protect Against
- Sophisticated targeted attacks
- Malicious code in trusted applications
- Social engineering attacks
- Supply chain compromises

### Additional Security Recommendations
- Keep system and applications updated
- Use proper file permissions
- Monitor system logs
- Implement network security controls
- Regular security audits

## Contributing

When adding new security patterns or overrides:
1. Document the reason clearly
2. Use specific patterns when possible
3. Test with real-world commands
4. Consider security implications
5. Update documentation

For questions or security concerns, please see the DevEx documentation at https://docs.devex.sh/security/
