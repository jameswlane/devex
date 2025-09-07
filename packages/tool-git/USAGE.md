# Git Tool Plugin - Usage Guide

The Git tool plugin provides comprehensive Git configuration, alias management, and status reporting capabilities through DevEx.

## Table of Contents

- [Installation](#installation)
- [Basic Configuration](#basic-configuration)
- [Alias Management](#alias-management)
- [Status Reporting](#status-reporting)
- [Advanced Features](#advanced-features)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Examples](#examples)

## Installation

### Prerequisites

- Git installed on the system (version 2.0 or higher recommended)
- DevEx CLI installed and configured

### Verification

```bash
# Check Git installation
git --version

# Check if Git plugin is available
devex plugin list | grep tool-git

# Test Git plugin functionality
devex plugin exec tool-git --help
```

## Basic Configuration

### User Configuration

#### Setting Up Git User Information
```bash
# Configure Git with name and email
devex plugin exec tool-git config --name "Your Name" --email "your.email@example.com"

# Configure interactively (prompts for missing information)
devex plugin exec tool-git config

# Show current configuration
devex plugin exec tool-git config --show
```

#### Checking Current Configuration
```bash
# View current Git user configuration
git config --global user.name
git config --global user.email

# View all global Git configuration
git config --global --list
```

### Default Configuration Settings

The Git plugin automatically applies sensible defaults when you run the config command:

#### Core Settings
```bash
# Editor and interface settings
core.editor = vim              # Default text editor
color.ui = auto               # Automatic color output
init.defaultBranch = main     # Default branch name for new repos
```

#### Merge and Diff Settings
```bash
# Merge behavior
pull.rebase = false           # Use merge instead of rebase for pulls
push.default = simple         # Push current branch to upstream
merge.conflictstyle = diff3   # Show common ancestor in conflicts

# Diff enhancements
diff.colorMoved = default     # Highlight moved code blocks
```

#### Performance and Cleanup
```bash
# Automatic maintenance
fetch.prune = true           # Remove stale remote-tracking branches
credential.helper = cache --timeout=3600  # Cache credentials for 1 hour
```

### Applying Configuration

```bash
# Apply all default settings
devex plugin exec tool-git config --defaults

# Apply configuration with user information
devex plugin exec tool-git config \
  --name "John Doe" \
  --email "john.doe@company.com" \
  --defaults
```

## Alias Management

### Installing Git Aliases

```bash
# Install all recommended aliases
devex plugin exec tool-git aliases --install

# Show available aliases before installing
devex plugin exec tool-git aliases --show

# List currently installed aliases
devex plugin exec tool-git aliases --list
```

### Common Aliases Provided

#### Basic Operations
```bash
git st          # git status
git br          # git branch
git co          # git checkout
git ci          # git commit
```

#### Enhanced Logging
```bash
git lg          # git log --oneline --graph --decorate --all
git last        # git log -1 HEAD --stat
git hist        # git log --pretty=format:'%h %ad | %s%d [%an]' --graph --date=short
```

#### Staging and Unstaging
```bash
git unstage     # git reset HEAD --
git staged      # git diff --cached
git unstaged    # git diff
```

#### Branch Management
```bash
git switch      # git checkout -b (create and switch to branch)
git delete      # git branch -d (delete branch)
git pushup      # git push --set-upstream origin
git track       # git branch --set-upstream-to
```

#### Advanced Workflows
```bash
git amend       # git commit --amend --no-edit
git fixup       # git commit --fixup
git squash      # git rebase -i --autosquash
git wip         # git add -A && git commit -m "WIP"
git undo        # git reset --soft HEAD~1
```

### Custom Alias Management

```bash
# Add custom aliases (not provided by plugin)
git config --global alias.save '!git add -A && git commit -m "SAVEPOINT"'
git config --global alias.please 'push --force-with-lease'
git config --global alias.commend 'commit --amend --no-edit'

# View all aliases
git config --global --get-regexp alias
```

## Status Reporting

### Enhanced Git Status

```bash
# Show enhanced Git status
devex plugin exec tool-git status

# Show detailed status with branch information
devex plugin exec tool-git status --detailed

# Show status for multiple repositories
devex plugin exec tool-git status --recursive /path/to/projects
```

### Status Output Features

The Git tool plugin enhances standard `git status` output with:

- **Branch information**: Current branch and tracking status
- **Ahead/behind counts**: How many commits ahead or behind upstream
- **Stash information**: Number of stashed changes
- **Submodule status**: Status of Git submodules if present
- **Clean status**: Clear indication when repository is clean

### Integration with Shell Prompts

The plugin can provide status information for shell prompt integration:

```bash
# Get status for shell prompt (returns exit codes)
devex plugin exec tool-git status --porcelain

# Get branch name for prompt
devex plugin exec tool-git status --branch-name

# Get ahead/behind counts
devex plugin exec tool-git status --ahead-behind
```

## Advanced Features

### SSH Configuration

#### SSH Key Generation and Setup
```bash
# Generate SSH key for Git
devex plugin exec tool-git ssh-setup --generate-key

# Generate key with specific algorithm
devex plugin exec tool-git ssh-setup --generate-key --algorithm ed25519

# Configure SSH key for GitHub
devex plugin exec tool-git ssh-setup --service github

# Configure SSH key for GitLab
devex plugin exec tool-git ssh-setup --service gitlab

# Test SSH connection
devex plugin exec tool-git ssh-test --service github
```

#### SSH Key Management
```bash
# List SSH keys
ls -la ~/.ssh/

# Add SSH key to agent
ssh-add ~/.ssh/id_ed25519

# Test SSH connection manually
ssh -T git@github.com
ssh -T git@gitlab.com
```

### Repository Templates

#### Creating and Managing Templates
```bash
# Create repository template
devex plugin exec tool-git template --create web-development

# Apply template to current repository
devex plugin exec tool-git template --apply web-development

# List available templates
devex plugin exec tool-git template --list

# Show template contents
devex plugin exec tool-git template --show web-development
```

#### Template Examples

**Web Development Template**:
```bash
# Creates:
# - .gitignore (Node.js, logs, environment files)
# - README.md template
# - Basic folder structure (src/, tests/, docs/)
# - Git hooks for linting
```

**Python Development Template**:
```bash
# Creates:
# - .gitignore (Python, virtual environments)
# - requirements.txt template
# - setup.py template
# - Basic project structure
```

### Git Hooks Integration

```bash
# Install development Git hooks
devex plugin exec tool-git hooks --install development

# Install pre-commit hooks for linting
devex plugin exec tool-git hooks --install pre-commit

# Show available hook templates
devex plugin exec tool-git hooks --list
```

## Configuration

### Plugin Configuration

```yaml
# ~/.devex/config.yaml
tools:
  git:
    auto_configure: true
    default_branch: main
    editor: vim
    
    # Default user information (can be overridden per-repo)
    user:
      name: "Your Name"
      email: "your.email@example.com"
    
    # Alias configuration
    aliases:
      enable_advanced: true
      custom_aliases:
        save: "!git add -A && git commit -m 'SAVEPOINT'"
        please: "push --force-with-lease"
    
    # SSH configuration
    ssh:
      default_algorithm: "ed25519"
      key_location: "~/.ssh/id_ed25519"
    
    # Template configuration
    templates:
      auto_apply: false
      default_template: "web-development"
```

### Environment Variables

```bash
# Git tool plugin configuration
export DEVEX_GIT_DEFAULT_BRANCH=main
export DEVEX_GIT_EDITOR=vim
export DEVEX_GIT_AUTO_ALIASES=true
export DEVEX_GIT_SSH_ALGORITHM=ed25519

# Git configuration overrides
export GIT_EDITOR=vim
export GIT_AUTHOR_NAME="Your Name"
export GIT_AUTHOR_EMAIL="your.email@example.com"
```

### Per-Repository Configuration

```bash
# Set repository-specific configuration
cd /path/to/repo
git config user.name "Project Specific Name"
git config user.email "project@company.com"

# Use different SSH key for specific repository
git config core.sshCommand "ssh -i ~/.ssh/project_key"
```

## Troubleshooting

### Common Issues

#### Git Not Found
```bash
# Error: git is not installed on this system
# Solution: Install Git
sudo apt update && sudo apt install git  # Ubuntu/Debian
sudo yum install git                     # CentOS/RHEL
brew install git                         # macOS
```

#### Configuration Issues
```bash
# Check current configuration
git config --list --show-origin

# Fix configuration issues
devex plugin exec tool-git config --reset
devex plugin exec tool-git config --name "Your Name" --email "your@email.com"

# Verify configuration
devex plugin exec tool-git config --verify
```

#### SSH Key Problems
```bash
# Test SSH connection
ssh -T git@github.com

# Check SSH agent
ssh-add -l

# Regenerate SSH key if needed
devex plugin exec tool-git ssh-setup --regenerate --service github
```

#### Alias Conflicts
```bash
# Check existing aliases
git config --global --get-regexp alias

# Remove conflicting alias
git config --global --unset alias.conflicting-name

# Reinstall DevEx aliases
devex plugin exec tool-git aliases --install --force
```

### Debug Information

```bash
# Show detailed Git plugin information
devex plugin status tool-git --detailed

# Show Git configuration debug info
devex plugin exec tool-git --debug config --show

# Generate Git debug report
devex debug git --output git-debug.log

# Show Git version and build info
git --version --build-options
```

## Examples

### New Developer Onboarding

```bash
#!/bin/bash
# setup-git-environment.sh - Complete Git setup for new developers

echo "Setting up Git environment..."

# Configure Git user information
read -p "Enter your full name: " name
read -p "Enter your email address: " email

devex plugin exec tool-git config --name "$name" --email "$email"

# Install helpful aliases
devex plugin exec tool-git aliases --install

# Generate SSH key for GitHub
devex plugin exec tool-git ssh-setup --generate-key --service github

echo "Git configuration completed!"
echo "Next steps:"
echo "1. Add your SSH key to GitHub/GitLab"
echo "2. Test connection with: ssh -T git@github.com"
```

### Repository Initialization Script

```bash
#!/bin/bash
# init-repo.sh - Initialize new repository with DevEx configuration

if [ -z "$1" ]; then
    echo "Usage: $0 <project-name> [template]"
    exit 1
fi

project_name=$1
template=${2:-web-development}

# Create project directory
mkdir "$project_name"
cd "$project_name"

# Initialize Git repository
git init

# Apply DevEx Git configuration
devex plugin exec tool-git config
devex plugin exec tool-git aliases --install

# Apply project template
devex plugin exec tool-git template --apply "$template"

# Initial commit
git add .
git commit -m "Initial commit with DevEx configuration"

echo "Repository '$project_name' initialized successfully!"
echo "Template '$template' applied"
```

### Team Configuration Sync

```bash
#!/bin/bash
# sync-team-git-config.sh - Synchronize Git configuration across team

# Export current configuration
devex plugin exec tool-git config --export > team-git-config.json

# Distribution script for team members
cat > apply-team-config.sh << 'EOF'
#!/bin/bash
# Apply team Git configuration
if [ -f "team-git-config.json" ]; then
    devex plugin exec tool-git config --import team-git-config.json
    echo "Team Git configuration applied successfully!"
else
    echo "team-git-config.json not found!"
    exit 1
fi
EOF

chmod +x apply-team-config.sh
echo "Team configuration exported to team-git-config.json"
echo "Share both files with team members"
```

### Multi-Repository Status Check

```bash
#!/bin/bash
# check-repos.sh - Check status of multiple repositories

repositories=(
    "$HOME/Projects/web-app"
    "$HOME/Projects/api-server"
    "$HOME/Projects/mobile-app"
    "$HOME/Projects/documentation"
)

for repo in "${repositories[@]}"; do
    if [ -d "$repo/.git" ]; then
        echo "=== $(basename "$repo") ==="
        cd "$repo"
        devex plugin exec tool-git status
        echo
    else
        echo "=== $(basename "$repo") === (not a git repository)"
        echo
    fi
done
```

### Automated Git Maintenance

```bash
#!/bin/bash
# git-maintenance.sh - Perform Git maintenance tasks

repositories=$(find "$HOME/Projects" -name ".git" -type d | sed 's/\.git$//')

for repo in $repositories; do
    echo "Maintaining: $(basename "$repo")"
    cd "$repo"
    
    # Fetch latest changes
    git fetch --prune
    
    # Clean up merged branches
    git branch --merged main | grep -v main | xargs -n 1 git branch -d 2>/dev/null
    
    # Run Git garbage collection
    git gc --auto
    
    echo "✅ $(basename "$repo") maintained"
done

echo "Git maintenance completed for all repositories"
```

## Best Practices

1. **Consistent Configuration**: Use the Git plugin to ensure consistent configuration across development environments
2. **Meaningful Aliases**: Learn and use the provided aliases to improve productivity
3. **SSH Key Management**: Use strong SSH keys (Ed25519) and test connections regularly
4. **Branch Naming**: Use consistent branch naming conventions (feature/, bugfix/, hotfix/)
5. **Commit Messages**: Write clear, descriptive commit messages
6. **Regular Maintenance**: Perform regular repository cleanup and maintenance
7. **Security**: Never commit sensitive information; use .gitignore effectively
8. **Team Synchronization**: Share Git configuration across team members

## Integration with DevEx CLI

The Git tool plugin integrates with DevEx's main functionality:

```bash
# Git configuration is applied during DevEx setup
devex setup  # Includes Git configuration

# Git tool is available in system configuration
devex system apply --tools git

# Integration with other DevEx tools
devex install git  # Installs Git if not present
devex detect stack  # May use Git to analyze repository
```

For more information about DevEx and other plugins, see the main [USAGE.md](../../USAGE.md) documentation.
