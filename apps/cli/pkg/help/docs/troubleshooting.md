# Troubleshooting

Common issues and solutions for DevEx users.

## Installation Issues

### DevEx Installation Problems

#### Binary Not Found After Installation
**Problem**: `devex: command not found` after installation.

**Solutions**:
```bash
# Check if binary exists
ls -la /usr/local/bin/devex
ls -la ~/.local/bin/devex

# Add to PATH if needed
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Verify PATH
echo $PATH | grep -E "(usr/local/bin|\.local/bin)"
```

#### Permission Denied During Installation
**Problem**: Permission errors when installing DevEx.

**Solutions**:
```bash
# Install to user directory instead
mkdir -p ~/.local/bin
wget -O ~/.local/bin/devex https://github.com/jameswlane/devex/releases/latest/download/devex-linux-amd64
chmod +x ~/.local/bin/devex

# Add to PATH
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

#### Download Failures
**Problem**: Network issues during installation.

**Solutions**:
```bash
# Use alternative download method
curl -L -o devex https://github.com/jameswlane/devex/releases/latest/download/devex-linux-amd64

# Use different mirror
wget --timeout=30 --tries=3 -O devex [URL]

# Check network connectivity
ping github.com
curl -I https://github.com
```

### Application Installation Problems

#### Package Manager Not Found
**Problem**: DevEx can't find package manager (apt, brew, etc.).

**Solutions**:
```bash
# Check system package manager
which apt || which dnf || which pacman || which brew

# Install missing package manager (if needed)
# On macOS: Install Homebrew
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Force specific installer
devex install docker --installer flatpak
```

#### Package Not Found
**Problem**: Application not available in package manager.

**Solutions**:
```bash
# Update package lists first
sudo apt update          # Debian/Ubuntu
brew update             # macOS
sudo dnf check-update   # Fedora

# Try alternative installer
devex install app-name --installer snap
devex install app-name --installer flatpak

# Search for alternative package names
apt search app-name
brew search app-name
```

#### Installation Permissions
**Problem**: Permission errors during package installation.

**Solutions**:
```bash
# Use --user flag when possible
devex install --user --all

# Fix sudo configuration
sudo visudo  # Add user to sudo group

# Install to user directories
pip install --user package
npm install -g --prefix ~/.local package
```

#### Docker Installation Issues
**Problem**: Docker installation fails or service won't start.

**Solutions**:
```bash
# Check Docker service status
systemctl status docker

# Start Docker service
sudo systemctl start docker
sudo systemctl enable docker

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Test Docker
docker run hello-world

# Restart Docker if needed
sudo systemctl restart docker
```

## Configuration Issues

### Configuration File Errors

#### YAML Syntax Errors
**Problem**: Invalid YAML syntax in configuration files.

**Solutions**:
```bash
# Validate configuration
devex config validate

# Check YAML syntax online or with tools
yamllint ~/.devex/applications.yaml

# Common YAML issues:
# - Incorrect indentation (use spaces, not tabs)
# - Missing quotes around special characters
# - Incorrect list formatting
```

**Example fixes**:
```yaml
# Wrong
categories:
development:
- name: code

# Correct
categories:
  development:
    - name: code
```

#### Configuration Not Loading
**Problem**: DevEx ignores configuration changes.

**Solutions**:
```bash
# Check configuration file locations
ls -la ~/.devex/

# Validate configuration
devex config validate

# Check file permissions
chmod 644 ~/.devex/*.yaml

# Reload configuration
devex config list
```

#### Conflicting Configurations
**Problem**: Multiple configuration files conflict.

**Solutions**:
```bash
# Check configuration precedence
devex config list --summary

# Reset to defaults
devex config reset --backup

# Merge configurations carefully
devex config import new-config.yaml --merge --validate
```

### Backup and Recovery Issues

#### Backup Corruption
**Problem**: Cannot restore from backup.

**Solutions**:
```bash
# List available backups
devex config backup list

# Verify backup integrity
devex config backup verify backup-id

# Try different backup
devex config backup restore older-backup-id

# Manual recovery
cp ~/.devex/backups/backup-*/applications.yaml ~/.devex/
```

#### No Backups Available
**Problem**: No backups when needed.

**Solutions**:
```bash
# Enable automatic backups
devex config backup --auto

# Create manual backup
devex config backup create "Manual backup"

# Export current config as emergency backup
devex config export --format bundle --output emergency-backup.zip
```

## Runtime Issues

### Command Execution Problems

#### Commands Hang or Freeze
**Problem**: DevEx commands don't respond.

**Solutions**:
```bash
# Use timeout
timeout 30 devex install --all

# Check for background processes
ps aux | grep devex

# Kill stuck processes
pkill devex

# Use verbose mode to debug
devex install --verbose --dry-run
```

#### TUI Display Issues
**Problem**: Terminal UI looks broken or garbled.

**Solutions**:
```bash
# Use non-TUI mode
devex install --no-tui --all

# Check terminal capabilities
echo $TERM
tput colors

# Update terminal
# Modern terminal recommended: kitty, alacritty, or latest gnome-terminal

# Set explicit TERM
export TERM=xterm-256color
```

#### Network Connectivity Issues
**Problem**: Downloads fail due to network problems.

**Solutions**:
```bash
# Test connectivity
ping google.com
curl -I https://github.com

# Use proxy if needed
export HTTP_PROXY=http://proxy:8080
export HTTPS_PROXY=http://proxy:8080

# Configure git proxy
git config --global http.proxy http://proxy:8080

# Skip network operations
devex install --offline  # Use cached packages only
```

### Performance Issues

#### Slow Installation
**Problem**: Installations take too long.

**Solutions**:
```bash
# Increase parallel jobs (use with caution)
devex install --parallel 5

# Clean cache to free space
devex cache cleanup

# Use faster mirrors
# Configure package manager to use local mirrors

# Check disk space
df -h ~/.devex/cache
```

#### High Memory Usage
**Problem**: DevEx uses too much memory.

**Solutions**:
```bash
# Reduce parallel installations
devex install --parallel 1

# Clear cache
devex cache cleanup --max-size 100MB

# Monitor memory usage
top | grep devex
```

## System-Specific Issues

### Linux Issues

#### Snap Package Problems
**Problem**: Snap packages don't install or run.

**Solutions**:
```bash
# Check snapd service
sudo systemctl status snapd
sudo systemctl start snapd

# Update snapd
sudo apt update && sudo apt install snapd

# Use alternative installer
devex install app --installer flatpak
```

#### Flatpak Issues
**Problem**: Flatpak applications have problems.

**Solutions**:
```bash
# Add Flathub repository
flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo

# Update Flatpak
sudo apt update && sudo apt install flatpak

# Check Flatpak installation
flatpak list
flatpak update
```

#### AppImage Problems
**Problem**: AppImage files don't execute.

**Solutions**:
```bash
# Make executable
chmod +x app.AppImage

# Install FUSE if needed
sudo apt install fuse

# Run with --appimage-extract-and-run
./app.AppImage --appimage-extract-and-run
```

### macOS Issues

#### Homebrew Problems
**Problem**: Brew commands fail or packages conflict.

**Solutions**:
```bash
# Update Homebrew
brew update
brew upgrade

# Fix common issues
brew doctor

# Clean up
brew cleanup
brew autoremove

# Reinstall Homebrew if needed
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/uninstall.sh)"
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

#### Mac App Store Issues
**Problem**: `mas` command doesn't work.

**Solutions**:
```bash
# Install mas
brew install mas

# Sign in to App Store first
mas signin you@example.com

# Check signed-in account
mas account
```

### Windows Issues

#### Windows Package Manager (winget) Problems
**Problem**: winget commands fail.

**Solutions**:
```powershell
# Update winget
winget upgrade --id Microsoft.AppInstaller

# Check winget version
winget --version

# Reset winget source
winget source reset
```

#### PowerShell Execution Policy
**Problem**: Scripts blocked by execution policy.

**Solutions**:
```powershell
# Check current policy
Get-ExecutionPolicy

# Set policy for current user
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser

# Bypass for single command
powershell -ExecutionPolicy Bypass -Command "command here"
```

## Error Messages

### Common Error Messages

#### "Configuration validation failed"
**Problem**: Configuration files have errors.

**Solutions**:
```bash
# Get detailed validation errors
devex config validate --verbose

# Auto-fix common issues
devex config validate --fix

# Reset configuration if corrupted
devex config reset --backup
```

#### "Installer not found"
**Problem**: Package manager not available.

**Solutions**:
```bash
# Check available installers
devex system info --installers

# Force different installer
devex install app --installer snap

# Install missing package manager
# See package manager installation above
```

#### "Package not found"
**Problem**: Application not available in repositories.

**Solutions**:
```bash
# Update package lists
sudo apt update  # or equivalent for your system

# Search for alternative names
apt search app-name

# Try different installer
devex install app --installer flatpak
```

#### "Permission denied"
**Problem**: Insufficient permissions.

**Solutions**:
```bash
# Use --user flag
devex install --user app

# Fix directory permissions
chmod 755 ~/.devex
chmod 644 ~/.devex/*.yaml

# Add user to sudo group (if appropriate)
sudo usermod -aG sudo $USER
```

## Getting Help

### Debug Information

#### Collect System Information
```bash
# System info
devex system info

# Configuration status
devex config list --summary

# Installation status
devex status --all --format json

# Cache information
devex cache stats
```

#### Enable Verbose Logging
```bash
# Run with verbose output
devex --verbose install docker

# Enable debug logging
export DEVEX_LOG_LEVEL=debug
devex install --all
```

### Community Support

#### GitHub Issues
- Check existing issues: https://github.com/jameswlane/devex/issues
- Create detailed bug report with system info and error messages
- Include DevEx version: `devex --version`

#### Discussions
- Community Q&A: https://github.com/jameswlane/devex/discussions
- Share configurations and templates

#### Documentation
- Full documentation: https://docs.devex.sh
- Command reference: `devex help`

### Creating Bug Reports

Include this information:
```bash
# DevEx version
devex --version

# System information
devex system info

# Error reproduction steps
devex command-that-failed --verbose

# Configuration (if relevant)
devex config list --format yaml
```

## Prevention

### Best Practices

#### Regular Maintenance
```bash
# Weekly: Update and clean
devex template update
devex cache cleanup --max-age 7d

# Before major changes: Backup
devex config backup create "Before update"

# After issues: Validate
devex config validate
devex status --all
```

#### Configuration Management
```bash
# Version control your configurations
git init ~/.devex
git add ~/.devex/*.yaml
git commit -m "DevEx configuration"

# Regular exports
devex config export --format bundle --output "backup-$(date +%Y%m%d).zip"
```

#### System Updates
- Keep system package managers updated
- Update DevEx regularly: `devex self-update`
- Monitor DevEx releases and changelog

---

*Still having issues? Check our [GitHub Issues](https://github.com/jameswlane/devex/issues) or start a [Discussion](https://github.com/jameswlane/devex/discussions).*
