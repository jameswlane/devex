# devex init

Initialize a new DevEx development environment configuration.

## Synopsis

```bash
devex init [flags]
```

## Description

The `init` command creates a new DevEx configuration through an interactive wizard. It detects your system, asks about your development preferences, and generates the necessary configuration files in `~/.devex/`.

This command is typically the first thing you run when setting up DevEx on a new machine or starting a new project.

## Options

```
      --force               Overwrite existing configuration without prompting
      --template string     Start with a specific template (e.g., "react-fullstack")
      --minimal             Create minimal configuration without interactive wizard
      --non-interactive     Skip interactive questions and use defaults
  -h, --help               help for init
```

## Global Flags

```
      --dry-run     Show what would be done without making changes
      --no-tui      Disable TUI and use simple text output
  -v, --verbose     Enable verbose output
```

## Interactive Wizard

When run without flags, `devex init` launches an interactive wizard that guides you through:

### 1. Development Type Selection
Choose your primary development focus:
- **Full-Stack Web Development** - React, Vue, Angular with backend
- **Backend API Development** - REST APIs, GraphQL, microservices
- **Frontend Development** - Modern web applications
- **Mobile Development** - React Native, Flutter, native apps
- **Data Science** - Python, R, Jupyter, ML tools
- **DevOps/SRE** - Infrastructure, monitoring, automation
- **Custom Setup** - Manual configuration

### 2. Programming Languages
Select languages you work with:
- JavaScript/TypeScript
- Python
- Go
- Java
- Rust
- C/C++
- Ruby
- PHP
- And more...

### 3. Development Tools
Choose tools to include:
- **Editors**: VS Code, IntelliJ IDEA, Vim/Neovim
- **Version Control**: Git, GitHub CLI
- **Containers**: Docker, Podman
- **Databases**: PostgreSQL, MySQL, MongoDB
- **And many more...**

### 4. System Preferences
Configure system-level settings:
- Terminal preferences
- Shell configuration
- Git setup
- SSH configuration

## Examples

### Basic Interactive Setup
```bash
devex init
```
Launches the full interactive wizard.

### Start with Template
```bash
devex init --template react-fullstack
```
Initialize using the React full-stack template as a starting point.

### Minimal Setup
```bash
devex init --minimal --non-interactive
```
Create a basic configuration without any interactive prompts.

### Force Overwrite
```bash
devex init --force
```
Overwrite existing configuration without prompting for confirmation.

### Dry Run
```bash
devex init --dry-run
```
See what files would be created without actually creating them.

## Templates

You can start with pre-configured templates:

- `web-fullstack` - Full-stack web development
- `react-frontend` - React frontend development
- `vue-frontend` - Vue.js frontend development
- `backend-api` - Backend API development
- `python-data` - Python data science
- `devops-tools` - DevOps and infrastructure tools
- `mobile-dev` - Mobile development tools

List available templates:
```bash
devex template list
```

## Files Created

The init command creates these files in `~/.devex/`:

```
~/.devex/
├── applications.yaml    # Application definitions
├── environment.yaml     # Languages and development tools
├── system.yaml         # Git, SSH, terminal settings
├── desktop.yaml        # Desktop environment settings (optional)
├── cache/              # Installation cache directory
├── backups/            # Configuration backups
└── templates/          # Custom templates
```

## Configuration Structure

### applications.yaml
Defines applications to install organized by category:
```yaml
categories:
  development:
    - name: code
      description: Visual Studio Code
      installer: snap
    - name: docker
      description: Container platform
      installer: apt
  
  system:
    - name: git
      description: Version control
      installer: apt
```

### environment.yaml
Defines programming languages and tools:
```yaml
languages:
  node:
    version: "18"
    installer: mise
  python:
    version: "3.11"
    installer: mise
    packages:
      - pip
      - virtualenv
```

### system.yaml
System-level configurations:
```yaml
git:
  user:
    name: "Your Name"
    email: "you@example.com"
  core:
    editor: "code --wait"

ssh:
  generate_key: true
  key_type: "ed25519"
```

## Post-Initialization

After running `devex init`, you can:

1. **Review configuration**:
   ```bash
   devex config list
   ```

2. **Install applications**:
   ```bash
   devex install --all
   ```

3. **Check status**:
   ```bash
   devex status --all
   ```

4. **Add more tools**:
   ```bash
   devex add
   ```

## Troubleshooting

### Permission Issues
```bash
# If you get permission errors, ensure ~/.devex directory is writable
chmod 755 ~/.devex
```

### Existing Configuration
```bash
# Back up existing config before overwriting
devex config backup create "Before reinit"
devex init --force
```

### Template Not Found
```bash
# List available templates
devex template list

# Update templates to latest
devex template update --all
```

### Non-Interactive Mode Issues
```bash
# Use minimal mode for automated setups
devex init --minimal --non-interactive
```

## Related Commands

- `devex config` - Manage configuration after initialization
- `devex template` - Work with templates
- `devex install` - Install configured applications
- `devex add` - Add more applications

## See Also

- [Quick Start Guide](../quick-start) - Complete setup walkthrough
- [Configuration Guide](../config) - Advanced configuration options
- [Template System](../templates) - Using and creating templates
