# DevEx Overview

Welcome to **DevEx** - the ultimate development environment management tool that streamlines your workflow and eliminates configuration headaches.

## What is DevEx?

DevEx is a powerful CLI tool designed to help developers quickly set up, manage, and maintain their development environments across different projects and teams. Whether you're onboarding new team members, switching between projects, or setting up a fresh machine, DevEx makes it effortless.

## Key Features

### 🚀 **Quick Setup**
- One-command environment initialization
- Pre-built templates for common tech stacks
- Automatic dependency resolution

### 🛠️ **Application Management**
- Cross-platform package installation
- Unified interface for multiple package managers
- Intelligent installer selection based on your system

### ⚙️ **Configuration Management**
- YAML-based configuration files
- Template system with inheritance
- Backup and versioning support

### 🎯 **Team Collaboration**
- Shareable configuration templates
- Custom team-specific setups
- Version-controlled environments

### 📊 **Progress Tracking**
- Beautiful terminal UI with progress indicators
- Detailed installation feedback
- Performance analytics and caching

## Core Concepts

### Applications
Applications are the software packages you want to install. DevEx supports various package managers:
- **apt** (Debian/Ubuntu)
- **dnf** (Fedora/RHEL)
- **pacman** (Arch Linux)
- **brew** (macOS)
- **flatpak** (Universal packages)
- **snap** (Universal packages)
- **mise** (Language runtime manager)
- **docker** (Container applications)
- **pip** (Python packages)

### Templates
Templates are pre-configured setups for common development scenarios:
- **Full-Stack Web Development**
- **Backend API Development**
- **Frontend Development**
- **Mobile Development**
- **Data Science**
- **DevOps/SRE**

### Configuration Files
DevEx uses four main configuration files:
- `applications.yaml` - Define which applications to install
- `environment.yaml` - Programming languages and development tools
- `system.yaml` - Git, SSH, and terminal settings
- `desktop.yaml` - Desktop environment customizations (optional)

## Getting Started

1. **Initialize a new project**:
   ```bash
   devex init
   ```

2. **Install applications**:
   ```bash
   devex install
   ```

3. **Check status**:
   ```bash
   devex status --all
   ```

4. **Manage configuration**:
   ```bash
   devex config list
   ```

## Why Choose DevEx?

- **Cross-Platform**: Works on Linux, macOS, and Windows
- **Fast**: Intelligent caching and parallel installation
- **Reliable**: Comprehensive error handling and recovery
- **User-Friendly**: Beautiful TUI with contextual help
- **Extensible**: Plugin architecture for custom workflows
- **Team-Ready**: Built for collaboration from day one

## What's Next?

- Read the [Quick Start Guide](quick-start) to get up and running in minutes
- Explore [Command Reference](commands) for detailed usage
- Learn about [Templates](templates) to accelerate your setup
- Check out [Configuration](config) for advanced customization

---

*DevEx - Because your development environment should just work.*
