# DevEx CLI Docker Testing Guide

A comprehensive guide to using the enhanced Docker testing infrastructure for the DevEx CLI tool.

## 📋 Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Distribution Support](#distribution-support)
- [Testing Scripts](#testing-scripts)
- [Test Suites](#test-suites)
- [Docker Compose](#docker-compose)
- [CI/CD Integration](#cicd-integration)
- [Performance & Security](#performance--security)
- [Troubleshooting](#troubleshooting)
- [Advanced Usage](#advanced-usage)

## 🌟 Overview

The DevEx CLI Docker testing infrastructure provides:

- **🐳 Multi-distribution testing** across 9 Linux distributions
- **🛡️ Security hardening** with passwordless sudo and minimal attack surface
- **⚡ Performance optimization** with multi-stage builds and parallel testing
- **🧪 Comprehensive test coverage** for all CLI commands and features
- **🚀 CI/CD integration** with GitHub Actions and automated workflows
- **🔧 Developer-friendly tools** for debugging and interactive testing

## 🚀 Quick Start

### Prerequisites

```bash
# Ensure Docker is installed and running
docker --version
docker info

# Navigate to CLI directory
cd /path/to/devex/apps/cli

# Ensure scripts are executable
chmod +x scripts/cli-docker-test-enhanced.sh
```

### Basic Usage

```bash
# 1. Build CLI binary and Docker image (Debian default)
./scripts/cli-docker-test-enhanced.sh build

# 2. Run basic tests
./scripts/cli-docker-test-enhanced.sh test basic

# 3. Run all test suites
./scripts/cli-docker-test-enhanced.sh test all

# 4. Test specific distribution
./scripts/cli-docker-test-enhanced.sh build --distro ubuntu
./scripts/cli-docker-test-enhanced.sh test all --distro ubuntu
```

### Quick Validation

```bash
# Test across all distributions in parallel (comprehensive)
./scripts/cli-docker-test-enhanced.sh test-parallel

# Interactive debugging session
./scripts/cli-docker-test-enhanced.sh shell --distro alpine

# Performance benchmarking
./scripts/cli-docker-test-enhanced.sh benchmark --distro debian
```

## 🐧 Distribution Support

### Supported Distributions

| Distribution | Dockerfile | Base Image | Package Manager | Priority | Status |
|-------------|------------|------------|-----------------|----------|---------|
| **Debian** | `Dockerfile.debian` | `debian:12-slim` | `apt` | 1 | ✅ Primary |
| **Ubuntu** | `Dockerfile.ubuntu` | `ubuntu:22.04` | `apt` | 1 | ✅ Primary |
| **Arch Linux** | `Dockerfile.arch` | `archlinux:latest` | `pacman` | 2 | ✅ Active |
| **Fedora** | `Dockerfile.fedora` | `fedora:39` | `dnf` | 2 | ✅ Active |
| **Alpine** | `Dockerfile.alpine` | `alpine:3.19` | `apk` | 2 | ✅ Active |
| **Red Hat** | `Dockerfile.redhat` | `ubi9/ubi:9.3` | `dnf` | 2 | ✅ Active |
| **openSUSE** | `Dockerfile.suse` | `opensuse/leap:15.5` | `zypper` | 2 | ✅ Active |
| **CentOS Stream** | `Dockerfile.centos-stream` | `centos:stream9` | `dnf` | 3 | ⚠️ Testing |
| **Manjaro** | `Dockerfile.manjaro` | `manjarolinux/base` | `pacman` | 3 | ⚠️ Testing |

### Special Variants

| Variant | Dockerfile | Purpose | Use Case |
|---------|------------|---------|----------|
| **Distroless** | `Dockerfile.distroless` | Ultra-secure, minimal | Production-like testing |
| **Minimal** | `Dockerfile.minimal` | Lightweight busybox | Resource-constrained testing |

### Testing Specific Distributions

```bash
# Test on Debian-based systems
./scripts/cli-docker-test-enhanced.sh build --distro debian
./scripts/cli-docker-test-enhanced.sh test all --distro debian

./scripts/cli-docker-test-enhanced.sh build --distro ubuntu
./scripts/cli-docker-test-enhanced.sh test all --distro ubuntu

# Test on Red Hat family
./scripts/cli-docker-test-enhanced.sh build --distro fedora
./scripts/cli-docker-test-enhanced.sh test apps --distro fedora

./scripts/cli-docker-test-enhanced.sh build --distro redhat
./scripts/cli-docker-test-enhanced.sh test config --distro redhat

# Test on Arch-based systems
./scripts/cli-docker-test-enhanced.sh build --distro arch
./scripts/cli-docker-test-enhanced.sh test system --distro arch

./scripts/cli-docker-test-enhanced.sh build --distro manjaro
./scripts/cli-docker-test-enhanced.sh test basic --distro manjaro

# Test on SUSE systems
./scripts/cli-docker-test-enhanced.sh build --distro suse
./scripts/cli-docker-test-enhanced.sh test templates --distro suse

# Test on lightweight systems
./scripts/cli-docker-test-enhanced.sh build --distro alpine
./scripts/cli-docker-test-enhanced.sh test recovery --distro alpine
```

## 🧪 Testing Scripts

### Enhanced Test Script (`cli-docker-test-enhanced.sh`)

#### Available Commands

| Command | Description | Example |
|---------|-------------|---------|
| `build` | Build CLI binary and Docker image | `./script build --distro ubuntu` |
| `test [SUITE]` | Run specific test suite | `./script test basic --distro alpine` |
| `test-parallel` | Test all distributions in parallel | `./script test-parallel` |
| `test-interactive` | Run interactive test | `./script test-interactive "setup --help"` |
| `shell` | Start interactive shell | `./script shell --distro debian` |
| `logs` | Inspect container logs | `./script logs --distro fedora` |
| `benchmark` | Run performance tests | `./script benchmark --distro alpine` |
| `clean` | Clean up Docker resources | `./script clean` |

#### Command Options

| Option | Description | Default | Example |
|--------|-------------|---------|---------|
| `--distro DISTRO` | Target distribution | `debian` | `--distro ubuntu` |
| `--parallel` | Enable parallel testing | `false` | `--parallel` |
| `--timeout SECONDS` | Test timeout | `300` | `--timeout 600` |
| `--verbose` | Enable verbose output | `false` | `--verbose` |
| `--test-suite SUITE` | Specify test suite | `all` | `--test-suite basic` |

#### Environment Variables

```bash
# Set default distribution
export DISTRO=alpine

# Enable parallel testing
export PARALLEL_TESTS=true

# Set test timeout
export TEST_TIMEOUT=600

# Enable verbose mode
export VERBOSE=true

# Set default test suite
export TEST_SUITE=basic
```

### Complete Command Examples

```bash
# Build and test with verbose output
./scripts/cli-docker-test-enhanced.sh build --distro ubuntu --verbose
./scripts/cli-docker-test-enhanced.sh test all --distro ubuntu --verbose --timeout 600

# Environment variable usage
DISTRO=alpine VERBOSE=true ./scripts/cli-docker-test-enhanced.sh test basic

# Parallel testing with timeout
./scripts/cli-docker-test-enhanced.sh test-parallel --timeout 900 --verbose

# Interactive testing
./scripts/cli-docker-test-enhanced.sh test-interactive "config validate" --distro fedora

# Security-focused testing
./scripts/cli-docker-test-enhanced.sh build --distro alpine
./scripts/cli-docker-test-enhanced.sh test basic --distro alpine
./scripts/cli-docker-test-enhanced.sh test all --distro alpine --timeout 300
```

## 📊 Test Suites

### Available Test Suites

| Suite | Commands Tested | Description | Typical Duration |
|-------|----------------|-------------|------------------|
| **basic** | `--version`, `--help`, `help` | Core CLI functionality | 30-60s |
| **config** | `config`, `init`, validation | Configuration management | 60-120s |
| **apps** | `install`, `list`, `add`, `remove`, `status` | Application management | 120-300s |
| **system** | `system`, `detect`, `shell`, `cache` | System commands | 60-180s |
| **templates** | `template list`, template operations | Template management | 60-120s |
| **recovery** | `rollback`, `undo` | Recovery operations | 60-120s |
| **all** | All of the above | Complete test coverage | 300-600s |

### Running Specific Test Suites

```bash
# Quick validation (basic functionality)
./scripts/cli-docker-test-enhanced.sh test basic --distro debian

# Configuration testing
./scripts/cli-docker-test-enhanced.sh test config --distro ubuntu --verbose

# Application management testing
./scripts/cli-docker-test-enhanced.sh test apps --distro fedora --timeout 300

# System command testing
./scripts/cli-docker-test-enhanced.sh test system --distro arch

# Template functionality testing
./scripts/cli-docker-test-enhanced.sh test templates --distro alpine

# Recovery command testing
./scripts/cli-docker-test-enhanced.sh test recovery --distro suse

# Comprehensive testing (all suites)
./scripts/cli-docker-test-enhanced.sh test all --distro redhat --timeout 600
```

### Test Suite Combinations

```bash
# Quick smoke test across multiple distributions
for distro in debian ubuntu alpine; do
    echo "Testing $distro..."
    ./scripts/cli-docker-test-enhanced.sh build --distro $distro
    ./scripts/cli-docker-test-enhanced.sh test basic --distro $distro
done

# Configuration validation across Red Hat family
for distro in fedora redhat centos-stream; do
    echo "Config testing on $distro..."
    ./scripts/cli-docker-test-enhanced.sh build --distro $distro
    ./scripts/cli-docker-test-enhanced.sh test config --distro $distro
done

# Performance comparison across lightweight distributions
for distro in alpine debian; do
    echo "Benchmarking $distro..."
    ./scripts/cli-docker-test-enhanced.sh build --distro $distro
    ./scripts/cli-docker-test-enhanced.sh benchmark --distro $distro
done
```

## 🐳 Docker Compose

### Enhanced Compose Configuration

The `docker-compose.enhanced.yml` provides orchestrated testing capabilities.

#### Available Services

| Service | Purpose | Command |
|---------|---------|---------|
| `test-debian` | Debian testing | `docker-compose up test-debian` |
| `test-ubuntu` | Ubuntu testing | `docker-compose up test-ubuntu` |
| `test-arch` | Arch Linux testing | `docker-compose up test-arch` |
| `test-fedora` | Fedora testing | `docker-compose up test-fedora` |
| `test-alpine` | Alpine testing | `docker-compose up test-alpine` |
| `test-redhat` | Red Hat testing | `docker-compose up test-redhat` |
| `test-suse` | openSUSE testing | `docker-compose up test-suse` |
| `test-orchestrator` | Parallel testing | `docker-compose up test-orchestrator` |

#### Service Profiles

| Profile | Services | Purpose |
|---------|----------|---------|
| `default` | Core test services | Standard testing |
| `monitoring` | Prometheus monitoring | Performance tracking |
| `reporting` | Test result aggregation | Report generation |

### Docker Compose Examples

```bash
# Individual distribution testing
docker-compose -f docker/docker-compose.enhanced.yml up test-debian
docker-compose -f docker/docker-compose.enhanced.yml up test-ubuntu
docker-compose -f docker/docker-compose.enhanced.yml up test-alpine

# Parallel testing across all distributions
docker-compose -f docker/docker-compose.enhanced.yml up test-orchestrator

# Performance monitoring (with Prometheus)
docker-compose -f docker/docker-compose.enhanced.yml --profile monitoring up

# Test result aggregation
docker-compose -f docker/docker-compose.enhanced.yml --profile reporting up test-aggregator

# Interactive shell access
docker-compose -f docker/docker-compose.enhanced.yml run test-debian bash

# Custom environment variables
TEST_DISTRO=ubuntu LOG_LEVEL=debug docker-compose -f docker/docker-compose.enhanced.yml up test-base

# Build specific service
docker-compose -f docker/docker-compose.enhanced.yml build test-alpine

# Scale testing
docker-compose -f docker/docker-compose.enhanced.yml up --scale test-debian=3
```

### Docker Compose with Environment Files

```bash
# Create environment file
cat > .env << EOF
TEST_DISTRO=alpine
LOG_LEVEL=debug
TEST_SUITE=basic
VERBOSE=true
EOF

# Use environment file
docker-compose -f docker/docker-compose.enhanced.yml --env-file .env up test-base
```

## 🚀 CI/CD Integration

### GitHub Actions Workflow

The `.github/workflows/docker-test.yml` provides automated testing.

#### Workflow Triggers

| Trigger | Scope | Purpose |
|---------|-------|---------|
| **Push** (main/develop) | Priority distributions | Core validation |
| **Pull Request** | Basic test suite | Change validation |
| **Schedule** (daily 2 AM) | All distributions | Comprehensive testing |
| **Manual** (with tags) | Custom scenarios | Targeted testing |

#### Workflow Jobs

| Job | Purpose | Distributions | Duration |
|-----|---------|---------------|----------|
| `build-cli` | Build CLI binary | N/A | 2-3 min |
| `test-distributions` | Matrix testing | 7 distributions | 5-10 min each |
| `test-comprehensive` | Full testing | All distributions | 15-30 min |
| `security-scan` | Vulnerability scanning | Debian, Alpine | 3-5 min |
| `benchmark` | Performance testing | Alpine | 5-10 min |

#### Triggering Specific Workflows

```bash
# Standard push (triggers basic testing)
git push origin feature-branch

# Comprehensive testing
git commit -m "feat: new feature [comprehensive]"
git push origin main

# Performance benchmarking
git commit -m "perf: optimization [benchmark]"
git push origin main

# Security-focused testing
git commit -m "security: hardening [security]"
git push origin main
```

### Local CI Simulation

```bash
# Simulate CI matrix testing locally
for distro in debian ubuntu arch fedora alpine; do
    echo "=== Testing $distro ==="
    ./scripts/cli-docker-test-enhanced.sh build --distro $distro
    ./scripts/cli-docker-test-enhanced.sh test basic --distro $distro
    ./scripts/cli-docker-test-enhanced.sh test config --distro $distro
    ./scripts/cli-docker-test-enhanced.sh test basic --distro $distro
done

# Simulate comprehensive testing
./scripts/cli-docker-test-enhanced.sh test-parallel --timeout 1800

# Simulate security scanning (manual)
docker run --rm -v $(pwd):/workspace aquasec/trivy:latest image devex-test-env:alpine

# Simulate performance benchmarking
./scripts/cli-docker-test-enhanced.sh benchmark --distro alpine --verbose
```

## 🛡️ Performance & Security

### Security Features

#### Container Security

```bash
# Verify non-root execution
docker run --rm devex-test-env:debian whoami
# Output: testuser

# Check sudo configuration (passwordless)
docker run --rm devex-test-env:debian sudo whoami
# Output: root (no password prompt)

# Verify file permissions
docker run --rm devex-test-env:debian ls -la ~/.local/share/devex
# Output: drwxr-xr-x testuser testuser

# Test privilege isolation
docker run --rm devex-test-env:debian cat /etc/shadow
# Output: Permission denied (correct behavior)
```

#### Security Validation

```bash
# Run security-focused tests
./scripts/cli-docker-test-enhanced.sh build --distro alpine
./scripts/cli-docker-test-enhanced.sh test basic --distro alpine
./scripts/cli-docker-test-enhanced.sh test all --distro alpine

# Test distroless variant (ultra-secure)
docker build -f docker/Dockerfile.distroless -t devex-test:distroless .
docker run --rm devex-test:distroless --version

# Test minimal variant
docker build -f docker/Dockerfile.minimal -t devex-test:minimal .
docker run --rm devex-test:minimal /home/testuser/.local/share/devex/bin/devex --version
```

### Performance Optimization

#### Multi-stage Build Benefits

```bash
# Compare image sizes
docker images | grep devex-test

# Example output:
# devex-test-env    debian     150MB    (multi-stage)
# devex-test-env    alpine     80MB     (multi-stage + Alpine)
# devex-test        distroless 25MB     (distroless)
# devex-test        minimal    15MB     (busybox)
```

#### Performance Benchmarking

```bash
# Benchmark across distributions
for distro in alpine debian ubuntu; do
    echo "=== Benchmarking $distro ==="
    time ./scripts/cli-docker-test-enhanced.sh build --distro $distro
    time ./scripts/cli-docker-test-enhanced.sh test basic --distro $distro
done

# Memory usage monitoring
docker stats devex-test-env:alpine --no-stream

# Container startup time
time docker run --rm devex-test-env:alpine /home/testuser/.local/share/devex/bin/devex --version
```

#### Resource Management

```bash
# Set resource limits
docker run --rm --memory=256m --cpus="0.5" \
    devex-test-env:alpine \
    /home/testuser/.local/share/devex/bin/devex --version

# Monitor resource usage
docker run --rm -d --name devex-monitor \
    devex-test-env:debian \
    sleep 300

docker stats devex-monitor --no-stream
docker rm -f devex-monitor
```

## 🔧 Troubleshooting

### Common Issues and Solutions

#### Build Issues

```bash
# Issue: Docker build fails
# Solution: Clean and rebuild
./scripts/cli-docker-test-enhanced.sh clean
docker system prune -f
./scripts/cli-docker-test-enhanced.sh build --distro debian

# Issue: CLI binary not found
# Solution: Build CLI first
cd /path/to/devex/apps/cli
task build
ls -la bin/devex  # Verify binary exists

# Issue: Dockerfile not found
# Solution: Check available distributions
ls docker/Dockerfile.*
./scripts/cli-docker-test-enhanced.sh --help
```

#### Test Issues

```bash
# Issue: Tests timing out
# Solution: Increase timeout
./scripts/cli-docker-test-enhanced.sh test all --distro debian --timeout 600

# Issue: Tests failing
# Solution: Run with verbose output
./scripts/cli-docker-test-enhanced.sh test basic --distro ubuntu --verbose

# Issue: Container health check fails
# Solution: Verify container setup
./scripts/cli-docker-test-enhanced.sh test basic --distro alpine
./scripts/cli-docker-test-enhanced.sh shell --distro alpine
# Inside container: ls -la ~/.local/share/devex
```

#### Permission Issues

```bash
# Issue: Permission denied errors
# Solution: Verify user setup
docker run --rm devex-test-env:debian id
docker run --rm devex-test-env:debian sudo id

# Issue: File ownership problems
# Solution: Check file permissions
docker run --rm devex-test-env:debian ls -la ~/.local/share/devex/bin/devex
docker run --rm devex-test-env:debian test -x ~/.local/share/devex/bin/devex && echo "Executable" || echo "Not executable"
```

### Debug Commands

```bash
# Interactive debugging session
./scripts/cli-docker-test-enhanced.sh shell --distro debian

# Inside container debugging:
# Check DevEx installation
ls -la ~/.local/share/devex/
echo $PATH
which devex

# Test DevEx commands manually
devex --version
devex --help
devex config validate
devex status

# Check system state
id
pwd
env | grep DEVEX
cat ~/.bashrc | grep devex

# Exit container
exit
```

#### Log Analysis

```bash
# Container logs
./scripts/cli-docker-test-enhanced.sh logs --distro ubuntu

# Docker container logs
docker run -d --name test-debug devex-test-env:debian sleep 300
docker logs test-debug
docker rm -f test-debug

# System logs during testing
dmesg | tail -20  # System messages
journalctl -f     # System journal (if available)
```

### Performance Debugging

```bash
# Measure execution time
time ./scripts/cli-docker-test-enhanced.sh test basic --distro alpine

# Memory usage analysis
docker run --rm --memory=512m \
    devex-test-env:debian \
    bash -c "
        /home/testuser/.local/share/devex/bin/devex --version
        cat /proc/meminfo | grep MemAvailable
    "

# CPU usage monitoring
docker run --rm --cpus="1.0" \
    devex-test-env:fedora \
    bash -c "
        time /home/testuser/.local/share/devex/bin/devex config validate
        cat /proc/loadavg
    "
```

## 🎯 Advanced Usage

### Custom Test Scenarios

#### Security Testing

```bash
# Test with minimal privileges
docker run --rm --user 1001:1001 --read-only \
    --tmpfs /tmp --tmpfs /var/tmp \
    devex-test-env:alpine \
    /home/testuser/.local/share/devex/bin/devex --version

# Test with network isolation
docker run --rm --network none \
    devex-test-env:debian \
    /home/testuser/.local/share/devex/bin/devex config validate

# Test with limited resources
docker run --rm --memory=128m --cpus="0.25" \
    devex-test-env:alpine \
    /home/testuser/.local/share/devex/bin/devex status
```

#### Performance Testing

```bash
# Parallel execution across distributions
{
    ./scripts/cli-docker-test-enhanced.sh test basic --distro debian &
    ./scripts/cli-docker-test-enhanced.sh test basic --distro alpine &
    ./scripts/cli-docker-test-enhanced.sh test basic --distro ubuntu &
    wait
    echo "All parallel tests completed"
}

# Stress testing
for i in {1..10}; do
    echo "Iteration $i"
    ./scripts/cli-docker-test-enhanced.sh test basic --distro alpine --timeout 60
done

# Memory leak detection
docker run --rm --memory=256m \
    devex-test-env:debian \
    bash -c "
        for i in {1..100}; do
            /home/testuser/.local/share/devex/bin/devex --version >/dev/null
        done
        cat /proc/meminfo | grep MemAvailable
    "
```

#### Custom Test Suites

```bash
# Create custom test function
run_custom_tests() {
    local distro=$1
    echo "Running custom tests on $distro"
    
    # Custom test sequence
    ./scripts/cli-docker-test-enhanced.sh build --distro $distro
    ./scripts/cli-docker-test-enhanced.sh test basic --distro $distro
    ./scripts/cli-docker-test-enhanced.sh test config --distro $distro
    ./scripts/cli-docker-test-enhanced.sh test basic --distro $distro
    
    echo "Custom tests completed for $distro"
}

# Run custom tests
run_custom_tests debian
run_custom_tests alpine
run_custom_tests ubuntu

# Interactive test development
./scripts/cli-docker-test-enhanced.sh shell --distro fedora
# Inside container:
# Develop and test new CLI features interactively
```

### Integration with External Tools

#### Monitoring Integration

```bash
# Prometheus metrics collection
docker run -d --name prometheus-devex \
    -p 9090:9090 \
    -v $(pwd)/docker/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml \
    prom/prometheus

# Access metrics at http://localhost:9090

# Grafana dashboard
docker run -d --name grafana-devex \
    -p 3000:3000 \
    grafana/grafana

# Access dashboard at http://localhost:3000
```

#### External Test Runners

```bash
# Integration with pytest
python3 -m pytest tests/docker_integration_test.py \
    --distro=debian \
    --test-suite=all \
    --timeout=600

# Integration with bats (Bash testing)
bats tests/docker_cli_tests.bats

# Integration with testinfra
testinfra --hosts=docker://devex-test-env:alpine tests/test_container.py
```

### Development Workflow Integration

#### Pre-commit Hooks

```bash
# Add to .git/hooks/pre-commit
#!/bin/bash
echo "Running DevEx CLI tests..."
cd apps/cli
./scripts/cli-docker-test-enhanced.sh build --distro alpine
./scripts/cli-docker-test-enhanced.sh test basic --distro alpine
echo "Pre-commit tests passed!"
```

#### IDE Integration

```bash
# VS Code tasks.json
{
    "version": "2.0.0",
    "tasks": [
        {
            "label": "DevEx Test Alpine",
            "type": "shell",
            "command": "./scripts/cli-docker-test-enhanced.sh",
            "args": ["test", "basic", "--distro", "alpine"],
            "group": "test",
            "presentation": {
                "echo": true,
                "reveal": "always",
                "focus": false,
                "panel": "shared"
            }
        }
    ]
}
```

#### Makefile Integration

```makefile
# Makefile
.PHONY: docker-test docker-test-all docker-clean

docker-test:
	./scripts/cli-docker-test-enhanced.sh build --distro alpine
	./scripts/cli-docker-test-enhanced.sh test all --distro alpine

docker-test-all:
	./scripts/cli-docker-test-enhanced.sh test-parallel

docker-clean:
	./scripts/cli-docker-test-enhanced.sh clean

docker-benchmark:
	./scripts/cli-docker-test-enhanced.sh benchmark --distro alpine

docker-shell:
	./scripts/cli-docker-test-enhanced.sh shell --distro debian
```

## 📚 Additional Resources

- **[Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)**
- **[Multi-stage Builds Guide](https://docs.docker.com/develop/dev-best-practices/#use-multi-stage-builds)**
- **[Distroless Images](https://github.com/GoogleContainerTools/distroless)**
- **[Container Security](https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html)**
- **[DevEx CLI Documentation](https://docs.devex.sh/)**

## 🤝 Contributing

When contributing to the Docker testing infrastructure:

1. **Test locally** before submitting changes
2. **Update documentation** for new features
3. **Follow security best practices**
4. **Maintain compatibility** across distributions
5. **Add comprehensive test coverage**

```bash
# Example contribution workflow
git checkout -b feature/new-distro-support
# Add new Dockerfile.newdistro
# Update test-matrix.yaml
# Test locally
./scripts/cli-docker-test-enhanced.sh build --distro newdistro
./scripts/cli-docker-test-enhanced.sh test all --distro newdistro
# Update documentation
# Submit pull request
```

---

**💡 Pro Tip**: Start with Alpine Linux for fastest testing, then validate on your target distributions. Use parallel testing for comprehensive validation and interactive shells for debugging complex issues.

**🔐 Security Note**: Always review container configurations and avoid running containers with elevated privileges unless absolutely necessary. The testing infrastructure is designed with security best practices in mind.
