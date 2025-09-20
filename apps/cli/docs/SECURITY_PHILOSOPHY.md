# DevEx Security Philosophy

## The Problem with Overly Restrictive Security

Our current security validation is too restrictive for real-world usage. Popular tools like [Omakub](https://github.com/basecamp/omakub/) and other development environment scripts succeed because they trust users while providing reasonable guardrails.

## Proposed Security Model: "Informed Consent + Smart Guardrails"

### Current Issues
- Hard-coded command whitelists don't scale with user customization
- Blocking legitimate shell patterns like `$(which command)` 
- Too restrictive for system administrators and DevOps workflows
- Prevents users from adapting DevEx to their specific environments

### New Approach: Tiered Security with User Choice

#### 1. **Preview + Confirmation Mode** (Default)
```bash
# Show users exactly what will be executed
devex install custom-app

📋 DevEx will execute the following commands:
  
  Pre-install:
  ✓ curl -fsSL https://example.com/setup.sh | bash
  ✓ sudo apt update
  
  Install:
  ✓ sudo apt install -y custom-package
  
  Post-install:
  ✓ ln -sf $(which custom-tool) ~/.local/bin/tool
  ✓ echo 'eval "$(custom-tool init bash)"' >> ~/.bashrc

⚠️  Security Analysis:
  • Downloads and executes remote script (medium risk)
  • Modifies system packages (requires sudo)
  • Modifies shell configuration (low risk)

Continue? [y/N/details/trust]
```

#### 2. **Security Levels**

**Level 1: Guided (Default)**
- Show preview of all commands
- Require confirmation for risky operations
- Block obviously destructive patterns (`rm -rf /`, fork bombs)
- Allow most development patterns

**Level 2: Permissive** 
- Minimal warnings for common dev patterns
- Only block clearly destructive commands
- Trust user judgment on custom configurations

**Level 3: Paranoid**
- Block anything not explicitly whitelisted
- Require individual confirmation for each command
- Maximum security for sensitive environments

#### 3. **Smart Pattern Detection**

Instead of blanket blocking, categorize commands:

```yaml
# Automatically allowed (no confirmation needed)
safe_patterns:
  - package_managers: "apt|brew|pip|npm install"
  - version_checks: ".*--version|.*-v"
  - shell_integration: 'eval "\$\(.*init.*\)"'
  - common_tools: "git|docker|curl|wget .*"

# Show warning but allow with confirmation
risky_patterns:
  - remote_execution: "curl.*|.*bash|wget.*|.*sh"
  - sudo_commands: "sudo (?!apt|brew|systemctl|docker)"
  - system_modification: "systemctl|service.*"

# Always block (override with --force-unsafe)
dangerous_patterns:
  - destruction: "rm -rf /|rm -rf /home|rm -rf /usr"
  - fork_bombs: ":\(\)\{.*:\|:&"
  - privilege_escalation: "chmod.*s|setuid"
```

#### 4. **Trust Levels for Sources**

```yaml
trusted_sources:
  - "*.github.com"
  - "get.docker.com"
  - "*.npmjs.org"
  
community_sources:  # Require confirmation
  - "raw.githubusercontent.com"
  - Custom repositories

untrusted_sources:  # Extra warnings
  - Unknown domains
  - IP addresses
```

### Implementation Strategy

#### Phase 1: Fix Immediate Blocking Issues
1. **Allow common development patterns**:
   - `sudo updatedb` for search tools
   - `$(which command)` substitution  
   - Shell init commands like `eval "$(tool init shell)"`
   - Directory creation before symlinks

2. **Update security validation** to be permissive by default:
   ```go
   // Only block obviously dangerous patterns
   if isDangerous(command) {
       return askUserConfirmation(command, "This command could damage your system")
   }
   // Allow everything else
   return nil
   ```

#### Phase 2: Enhanced User Experience
1. **Command preview system**
2. **Interactive confirmation prompts**
3. **Security level configuration**
4. **Trusted source management**

#### Phase 3: Advanced Features
1. **Sandboxing for risky operations**
2. **Rollback capabilities**
3. **Audit logging**
4. **Team/organization security policies**

### Key Principles

1. **Trust Users**: Most DevEx users are developers/admins who know what they're doing
2. **Inform, Don't Block**: Show what will happen, let users decide
3. **Sensible Defaults**: Block obviously destructive operations
4. **Customizable**: Allow teams to set their own security policies
5. **Transparent**: Never hide what commands are being executed

### Inspiration from Successful Tools

- **Homebrew**: Warns about untrusted sources but doesn't block
- **rustup**: Shows what will be installed, requires confirmation
- **Omakub**: Trusts users, focuses on good UX over paranoid security
- **Docker**: Powerful but doesn't block reasonable operations

### Emergency Override

For advanced users who need maximum flexibility:
```bash
devex install --security=off custom-dangerous-app
devex install --force-unsafe custom-app
```

This approach maintains security for novice users while giving power users the flexibility they need.
