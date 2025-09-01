# devex install

Install applications and packages defined in your DevEx configuration.

## Synopsis

```bash
devex install [applications...] [flags]
```

## Description

The `install` command installs applications defined in your DevEx configuration files. It automatically selects the best package manager for your system and handles dependencies, version management, and error recovery.

DevEx supports multiple package managers and can install:
- System packages (apt, dnf, pacman, brew)
- Universal packages (flatpak, snap, AppImage)
- Programming language tools (mise, pip, npm, cargo)
- Container applications (Docker)
- Development tools and IDEs

## Options

```
      --all                 Install all configured applications
      --category string     Install applications from specific category
      --installer string    Force specific installer (apt, brew, flatpak, etc.)
      --parallel int        Number of parallel installations (default 3)
      --user                Install to user directory when possible
      --update             Update existing packages during installation
      --skip-dependencies   Skip dependency installation
      --force              Force installation even if already installed
  -h, --help               help for install
```

## Global Flags

```
      --dry-run     Show what would be done without making changes
      --no-tui      Disable TUI and use simple text output
  -v, --verbose     Enable verbose output
```

## Usage Patterns

### Install All Applications
```bash
devex install --all
```
Installs every application defined in your configuration files.

### Install Specific Applications
```bash
devex install docker code python3
```
Install only the specified applications.

### Install by Category
```bash
devex install --category development
```
Install all applications in the "development" category.

### Force Specific Installer
```bash
devex install docker --installer snap
```
Use Snap to install Docker instead of the default installer.

### User Installation
```bash
devex install --user --all
```
Install applications to user directories when possible (no sudo required).

### Update During Install
```bash
devex install --all --update
```
Update existing packages to their latest versions during installation.

## Supported Package Managers

DevEx automatically detects and uses the best available package manager:

### Linux
- **apt** (Debian, Ubuntu) - `sudo apt install`
- **dnf** (Fedora, RHEL) - `sudo dnf install`
- **pacman** (Arch Linux) - `sudo pacman -S`
- **zypper** (openSUSE) - `sudo zypper install`
- **flatpak** - Universal packages
- **snap** - Universal packages

### macOS
- **brew** (Homebrew) - Primary package manager
- **mas** (Mac App Store) - App Store applications

### Windows
- **winget** - Windows Package Manager
- **chocolatey** - Community packages
- **scoop** - Command-line installer

### Universal
- **mise** - Language runtime manager
- **docker** - Container applications
- **pip** - Python packages
- **npm** - Node.js packages
- **cargo** - Rust packages

## Installation Process

### 1. Planning Phase
DevEx analyzes your configuration and creates an installation plan:
```
📋 Installation Plan
├── System Packages (apt): 5 packages
├── Development Tools: 3 packages  
├── Language Runtimes (mise): 2 runtimes
└── Container Apps (docker): 1 container

Total: 11 packages to install
Estimated time: 5-8 minutes
```

### 2. Dependency Resolution
Automatically handles dependencies and installation order:
```
🔗 Resolving Dependencies
✓ git → Required by development workflow
✓ curl → Required by mise installer
✓ docker → Requires containerd service
```

### 3. Installation Execution
Installs packages with progress tracking:
```
📦 Installing Applications

🐧 System Packages (apt)
  ✓ git (2.34.1)
  ✓ curl (7.81.0)
  → docker.io (installing...)

🛠️  Development Tools
  → Visual Studio Code (downloading...)

🐍 Language Runtimes (mise)
  → python@3.11.0 (installing...)

Installation Progress: 6/11 packages (55%)
```

### 4. Verification
Verifies installations completed successfully:
```
✅ Installation Complete

Installed: 11/11 packages
Failed: 0 packages
Time: 4m 23s

All applications are ready to use!
```

## Examples

### Basic Installation
```bash
# Install everything in configuration
devex install --all

# Install specific applications
devex install git docker code
```

### Category-Based Installation
```bash
# Install development tools only
devex install --category development

# Install system utilities
devex install --category system

# Install optional applications
devex install --category optional
```

### Advanced Installation Options
```bash
# Parallel installation for speed
devex install --all --parallel 5

# Force reinstallation
devex install docker --force

# Install without sudo (where possible)
devex install --all --user

# Update existing packages
devex install --all --update
```

### Installer Selection
```bash
# Use specific installer
devex install firefox --installer flatpak

# Prefer snap packages
devex install --all --installer snap

# Use Homebrew on macOS
devex install --all --installer brew
```

## Configuration Examples

### applications.yaml Structure
```yaml
categories:
  development:
    - name: code
      description: Visual Studio Code
      installers:
        linux: snap
        macos: brew
        windows: winget
    
    - name: docker
      description: Container platform
      installers:
        linux: apt
        macos: brew
        windows: winget
      post_install:
        - systemctl enable docker
        - usermod -aG docker $USER
  
  languages:
    - name: python
      version: "3.11"
      installer: mise
      packages:
        - pip
        - virtualenv
        - black
```

### Per-Platform Configuration
```yaml
applications:
  git:
    linux:
      installer: apt
      package: git
    macos:
      installer: brew
      package: git
    windows:
      installer: winget
      package: Git.Git
```

## Error Handling

### Installation Failures
```bash
# Retry failed installations
devex install --all --verbose

# Skip failing packages
devex install --all --continue-on-error

# Install with alternative installer
devex install failed-package --installer flatpak
```

### Permission Issues
```bash
# Install to user directory
devex install --user

# Fix Docker permissions (Linux)
sudo usermod -aG docker $USER
newgrp docker
```

### Network Issues
```bash
# Use cached packages when available
devex install --offline

# Retry with different mirror
devex install --mirror alternate
```

## Performance Optimization

### Parallel Installation
```bash
# Increase parallel jobs (use with caution)
devex install --all --parallel 8
```

### Caching
DevEx automatically caches downloads:
```bash
# View cache usage
devex cache stats

# Clean old cache files
devex cache cleanup --max-age 30d
```

### Installation Analytics
```bash
# View installation performance
devex cache analyze

# See installation history
devex status --detailed
```

## Troubleshooting

### Common Issues

**Package Not Found**
```bash
# Update package lists
sudo apt update  # on Debian/Ubuntu
brew update       # on macOS

# Try alternative installer
devex install package --installer snap
```

**Permission Denied**
```bash
# Use --user flag
devex install --user package

# Fix sudo permissions
sudo visudo  # add user to sudo group
```

**Service Not Starting**
```bash
# Check service status
systemctl status docker

# Enable and start service
sudo systemctl enable --now docker
```

**Disk Space Issues**
```bash
# Clean package caches
devex cache cleanup

# Check available space
df -h ~/.devex/cache
```

## Related Commands

- `devex status` - Check installation status
- `devex uninstall` - Remove applications
- `devex add` - Add new applications to configuration
- `devex config` - Manage configuration files
- `devex cache` - Manage installation cache

## See Also

- [Application Configuration](../config) - Configuring applications
- [Package Managers](../installers) - Supported installers
- [Troubleshooting](../troubleshooting) - Common installation issues
