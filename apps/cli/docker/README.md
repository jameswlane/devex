# DevEx CLI Docker Testing Infrastructure

This directory contains a comprehensive Docker testing framework for the DevEx CLI tool, supporting multiple Linux distributions, security hardening, and CI/CD integration.

## 🚀 Quick Start

```bash
# Build and test on Debian (default)
./scripts/cli-docker-test-enhanced.sh build
./scripts/cli-docker-test-enhanced.sh test all

# Test on specific distribution
./scripts/cli-docker-test-enhanced.sh build --distro alpine
./scripts/cli-docker-test-enhanced.sh test basic --distro alpine

# Test across all distributions in parallel
./scripts/cli-docker-test-enhanced.sh test-parallel

# Interactive debugging
./scripts/cli-docker-test-enhanced.sh shell --distro ubuntu
```

## 🐳 Supported Distributions

| Distribution | Base Image | Package Manager | Priority | CI Enabled |
|-------------|------------|-----------------|----------|------------|
| **Debian** | `debian:12-slim` | `apt` | 1 | ✅ |
| **Ubuntu** | `ubuntu:22.04` | `apt` | 1 | ✅ |
| **Arch Linux** | `archlinux:latest` | `pacman` | 2 | ✅ |
| **Fedora** | `fedora:39` | `dnf` | 2 | ✅ |
| **Alpine** | `alpine:3.19` | `apk` | 2 | ✅ |
| **Red Hat** | `ubi9/ubi:9.3` | `dnf` | 2 | ✅ |
| **openSUSE** | `opensuse/leap:15.5` | `zypper` | 2 | ✅ |
| **CentOS Stream** | `centos:stream9` | `dnf` | 3 | ⚠️ |
| **Manjaro** | `manjarolinux/base` | `pacman` | 3 | ⚠️ |

## 🔧 Enhanced Features

### 🛡️ Security Improvements

- **Passwordless sudo**: Removed hardcoded passwords, using `NOPASSWD` sudoers configuration
- **Non-root execution**: All tests run as `testuser` with minimal privileges
- **Multi-stage builds**: Reduced attack surface and image size
- **Health checks**: Container health validation
- **Minimal base images**: Using slim/minimal variants where available

### 📊 Testing Capabilities

#### Test Suites

- **Basic**: Core CLI functionality (`--version`, `--help`, `help`)
- **Configuration**: Config management (`config`, `init`, validation)
- **Applications**: App management (`install`, `uninstall`, `list`, `status`)
- **System**: System commands (`system`, `detect`, `shell`, `cache`)
- **Templates**: Template management (`template list`, etc.)
- **Recovery**: Recovery operations (`rollback`, `undo`)

#### Test Modes

- **Individual**: Test single commands on specific distributions
- **Suite**: Run complete test suites
- **Parallel**: Test across all distributions simultaneously
- **Interactive**: Manual testing with shell access
- **Benchmark**: Performance testing and metrics
- **Security**: Security validation and vulnerability scanning

### 🚀 Performance Optimizations

#### Standard Dockerfiles
Multi-stage builds with optimized layers:
- Base system setup (dependencies, users)
- DevEx preparation (directories, environment)
- Health checks and monitoring

#### Minimal Images
- **Distroless**: `gcr.io/distroless/static-debian12` - Ultra-secure, minimal attack surface
- **Busybox**: `busybox:1.36-glibc` - Lightweight testing environment

## 📋 Available Scripts

### Enhanced Test Script (`cli-docker-test-enhanced.sh`)

```bash
# Commands
build                    # Build CLI binary and Docker image
test [SUITE]            # Run test suite (basic|config|apps|system|templates|recovery|all)
test-parallel           # Run tests across all distributions in parallel
test-interactive CMD    # Run interactive test with specific command
shell                   # Start interactive shell in container
logs                    # Inspect logs within container
health-check           # Run container health checks
benchmark              # Run performance benchmarks
clean                  # Clean up Docker containers and images

# Options
--distro DISTRO        # Target distribution
--parallel             # Enable parallel testing
--timeout SECONDS      # Test timeout in seconds
--verbose              # Enable verbose output
--test-suite SUITE     # Specify test suite to run
```

### Docker Compose Support

```bash
# Standard testing
docker-compose -f docker-compose.enhanced.yml up test-debian

# Parallel testing across all distributions
docker-compose -f docker-compose.enhanced.yml up test-orchestrator

# Performance monitoring
docker-compose -f docker-compose.enhanced.yml --profile monitoring up

# Test result aggregation
docker-compose -f docker-compose.enhanced.yml --profile reporting up test-aggregator
```

## 🔬 Test Matrix Configuration

The `test-matrix.yaml` file defines comprehensive testing scenarios:

```yaml
# Example test scenario
test_scenarios:
  smoke_test:
    name: "Smoke Test"
    distributions: ["debian", "ubuntu"]
    test_suites: ["basic", "configuration"]
    timeout: 300
    
  comprehensive_test:
    name: "Comprehensive Test"
    distributions: "all"
    test_suites: "all"
    timeout: 1800
```

## 🔄 CI/CD Integration

### GitHub Actions

The `.github/workflows/docker-test.yml` provides:

- **Matrix testing** across multiple distributions
- **Parallel execution** for faster feedback
- **Security scanning** with Trivy
- **Performance benchmarking**
- **Artifact management** for test results
- **Container registry** integration

### Pipeline Triggers

- **Push/PR**: Core distributions (Debian, Ubuntu, Alpine)
- **Schedule**: Daily comprehensive testing across all distributions
- **Manual**: Benchmark and security testing with special commit messages

## 🛠️ Development Workflow

### 1. Local Testing

```bash
# Quick validation
./scripts/cli-docker-test-enhanced.sh build --distro debian
./scripts/cli-docker-test-enhanced.sh test basic --distro debian

# Comprehensive local testing
./scripts/cli-docker-test-enhanced.sh test-parallel
```

### 2. Debugging

```bash
# Interactive shell for debugging
./scripts/cli-docker-test-enhanced.sh shell --distro alpine

# Inside container
devex --version
devex config validate
devex status
```

### 3. Performance Analysis

```bash
# Run benchmarks
./scripts/cli-docker-test-enhanced.sh benchmark --distro alpine

# Monitor resource usage
docker stats devex-test-alpine
```

## 📈 Performance Metrics

The testing infrastructure tracks:

- **Execution time** per command and test suite
- **Memory usage** during CLI operations
- **Container startup time**
- **Success rates** across distributions
- **Test coverage** metrics

## 🔐 Security Features

### Container Security
- Non-root user execution
- Minimal package installations
- No hardcoded passwords
- Secure defaults for file permissions
- Isolated container environments

### Vulnerability Scanning
- Trivy integration for image scanning
- Regular security updates via base image updates
- SARIF reporting for security findings

### Best Practices
- Multi-stage builds to reduce attack surface
- Distroless options for production-like testing
- Health checks for container validation
- Resource limits and constraints

## 🚨 Troubleshooting

### Common Issues

1. **Build Failures**
   ```bash
   # Check Docker daemon
   docker info
   
   # Rebuild from scratch
   ./scripts/cli-docker-test-enhanced.sh clean
   ./scripts/cli-docker-test-enhanced.sh build --distro debian
   ```

2. **Test Failures**
   ```bash
   # Run with verbose output
   ./scripts/cli-docker-test-enhanced.sh test basic --distro debian --verbose
   
   # Check container logs
   ./scripts/cli-docker-test-enhanced.sh logs --distro debian
   ```

3. **Permission Issues**
   ```bash
   # Verify user setup
   docker run --rm devex-test-env:debian whoami
   docker run --rm devex-test-env:debian ls -la ~/.local/share/devex
   ```

### Debug Commands

```bash
# Container health check
./scripts/cli-docker-test-enhanced.sh health-check --distro ubuntu

# Interactive debugging
./scripts/cli-docker-test-enhanced.sh shell --distro arch

# Performance analysis
./scripts/cli-docker-test-enhanced.sh benchmark --distro alpine
```

## 📚 Additional Resources

- [DevEx CLI Documentation](https://docs.devex.sh/)
- [Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [Multi-stage Builds](https://docs.docker.com/develop/dev-best-practices/#use-multi-stage-builds)
- [Distroless Images](https://github.com/GoogleContainerTools/distroless)

## 🤝 Contributing

When adding new distributions or improving the testing infrastructure:

1. **Add Dockerfile**: Create `Dockerfile.{distro}` following the multi-stage pattern
2. **Update Matrix**: Add distribution to `test-matrix.yaml`
3. **Test Locally**: Verify with enhanced test script
4. **Update Documentation**: Add to this README and supported distributions table
5. **CI Integration**: Ensure CI pipeline includes new distribution

## 📄 License

This testing infrastructure is part of the DevEx project and follows the same license terms.
