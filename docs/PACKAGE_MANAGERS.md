# DevEx Package Manager Plugins

This guide provides comprehensive documentation for all DevEx package manager plugins, including practical examples, configuration options, and troubleshooting guidance.

## Table of Contents

- [Overview](#overview)
- [APT Package Manager](#apt-package-manager)
- [Docker Package Manager](#docker-package-manager)
- [Pip Package Manager](#pip-package-manager)
- [Flatpak Package Manager](#flatpak-package-manager)
- [DNF Package Manager](#dnf-package-manager)
- [Pacman Package Manager](#pacman-package-manager)
- [Other Package Managers](#other-package-managers)
- [Cross-Platform Usage](#cross-platform-usage)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

## Overview

DevEx supports multiple package managers across different platforms, automatically detecting the best available option and providing a unified interface for package management operations.

### Supported Package Managers

| Package Manager | Platform | Status | Primary Use |
|-----------------|----------|--------|-------------|
| APT | Ubuntu/Debian | ✅ Production | System packages |
| DNF | Fedora/RHEL | 🚧 In Development | System packages |
| Pacman | Arch Linux | 📋 Planned | System packages |
| Flatpak | Universal Linux | ✅ Production | Sandboxed apps |
| Docker | Cross-platform | ✅ Production | Containerized apps |
| Pip | Cross-platform | ✅ Production | Python packages |
| Brew | macOS/Linux | 📋 Planned | Cross-platform packages |
| Mise | Cross-platform | ✅ Production | Development tools |

## APT Package Manager

The APT (Advanced Package Tool) plugin provides comprehensive package management for Ubuntu, Debian, and derived distributions.

### Basic Operations

#### Installing Packages
```bash
# Install single package
devex plugin exec package-manager-apt install git

# Install multiple packages
devex plugin exec package-manager-apt install git curl jq htop

# Install with automatic dependency resolution
devex plugin exec package-manager-apt install build-essential
```

#### Removing Packages
```bash
# Remove single package
devex plugin exec package-manager-apt remove old-package

# Remove multiple packages
devex plugin exec package-manager-apt remove package1 package2

# Check what's installed before removal
devex plugin exec package-manager-apt is-installed package-name
```

#### System Maintenance
```bash
# Update package lists
devex plugin exec package-manager-apt update

# Upgrade all packages
devex plugin exec package-manager-apt upgrade

# Search for packages
devex plugin exec package-manager-apt search "text editor"

# List installed packages
devex plugin exec package-manager-apt list

# Get package information
devex plugin exec package-manager-apt info firefox
```

### Repository Management

#### Adding Custom Repositories
```bash
# Add repository with GPG key
devex plugin exec package-manager-apt add-repository \
  "https://download.docker.com/linux/ubuntu/gpg" \
  "/usr/share/keyrings/docker-archive-keyring.gpg" \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu focal stable" \
  "/etc/apt/sources.list.d/docker.list"
```

#### Repository Validation
```bash
# Validate all repository configurations
devex plugin exec package-manager-apt validate-repository

# Remove repository
devex plugin exec package-manager-apt remove-repository \
  "/etc/apt/sources.list.d/docker.list" \
  "/usr/share/keyrings/docker-archive-keyring.gpg"
```

### Advanced Usage

#### Development Environment Setup
```bash
# Install development essentials
devex plugin exec package-manager-apt install \
  build-essential \
  git \
  curl \
  wget \
  vim \
  htop \
  tree

# Install Node.js development environment
devex plugin exec package-manager-apt add-repository \
  "https://deb.nodesource.com/gpgkey/nodesource.gpg.key" \
  "/usr/share/keyrings/nodesource-keyring.gpg" \
  "deb [signed-by=/usr/share/keyrings/nodesource-keyring.gpg] https://deb.nodesource.com/node_18.x focal main" \
  "/etc/apt/sources.list.d/nodesource.list"

devex plugin exec package-manager-apt update
devex plugin exec package-manager-apt install nodejs
```

#### Database Setup
```bash
# Install PostgreSQL
devex plugin exec package-manager-apt install postgresql postgresql-contrib

# Install MySQL
devex plugin exec package-manager-apt install mysql-server mysql-client

# Install Redis
devex plugin exec package-manager-apt install redis-server
```

### Error Handling

The APT plugin includes robust error handling for common issues:

```bash
# Package not found
devex plugin exec package-manager-apt install nonexistent-package
# Error: package 'nonexistent-package' not found in any repository

# Dependency conflicts
devex plugin exec package-manager-apt install conflicting-package
# Error: failed to install packages [conflicting-package]: dependency conflict

# Repository issues
devex plugin exec package-manager-apt add-repository invalid-url
# Error: repository validation failed: invalid URL format
```

## Docker Package Manager

The Docker plugin manages Docker containers, images, and Docker Compose services.

### Container Management

#### Basic Container Operations
```bash
# Check Docker status
devex plugin exec package-manager-docker status

# Start containers
devex plugin exec package-manager-docker start container-name

# Stop containers
devex plugin exec package-manager-docker stop container-name

# Restart containers
devex plugin exec package-manager-docker restart container-name

# List running containers
devex plugin exec package-manager-docker list

# View container logs
devex plugin exec package-manager-docker logs container-name

# Execute commands in containers
devex plugin exec package-manager-docker exec container-name /bin/bash
```

### Image Management

#### Working with Docker Images
```bash
# Pull images from registry
devex plugin exec package-manager-docker pull postgres:13
devex plugin exec package-manager-docker pull redis:alpine
devex plugin exec package-manager-docker pull node:16-alpine

# List local images
devex plugin exec package-manager-docker images

# Remove images
devex plugin exec package-manager-docker rmi old-image:tag

# Build images from Dockerfile
devex plugin exec package-manager-docker build -t my-app:latest .
```

### Docker Compose Integration

#### Managing Multi-Container Applications
```bash
# Start services defined in docker-compose.yml
devex plugin exec package-manager-docker compose up

# Start services in background
devex plugin exec package-manager-docker compose up -d

# Stop and remove services
devex plugin exec package-manager-docker compose down

# View service logs
devex plugin exec package-manager-docker compose logs

# Scale services
devex plugin exec package-manager-docker compose scale web=3
```

### Development Environment Setup

#### Common Development Stacks
```bash
# PostgreSQL for development
devex plugin exec package-manager-docker pull postgres:13
devex plugin exec package-manager-docker start postgres-dev \
  -e POSTGRES_DB=devdb \
  -e POSTGRES_USER=devuser \
  -e POSTGRES_PASSWORD=devpass \
  -p 5432:5432

# Redis for caching
devex plugin exec package-manager-docker pull redis:alpine
devex plugin exec package-manager-docker start redis-dev \
  -p 6379:6379

# MySQL for database work
devex plugin exec package-manager-docker pull mysql:8.0
devex plugin exec package-manager-docker start mysql-dev \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=devdb \
  -p 3306:3306
```

## Pip Package Manager

The Pip plugin manages Python packages and virtual environments.

### Virtual Environment Management

#### Creating and Managing Virtual Environments
```bash
# Create virtual environment
devex plugin exec package-manager-pip create-venv myproject

# Activate virtual environment
devex plugin exec package-manager-pip activate myproject

# Deactivate virtual environment
devex plugin exec package-manager-pip deactivate

# List virtual environments
devex plugin exec package-manager-pip list-venvs

# Remove virtual environment
devex plugin exec package-manager-pip remove-venv myproject
```

### Package Installation

#### Installing Python Packages
```bash
# Install single package
devex plugin exec package-manager-pip install requests

# Install multiple packages
devex plugin exec package-manager-pip install requests flask django

# Install from requirements file
devex plugin exec package-manager-pip install-requirements requirements.txt

# Install development dependencies
devex plugin exec package-manager-pip install pytest black flake8 mypy

# Install package in editable mode
devex plugin exec package-manager-pip install -e .
```

### Project Setup Examples

#### Django Project Setup
```bash
# Create virtual environment for Django project
devex plugin exec package-manager-pip create-venv django-project
devex plugin exec package-manager-pip activate django-project

# Install Django and dependencies
devex plugin exec package-manager-pip install \
  django \
  djangorestframework \
  psycopg2-binary \
  python-decouple \
  gunicorn

# Create requirements file
devex plugin exec package-manager-pip freeze > requirements.txt
```

#### Data Science Environment
```bash
# Create data science environment
devex plugin exec package-manager-pip create-venv data-science
devex plugin exec package-manager-pip activate data-science

# Install data science stack
devex plugin exec package-manager-pip install \
  jupyter \
  pandas \
  numpy \
  matplotlib \
  seaborn \
  scikit-learn \
  tensorflow

# Install development tools
devex plugin exec package-manager-pip install \
  black \
  isort \
  flake8 \
  pytest
```

## Flatpak Package Manager

The Flatpak plugin manages sandboxed applications across Linux distributions.

### Basic Operations

#### Installing Applications
```bash
# Install application from Flathub
devex plugin exec package-manager-flatpak install org.mozilla.firefox

# Install from specific remote
devex plugin exec package-manager-flatpak install --from flathub org.gimp.GIMP

# Install multiple applications
devex plugin exec package-manager-flatpak install \
  com.visualstudio.code \
  org.libreoffice.LibreOffice \
  org.mozilla.Thunderbird
```

#### Managing Applications
```bash
# List installed applications
devex plugin exec package-manager-flatpak list

# Update all applications
devex plugin exec package-manager-flatpak update

# Update specific application
devex plugin exec package-manager-flatpak update org.mozilla.firefox

# Remove application
devex plugin exec package-manager-flatpak remove org.gimp.GIMP

# Search for applications
devex plugin exec package-manager-flatpak search "text editor"
```

### Repository Management

#### Working with Remotes
```bash
# Add Flathub repository
devex plugin exec package-manager-flatpak add-repo \
  flathub https://flathub.org/repo/flathub.flatpakrepo

# List configured repositories
devex plugin exec package-manager-flatpak list-repos

# Remove repository
devex plugin exec package-manager-flatpak remove-repo flathub

# Update repository metadata
devex plugin exec package-manager-flatpak update-repos
```

### Common Application Categories

#### Development Tools
```bash
# Install development applications
devex plugin exec package-manager-flatpak install com.visualstudio.code
devex plugin exec package-manager-flatpak install org.gnome.Builder
devex plugin exec package-manager-flatpak install com.jetbrains.IntelliJ-IDEA-Community

# Install Git GUI clients
devex plugin exec package-manager-flatpak install com.github.Murmele.Gittyup
devex plugin exec package-manager-flatpak install org.gnome.gitg
```

#### Media and Graphics
```bash
# Install media applications
devex plugin exec package-manager-flatpak install org.gimp.GIMP
devex plugin exec package-manager-flatpak install org.inkscape.Inkscape
devex plugin exec package-manager-flatpak install org.blender.Blender
devex plugin exec package-manager-flatpak install org.audacityteam.Audacity
```

## DNF Package Manager

The DNF plugin provides package management for Fedora, RHEL, and CentOS systems.

### Basic Operations

#### Package Management
```bash
# Install packages
devex plugin exec package-manager-dnf install git curl vim

# Remove packages
devex plugin exec package-manager-dnf remove old-package

# Update system
devex plugin exec package-manager-dnf update

# Search packages
devex plugin exec package-manager-dnf search "text editor"

# Get package info
devex plugin exec package-manager-dnf info firefox
```

### Repository Management

#### Working with Repositories
```bash
# Enable repository
devex plugin exec package-manager-dnf enable-repo rpmfusion-free

# Disable repository
devex plugin exec package-manager-dnf disable-repo updates-testing

# List repositories
devex plugin exec package-manager-dnf list-repos
```

## Cross-Platform Usage

### Package Manager Selection

DevEx automatically selects the appropriate package manager based on:

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

### Unified Commands

DevEx provides unified commands that work across package managers:

```bash
# Install using best available package manager
devex install git docker vscode

# Force specific package manager
devex install --package-manager apt git curl

# Use fallback if primary fails
devex install --with-fallback docker
```

## Configuration

### Global Configuration

Configure package manager preferences in `~/.devex/config.yaml`:

```yaml
package_managers:
  # Preferred package manager (auto-detected if not set)
  preferred: apt
  
  # Fallback managers in order of preference
  fallback:
    - flatpak
    - docker
    - pip
  
  # Manager-specific settings
  apt:
    auto_update: true
    recommend_packages: true
    install_suggests: false
  
  docker:
    auto_pull_latest: false
    use_compose_v2: true
  
  pip:
    use_venv: true
    default_venv_path: "~/.local/share/venvs"
  
  flatpak:
    auto_add_flathub: true
    system_install: false
```

### Environment Variables

Control package manager behavior with environment variables:

```bash
# Force specific package manager
export DEVEX_PACKAGE_MANAGER=apt

# Enable verbose output for debugging
export DEVEX_PACKAGE_VERBOSE=true

# Set timeout for operations
export DEVEX_PACKAGE_TIMEOUT=300

# Configure proxy settings
export DEVEX_HTTP_PROXY=http://proxy:8080
export DEVEX_HTTPS_PROXY=https://proxy:8080
```

### Plugin-Specific Configuration

#### APT Configuration
```yaml
apt:
  sources_list_d: "/etc/apt/sources.list.d"
  keyrings_path: "/usr/share/keyrings"
  auto_update_interval: "daily"
  dpkg_options: "--force-confdef"
  apt_get_options: "-y --no-install-recommends"
```

#### Docker Configuration
```yaml
docker:
  daemon_socket: "/var/run/docker.sock"
  registry: "docker.io"
  compose_version: "v2"
  build_context: "."
  default_network: "bridge"
```

#### Pip Configuration
```yaml
pip:
  index_url: "https://pypi.org/simple"
  extra_index_urls:
    - "https://pypi.python.org/simple"
  trusted_hosts:
    - "pypi.org"
    - "pypi.python.org"
  venv_base_path: "~/.local/share/venvs"
  requirements_file: "requirements.txt"
```

## Troubleshooting

### Common Issues and Solutions

#### Package Manager Not Found
```bash
# Error: package manager not available
devex system check-requirements --package-manager apt

# Install missing package manager
sudo apt update
sudo apt install apt-transport-https ca-certificates gnupg lsb-release
```

#### Repository Issues
```bash
# Error: repository cannot be accessed
devex plugin exec package-manager-apt validate-repository

# Fix repository configuration
devex plugin exec package-manager-apt remove-repository problematic-repo
devex plugin exec package-manager-apt add-repository correct-repo-info
```

#### Dependency Conflicts
```bash
# Error: dependency conflicts
devex install --dry-run package-name  # Preview changes
devex install --force package-name    # Force installation
devex install --resolve-conflicts package-name  # Auto-resolve
```

#### Permission Problems
```bash
# Error: insufficient permissions
devex system check-permissions --package-manager docker

# Fix Docker permissions
sudo usermod -aG docker $USER
newgrp docker
```

#### Network Connectivity
```bash
# Error: cannot reach repositories
devex system test-connectivity --package-managers

# Configure proxy
devex config set network.proxy http://proxy:8080

# Work offline with cached packages
devex --offline install cached-packages
```

### Debug Information

#### Getting Debug Information
```bash
# Show package manager status
devex system status --package-managers

# Generate debug report
devex debug package-managers --output debug.log

# Show detailed plugin information
devex plugin list --detailed
devex plugin status package-manager-apt

# Test specific operations
devex plugin exec package-manager-apt --debug info git
```

#### Logging Configuration
```bash
# Enable debug logging
export DEVEX_LOG_LEVEL=debug
devex install --verbose package-name

# Log to file
devex install package-name 2>&1 | tee install.log

# Show only errors
devex install package-name 2>/dev/null
```

### Performance Optimization

#### Speeding Up Operations
```bash
# Use parallel operations
devex config set package_managers.parallel_operations true

# Cache package lists
devex config set package_managers.cache_duration 3600

# Skip unnecessary checks
devex install --skip-verification package-name

# Use fastest mirror
devex config set package_managers.auto_select_mirror true
```

For more detailed information about specific package managers, refer to their individual documentation in the `packages/` directory.
