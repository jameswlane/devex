# DevEx Troubleshooting Guide

This comprehensive guide helps you diagnose and resolve common issues with DevEx CLI and its plugins.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Installation Issues](#installation-issues)
- [Configuration Problems](#configuration-problems)
- [Plugin Issues](#plugin-issues)
- [Package Manager Problems](#package-manager-problems)
- [Permission Issues](#permission-issues)
- [Network and Connectivity](#network-and-connectivity)
- [Performance Issues](#performance-issues)
- [Debug Information](#debug-information)
- [Getting Help](#getting-help)

## Quick Diagnostics

### System Health Check
```bash
# Comprehensive system check
devex system check

# Check specific components
devex system check --package-managers
devex system check --permissions
devex system check --connectivity
devex system check --plugins

# Show system information
devex system info
```

### Common First Steps
```bash
# 1. Check DevEx version
devex --version

# 2. Verify configuration
devex config validate

# 3. Show current status
devex status

# 4. Check plugin availability
devex plugin list

# 5. Test basic functionality
devex --dry-run install git
```

## Installation Issues

### DevEx CLI Installation Problems

#### Installation Script Fails
```bash
# Error: Installation script cannot download or execute
# Solutions:

# 1. Check internet connectivity
ping -c 4 github.com
curl -I https://devex.sh/install

# 2. Manual installation
wget https://github.com/jameswlane/devex/releases/latest/download/devex-linux-amd64.tar.gz
tar -xzf devex-linux-amd64.tar.gz
sudo mv devex /usr/local/bin/
chmod +x /usr/local/bin/devex

# 3. Alternative installation methods
# Using package managers
curl -fsSL https://devex.sh/install | bash
# Or build from source
git clone https://github.com/jameswlane/devex.git
cd devex/apps/cli
go build -o devex cmd/main.go
```

#### Binary Compatibility Issues
```bash
# Error: cannot execute binary file
file $(which devex)  # Check architecture

# Solutions:
# 1. Download correct architecture
uname -m  # Check your architecture
# Download matching binary: devex-linux-amd64, devex-linux-arm64, etc.

# 2. Check execution permissions
ls -la $(which devex)
chmod +x $(which devex)

# 3. Check dynamic libraries
ldd $(which devex)  # Linux
otool -L $(which devex)  # macOS
```

#### PATH Issues
```bash
# Error: devex: command not found
# Solutions:

# 1. Check if DevEx is installed
find /usr -name devex 2>/dev/null
find $HOME -name devex 2>/dev/null

# 2. Add to PATH temporarily
export PATH=$PATH:/usr/local/bin

# 3. Add to PATH permanently
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.bashrc
source ~/.bashrc

# 4. Verify PATH
echo $PATH | tr ':' '\n' | grep -E "(usr|local|bin)"
```

### Plugin Installation Issues

#### Automatic Plugin Download Fails
```bash
# Error: Failed to download plugins
# Solutions:

# 1. Check network connectivity
devex system test-connectivity

# 2. Use offline mode
devex --offline install local-packages

# 3. Manual plugin installation
devex plugin install --local /path/to/plugin

# 4. Skip plugin downloads
devex --skip-plugin-download install packages
```

#### Plugin Compatibility Issues
```bash
# Error: Plugin version incompatible
# Solutions:

# 1. Update DevEx to latest version
devex update

# 2. Update plugins
devex plugin update --all

# 3. Check plugin compatibility
devex plugin info package-manager-apt
devex plugin deps package-manager-apt

# 4. Reinstall problematic plugins
devex plugin uninstall package-manager-apt
devex plugin install package-manager-apt
```

## Configuration Problems

### Configuration File Issues

#### Invalid Configuration Format
```bash
# Error: failed to parse configuration file
# Solutions:

# 1. Validate YAML syntax
devex config validate

# 2. Show configuration file location
devex config sources

# 3. Reset to defaults
devex config reset

# 4. Manual validation
yamllint ~/.devex/config.yaml
```

#### Configuration Not Loading
```bash
# Error: Configuration not being applied
# Solutions:

# 1. Check configuration hierarchy
devex config sources --detailed

# 2. Show effective configuration
devex config show

# 3. Test environment variables
env | grep DEVEX

# 4. Debug configuration loading
devex --debug config show
```

#### Configuration Conflicts
```bash
# Error: Conflicting configuration values
# Solutions:

# 1. Show configuration precedence
devex config sources --precedence

# 2. Clear environment variables
unset $(env | grep DEVEX | cut -d= -f1)

# 3. Use specific configuration file
devex --config /path/to/config.yaml install packages

# 4. Override with flags
devex install --verbose --dry-run packages
```

### Environment Variable Issues

```bash
# Debug environment variables
env | grep -E "(DEVEX|PATH|HOME)" | sort

# Common environment variable fixes
export DEVEX_LOG_LEVEL=debug
export DEVEX_CONFIG_PATH="$HOME/.devex/config.yaml"
export DEVEX_CACHE_DIR="$HOME/.cache/devex"
export DEVEX_DATA_DIR="$HOME/.local/share/devex"
```

## Plugin Issues

### Plugin Execution Failures

#### Plugin Not Found
```bash
# Error: plugin 'package-manager-apt' not found
# Solutions:

# 1. List available plugins
devex plugin list

# 2. Install missing plugin
devex plugin install package-manager-apt

# 3. Check plugin path
ls -la ~/.local/share/devex/plugins/

# 4. Refresh plugin cache
devex plugin refresh
```

#### Plugin Permission Errors
```bash
# Error: permission denied executing plugin
# Solutions:

# 1. Check plugin permissions
ls -la ~/.local/share/devex/plugins/package-manager-apt

# 2. Fix permissions
chmod +x ~/.local/share/devex/plugins/package-manager-apt

# 3. Check ownership
sudo chown -R $USER:$USER ~/.local/share/devex/

# 4. Run with appropriate privileges
sudo devex install system-packages
```

#### Plugin Crashes or Hangs
```bash
# Solutions:

# 1. Enable debug logging
devex plugin exec --debug package-manager-apt install git

# 2. Set timeouts
devex plugin exec --timeout 300 package-manager-apt install git

# 3. Check system resources
top
df -h
free -h

# 4. Kill hanging processes
pkill -f package-manager-apt
```

### Plugin-Specific Issues

#### APT Plugin Issues
```bash
# Common APT plugin problems and solutions:

# 1. Package not found
devex plugin exec package-manager-apt update
devex plugin exec package-manager-apt search package-name

# 2. Repository issues
devex plugin exec package-manager-apt validate-repository
sudo apt update

# 3. GPG key problems
sudo apt-key update
devex plugin exec package-manager-apt remove-repository problematic-repo

# 4. Lock file issues
sudo rm /var/lib/apt/lists/lock
sudo rm /var/cache/apt/archives/lock
sudo rm /var/lib/dpkg/lock*
```

#### Docker Plugin Issues
```bash
# Common Docker plugin problems and solutions:

# 1. Docker daemon not running
sudo systemctl start docker
sudo systemctl enable docker

# 2. Permission denied
sudo usermod -aG docker $USER
newgrp docker

# 3. Out of disk space
docker system prune -a
docker volume prune

# 4. Network issues
docker network ls
docker network prune
```

#### Git Plugin Issues
```bash
# Common Git plugin problems and solutions:

# 1. Git not installed
sudo apt install git  # Ubuntu/Debian
brew install git      # macOS

# 2. SSH key issues
ssh-keygen -t ed25519 -C "your_email@example.com"
ssh-add ~/.ssh/id_ed25519
ssh -T git@github.com

# 3. Configuration issues
git config --global --unset-all user.name
git config --global --unset-all user.email
devex plugin exec tool-git config --reset
```

## Package Manager Problems

### Package Manager Detection Issues

```bash
# Error: No suitable package manager found
# Solutions:

# 1. Check available package managers
devex system detect --package-managers

# 2. Install package manager
# Ubuntu/Debian: apt is usually pre-installed
# Fedora/RHEL: dnf is usually pre-installed
# Arch: pacman is usually pre-installed

# 3. Force specific package manager
devex install --package-manager apt packages

# 4. Install universal package managers
# Flatpak
sudo apt install flatpak
flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo
```

### Package Installation Failures

#### Dependency Conflicts
```bash
# Error: package has unmet dependencies
# Solutions:

# 1. Update package lists
sudo apt update
devex plugin exec package-manager-apt update

# 2. Fix broken packages
sudo apt --fix-broken install

# 3. Use different installation method
devex install --package-manager flatpak problematic-package

# 4. Install dependencies manually
sudo apt install package-dependencies
devex install package-name
```

#### Repository Issues
```bash
# Error: repository not accessible
# Solutions:

# 1. Check network connectivity
ping archive.ubuntu.com
ping security.ubuntu.com

# 2. Update repository configuration
sudo apt update
sudo apt-get update --fix-missing

# 3. Change repository mirrors
sudo sed -i 's/archive.ubuntu.com/mirror.example.com/g' /etc/apt/sources.list

# 4. Disable problematic repositories
sudo apt-add-repository --remove ppa:problematic/ppa
```

## Permission Issues

### File System Permissions

#### DevEx Directory Permissions
```bash
# Fix DevEx directory permissions
sudo chown -R $USER:$USER ~/.devex
sudo chown -R $USER:$USER ~/.local/share/devex
sudo chown -R $USER:$USER ~/.cache/devex

# Set correct permissions
chmod -R 755 ~/.devex
chmod -R 755 ~/.local/share/devex
chmod -R 755 ~/.cache/devex
```

#### System Package Installation
```bash
# Error: permission denied installing system packages
# Solutions:

# 1. Check sudo access
sudo -v

# 2. Add user to sudo group
sudo usermod -aG sudo $USER

# 3. Configure sudo for specific commands
echo "$USER ALL=(ALL) NOPASSWD: /usr/bin/apt" | sudo tee /etc/sudoers.d/devex-apt

# 4. Use package manager with elevated privileges
sudo devex install system-packages
```

#### Docker Permissions
```bash
# Error: permission denied connecting to Docker daemon
# Solutions:

# 1. Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# 2. Fix Docker socket permissions
sudo chmod 666 /var/run/docker.sock

# 3. Restart Docker daemon
sudo systemctl restart docker

# 4. Verify group membership
groups $USER | grep docker
```

## Network and Connectivity

### Network Connectivity Issues

#### Internet Connectivity
```bash
# Test basic connectivity
ping -c 4 8.8.8.8
ping -c 4 google.com

# Test specific services
ping -c 4 github.com
ping -c 4 archive.ubuntu.com
ping -c 4 docker.io
```

#### DNS Resolution
```bash
# Test DNS resolution
nslookup github.com
dig github.com

# Fix DNS issues
# 1. Add public DNS servers
echo "nameserver 8.8.8.8" | sudo tee -a /etc/resolv.conf
echo "nameserver 1.1.1.1" | sudo tee -a /etc/resolv.conf

# 2. Restart DNS service
sudo systemctl restart systemd-resolved
```

#### Proxy Configuration
```bash
# Configure proxy for DevEx
export HTTP_PROXY=http://proxy:8080
export HTTPS_PROXY=https://proxy:8080
export NO_PROXY=localhost,127.0.0.1

# Configure proxy for package managers
# APT
echo 'Acquire::http::proxy "http://proxy:8080";' | sudo tee /etc/apt/apt.conf.d/95proxies

# Docker
sudo mkdir -p /etc/systemd/system/docker.service.d/
cat << EOF | sudo tee /etc/systemd/system/docker.service.d/http-proxy.conf
[Service]
Environment="HTTP_PROXY=http://proxy:8080"
Environment="HTTPS_PROXY=https://proxy:8080"
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

#### SSL/TLS Issues
```bash
# Error: SSL certificate verification failed
# Solutions:

# 1. Update CA certificates
sudo apt update && sudo apt install ca-certificates
sudo update-ca-certificates

# 2. Check system time
date
sudo ntpdate -s time.nist.gov

# 3. Temporary SSL bypass (not recommended for production)
export PYTHONHTTPSVERIFY=0
git config --global http.sslverify false
```

## Performance Issues

### Slow Installation
```bash
# Optimize installation performance

# 1. Use parallel operations
devex config set package_managers.parallel_operations true

# 2. Use fastest mirrors
devex config set package_managers.auto_select_mirror true

# 3. Increase timeouts
export DEVEX_TIMEOUT=600

# 4. Clean caches
devex system clean-cache
sudo apt clean
docker system prune
```

### High Resource Usage
```bash
# Monitor system resources
top
htop
iotop
df -h
free -h

# Limit resource usage
devex config set system.max_cpu_percent 80
devex config set system.max_memory_mb 2048

# Clean up temporary files
sudo apt autoremove
sudo apt autoclean
docker system prune
devex system clean-temp
```

### Disk Space Issues
```bash
# Check disk usage
df -h
du -sh ~/.devex
du -sh ~/.local/share/devex
du -sh ~/.cache/devex

# Clean up disk space
# Package caches
sudo apt autoremove
sudo apt autoclean

# Docker cleanup
docker system prune -a
docker volume prune

# DevEx cleanup
devex system clean-cache
devex system clean-temp
```

## Debug Information

### Generating Debug Reports

```bash
# Generate comprehensive debug report
devex debug generate-report --output devex-debug.txt

# Generate component-specific debug info
devex debug system --output system-debug.txt
devex debug plugins --output plugins-debug.txt
devex debug config --output config-debug.txt

# Generate debug info for specific operations
devex --debug install --dry-run problematic-package > debug.log 2>&1
```

### Logging Configuration

```bash
# Enable debug logging
export DEVEX_LOG_LEVEL=debug
devex --verbose operation

# Log to file
devex install packages 2>&1 | tee install.log

# Real-time log monitoring
tail -f ~/.local/share/devex/logs/devex.log
```

### System Information Collection

```bash
# Collect system information for support
cat > system-info.txt << EOF
DevEx Version: $(devex --version)
OS: $(lsb_release -a 2>/dev/null || uname -a)
Architecture: $(uname -m)
Shell: $SHELL
User: $USER
Home: $HOME
PATH: $PATH

Package Managers:
$(devex system detect --package-managers)

Plugin Status:
$(devex plugin list)

Configuration:
$(devex config show)
EOF
```

## Getting Help

### Self-Help Resources

```bash
# Built-in help
devex --help
devex install --help
devex plugin --help

# Command-specific help
devex help install
devex help config

# Show examples
devex help install --examples
devex help config --examples
```

### Community Resources

- **Documentation**: https://docs.devex.sh/
- **GitHub Issues**: https://github.com/jameswlane/devex/issues
- **Discussions**: https://github.com/jameswlane/devex/discussions

### Reporting Issues

When reporting issues, include:

1. **DevEx version**: `devex --version`
2. **Operating system**: `lsb_release -a` or `uname -a`
3. **Command that failed**: Exact command and arguments
4. **Error messages**: Complete error output
5. **Debug information**: `devex debug generate-report`
6. **Configuration**: `devex config show` (remove sensitive data)

```bash
# Generate issue report template
cat > issue-report.md << EOF
## Issue Description
Brief description of the issue

## Environment
- DevEx Version: $(devex --version)
- OS: $(lsb_release -d 2>/dev/null | cut -f2 || uname -s)
- Architecture: $(uname -m)

## Steps to Reproduce
1. Command executed
2. Expected behavior
3. Actual behavior

## Error Output
\`\`\`
(Paste error output here)
\`\`\`

## Additional Context
(Any additional information)
EOF
```

### Emergency Recovery

If DevEx is completely broken:

```bash
# 1. Reset configuration
rm -rf ~/.devex
rm -rf ~/.local/share/devex
rm -rf ~/.cache/devex

# 2. Reinstall DevEx
curl -fsSL https://devex.sh/install | bash

# 3. Verify installation
devex --version
devex system check

# 4. Reconfigure
devex init
devex config validate
```

## Prevention and Best Practices

### Regular Maintenance

```bash
# Weekly maintenance script
#!/bin/bash
echo "Running DevEx maintenance..."

# Update DevEx
devex update

# Update plugins
devex plugin update --all

# Clean caches
devex system clean-cache

# Validate configuration
devex config validate

# Check system health
devex system check

echo "Maintenance completed!"
```

### Backup and Recovery

```bash
# Backup DevEx configuration
tar -czf devex-backup-$(date +%Y%m%d).tar.gz ~/.devex ~/.local/share/devex

# Restore from backup
tar -xzf devex-backup-20240115.tar.gz -C ~/

# Export configuration
devex config export --output devex-config-backup.yaml

# Import configuration
devex config import devex-config-backup.yaml
```

### Health Monitoring

```bash
# Create monitoring script
#!/bin/bash
# devex-health-check.sh

if ! devex system check >/dev/null 2>&1; then
    echo "DevEx health check failed!"
    devex system check
    exit 1
fi

echo "DevEx health check passed"
```

This troubleshooting guide covers the most common issues you may encounter with DevEx. For additional help or to report new issues, please refer to the community resources listed above.
