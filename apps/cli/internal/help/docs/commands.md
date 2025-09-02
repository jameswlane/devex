# Command Reference

Complete reference for all DevEx commands with examples and options.

## Global Flags

All commands support these global flags:

- `--verbose`, `-v` - Enable verbose output
- `--dry-run` - Show what would be done without making changes
- `--help`, `-h` - Show help for any command
- `--no-tui` - Disable TUI and use simple text output

## Core Commands

### `devex init`

Initialize a new DevEx configuration with an interactive wizard.

```bash
devex init [flags]
```

**Options:**
- `--force` - Overwrite existing configuration
- `--template <name>` - Start with a specific template
- `--minimal` - Create minimal configuration
- `--non-interactive` - Skip interactive wizard

**Examples:**
```bash
# Interactive setup
devex init

# Start with React template
devex init --template react-fullstack

# Force overwrite existing config
devex init --force

# Minimal setup without wizard
devex init --minimal --non-interactive
```

### `devex install`

Install applications and packages defined in your configuration.

```bash
devex install [apps...] [flags]
```

**Options:**
- `--all` - Install all configured applications
- `--category <name>` - Install specific category
- `--installer <name>` - Force specific installer (apt, brew, etc.)
- `--parallel <num>` - Number of parallel installations
- `--user` - Install to user directory when possible
- `--update` - Update existing packages during install

**Examples:**
```bash
# Install all configured apps
devex install --all

# Install specific applications
devex install docker code python3

# Install development tools category
devex install --category development

# Use specific installer
devex install --installer brew

# Install with updates
devex install --all --update
```

### `devex uninstall`

Remove applications and clean up configurations.

```bash
devex uninstall [apps...] [flags]
```

**Options:**
- `--all` - Remove all installed applications
- `--category <name>` - Remove specific category
- `--purge` - Remove configuration files too
- `--keep-config` - Keep configuration entries
- `--force` - Skip dependency checks

**Examples:**
```bash
# Remove specific application
devex uninstall docker

# Remove with configuration
devex uninstall docker --purge

# Remove entire category
devex uninstall --category development

# Force removal without dependency checks
devex uninstall --all --force
```

### `devex add`

Add new applications to your configuration interactively.

```bash
devex add [flags]
```

**Options:**
- `--category <name>` - Browse specific category
- `--search <term>` - Search for applications
- `--batch` - Add multiple applications in one session

**Examples:**
```bash
# Interactive application browser
devex add

# Search for specific tools
devex add --search "database"

# Browse development tools
devex add --category development
```

### `devex remove`

Remove applications from configuration with dependency checking.

```bash
devex remove [apps...] [flags]
```

**Options:**
- `--force` - Skip dependency warnings
- `--keep-installed` - Remove from config but keep installed

**Examples:**
```bash
# Remove from configuration
devex remove obsolete-tool

# Remove but keep installed
devex remove --keep-installed temporary-tool
```

### `devex status`

Check the status of your development environment.

```bash
devex status [apps...] [flags]
```

**Options:**
- `--all` - Show status of all applications
- `--category <name>` - Check specific category
- `--format <format>` - Output format (table, json, yaml)
- `--fix` - Attempt to fix issues automatically
- `--detailed` - Show detailed version information

**Examples:**
```bash
# Check all applications
devex status --all

# Check specific apps
devex status docker git python3

# Check with auto-fix
devex status --all --fix

# JSON output for scripts
devex status --all --format json
```

## Configuration Management

### `devex config`

Manage DevEx configuration files and settings.

```bash
devex config <subcommand> [flags]
```

#### `devex config list`

List all configuration files and settings.

```bash
devex config list [flags]
```

**Options:**
- `--format <format>` - Output format (table, json, yaml)
- `--category <name>` - Show specific category

#### `devex config validate`

Validate configuration files for syntax and logic errors.

```bash
devex config validate [files...] [flags]
```

**Options:**
- `--fix` - Attempt to fix common issues automatically

#### `devex config backup`

Manage configuration backups.

```bash
devex config backup <action> [flags]
```

**Actions:**
- `create [description]` - Create new backup
- `list` - List available backups
- `restore <backup-id>` - Restore from backup
- `delete <backup-id>` - Delete backup
- `cleanup` - Remove old backups

**Options:**
- `--tags <tags>` - Add tags to backup (comma-separated)
- `--compress` - Compress backup files
- `--auto` - Enable automatic backups

**Examples:**
```bash
# Create backup with description
devex config backup create "Before team template"

# Create tagged backup
devex config backup create "Pre-migration" --tags "migration,v2"

# List all backups
devex config backup list

# Restore specific backup
devex config backup restore backup-20240115-143022
```

#### `devex config export`

Export configuration in various formats.

```bash
devex config export [flags]
```

**Options:**
- `--format <format>` - Export format (yaml, json, bundle)
- `--output <file>` - Output file path
- `--compress` - Create compressed bundle
- `--include-cache` - Include cache in bundle

**Examples:**
```bash
# Export as YAML
devex config export --format yaml

# Create bundle for team sharing
devex config export --format bundle --output team-config.zip

# Include cache data
devex config export --format bundle --include-cache
```

#### `devex config import`

Import configuration from files or URLs.

```bash
devex config import <source> [flags]
```

**Options:**
- `--merge` - Merge with existing configuration
- `--backup` - Create backup before import
- `--validate` - Validate before importing

**Examples:**
```bash
# Import from file
devex config import team-config.yaml

# Import and merge
devex config import new-tools.yaml --merge

# Import from URL
devex config import https://company.com/devex-template.yaml
```

#### `devex config reset`

Reset configuration to defaults.

```bash
devex config reset [flags]
```

**Options:**
- `--keep-personal` - Keep personal customizations
- `--backup` - Create backup before reset

#### `devex config migrate`

Migrate configuration to newer versions.

```bash
devex config migrate [flags]
```

**Options:**
- `--from <version>` - Specify source version
- `--to <version>` - Specify target version
- `--backup` - Create backup before migration

## Template Management

### `devex template`

Manage configuration templates for quick setup.

```bash
devex template <subcommand> [flags]
```

#### `devex template list`

List available templates.

```bash
devex template list [flags]
```

**Options:**
- `--category <name>` - Filter by category
- `--remote` - Include remote templates
- `--format <format>` - Output format

#### `devex template apply`

Apply a template to current configuration.

```bash
devex template apply <template> [flags]
```

**Options:**
- `--merge` - Merge with existing configuration
- `--backup` - Create backup before applying
- `--variables <file>` - Variable substitution file

**Examples:**
```bash
# Apply React template
devex template apply react-fullstack

# Apply with backup
devex template apply backend-api --backup

# Apply with custom variables
devex template apply custom-stack --variables vars.yaml
```

#### `devex template create`

Create a new template from current configuration.

```bash
devex template create <name> [flags]
```

**Options:**
- `--description <text>` - Template description
- `--category <name>` - Template category
- `--tags <tags>` - Template tags
- `--public` - Make template public (if supported)

#### `devex template update`

Update templates to latest versions.

```bash
devex template update [template] [flags]
```

**Options:**
- `--all` - Update all templates
- `--force` - Force update even if no changes
- `--format <format>` - Output format

## Cache Management

### `devex cache`

Manage installation cache and performance data.

```bash
devex cache <subcommand> [flags]
```

#### `devex cache cleanup`

Clean up cached files and data.

```bash
devex cache cleanup [flags]
```

**Options:**
- `--max-size <size>` - Maximum cache size (e.g., 1GB)
- `--max-age <duration>` - Maximum age (e.g., 30d)
- `--force` - Don't prompt for confirmation

**Examples:**
```bash
# Clean cache older than 30 days
devex cache cleanup --max-age 30d

# Limit cache to 500MB
devex cache cleanup --max-size 500MB

# Force cleanup without prompts
devex cache cleanup --force
```

#### `devex cache analyze`

Analyze cache performance and usage.

```bash
devex cache analyze [flags]
```

**Options:**
- `--format <format>` - Output format (table, json)
- `--detailed` - Show detailed analysis

#### `devex cache stats`

Show cache statistics.

```bash
devex cache stats [flags]
```

## System Commands

### `devex system`

System-level operations and diagnostics.

```bash
devex system <subcommand> [flags]
```

#### `devex system info`

Show system information and compatibility.

```bash
devex system info [flags]
```

**Options:**
- `--format <format>` - Output format
- `--check-compat` - Check compatibility with DevEx

#### `devex system doctor`

Run comprehensive system diagnostics.

```bash
devex system doctor [flags]
```

**Options:**
- `--fix` - Attempt to fix issues automatically
- `--verbose` - Show detailed diagnostic information

### `devex completion`

Generate shell completion scripts.

```bash
devex completion <shell> [flags]
```

**Supported shells:**
- `bash`
- `zsh`
- `fish`
- `powershell`

**Examples:**
```bash
# Generate bash completion
devex completion bash > /etc/bash_completion.d/devex

# Install zsh completion
devex completion zsh > "${fpath[1]}/_devex"
```

### `devex help`

Show help information with interactive viewer.

```bash
devex help [topic] [flags]
```

**Options:**
- `--search <term>` - Search help topics

**Examples:**
```bash
# Interactive help browser
devex help

# Show specific topic
devex help install

# Search help topics
devex help --search "template"
```

### `devex version`

Show version information.

```bash
devex version [flags]
```

**Options:**
- `--short` - Show only version number
- `--json` - Output as JSON

## Advanced Usage

### Environment Variables

- `DEVEX_CONFIG_DIR` - Override config directory (default: ~/.devex)
- `DEVEX_CACHE_DIR` - Override cache directory (default: ~/.devex/cache)
- `DEVEX_LOG_LEVEL` - Set log level (debug, info, warn, error)
- `DEVEX_NO_TUI` - Disable TUI globally
- `DEVEX_INSTALLER` - Force specific installer globally

### Configuration File Precedence

1. Command-line flags
2. Environment variables
3. User config files (`~/.devex/`)
4. System defaults

### Exit Codes

- `0` - Success
- `1` - General error
- `2` - Command usage error
- `3` - Configuration error
- `4` - Installation error
- `5` - Network error

### Scripting and Automation

```bash
# Check if DevEx is properly configured
devex status --all --format json | jq '.status == "ok"'

# Install with error handling
if ! devex install --all; then
    echo "Installation failed"
    exit 1
fi

# Batch operations
devex config backup create "Automated backup $(date +%Y%m%d)"
devex install --all --dry-run > planned-changes.txt
```

---

*For more detailed help on any command, use `devex <command> --help` or the interactive help system with `devex help`.*
