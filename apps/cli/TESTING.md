# DevEx CLI Testing Guide

This guide provides multiple testing approaches for rapid development and debugging.

## Quick Testing (Fastest - 5 seconds)

```bash
# Build and test locally
./test-cli.sh setup --verbose

# Test specific features
./test-cli.sh setup --non-interactive  # Test automated mode
./test-cli.sh --help                   # Test help
```

## Docker Environment Testing (Comprehensive)

### Setup
```bash
# Build Docker development environment
./docker-dev.sh
```

### Testing Commands
```bash
# Interactive setup with verbose logging
./docker-dev.sh setup-verbose

# Test automated mode
./docker-dev.sh setup-auto

# Get shell access for manual testing
./docker-dev.sh debug

# Run setup and capture all logs
./docker-dev.sh logs
```

### Manual Testing in Docker
```bash
# Start debug environment
./docker-dev.sh debug

# Inside Docker container:
devex setup --verbose
devex setup --non-interactive
devex --help

# Check logs
find /home/devex -name '*.log' -exec cat {} \;
```

## Issue-Specific Testing

### Git Configuration Input
```bash
./test-cli.sh setup --verbose
# Navigate to git config step
# Test:
# - ↑/↓ navigation between fields
# - Enter to edit field
# - Type name/email
# - Escape to cancel editing
# - 'n' to continue (only when both fields filled)
```

### Streaming Installer Panic
**Current Status:** Temporarily disabled, using fallback installer

```bash
# Test current fallback behavior
./test-cli.sh setup --verbose
# Should complete without hanging

# To re-enable streaming installer for debugging:
# 1. Edit pkg/commands/setup.go
# 2. Comment out fallback installer lines 781-786
# 3. Uncomment TUI installer lines 789-795
# 4. ./test-cli.sh setup --verbose
```

### Docker Installation
```bash
# Test Docker setup automation
./docker-dev.sh setup-verbose
# Should automatically:
# - Install docker.io via APT
# - Enable Docker service
# - Start Docker service  
# - Add user to docker group
# - Test Docker daemon accessibility
```

## Development Workflow

### Fast Iteration
1. Make code changes
2. `./test-cli.sh setup --verbose` (5 seconds)
3. Debug issues immediately
4. Repeat

### Comprehensive Testing
1. Make code changes
2. `./docker-dev.sh setup-verbose` (30 seconds)
3. Test in clean Ubuntu environment
4. Verify all features work

### Debugging Crashes/Panics
1. `./docker-dev.sh debug`
2. Run `devex setup --verbose` manually
3. Check logs: `find /home/devex -name '*.log' -exec cat {} \;`
4. Add debug prints to code
5. Rebuild and test: `exit && ./docker-dev.sh setup-verbose`

## Log Analysis

### Log Locations
- Local testing: `~/.local/share/devex/logs/`
- Docker testing: `/home/devex/.local/share/devex/logs/`

### Key Log Messages
```bash
# Success indicators
grep "Installation completed successfully" logs/*

# Error indicators  
grep "ERROR" logs/*
grep "panic" logs/*
grep "failed" logs/*

# Installation progress
grep "App to install" logs/*
grep "Starting streaming installer" logs/*
```

## Common Issues & Solutions

### 1. Git Input Not Working
**Test:** Navigate to git config step, try typing

**Expected:** Visual cursor, text appears as you type

**Debug:** Check Update() method handles StepGitConfig properly

### 2. Streaming Installer Panic
**Current:** Fallback installer active

**Debug:** Re-enable TUI installer, add more panic recovery

### 3. Docker Permission Issues
**Test:** Check if Docker commands work without sudo

**Expected:** User in docker group, service running

**Debug:** Check setupDockerService() logs

## Performance Testing

### Memory Usage
```bash
# Monitor during installation
./docker-dev.sh debug
# In container:
devex setup --verbose &
top -p $(pgrep devex)
```

### Installation Speed
```bash
time ./test-cli.sh setup --non-interactive
```

## Contributing Tests

When adding new features:

1. **Add manual test cases** to this guide
2. **Test both interactive and automated modes**
3. **Verify error handling** with invalid inputs
4. **Test edge cases** (empty configs, network failures, etc.)
5. **Document expected behavior** vs actual results

## Troubleshooting

### Build Failures
```bash
# Check Go modules
go mod tidy

# Verify imports
goimports -w .

# Run linters
golangci-lint run
```

### Docker Issues
```bash
# Rebuild environment
docker rmi devex-dev
./docker-dev.sh

# Check Docker daemon
sudo systemctl status docker
```

### Permission Issues
```bash
# Fix script permissions
chmod +x test-cli.sh docker-dev.sh

# Check file ownership
ls -la bin/devex
```

This testing framework allows for rapid iteration and comprehensive debugging without the slow build/release cycle.
