# DevEx Tool Plugins

This guide provides comprehensive documentation for DevEx tool plugins that configure and manage development tools and environment settings.

## Table of Contents

- [Overview](#overview)
- [Git Tool Plugin](#git-tool-plugin)
- [Shell Tool Plugin](#shell-tool-plugin)
- [Stack Detector Plugin](#stack-detector-plugin)
- [Common Usage Patterns](#common-usage-patterns)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

## Overview

DevEx tool plugins provide specialized functionality for configuring development tools, managing shell environments, and analyzing project stacks. Unlike package manager plugins that install software, tool plugins focus on configuration and environment setup.

### Available Tool Plugins

| Plugin | Purpose | Status | Primary Use |
|--------|---------|--------|-------------|
| tool-git | Git configuration and aliases | ✅ Production | Version control setup |
| tool-shell | Shell configuration and switching | ✅ Production | Terminal environment |
| tool-stackdetector | Project stack analysis | ✅ Production | Technology detection |

## Git Tool Plugin

The Git tool plugin provides comprehensive Git configuration, including user settings, sensible defaults, and helpful aliases.

### Basic Configuration

#### User Configuration
```bash
# Configure Git user information
devex plugin exec tool-git config --name "Your Name" --email "your.email@example.com"

# Show current configuration
devex plugin exec tool-git config

# Configure with interactive prompts
devex git config --setup
```

#### Global Git Settings
```bash
# Apply sensible defaults
devex plugin exec tool-git config --defaults

# Show current Git configuration
git config --global --list
```

### Default Configuration Settings

The Git plugin applies these sensible defaults:

```bash
# Core settings
git config --global core.editor "vim"
git config --global color.ui "auto"
git config --global init.defaultBranch "main"

# Merge and diff settings
git config --global pull.rebase "false"
git config --global push.default "simple"
git config --global merge.conflictstyle "diff3"
git config --global diff.colorMoved "default"

# Performance and cleanup
git config --global fetch.prune "true"
git config --global credential.helper "cache --timeout=3600"
```

### Git Aliases

#### Installing Common Aliases
```bash
# Install helpful Git aliases
devex plugin exec tool-git aliases --install

# List available aliases
devex plugin exec tool-git aliases --list

# Show alias definitions
devex plugin exec tool-git aliases --show
```

#### Common Aliases Included
```bash
# Status and information
git st      # git status
git br      # git branch
git co      # git checkout
git ci      # git commit

# Logging and history
git lg      # git log --oneline --graph --decorate --all
git last    # git log -1 HEAD
git unstage # git reset HEAD --

# Branch management
git switch  # git checkout -b
git delete  # git branch -d
git pushup  # git push --set-upstream origin

# Advanced workflows
git amend   # git commit --amend --no-edit
git fixup   # git commit --fixup
git squash  # git rebase -i --autosquash
```

### Advanced Git Configuration

#### SSH Configuration
```bash
# Generate SSH key for Git
devex plugin exec tool-git ssh-setup --generate-key

# Configure SSH key for GitHub/GitLab
devex plugin exec tool-git ssh-setup --service github

# Test SSH connection
devex plugin exec tool-git ssh-test --service github
```

#### Repository Templates
```bash
# Create Git repository template
devex plugin exec tool-git template --create development

# Apply template to new repository
devex plugin exec tool-git template --apply development

# List available templates
devex plugin exec tool-git template --list
```

### Git Status Integration

#### Enhanced Status Display
```bash
# Show enhanced Git status
devex plugin exec tool-git status

# Show status with branch information
devex plugin exec tool-git status --detailed

# Show status for multiple repositories
devex plugin exec tool-git status --recursive
```

### Practical Examples

#### New Project Setup
```bash
# Complete Git setup for new project
cd new-project
git init
devex plugin exec tool-git config --name "Your Name" --email "your@email.com"
devex plugin exec tool-git aliases --install

# Create initial commit with sensible gitignore
echo "node_modules/" > .gitignore
echo ".env" >> .gitignore
git add .
git commit -m "Initial commit with DevEx configuration"
```

#### Team Configuration Sync
```bash
# Export Git configuration
devex plugin exec tool-git config --export team-git-config.json

# Apply team configuration
devex plugin exec tool-git config --import team-git-config.json

# Verify configuration
devex plugin exec tool-git config --verify
```

## Shell Tool Plugin

The Shell tool plugin manages shell configuration, environment switching, and customization across different shell types.

### Shell Detection and Information

#### Current Shell Information
```bash
# Detect current shell
devex plugin exec tool-shell detect

# Show shell information
devex plugin exec tool-shell info

# List available shells
devex plugin exec tool-shell list-available
```

### Shell Setup and Configuration

#### Initial Shell Setup
```bash
# Set up current shell with DevEx configurations
devex plugin exec tool-shell setup

# Set up specific shell
devex plugin exec tool-shell setup --shell zsh

# Apply configurations to all available shells
devex plugin exec tool-shell setup --all
```

#### Configuration Management
```bash
# Show current shell configuration
devex plugin exec tool-shell config --show

# Edit shell configuration
devex plugin exec tool-shell config --edit

# Apply custom configuration
devex plugin exec tool-shell config --apply custom-config.sh

# Reset to defaults
devex plugin exec tool-shell config --reset
```

### Shell Switching

#### Changing Default Shell
```bash
# Switch to Zsh
devex plugin exec tool-shell switch zsh

# Switch to Bash
devex plugin exec tool-shell switch bash

# Switch to Fish
devex plugin exec tool-shell switch fish

# Verify shell change
devex plugin exec tool-shell switch --verify
```

#### Temporary Shell Sessions
```bash
# Start temporary Zsh session
devex plugin exec tool-shell switch --temp zsh

# Exit temporary session
exit
```

### Configuration Backup and Restore

#### Backup Management
```bash
# Create configuration backup
devex plugin exec tool-shell backup --create

# List available backups
devex plugin exec tool-shell backup --list

# Restore from backup
devex plugin exec tool-shell backup --restore latest

# Remove old backups
devex plugin exec tool-shell backup --clean --older-than 30d
```

### Shell-Specific Features

#### Bash Configuration
```bash
# Apply Bash-specific optimizations
devex plugin exec tool-shell setup --shell bash --optimize

# Install Bash completion
devex plugin exec tool-shell setup --shell bash --completion

# Configure Bash prompt
devex plugin exec tool-shell config --shell bash --prompt colorful
```

#### Zsh Configuration
```bash
# Set up Zsh with Oh My Zsh
devex plugin exec tool-shell setup --shell zsh --framework oh-my-zsh

# Configure Zsh theme
devex plugin exec tool-shell config --shell zsh --theme powerlevel10k

# Install Zsh plugins
devex plugin exec tool-shell config --shell zsh --plugins "git docker kubectl npm"
```

#### Fish Configuration
```bash
# Set up Fish shell
devex plugin exec tool-shell setup --shell fish

# Configure Fish theme
devex plugin exec tool-shell config --shell fish --theme bobthefish

# Install Fisher package manager
devex plugin exec tool-shell setup --shell fish --package-manager fisher
```

### Applied Configurations

The Shell plugin applies these configurations automatically:

#### Bash Configuration
```bash
# History settings
export HISTSIZE=10000
export HISTFILESIZE=20000
export HISTCONTROL=ignoreboth:erasedups

# Color support
export CLICOLOR=1

# Useful aliases
alias ll='ls -la'
alias la='ls -A'
alias ..='cd ..'
alias ...='cd ../..'
```

#### Zsh Configuration
```bash
# History settings
HISTSIZE=10000
SAVEHIST=20000
setopt HIST_IGNORE_DUPS
setopt HIST_IGNORE_SPACE
setopt SHARE_HISTORY

# Color support
autoload -U colors && colors

# Enhanced completion
autoload -Uz compinit && compinit
```

#### Fish Configuration
```bash
# Color settings
set -g fish_color_command green
set -g fish_color_error red
set -g fish_color_param cyan

# Useful aliases
alias ll 'ls -la'
alias la 'ls -A'
alias .. 'cd ..'
```

### Practical Examples

#### Development Environment Setup
```bash
# Set up comprehensive shell environment
devex plugin exec tool-shell setup
devex plugin exec tool-shell switch zsh
devex plugin exec tool-shell config --theme powerlevel10k --plugins "git docker kubectl"

# Verify setup
devex plugin exec tool-shell config --show
```

#### Cross-Shell Configuration
```bash
# Configure all available shells
for shell in bash zsh fish; do
    devex plugin exec tool-shell setup --shell $shell
done

# Apply consistent aliases across shells
devex plugin exec tool-shell config --apply-aliases --all-shells
```

## Stack Detector Plugin

The Stack Detector plugin analyzes projects to identify technologies, frameworks, and dependencies.

### Project Analysis

#### Basic Stack Detection
```bash
# Detect technologies in current directory
devex plugin exec tool-stackdetector detect

# Analyze specific directory
devex plugin exec tool-stackdetector analyze /path/to/project

# Generate detailed report
devex plugin exec tool-stackdetector report --output project-analysis.json
```

#### Recursive Analysis
```bash
# Analyze all projects in directory
devex plugin exec tool-stackdetector detect --recursive ~/Projects

# Analyze with depth limit
devex plugin exec tool-stackdetector detect --recursive --max-depth 2

# Skip certain directories
devex plugin exec tool-stackdetector detect --recursive --ignore "node_modules,.git,build"
```

### Detection Capabilities

The Stack Detector can identify:

#### Programming Languages
```bash
# Languages detected from file extensions and content
- JavaScript/TypeScript (package.json, .js, .ts files)
- Python (requirements.txt, setup.py, .py files)
- Go (go.mod, .go files)
- Java (pom.xml, build.gradle, .java files)
- Rust (Cargo.toml, .rs files)
- PHP (composer.json, .php files)
- Ruby (Gemfile, .rb files)
- C/C++ (Makefile, CMakeLists.txt, .c, .cpp files)
```

#### Frameworks and Tools
```bash
# Frontend frameworks
- React (package.json dependencies)
- Vue.js (vue.config.js, Vue dependencies)
- Angular (angular.json, @angular dependencies)
- Svelte (svelte.config.js)

# Backend frameworks
- Express.js (express dependency)
- Django (requirements.txt with Django)
- Flask (requirements.txt with Flask)
- Spring Boot (pom.xml with Spring dependencies)
- Rails (Gemfile with rails)

# Build tools
- Webpack (webpack.config.js)
- Vite (vite.config.js)
- Gulp (gulpfile.js)
- Gradle (build.gradle)
- Maven (pom.xml)
```

#### Infrastructure and Services
```bash
# Containerization
- Docker (Dockerfile, docker-compose.yml)
- Kubernetes (*.yaml with kind: Deployment)

# Databases
- PostgreSQL (pg_* files, postgres dependencies)
- MySQL (mysql dependencies)
- MongoDB (mongo dependencies)
- Redis (redis dependencies)

# Cloud services
- AWS (aws-cli, boto3 dependencies)
- Azure (azure-cli dependencies)
- Google Cloud (gcloud configurations)
```

### Analysis Output

#### Report Formats
```bash
# JSON output (default)
devex plugin exec tool-stackdetector report --format json

# YAML output
devex plugin exec tool-stackdetector report --format yaml

# Markdown report
devex plugin exec tool-stackdetector report --format markdown

# Plain text summary
devex plugin exec tool-stackdetector report --format text
```

#### Sample JSON Output
```json
{
  "project_path": "/home/user/my-project",
  "analysis_date": "2024-01-15T10:30:00Z",
  "languages": [
    {
      "name": "JavaScript",
      "confidence": 0.95,
      "files_count": 42,
      "primary": true
    },
    {
      "name": "TypeScript",
      "confidence": 0.87,
      "files_count": 15,
      "primary": false
    }
  ],
  "frameworks": [
    {
      "name": "React",
      "version": "18.2.0",
      "confidence": 0.98
    },
    {
      "name": "Express.js",
      "version": "4.18.1",
      "confidence": 0.92
    }
  ],
  "build_tools": [
    {
      "name": "Webpack",
      "config_file": "webpack.config.js"
    }
  ],
  "databases": [
    {
      "name": "PostgreSQL",
      "evidence": ["pg", "postgres"]
    }
  ],
  "recommendations": [
    "Install Node.js 18+ for optimal React development",
    "Consider using TypeScript consistently throughout the project",
    "PostgreSQL development environment setup recommended"
  ]
}
```

### Practical Examples

#### New Project Analysis
```bash
# Clone a repository and analyze it
git clone https://github.com/example/project.git
cd project
devex plugin exec tool-stackdetector analyze

# Generate setup recommendations
devex plugin exec tool-stackdetector report --recommendations --format markdown > SETUP.md
```

#### Development Environment Setup
```bash
# Analyze project and auto-configure DevEx
cd my-project
devex plugin exec tool-stackdetector detect --auto-configure

# This would:
# 1. Detect Node.js/React project
# 2. Install Node.js via package manager
# 3. Install development dependencies
# 4. Configure editor settings
# 5. Set up debugging configuration
```

#### Multi-Project Analysis
```bash
# Analyze all projects in workspace
devex plugin exec tool-stackdetector analyze ~/Workspace --recursive --output workspace-analysis.json

# Generate technology summary
devex plugin exec tool-stackdetector report --input workspace-analysis.json --summary
```

## Common Usage Patterns

### Complete Development Setup

#### New Machine Setup
```bash
# Configure Git
devex plugin exec tool-git config --name "Your Name" --email "your@email.com"
devex plugin exec tool-git aliases --install

# Set up shell environment
devex plugin exec tool-shell setup
devex plugin exec tool-shell switch zsh

# Analyze existing projects
find ~/Projects -name ".git" -type d | while read gitdir; do
    project_dir=$(dirname "$gitdir")
    echo "Analyzing: $project_dir"
    devex plugin exec tool-stackdetector analyze "$project_dir"
done
```

#### Project Onboarding
```bash
# Clone and setup new project
git clone https://github.com/company/project.git
cd project

# Analyze project stack
devex plugin exec tool-stackdetector analyze --recommendations

# Configure Git for project
devex plugin exec tool-git config --project

# Set up appropriate shell environment
devex plugin exec tool-shell config --project-specific
```

### Team Configuration Sync

#### Export Team Settings
```bash
# Export Git configuration
devex plugin exec tool-git config --export team-git-config.json

# Export shell configuration
devex plugin exec tool-shell config --export team-shell-config.sh

# Create team setup script
cat > team-setup.sh << 'EOF'
#!/bin/bash
devex plugin exec tool-git config --import team-git-config.json
devex plugin exec tool-shell config --import team-shell-config.sh
devex plugin exec tool-shell switch zsh
EOF
```

#### Import Team Settings
```bash
# Apply team configuration
chmod +x team-setup.sh
./team-setup.sh

# Verify configuration
devex plugin exec tool-git config --verify
devex plugin exec tool-shell config --show
```

## Configuration

### Global Configuration

Configure tool plugins in `~/.devex/config.yaml`:

```yaml
tools:
  git:
    auto_configure: true
    default_branch: main
    editor: vim
    aliases:
      - st: status
      - co: checkout
      - br: branch
      - ci: commit
    
  shell:
    preferred: zsh
    auto_setup: true
    backup_configs: true
    frameworks:
      zsh: oh-my-zsh
      bash: bash-it
    
  stackdetector:
    auto_analyze: true
    confidence_threshold: 0.7
    ignore_patterns:
      - node_modules
      - .git
      - build
      - dist
```

### Environment Variables

```bash
# Git tool configuration
export DEVEX_GIT_DEFAULT_BRANCH=main
export DEVEX_GIT_EDITOR=vim
export DEVEX_GIT_AUTO_ALIASES=true

# Shell tool configuration
export DEVEX_SHELL_PREFERRED=zsh
export DEVEX_SHELL_AUTO_SWITCH=true
export DEVEX_SHELL_BACKUP=true

# Stack detector configuration
export DEVEX_DETECTOR_AUTO_ANALYZE=true
export DEVEX_DETECTOR_RECURSIVE=false
export DEVEX_DETECTOR_MAX_DEPTH=3
```

## Troubleshooting

### Git Tool Issues

#### Configuration Problems
```bash
# Check Git installation
which git
git --version

# Verify configuration
devex plugin exec tool-git config --verify

# Reset to defaults
devex plugin exec tool-git config --reset

# Debug configuration issues
devex plugin exec tool-git --debug config --show
```

#### SSH Key Issues
```bash
# Test SSH connection
devex plugin exec tool-git ssh-test

# Regenerate SSH key
devex plugin exec tool-git ssh-setup --regenerate

# Check SSH agent
ssh-add -l
```

### Shell Tool Issues

#### Shell Switching Problems
```bash
# Check available shells
cat /etc/shells
which zsh bash fish

# Verify current shell
echo $SHELL
echo $0

# Fix shell switching
devex plugin exec tool-shell switch --force zsh

# Check for permission issues
sudo chsh -s $(which zsh) $USER
```

#### Configuration Conflicts
```bash
# Backup current configuration
devex plugin exec tool-shell backup --create emergency

# Reset configuration
devex plugin exec tool-shell config --reset

# Restore from backup if needed
devex plugin exec tool-shell backup --restore emergency
```

### Stack Detector Issues

#### Analysis Failures
```bash
# Check project permissions
ls -la
find . -name "package.json" -o -name "requirements.txt"

# Run with debug output
devex plugin exec tool-stackdetector --debug analyze

# Manual analysis
devex plugin exec tool-stackdetector detect --verbose --no-cache
```

#### Performance Issues
```bash
# Limit analysis scope
devex plugin exec tool-stackdetector analyze --max-files 1000

# Use cache for repeated analysis
devex plugin exec tool-stackdetector analyze --use-cache

# Skip large directories
devex plugin exec tool-stackdetector analyze --ignore "node_modules,venv,.git"
```

### Getting Debug Information

```bash
# Show tool plugin status
devex plugin status tool-git tool-shell tool-stackdetector

# Generate debug report
devex debug tools --output tools-debug.log

# Check plugin dependencies
devex plugin deps tool-git
devex plugin deps tool-shell
devex plugin deps tool-stackdetector
```

For more detailed information about specific tools and advanced configuration options, refer to the individual plugin documentation in their respective directories.
