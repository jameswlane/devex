# Quick Start Guide

Get up and running with DevEx in just a few minutes!

## Prerequisites

- Linux (Ubuntu 18.04+), macOS (10.15+), or Windows 10+
- Internet connection for package downloads
- Modern terminal with Unicode support

## Step 1: Install DevEx

### One-Line Installation (Recommended)

```bash
wget -qO- https://devex.sh/install | bash
```

Or with curl:

```bash
curl -fsSL https://devex.sh/install | bash
```

### Verify Installation

```bash
devex --version
```

## Step 2: Initialize Your First Project

Create a new development environment:

```bash
devex init
```

This launches an interactive wizard that will:
- Detect your operating system and platform
- Ask about your development stack preferences
- Create initial configuration files
- Set up the directory structure in `~/.devex/`

Example initialization flow:
```
Welcome to DevEx! Let's set up your development environment.

? What type of development do you primarily do?
  > Full-Stack Web Development
    Backend API Development
    Frontend Development
    Mobile Development
    Data Science
    DevOps/SRE
    Custom Setup

? Which programming languages do you use? (Select multiple)
  ✓ JavaScript/TypeScript
  ✓ Python
  ✓ Go
    Java
    Rust
    Other

? Do you want to include development tools?
  ✓ Docker
  ✓ Git
  ✓ VS Code
    IntelliJ IDEA
    Other editors

Creating configuration files...
✓ Created ~/.devex/applications.yaml
✓ Created ~/.devex/environment.yaml
✓ Created ~/.devex/system.yaml
✓ Set up cache directory

Your DevEx environment is ready!
```

## Step 3: Install Your Development Tools

Install everything defined in your configuration:

```bash
devex install
```

DevEx will:
- Analyze your system and select the best package managers
- Install applications in the optimal order
- Show progress with a beautiful terminal interface
- Handle dependencies automatically

Example installation:
```
Installing development environment...

📦 Installing System Packages (apt)
  ✓ curl (already installed)
  ✓ git (already installed)
  → docker.io (installing...)

🐍 Installing Python Packages
  → pip (installing...)

📱 Installing Applications
  → Visual Studio Code (installing...)

Installation completed successfully!
Installed 12 packages in 3m 42s
```

## Step 4: Check Your Environment

Verify everything is working:

```bash
devex status --all
```

This shows the status of all configured applications:
```
Development Environment Status

✓ System Tools
  ✓ git (v2.34.1)
  ✓ curl (v7.81.0)
  ✓ docker (v20.10.17)

✓ Development Tools
  ✓ code (v1.74.3)
  ✓ python3 (v3.10.6)
  ✓ node (v18.12.1)

✓ Languages & Runtimes
  ✓ go (v1.19.4)
  ✓ python (v3.10.6)

All systems operational! 🚀
```

## Common Next Steps

### Add More Applications

```bash
devex add
```

This opens an interactive browser to add new tools:
```
? What would you like to add?
  Applications
  > Development Tools
    Language Runtimes
    Desktop Applications

? Select development tools to add:
  ✓ Postman
  ✓ DBeaver
    Figma
    Slack
```

### Manage Configuration

```bash
# View current configuration
devex config list

# Create a backup
devex config backup create "Before major changes"

# Export for sharing
devex config export --format yaml
```

### Use Templates

```bash
# Browse available templates
devex template list

# Apply a template for React development
devex template apply react-fullstack

# Create custom template
devex template create my-stack
```

## Pro Tips

### 🔄 Stay Updated
```bash
# Update DevEx itself
devex self-update

# Update all installed packages
devex update --all
```

### 💾 Backup Before Changes
```bash
# DevEx automatically creates backups, but you can create manual ones
devex config backup create "Before team template"
```

### 🔍 Explore Commands
```bash
# Get help for any command
devex help install
devex install --help

# Interactive help system
devex help
```

### 🎯 Team Collaboration
```bash
# Export your configuration for team sharing
devex config export --format bundle > my-devenv.zip

# Apply a team configuration
devex config import my-devenv.zip
```

### ⚡ Performance Optimization
```bash
# Clean old cached files
devex cache cleanup

# Analyze installation performance
devex cache analyze
```

## Common Workflows

### Setting Up a New Machine

1. Install DevEx
2. Import existing configuration: `devex config import team-config.yaml`
3. Install everything: `devex install`
4. Verify: `devex status --all`

### Starting a New Project

1. `devex template list` - Browse available templates
2. `devex template apply <template-name>` - Apply project template
3. `devex add` - Add any additional tools needed
4. `devex install` - Install new additions

### Team Onboarding

1. Team lead exports config: `devex config export --format bundle`
2. New team member imports: `devex config import team-config.zip`
3. Install: `devex install`
4. Customize: `devex add` (for personal preferences)

## Troubleshooting

### Installation Issues

If installations fail:
```bash
# Check system compatibility
devex status --system

# Retry with verbose output
devex install --verbose

# Use different installer
devex install --installer snap
```

### Permission Problems

For permission issues:
```bash
# Install to user directory
devex install --user

# Fix PATH if needed
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
```

### Configuration Issues

```bash
# Validate configuration
devex config validate

# Reset to defaults
devex config reset

# Restore from backup
devex config restore
```

## What's Next?

- 📖 Read the [Command Reference](commands) for detailed usage
- 🎨 Learn about [Templates](templates) for quick project setup
- ⚙️ Explore [Configuration](config) for advanced customization
- 🐛 Check [Troubleshooting](troubleshooting) if you run into issues
- 💬 Join our [community](https://github.com/jameswlane/devex/discussions) for support

---

*You're now ready to streamline your development workflow with DevEx! 🚀*
