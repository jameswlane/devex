# APT Package Manager Plugin - Usage Guide

The APT (Advanced Package Tool) plugin provides comprehensive package management for Ubuntu, Debian, and derived distributions through DevEx.

## Table of Contents

- [Installation](#installation)
- [Basic Usage](#basic-usage)
- [Repository Management](#repository-management)
- [Advanced Features](#advanced-features)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Examples](#examples)

## Installation

The APT plugin is automatically available on Debian/Ubuntu systems when DevEx detects APT as the system package manager.

### Prerequisites

- Ubuntu 18.04+ or Debian 10+
- APT package manager installed
- Sudo privileges for system operations

### Verification

```bash
# Check if APT plugin is available
devex plugin list | grep package-manager-apt

# Test APT plugin functionality
devex plugin exec package-manager-apt --help
```

## Basic Usage

### Package Installation

#### Single Package Installation
```bash
# Install a single package
devex plugin exec package-manager-apt install git

# Install with verbose output
devex plugin exec package-manager-apt --verbose install docker.io
```

#### Multiple Package Installation
```bash
# Install multiple packages at once
devex plugin exec package-manager-apt install git curl wget htop tree

# Install development essentials
devex plugin exec package-manager-apt install \
  build-essential \
  git \
  curl \
  vim \
  htop \
  tree \
  jq
```

### Package Removal

```bash
# Remove a single package
devex plugin exec package-manager-apt remove old-package

# Remove multiple packages
devex plugin exec package-manager-apt remove package1 package2 package3

# Check if package is installed before removal
devex plugin exec package-manager-apt is-installed package-name
```

### System Maintenance

#### Update Operations
```bash
# Update package lists
devex plugin exec package-manager-apt update

# Upgrade all installed packages
devex plugin exec package-manager-apt upgrade

# Combined update and upgrade
devex plugin exec package-manager-apt update && \
devex plugin exec package-manager-apt upgrade
```

#### Package Search and Information
```bash
# Search for packages
devex plugin exec package-manager-apt search "text editor"
devex plugin exec package-manager-apt search python

# Get package information
devex plugin exec package-manager-apt info firefox
devex plugin exec package-manager-apt info docker.io

# List installed packages
devex plugin exec package-manager-apt list

# List installed packages matching pattern
devex plugin exec package-manager-apt list --installed | grep python
```

### Package Status Checking

```bash
# Check if specific packages are installed
devex plugin exec package-manager-apt is-installed git
devex plugin exec package-manager-apt is-installed docker.io curl wget

# Bulk check installation status
for pkg in git docker.io python3 nodejs; do
    if devex plugin exec package-manager-apt is-installed $pkg; then
        echo "$pkg is installed"
    else
        echo "$pkg is not installed"
    fi
done
```

## Repository Management

### Adding Custom Repositories

#### Docker Repository Example
```bash
# Add Docker's official repository
devex plugin exec package-manager-apt add-repository \
  "https://download.docker.com/linux/ubuntu/gpg" \
  "/usr/share/keyrings/docker-archive-keyring.gpg" \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
  "/etc/apt/sources.list.d/docker.list"

# Update package lists after adding repository
devex plugin exec package-manager-apt update

# Install Docker from the new repository
devex plugin exec package-manager-apt install docker-ce docker-ce-cli containerd.io
```

#### Node.js Repository Example
```bash
# Add Node.js 18.x repository
devex plugin exec package-manager-apt add-repository \
  "https://deb.nodesource.com/gpgkey/nodesource.gpg.key" \
  "/usr/share/keyrings/nodesource-keyring.gpg" \
  "deb [signed-by=/usr/share/keyrings/nodesource-keyring.gpg] https://deb.nodesource.com/node_18.x $(lsb_release -cs) main" \
  "/etc/apt/sources.list.d/nodesource.list" \
  "true"

devex plugin exec package-manager-apt update
devex plugin exec package-manager-apt install nodejs
```

### Repository Validation and Cleanup

```bash
# Validate all repository configurations
devex plugin exec package-manager-apt validate-repository

# Remove a repository
devex plugin exec package-manager-apt remove-repository \
  "/etc/apt/sources.list.d/docker.list" \
  "/usr/share/keyrings/docker-archive-keyring.gpg"
```

## Advanced Features

### Parallel Package Validation

The APT plugin includes performance optimizations for handling multiple packages:

```bash
# Install multiple packages with optimized validation
devex plugin exec package-manager-apt install \
  git \
  docker.io \
  nodejs \
  python3 \
  golang \
  postgresql \
  redis-server \
  nginx
```

The plugin automatically:
- Validates packages in parallel (up to 5 concurrent operations)
- Checks availability before installation
- Verifies installation success
- Handles dependency conflicts gracefully

### Error Handling and Recovery

```bash
# Install with detailed error reporting
devex plugin exec package-manager-apt --debug install problematic-package

# Skip already installed packages automatically
devex plugin exec package-manager-apt install git docker.io  # Git already installed
# Output: Package git is already installed, skipping
```

### Security Features

The plugin includes security validations:
- GPG key verification for repositories
- Package name validation to prevent injection
- File path validation for repository configuration
- Secure temporary file handling

## Configuration

### Plugin Configuration

The APT plugin can be configured through DevEx's configuration system:

```yaml
# ~/.devex/config.yaml
package_managers:
  apt:
    auto_update: true
    recommend_packages: false
    install_suggests: false
    timeout: 300
    max_parallel_operations: 5
    sources_list_d: "/etc/apt/sources.list.d"
    keyrings_path: "/usr/share/keyrings"
```

### Environment Variables

```bash
# Control APT plugin behavior
export DEVEX_APT_AUTO_UPDATE=true
export DEVEX_APT_TIMEOUT=300
export DEVEX_APT_VERBOSE=true
export DEVEX_APT_MAX_WORKERS=5
```

## Troubleshooting

### Common Issues

#### Package Not Found
```bash
# Error: package 'nonexistent-package' not found in any repository
# Solution: Update package lists and search for similar packages
devex plugin exec package-manager-apt update
devex plugin exec package-manager-apt search "similar-name"
```

#### Repository Issues
```bash
# Error: repository validation failed
# Solution: Check repository URL and GPG key
devex plugin exec package-manager-apt validate-repository

# Remove and re-add problematic repository
devex plugin exec package-manager-apt remove-repository problem-repo
devex plugin exec package-manager-apt add-repository correct-repo-info
```

#### Permission Problems
```bash
# Error: insufficient permissions
# Solution: Ensure sudo access and correct user permissions
sudo -v  # Refresh sudo credentials

# Check if user is in sudo group
groups $USER | grep sudo
```

#### Network Connectivity
```bash
# Error: cannot reach repositories
# Solution: Check network connectivity and proxy settings
ping archive.ubuntu.com
ping security.ubuntu.com

# Configure proxy if needed (in /etc/apt/apt.conf.d/95proxies)
echo 'Acquire::http::proxy "http://proxy:8080";' | sudo tee /etc/apt/apt.conf.d/95proxies
```

### Debug Information

```bash
# Enable debug output for troubleshooting
devex plugin exec package-manager-apt --debug info package-name

# Check APT configuration
apt-config dump

# Verify repository sources
cat /etc/apt/sources.list
ls -la /etc/apt/sources.list.d/
```

## Examples

### Development Environment Setup

#### Web Development Stack
```bash
# Install web development tools
devex plugin exec package-manager-apt update
devex plugin exec package-manager-apt install \
  git \
  curl \
  wget \
  build-essential \
  software-properties-common \
  apt-transport-https \
  ca-certificates \
  gnupg \
  lsb-release

# Add Node.js repository
devex plugin exec package-manager-apt add-repository \
  "https://deb.nodesource.com/gpgkey/nodesource.gpg.key" \
  "/usr/share/keyrings/nodesource-keyring.gpg" \
  "deb [signed-by=/usr/share/keyrings/nodesource-keyring.gpg] https://deb.nodesource.com/node_18.x $(lsb_release -cs) main" \
  "/etc/apt/sources.list.d/nodesource.list"

devex plugin exec package-manager-apt update
devex plugin exec package-manager-apt install nodejs

# Verify installation
node --version
npm --version
```

#### Database Development Environment
```bash
# Install PostgreSQL
devex plugin exec package-manager-apt install \
  postgresql \
  postgresql-contrib \
  postgresql-client

# Install MySQL
devex plugin exec package-manager-apt install \
  mysql-server \
  mysql-client

# Install Redis
devex plugin exec package-manager-apt install redis-server

# Install database GUI tools
devex plugin exec package-manager-apt install \
  pgadmin4 \
  mysql-workbench \
  redis-tools
```

#### Container Development Stack
```bash
# Add Docker repository
devex plugin exec package-manager-apt add-repository \
  "https://download.docker.com/linux/ubuntu/gpg" \
  "/usr/share/keyrings/docker-archive-keyring.gpg" \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
  "/etc/apt/sources.list.d/docker.list"

devex plugin exec package-manager-apt update
devex plugin exec package-manager-apt install \
  docker-ce \
  docker-ce-cli \
  containerd.io \
  docker-compose-plugin

# Add user to docker group
sudo usermod -aG docker $USER

# Install Kubernetes tools
devex plugin exec package-manager-apt install kubectl
```

### System Administration Tools

```bash
# Install system monitoring and administration tools
devex plugin exec package-manager-apt install \
  htop \
  iotop \
  nethogs \
  tree \
  ncdu \
  jq \
  yq \
  unzip \
  zip \
  rsync \
  ssh \
  vim \
  tmux \
  screen
```

### Cleanup and Maintenance Script

```bash
#!/bin/bash
# cleanup-system.sh - Automated system maintenance

# Update package lists
devex plugin exec package-manager-apt update

# Upgrade all packages
devex plugin exec package-manager-apt upgrade

# Clean package cache
sudo apt autoremove
sudo apt autoclean

# Validate repository configurations
devex plugin exec package-manager-apt validate-repository

echo "System maintenance completed successfully"
```

## Best Practices

1. **Always update before installing**: Run `update` before installing new packages
2. **Validate repositories**: Use `validate-repository` after adding new repositories
3. **Check installation**: Use `is-installed` to verify successful installation
4. **Use specific package names**: Prefer `docker-ce` over `docker` for clarity
5. **Handle errors gracefully**: Check return codes in scripts
6. **Keep repositories clean**: Remove unused repositories to reduce maintenance overhead
7. **Monitor security**: Regularly update packages to get security fixes

## Integration with DevEx CLI

The APT plugin integrates seamlessly with DevEx's main CLI:

```bash
# Use through DevEx main CLI
devex install git docker  # Automatically uses APT plugin on Ubuntu/Debian

# Direct plugin usage for advanced features
devex plugin exec package-manager-apt add-repository ...

# Combined workflow
devex system detect  # Detects APT as available package manager
devex install --package-manager apt git docker  # Force APT usage
```

For more information about DevEx and other plugins, see the main [USAGE.md](../USAGE.md) documentation.
