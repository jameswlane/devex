# DevEx CLI Usage Guide

DevEx is a comprehensive CLI tool for automating development environment setup across multiple platforms. This guide provides practical examples and real-world usage scenarios to help you get the most out of DevEx.

## Table of Contents

- [Quick Start](#quick-start)
- [Core Commands](#core-commands)
- [Installation Examples](#installation-examples)
- [Configuration](#configuration)
- [Package Manager Integration](#package-manager-integration)
- [Tool Configuration](#tool-configuration)
- [Template Usage](#template-usage)
- [Common Workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Basic Installation
```bash
# Install DevEx
wget -qO- https://devex.sh/install | bash

# Verify installation
devex --version

# Run guided setup
devex
```

### One-Command Setup
```bash
# Install default development environment
devex install

# Install with verbose output
devex install --verbose

# Preview what would be installed
devex install --dry-run
```

## Core Commands

DevEx provides a comprehensive set of commands for environment management:

### Installation Commands
```bash
# Install default applications
devex install

# Install specific applications
devex install docker git vscode

# Install by category
devex install --categories development,databases

# Show installation preview
devex install --dry-run --verbose
```

### System Management
```bash
# Check system status
devex status

# Configure system settings
devex system apply

# Detect current environment
devex detect

# Initialize configuration
devex init
```

### Configuration Management
```bash
# Show current configuration
devex config show

# Edit configuration interactively
devex config edit

# Validate configuration
devex config validate

# Reset to defaults
devex config reset
```

### Template Operations
```bash
# List available templates
devex template list

# Apply a template
devex template apply web-development

# Create custom template
devex template create my-setup

# Show template details
devex template show backend
```

## Installation Examples

### Development Environment Setup

#### Web Development Stack
```bash
# Complete web development environment
devex template apply web-development

# Or install individual components
devex install node npm yarn vscode chrome firefox git
```

#### Backend Development
```bash
# Apply backend template
devex template apply backend

# Manual backend setup
devex install --categories development,database,container
```

#### Full Stack Development
```bash
# Install everything for full-stack development
devex install docker postgresql redis nodejs python git vscode \
  --categories development,databases,containers
```

### Language-Specific Environments

#### Python Development
```bash
# Python development setup
devex install python3 python3-pip python3-venv pylint black pytest

# With data science tools
devex install python3 jupyter-notebook pandas numpy matplotlib
```

#### Go Development
```bash
# Go development environment
devex install golang gopls go-tools vscode-go

# With additional tools
devex install golang delve goreleaser golangci-lint
```

#### JavaScript/Node.js Development
```bash
# Node.js with package managers
devex install nodejs npm yarn pnpm

# With development tools
devex install nodejs typescript eslint prettier jest
```

### Database Setup Examples

#### PostgreSQL Setup
```bash
# Install and configure PostgreSQL
devex install postgresql postgresql-contrib

# Verify installation
devex system status postgresql

# Configure for development
devex config database postgresql --dev-mode
```

#### Multi-Database Environment
```bash
# Install multiple databases
devex install postgresql mysql redis mongodb

# Check all database services
devex status --services databases
```

## Configuration

DevEx uses a hierarchical configuration system with multiple sources:

### Configuration Hierarchy
1. **Command-line flags** (highest priority)
2. **Environment variables** (`DEVEX_*`)
3. **Configuration files** (`~/.devex/config.yaml`)
4. **Default values** (lowest priority)

### Configuration Locations
```bash
# Global configuration
~/.devex/config.yaml

# User-specific overrides
~/.local/share/devex/config/

# Template configurations
~/.local/share/devex/templates/
```

### Example Configuration File
```yaml
# ~/.devex/config.yaml
global:
  verbose: true
  dry-run: false
  log-level: info
  
categories:
  - development
  - databases
  
package_managers:
  preferred: apt
  fallback: flatpak
  
applications:
  default:
    - git
    - docker
    - vscode
    
system:
  shell: zsh
  theme: dark
  
plugins:
  auto_download: true
  update_check: daily
```

### Environment Variables
```bash
# Set global preferences
export DEVEX_VERBOSE=true
export DEVEX_DRY_RUN=false
export DEVEX_LOG_LEVEL=debug

# Package manager preferences
export DEVEX_PACKAGE_MANAGER=apt
export DEVEX_FALLBACK_MANAGER=flatpak

# Plugin configuration
export DEVEX_PLUGIN_AUTO_DOWNLOAD=true
export DEVEX_OFFLINE_MODE=false
```

## Package Manager Integration

DevEx integrates with multiple package managers across platforms:

### Linux Package Managers

#### APT (Ubuntu/Debian)
```bash
# Use APT exclusively
devex install docker git --package-manager apt

# Add custom repository
devex system add-repo "deb https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"

# Install from custom repo
devex install docker-ce docker-ce-cli containerd.io
```

#### DNF (Fedora/RHEL)
```bash
# Use DNF for installation
devex install --package-manager dnf docker git podman

# Enable additional repositories
devex system enable-repo rpmfusion-free rpmfusion-nonfree
```

#### Flatpak (Universal)
```bash
# Install applications via Flatpak
devex install --package-manager flatpak \
  org.gnome.Builder \
  com.visualstudio.code \
  org.mozilla.firefox

# Add Flathub repository
devex system add-repo flatpak https://flathub.org/repo/flathub.flatpakrepo
```

### Cross-Platform Managers

#### Docker Integration
```bash
# Install Docker and common images
devex install docker docker-compose

# Pull development images
devex docker pull postgres:13 redis:alpine node:16-alpine

# Set up development containers
devex docker setup-dev-environment
```

#### Python Package Management
```bash
# Install Python tools
devex install python3 python3-pip python3-venv

# Create virtual environment
devex python create-venv myproject

# Install packages in virtual environment
devex python install-requirements requirements.txt
```

### Package Manager Priority
DevEx automatically selects the best package manager based on:

1. **Platform detection** (OS and distribution)
2. **Package availability** 
3. **User preferences**
4. **Fallback options**

```bash
# View detected package managers
devex system detect --package-managers

# Set preferred package manager
devex config set package-manager.preferred apt
devex config set package-manager.fallback flatpak
```

## Tool Configuration

### Git Configuration
```bash
# Configure Git globally
devex git config --global user.name "Your Name"
devex git config --global user.email "your.email@example.com"

# Set up common aliases
devex git aliases --install

# Configure SSH for Git
devex git ssh-setup --generate-key
```

### Shell Configuration
```bash
# Switch to Zsh
devex shell switch zsh

# Configure shell with Oh My Zsh
devex shell setup --framework oh-my-zsh

# Apply custom shell configuration
devex shell config --theme powerlevel10k --plugins "git docker kubectl"

# Backup current shell configuration
devex shell backup --create
```

### Development Environment Detection
```bash
# Detect current project stack
devex detect stack

# Show detected languages and frameworks
devex detect languages --detailed

# Generate recommended configuration
devex detect generate-config
```

## Template Usage

Templates provide pre-configured environments for specific use cases:

### Available Templates
```bash
# List all templates
devex template list

# Show template details
devex template show web-development
devex template show backend
devex template show data-science
```

### Applying Templates
```bash
# Apply web development template
devex template apply web-development

# Apply with customizations
devex template apply backend --databases postgresql,redis --languages go,python

# Preview template application
devex template apply web-development --dry-run
```

### Custom Templates
```bash
# Create custom template from current setup
devex template create my-setup --from-current

# Edit template
devex template edit my-setup

# Share template
devex template export my-setup --output my-setup.yaml
```

### Template Configuration
```yaml
# Example custom template
metadata:
  name: my-development
  version: 1.0.0
  description: My custom development environment
  
applications:
  - name: git
    category: development
    default: true
  - name: docker
    category: container
    default: true
  - name: vscode
    category: editor
    default: true
    
environment:
  shell: zsh
  languages:
    - node
    - python
    - go
    
system:
  git_config: true
  ssh_config: true
  directories:
    - ~/Projects
    - ~/Scripts
```

## Common Workflows

### New Machine Setup
```bash
# Complete setup for a new development machine
devex init
devex template apply web-development
devex git config --setup
devex shell switch zsh
devex system apply --all
```

### Project-Specific Environment
```bash
# Set up environment for a specific project
cd my-project
devex detect stack
devex install $(devex detect languages --packages)
devex template apply $(devex detect template --recommend)
```

### Team Environment Synchronization
```bash
# Export current environment configuration
devex config export --output team-setup.yaml

# Apply team configuration on another machine
devex config import team-setup.yaml
devex install --from-config team-setup.yaml
```

### CI/CD Environment Setup
```bash
# Set up CI environment
devex install --categories development,testing --minimal
devex config set ci_mode true

# Verify environment
devex status --ci-check
```

### Development Environment Updates
```bash
# Update all installed packages
devex system update --all

# Update specific categories
devex system update --categories development,databases

# Check for available updates
devex system check-updates --list
```

## Troubleshooting

### Common Issues and Solutions

#### Installation Failures
```bash
# Check system requirements
devex system check-requirements

# Verify package manager availability
devex system verify-managers

# Clear package manager caches
devex system clean-cache

# Retry installation with verbose output
devex install docker --verbose --debug
```

#### Configuration Issues
```bash
# Validate configuration
devex config validate

# Reset to defaults
devex config reset

# Show configuration sources
devex config sources --detailed

# Debug configuration loading
devex --verbose config show
```

#### Permission Problems
```bash
# Check required permissions
devex system check-permissions

# Fix common permission issues
devex system fix-permissions

# Run with elevated privileges when needed
sudo devex install docker --system-wide
```

#### Network and Connectivity
```bash
# Test connectivity
devex system test-connectivity

# Work in offline mode
devex --offline install local-packages

# Configure proxy settings
devex config set network.proxy "http://proxy:8080"
```

### Debug Information
```bash
# Show system information
devex system info

# Generate debug report
devex debug generate-report --output debug-report.txt

# Enable verbose logging
export DEVEX_LOG_LEVEL=debug
devex install --verbose
```

### Getting Help
```bash
# Show command help
devex install --help
devex config --help

# Show contextual help
devex help install
devex help config

# Show examples for specific commands
devex help install --examples
```

### Plugin Issues
```bash
# List available plugins
devex plugin list

# Check plugin status
devex plugin status

# Update plugins
devex plugin update --all

# Troubleshoot plugin issues
devex plugin debug --plugin package-manager-apt
```

For more detailed information about specific plugins and advanced usage, see the plugin-specific documentation in the `packages/` directory.
