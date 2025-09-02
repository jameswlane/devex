# devex recover

Intelligent error recovery and troubleshooting assistance for DevEx operations.

## Overview

The `devex recover` command provides automated error analysis and guided recovery assistance when DevEx operations fail. It analyzes errors, suggests appropriate recovery strategies, and can automatically execute recovery procedures.

## Usage

```bash
devex recover [flags]
devex recover --error "error message" --operation "operation-name"
devex recover --analyze-last
devex recover --interactive
devex recover --execute <option-id>
```

## Key Features

### 🤖 **Intelligent Error Analysis**
- Analyzes error messages and operation context
- Identifies common failure patterns and causes
- Suggests recovery strategies ranked by priority

### 🛠️ **Automated Recovery**
- Executes recovery procedures automatically when safe
- Uses existing backup and undo systems
- Provides manual guidance for complex issues

### 📊 **Recovery Strategies**
- **Backup Restore**: Restore from recent backups
- **Undo Operations**: Reverse recent changes
- **Config Reset**: Reset corrupted configurations
- **Cache Cleanup**: Clear problematic cache data
- **System Repair**: Fix permissions and dependencies

## Command Options

### Analysis Flags

```bash
--error string         Error message to analyze
--operation string     Operation that failed (install, config, etc.)
--analyze-last         Analyze the most recent error from logs
--list-options         Show all available recovery capabilities
```

### Execution Flags

```bash
--execute string       Execute specific recovery option by ID
--interactive          Launch interactive recovery wizard
--dry-run             Show what would be done without executing
```

### Output Flags

```bash
--format string        Output format (table, json, yaml) [default: table]
```

## Examples

### Basic Error Analysis

```bash
# Analyze a specific error
devex recover --error "failed to install docker" --operation "install"

# Analyze the last failed operation
devex recover --analyze-last
```

### Interactive Recovery

```bash
# Launch the recovery wizard
devex recover --interactive

# List all recovery capabilities
devex recover --list-options
```

### Automated Recovery

```bash
# Execute a specific recovery option
devex recover --execute restore-recent-backup

# Preview what would be done
devex recover --execute cleanup-cache --dry-run
```

### Output Formats

```bash
# JSON output for scripting
devex recover --analyze-last --format json

# YAML output
devex recover --error "config invalid" --operation "config" --format yaml
```

## Recovery Types

### 1. Backup Restoration 🔄

**When Used:**
- Configuration corruption
- Failed operations that modified config
- Need to return to known good state

**Available Options:**
- Restore from most recent backup
- Choose from backup history
- Emergency configuration reset

```bash
devex recover --execute restore-recent-backup
```

### 2. Undo Operations ↩️

**When Used:**
- Recent changes caused issues
- Need to reverse specific operations
- Operation completed but with problems

**Available Options:**
- Undo last operation
- Rollback to specific version
- Selective change reversal

```bash
devex recover --execute undo-recent-operation
```

### 3. Configuration Recovery 📝

**When Used:**
- YAML parsing errors
- Invalid configuration syntax
- Permission issues

**Available Options:**
- Fix YAML syntax errors
- Reset to default configuration
- Repair file permissions

```bash
devex recover --execute fix-permissions
```

### 4. Installation Recovery 📦

**When Used:**
- Package installation failures
- Dependency conflicts
- Network/download issues

**Available Options:**
- Update package manager cache
- Retry with alternative installers
- Force reinstallation
- Clear installation cache

```bash
devex recover --execute update-package-cache
```

### 5. System Cleanup 🧹

**When Used:**
- Disk space issues
- Corrupted cache data
- Performance problems

**Available Options:**
- Clear DevEx cache
- Remove temporary files
- Clean up old backups

```bash
devex recover --execute cleanup-cache
```

### 6. Manual Guidance 📖

**When Used:**
- Complex issues requiring human intervention
- System-specific problems
- Debugging assistance

**Available Options:**
- Step-by-step troubleshooting
- System health checks
- Help system integration

## Recovery Priorities

### 🚨 **Critical**
- Operations that address severe issues
- High confidence of success
- Minimal risk of additional problems

### ✅ **Recommended** 
- Standard recovery procedures
- Good success rate
- Well-tested solutions

### 💡 **Optional**
- Alternative approaches
- Lower confidence solutions
- May have side effects

### 🔧 **Last Resort**
- Manual intervention required
- Higher risk procedures
- Complex troubleshooting steps

## Interactive Mode

The interactive recovery wizard provides guided assistance:

```bash
devex recover --interactive
```

**Features:**
- Step-by-step problem identification
- Contextual question prompts
- Automated recovery execution
- Manual guidance when needed

## Integration with Other Commands

Recovery suggestions are automatically shown when commands fail:

```bash
devex install docker
# If this fails, automatic recovery suggestions appear:
# 💡 Recovery Suggestions:
#    ✅ Update Package Manager Cache
#       → devex recover --execute update-package-cache
#    💡 Try Alternative Installer
#       → Try with --installer flatpak (manual)
```

## Recovery Logging

All recovery operations are logged for audit and debugging:

```bash
# Logs are saved to:
~/.devex/logs/recovery/recovery-20240101-120000.json
```

## Safety Features

### Backup Before Recovery
- Automatic backup creation before destructive operations
- Multiple restore points maintained
- Recovery operation history

### Risk Assessment
- Each recovery option includes risk evaluation
- Clear warnings for potentially destructive operations
- Confirmation required for high-risk procedures

### Rollback Capability
- Recovery operations can themselves be undone
- Multiple recovery attempt tracking
- Fallback to manual procedures if automation fails

## Advanced Usage

### Scripting Support

```bash
# Get recovery options in JSON
options=$(devex recover --analyze-last --format json)

# Execute the first recommended option
devex recover --execute $(echo "$options" | jq -r '.recovery_options[0].id')
```

### Custom Recovery Contexts

```bash
# Provide detailed context for better analysis
devex recover \
  --error "Connection refused: docker daemon not running" \
  --operation "install" \
  --format json > recovery-analysis.json
```

## Error Patterns

Common error patterns and their typical recovery strategies:

| Error Pattern | Typical Recovery | Command |
|---------------|------------------|---------|
| `permission denied` | Fix permissions | `--execute fix-permissions` |
| `package not found` | Update cache | `--execute update-package-cache` |
| `invalid yaml` | Restore config | `--execute restore-recent-backup` |
| `disk space` | Clean cache | `--execute cleanup-cache` |
| `connection timeout` | Retry with fallback | `--execute retry-with-fallback` |

## Integration Examples

### In Installation Scripts

```bash
#!/bin/bash
if ! devex install docker; then
    echo "Installation failed, attempting automatic recovery..."
    devex recover --error "docker install failed" --operation "install" --execute restore-recent-backup
fi
```

### In CI/CD Pipelines

```yaml
- name: Setup Development Environment
  run: |
    devex install --all || {
      devex recover --analyze-last --format json > recovery.json
      devex recover --execute $(jq -r '.recovery_options[0].id' recovery.json)
      devex install --all  # Retry after recovery
    }
```

## Troubleshooting

### Recovery Command Itself Fails

```bash
# Emergency reset when recovery is broken
devex config reset --emergency

# Manual recovery using backups
devex config backup list
devex config backup restore <backup-id>
```

### No Recovery Options Available

```bash
# Get manual troubleshooting guidance
devex help troubleshooting

# Check system status
devex status --all --verbose

# Validate configuration
devex config validate --verbose
```

## Related Commands

- [`devex undo`](undo.md) - Undo specific operations
- [`devex config backup`](config.md#backup-management) - Manual backup management
- [`devex status`](status.md) - System health checks
- [`devex help troubleshooting`](../troubleshooting.md) - Manual troubleshooting guide

## See Also

- [Troubleshooting Guide](../troubleshooting.md)
- [Configuration Management](config.md)
- [Backup & Restore](../backup-restore.md)
- [Error Codes Reference](../error-codes.md)

