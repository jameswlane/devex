# DevEx Testing Guide

This guide explains how to safely test DevEx without affecting your development environment.

## Quick Start

```bash
# Build test containers
./scripts/test-docker.sh build ubuntu

# Start interactive testing session
./scripts/test-docker.sh shell ubuntu

# Run automated tests
./scripts/test-docker.sh test ubuntu --dry-run
```

## Testing Methods

### 1. Docker Container Testing (Recommended)

**Advantages:**
- Completely isolated from host system
- Fast setup and teardown
- Multiple distro testing
- Reproducible environments
- Easy cleanup

**Setup:**
```bash
# Build containers for both Ubuntu and Debian
./scripts/test-docker.sh build

# Or build specific distro
./scripts/test-docker.sh build ubuntu
./scripts/test-docker.sh build debian
```

**Interactive Testing:**
```bash
# Start shell in container
./scripts/test-docker.sh shell ubuntu

# Inside container, test DevEx commands:
./bin/devex --help
./bin/devex install --dry-run
./bin/devex install --category "System Tools"
```

**Automated Testing:**
```bash
# Quick validation
./scripts/test-docker.sh test ubuntu --dry-run

# Test with specific config
./scripts/test-docker.sh test ubuntu --config minimal-test.yaml

# Test actual installations (safe in container)
./scripts/test-docker.sh test debian
```

### 2. Test Configurations

Use safe, minimal configs for testing:

**`test/configs/minimal-test.yaml`** - Basic system tools (curl, htop, tree)
**`test/configs/programming-test.yaml`** - Language runtimes via mise

Copy test configs to container:
```bash
# In container
cp test/configs/minimal-test.yaml ~/.devex/apps.yaml
./bin/devex install --dry-run
```

### 3. VM Testing (For Full System Testing)

**When to use:** Testing GNOME settings, desktop themes, full system integration

**Setup with multipass:**
```bash
# Install multipass
sudo snap install multipass

# Create test VM
multipass launch ubuntu -n devex-test -c 2 -m 4G -d 20G

# Transfer DevEx to VM
multipass transfer . devex-test:/home/ubuntu/devex

# Shell into VM
multipass shell devex-test

# Install desktop environment if needed
sudo apt update && sudo apt install ubuntu-desktop-minimal
```

## Development Workflow

### 1. Feature Development
```bash
# Make code changes
vim pkg/installers/apt/apt.go

# Test in container
./scripts/test-docker.sh build ubuntu
./scripts/test-docker.sh shell ubuntu

# In container - rebuild and test
go build -o bin/devex ./cmd/main.go
./bin/devex install --dry-run
```

### 2. Testing New Installers
```bash
# Create minimal test config
cat > test/configs/new-installer-test.yaml << EOF
apps:
  - name: "test-app"
    install_method: "your-new-method"
    install_command: "test-command"
EOF

# Test in container
./scripts/test-docker.sh shell ubuntu
cp test/configs/new-installer-test.yaml ~/.devex/apps.yaml
./bin/devex install --dry-run
```

### 3. Integration Testing
```bash
# Test multiple distros
./scripts/test-docker.sh test ubuntu
./scripts/test-docker.sh test debian

# Test with different configs
./scripts/test-docker.sh test ubuntu --config minimal-test.yaml
./scripts/test-docker.sh test ubuntu --config programming-test.yaml
```

## Available Test Commands

### Docker Script Commands
```bash
./scripts/test-docker.sh build [ubuntu|debian]     # Build containers
./scripts/test-docker.sh shell [ubuntu|debian]     # Interactive shell
./scripts/test-docker.sh test [ubuntu|debian]      # Run tests
./scripts/test-docker.sh clean                     # Clean up
./scripts/test-docker.sh logs [ubuntu|debian]      # View logs
```

### In-Container DevEx Testing
```bash
# Basic validation
./bin/devex --version
./bin/devex --help

# Safe testing
./bin/devex install --dry-run
./bin/devex install --dry-run --verbose

# Category-specific testing
./bin/devex install --category "System Tools" --dry-run
./bin/devex install --category "Programming Languages" --dry-run

# Actual installation (safe in container)
./bin/devex install --category "System Tools"
```

## Test Safety Guidelines

### Safe to Test in Containers
- All package installations
- Configuration file operations
- Database operations
- Shell script execution
- Dependency checking

### Requires VM Testing
- GNOME desktop settings
- Desktop environment themes
- Full system integration
- Reboot-required changes

### Never Test on Host System
- System-wide package installations
- Modifying system configurations
- Desktop environment changes
- Installing unknown software

## Troubleshooting

### Container Build Issues
```bash
# Clean rebuild
./scripts/test-docker.sh clean
./scripts/test-docker.sh build ubuntu

# Check logs
./scripts/test-docker.sh logs ubuntu
```

### Permission Issues in Container
```bash
# Container runs as testuser with sudo access
sudo apt update  # Works in container
```

### Testing Specific Features
```bash
# Test APT installer
./bin/devex install --category "System Tools" --dry-run

# Test configuration loading
./bin/devex system info

# Test database operations
./bin/devex system list-apps
```

## Adding New Test Scenarios

### Create New Test Config
```bash
# Add to test/configs/
cat > test/configs/my-feature-test.yaml << EOF
apps:
  - name: "my-test-app"
    # your test configuration
EOF
```

### Add to CI Pipeline
```bash
# In .github/workflows/
./scripts/test-docker.sh test ubuntu --config my-feature-test.yaml
```

This testing setup lets you safely develop and validate DevEx features without risking your host system!
