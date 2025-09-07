# Integration Tests

This document explains how to safely run integration tests for the DevEx CLI plugins.

## Overview

Integration tests are designed to test real system interactions but are isolated from normal test runs using Go build tags. These tests can potentially:

- Modify system package manager state
- Install/remove system packages
- Change shell configurations
- Create/modify configuration files
- Interact with Docker daemon
- Install Python packages via pip
- Modify Flatpak installations

## Safety Measures

All integration tests are protected by the `//go:build integration` build tag, which means:

- They **DO NOT** run during normal `go test` or `ginkgo` execution
- They **ONLY** run when explicitly requested with the `integration` build tag
- Most tests are **automatically skipped** unless specific system conditions are met

## Running Integration Tests

### Prerequisites

⚠️ **WARNING**: Only run integration tests in isolated environments such as:
- Docker containers
- Virtual machines  
- Dedicated test systems
- CI/CD environments

**NEVER** run integration tests on:
- Production systems
- Development machines with important configurations
- Systems where package installation could cause conflicts

### Running All Integration Tests

```bash
# From the repository root directory
cd /path/to/devex

# Run all integration tests (many will be skipped automatically)
go test -tags=integration ./...

# Run with Ginkgo for better output
ginkgo -tags=integration run ./...
```

### Running Package-Specific Integration Tests

```bash
# APT package manager tests
cd packages/package-manager-apt
go test -tags=integration

# Docker package manager tests  
cd packages/package-manager-docker
go test -tags=integration

# Flatpak package manager tests
cd packages/package-manager-flatpak
go test -tags=integration

# Pip package manager tests
cd packages/package-manager-pip
go test -tags=integration

# Shell tool tests
cd packages/tool-shell
go test -tags=integration

# CLI installer pipeline tests
cd apps/cli
go test -tags=integration ./internal/installers
```

### Test Requirements by Package

#### package-manager-apt
- **System**: Debian/Ubuntu with APT
- **Permissions**: Root/sudo access for package installation
- **Network**: Internet connectivity for package downloads
- **Risk**: Can install/remove system packages

#### package-manager-flatpak  
- **System**: Linux with Flatpak support
- **Permissions**: User permissions (some tests need root for system-wide)
- **Network**: Internet connectivity for Flathub
- **Risk**: Can install/remove applications

#### package-manager-docker
- **System**: Linux/macOS with Docker
- **Permissions**: Docker group membership or root
- **Network**: Internet connectivity for image pulls
- **Risk**: Can start/stop containers, pull/remove images

#### package-manager-pip
- **System**: Python with pip installed
- **Permissions**: User permissions (system install tests need root)
- **Network**: Internet connectivity for PyPI
- **Risk**: Can install/remove Python packages

#### tool-shell
- **System**: Unix-like with shell binaries (bash, zsh, fish)
- **Permissions**: User permissions (shell switching needs chsh access)
- **Risk**: Can modify shell configurations, change default shell

## Test Behavior

### Automatic Skipping
Most integration tests use `Skip()` directives that prevent execution unless:
- Required binaries are available (`apt`, `docker`, `flatpak`, etc.)
- Required permissions are present
- Network connectivity is available
- Other prerequisites are met

### Test Isolation
Tests create temporary directories and use controlled environments where possible:
- Temporary files in `/tmp/` or similar
- Isolated configuration directories
- Test-specific virtual environments (for pip)

### Validation and Safety
All integration tests include extensive input validation to prevent:
- Command injection attacks
- Directory traversal attacks  
- Execution of malicious commands
- Modification of critical system files

## Example Safe Execution

### Using Docker for APT Tests

```bash
# Run in Ubuntu container
docker run -it ubuntu:20.04 bash

# Inside container:
apt update && apt install -y git golang-go
git clone <repository>
cd devex/packages/package-manager-apt
go test -tags=integration
```

### Using VM for Full Integration Tests

```bash
# In a dedicated test VM:
git clone <repository>
cd devex

# Run all integration tests
ginkgo -tags=integration run ./...
```

## CI/CD Integration

Integration tests can be safely run in CI/CD environments:

```yaml
# GitHub Actions example
- name: Run Integration Tests
  run: |
    go test -tags=integration ./...
  env:
    # Tests run in isolated GitHub Actions environment
    CI: true
```

## Troubleshooting

### Tests Are Skipped
Most skipping is intentional for safety. Tests skip when:
- Required system packages are not installed
- Insufficient permissions
- Network connectivity issues
- System compatibility problems

### Permission Errors
Some tests require elevated permissions:
- Use `sudo` for system package operations
- Add user to `docker` group for Docker tests
- Ensure shell change permissions for shell tests

### Network Issues
Tests requiring network access will fail if:
- Internet connectivity is unavailable
- Package repositories are unreachable
- Network timeouts occur

## Cleanup

Integration tests attempt to clean up after themselves, but manual cleanup may be needed:

```bash
# Remove test packages (example)
sudo apt remove <test-packages>
docker system prune -f
pip uninstall <test-packages>
```

## Verifying Build Tag Configuration

To confirm that integration tests are properly isolated:

```bash
# This should run unit tests only (no integration tests)
cd packages/package-manager-apt
go test -v

# This should attempt to run integration tests (may have build errors, but shows tests are isolated)
go test -tags=integration -v

# Verify no integration test files are compiled without the tag
go list -f '{{.GoFiles}}' .  # Should not include integration_test.go
go list -f '{{.TestGoFiles}}' .  # Should not include integration_test.go

# With integration tag, files should be included
go list -tags=integration -f '{{.TestGoFiles}}' .  # Should include integration_test.go
```

## Best Practices

1. **Always use isolated environments** for integration testing
2. **Review test code** before running to understand system impact
3. **Monitor system resources** during test execution
4. **Keep backups** of important configurations
5. **Run tests individually** first to verify safety
6. **Use version control** to track configuration changes
7. **Verify build tag isolation** before making changes

## Contributing

When adding new integration tests:

1. Always use the `//go:build integration` build tag
2. Include extensive input validation 
3. Use `Skip()` for unsafe conditions
4. Create isolated test environments when possible
5. Document any system requirements
6. Test in containers/VMs before submission

For questions about integration tests, please refer to the main project documentation or open an issue.
