# devex config

Manage DevEx configuration files, backups, and settings.

## Synopsis

```bash
devex config <subcommand> [flags]
```

## Description

The `config` command provides comprehensive configuration management for your DevEx environment. It includes operations for viewing, validating, backing up, exporting, importing, and resetting configuration files.

DevEx uses four main configuration files:
- `applications.yaml` - Application definitions and categories
- `environment.yaml` - Programming languages and development tools  
- `system.yaml` - Git, SSH, and terminal settings
- `desktop.yaml` - Desktop environment customizations (optional)

## Subcommands

### `devex config list`

List all configuration files and their contents.

```bash
devex config list [flags]
```

**Options:**
```
      --format string     Output format (table, json, yaml) (default "table")
      --category string   Show specific category only
      --summary          Show summary instead of full content
```

**Examples:**
```bash
# Show all configuration in table format
devex config list

# Show as JSON for scripting
devex config list --format json

# Show only development applications
devex config list --category development

# Show configuration summary
devex config list --summary
```

### `devex config validate`

Validate configuration files for syntax and logic errors.

```bash
devex config validate [files...] [flags]
```

**Options:**
```
      --fix              Attempt to fix common issues automatically
      --strict           Use strict validation rules
      --schema string    Path to custom schema file
```

**Examples:**
```bash
# Validate all configuration files
devex config validate

# Validate specific file
devex config validate ~/.devex/applications.yaml

# Validate and auto-fix issues
devex config validate --fix

# Strict validation with all rules
devex config validate --strict
```

### `devex config backup`

Manage configuration backups with versioning and metadata.

```bash
devex config backup <action> [flags]
```

#### Actions

**create [description]** - Create a new backup
```bash
devex config backup create [description] [flags]
```

**Options:**
```
      --tags strings     Add tags to backup (comma-separated)
      --compress         Compress backup files
      --include-cache    Include cache directory in backup
```

**Examples:**
```bash
# Create backup with description
devex config backup create "Before team template"

# Create backup with tags
devex config backup create "Migration prep" --tags "migration,v2,production"

# Create compressed backup
devex config backup create --compress "Full backup"
```

**list** - List available backups
```bash
devex config backup list [flags]
```

**Options:**
```
      --format string    Output format (table, json)
      --limit int        Number of backups to show (default 20)
```

**restore <backup-id>** - Restore configuration from backup
```bash
devex config backup restore <backup-id> [flags]
```

**Options:**
```
      --verify           Verify backup integrity before restore
      --backup-current   Create backup of current config before restore
```

**delete <backup-id>** - Delete a specific backup
```bash
devex config backup delete <backup-id> [flags]
```

**cleanup** - Remove old backups based on retention policy
```bash
devex config backup cleanup [flags]
```

**Options:**
```
      --keep-count int   Number of backups to keep (default 10)
      --keep-days int    Days to keep backups (default 30)
      --keep-tagged      Keep tagged backups regardless of age
```

### `devex config export`

Export configuration in various formats for sharing or backup.

```bash
devex config export [flags]
```

**Options:**
```
      --format string       Export format (yaml, json, bundle) (default "yaml")
      --output string       Output file path
      --compress            Create compressed bundle
      --include-cache       Include cache directory
      --include-backups     Include backup history
      --template            Export as template
```

**Examples:**
```bash
# Export as YAML to stdout
devex config export

# Export as JSON file
devex config export --format json --output my-config.json

# Create sharable bundle
devex config export --format bundle --output team-setup.zip

# Export as template
devex config export --template --output my-template.yaml

# Full export with cache and backups
devex config export --format bundle --include-cache --include-backups
```

### `devex config import`

Import configuration from files, URLs, or bundles.

```bash
devex config import <source> [flags]
```

**Options:**
```
      --merge               Merge with existing configuration
      --backup              Create backup before import
      --validate            Validate configuration before import
      --force               Force import even with validation errors
      --variables string    Variable substitution file
```

**Examples:**
```bash
# Import from local file
devex config import team-config.yaml

# Import and merge with existing config
devex config import additional-tools.yaml --merge

# Import with backup and validation
devex config import new-setup.yaml --backup --validate

# Import from URL
devex config import https://company.com/devex-template.yaml

# Import bundle
devex config import team-bundle.zip

# Import with variable substitution
devex config import template.yaml --variables vars.yaml
```

### `devex config reset`

Reset configuration to defaults or specific state.

```bash
devex config reset [flags]
```

**Options:**
```
      --keep-personal       Keep personal customizations
      --backup              Create backup before reset
      --to-template string  Reset to specific template
      --confirm             Skip confirmation prompt
```

**Examples:**
```bash
# Reset to defaults with backup
devex config reset --backup

# Keep personal settings
devex config reset --keep-personal

# Reset to specific template
devex config reset --to-template react-fullstack
```

### `devex config migrate`

Migrate configuration to newer format versions.

```bash
devex config migrate [flags]
```

**Options:**
```
      --from string    Source version (auto-detected if not specified)
      --to string      Target version (default: latest)
      --backup         Create backup before migration
      --dry-run       Show migration plan without executing
```

**Examples:**
```bash
# Auto-migrate to latest version
devex config migrate --backup

# Migrate from specific version
devex config migrate --from v1.0 --to v2.0

# Show migration plan
devex config migrate --dry-run
```

### `devex config edit`

Edit configuration files with your preferred editor.

```bash
devex config edit [file] [flags]
```

**Options:**
```
      --editor string    Override default editor
      --validate        Validate after editing
```

**Examples:**
```bash
# Edit applications.yaml
devex config edit applications

# Edit with specific editor
devex config edit system --editor vim

# Edit and validate
devex config edit --validate
```

### `devex config diff`

Compare configuration files or versions.

```bash
devex config diff [source] [target] [flags]
```

**Examples:**
```bash
# Compare with backup
devex config diff current backup-20240115-143022

# Compare configuration files
devex config diff ~/.devex/applications.yaml ~/.devex/backups/apps.yaml.bak

# Compare with template
devex config diff current template:react-fullstack
```

## Configuration File Structure

### applications.yaml
```yaml
# Application definitions organized by categories
categories:
  development:
    - name: code
      description: Visual Studio Code
      installers:
        linux: snap
        macos: brew
        windows: winget
      
  system:
    - name: git
      description: Version control system
      installer: system  # Uses system package manager
```

### environment.yaml
```yaml
# Programming languages and development environments
languages:
  node:
    version: "18"
    installer: mise
    global_packages:
      - npm
      - yarn
      - typescript

  python:
    version: "3.11"
    installer: mise
    packages:
      - pip
      - virtualenv
      - black
      - pytest

fonts:
  - name: "JetBrains Mono"
    installer: system
```

### system.yaml
```yaml
# System-level configurations
git:
  user:
    name: "Your Name"
    email: "you@example.com"
  core:
    editor: "code --wait"
  init:
    defaultBranch: "main"

ssh:
  generate_key: true
  key_type: "ed25519"
  key_comment: "DevEx generated key"

terminal:
  shell: "zsh"
  theme: "oh-my-zsh"
```

### desktop.yaml (Optional)
```yaml
# Desktop environment customizations
gnome:
  extensions:
    - "user-theme"
    - "dash-to-dock"
  
  settings:
    theme: "Adwaita-dark"
    icon_theme: "Papirus"

kde:
  theme: "Breeze Dark"
  widgets:
    - "weather"
    - "system-monitor"
```

## Backup Management

### Automatic Backups
DevEx automatically creates backups before:
- Configuration imports
- Template applications  
- Major configuration changes
- Version migrations

### Manual Backup Management
```bash
# Create tagged backup
devex config backup create "Before experiment" --tags "experimental,testing"

# List recent backups
devex config backup list --limit 5

# Restore specific backup
devex config backup restore backup-20240115-143022

# Clean old backups
devex config backup cleanup --keep-count 5 --keep-days 14
```

### Backup Storage
Backups are stored in `~/.devex/backups/` with metadata:
```
~/.devex/backups/
├── backup-20240115-143022/
│   ├── applications.yaml
│   ├── environment.yaml
│   ├── system.yaml
│   └── .metadata.json
└── backup-20240110-091530/
    ├── applications.yaml
    └── .metadata.json
```

## Import/Export Workflows

### Team Configuration Sharing
```bash
# Team lead exports configuration
devex config export --format bundle --output team-devex.zip

# Team member imports
devex config import team-devex.zip --backup --merge
```

### Template Creation
```bash
# Export current config as template
devex config export --template --output my-stack-template.yaml

# Apply template to new environment
devex config import my-stack-template.yaml
```

### Migration Between Machines
```bash
# Export from old machine
devex config export --format bundle --include-cache --output full-backup.zip

# Import on new machine
devex config import full-backup.zip
```

## Validation and Testing

### Configuration Validation
```bash
# Basic validation
devex config validate

# Strict validation with all rules
devex config validate --strict

# Auto-fix common issues
devex config validate --fix
```

### Test Configuration Changes
```bash
# Dry-run installation with new config
devex install --all --dry-run

# Validate before committing changes
devex config validate && devex config backup create "Validated config"
```

## Troubleshooting

### Configuration Errors
```bash
# Validate and get detailed errors
devex config validate --verbose

# Reset corrupted configuration
devex config reset --backup
```

### Backup Issues
```bash
# Verify backup integrity
devex config backup restore backup-id --verify --dry-run

# Force restore ignoring verification
devex config backup restore backup-id --force
```

### Import/Export Problems
```bash
# Validate before import
devex config import file.yaml --validate --dry-run

# Import with detailed logging
devex config import file.yaml --verbose
```

## Related Commands

- `devex init` - Initialize new configuration
- `devex template` - Work with configuration templates
- `devex install` - Install applications from configuration
- `devex status` - Check configuration status

## See Also

- [Configuration Guide](../config) - Detailed configuration documentation
- [Template System](../templates) - Using and creating templates
- [Backup and Recovery](../backup) - Backup strategies and recovery
