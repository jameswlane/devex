# DevEx CLI Testing Guide

This guide provides step-by-step testing approaches while leveraging Docker environments and volume mappings for rapid and flexible testing.

---

## Quick Local Testing (Fastest - 5 Seconds)
```
bash
# Build and test the CLI locally
./cli-docker-test.sh build --distro debian
./cli-docker-test.sh run --distro debian setup --verbose

# Test specific features
./cli-docker-test.sh run --distro debian setup --non-interactive  # Test automated mode
./cli-docker-test.sh run --distro debian --help                   # Test help command
```
---

## Comprehensive Docker Environment Testing

### Pre-Setup
Configure the following volume mappings for dynamic testing:
- **Host Directories**:
  - `apps/cli/assets` (for assets like static files)
  - `apps/cli/bin` (for the CLI binary)
  - `apps/cli/config` (for configurations)
  - `apps/cli/help` (for help documents)
- **Mapped Paths Inside Container**:
  - `~/.local/share/devex/assets`
  - `~/.local/share/devex/bin`
  - `~/.local/share/devex/config`
  - `~/.local/share/devex/help`

The `cli-docker-test.sh` script automatically handles these mappings during runtime.

---

### Building Docker Images for Testing
```
bash
# Build Docker image for a specific distribution
./cli-docker-test.sh build --distro debian
./cli-docker-test.sh build --distro ubuntu
./cli-docker-test.sh build --distro suse
```
---

### Testing Commands
```
bash
# Interactive setup with verbose logging
./cli-docker-test.sh run --distro debian setup --verbose

# Test automated mode
./cli-docker-test.sh run --distro debian setup --non-interactive

# Run specific commands (e.g., help)
./cli-docker-test.sh run --distro debian --help

# Start a debug shell for manual testing
./cli-docker-test.sh shell --distro ubuntu
```
---

### Manual Testing in Docker Environment
```
bash
# Start debug environment
./cli-docker-test.sh shell --distro debian

# Inside the Docker container:
devex setup --verbose                   # Run setup interactively
devex setup --non-interactive           # Test non-interactive mode
devex --help                            # Test help command

# Verify host and container mappings
ls -la ~/.local/share/devex

# Check logs inside container
find ~/.local/share/devex/logs -name '*.log' -exec cat {} \;
```
---

## Volume Mapping Details

The `cli-docker-test.sh` script takes care of dynamically mounting the following directories:

| Host Path                    | Container Path                        |
|------------------------------|---------------------------------------|
| `apps/cli/assets`            | `~/.local/share/devex/assets`        |
| `apps/cli/bin`               | `~/.local/share/devex/bin`           |
| `apps/cli/config`            | `~/.local/share/devex/config`        |
| `apps/cli/help`              | `~/.local/share/devex/help`          |

When making changes to these directories, they are reflected in the container immediately, without rebuilding the Docker image.

---

## Contributing Tests

When adding new features or fixing bugs:
1. **Update Testing Scenarios**:
   - Ensure the feature is testable in both **interactive** and **automated modes**.
2. **Test Volume Mappings**:
   - Modify the source files in directories like `apps/cli/assets` or `apps/cli/config`, and test immediate availability inside containers.
3. **Verify Logs**:
   - Ensure logs are created in `~/.local/share/devex/logs/` and validate errors and expected output.
4. **Edge Case Handling**:
   - Test interactive behavior (e.g., inputs during `setup`) and failure conditions (e.g., missing files, invalid configurations).

---

## Troubleshooting and Common Issues

### Log Analysis
- **Locations**:
  - Local: `~/.local/share/devex/logs/`
  - Docker: `~/.local/share/devex/logs/`
- **Search Patterns**:
  ```bash
  # Success
  grep "Installation completed successfully" ~/.local/share/devex/logs/*

  # Errors
  grep "ERROR" ~/.local/share/devex/logs/*
  grep "panic" ~/.local/share/devex/logs/*
  grep "failed" ~/.local/share/devex/logs/*
  ```

### Debugging Docker Issues
```
bash
# Test mappings manually
./cli-docker-test.sh shell --distro ubuntu
# Inside container, verify paths:
ls -la ~/.local/share/devex

# Check Docker daemon
sudo systemctl status docker
```
### Build or Permission Issues
```
bash
# Fix file permissions for test scripts
chmod +x ./cli-docker-test.sh

# Rebuild Docker image if out of sync
./cli-docker-test.sh build --distro ubuntu
```
By following this updated guide, you'll ensure consistent and flexible testing with minimal setup overhead. Let me know if you'd like further assistance!

