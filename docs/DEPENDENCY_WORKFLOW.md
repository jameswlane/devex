# Dependency Installation Workflow

This document describes the enhanced dependency checking and installation workflow implemented in DevEx CLI.

## Overview

The dependency workflow automatically detects, validates, and installs platform-specific dependencies before attempting to install applications. This ensures that tools like GPG, curl, and other system utilities are available when needed.

## How It Works

### 1. **Platform Detection**
The system automatically detects:
- Operating System (linux, darwin, windows)
- Distribution (debian, ubuntu, fedora, arch, etc.)
- Architecture (amd64, arm64, etc.)

### 2. **Dependency Resolution**
For each application, the system:
- Checks if the app has `platform_requirements` configured
- Matches current platform against requirement specifications
- Extracts the list of required dependencies

### 3. **Security Validation**
All package names are validated to prevent injection attacks:
- Must start with alphanumeric character
- Can contain: letters, numbers, hyphens, plus signs, dots, underscores
- Maximum length: 255 characters
- No shell metacharacters or dangerous patterns

### 4. **Parallel Dependency Checking**
Dependencies are checked in parallel for performance:
- Multiple `which` commands run concurrently
- Results collected and processed efficiently
- Context cancellation supported

### 5. **Automatic Installation**
Missing dependencies are installed via the platform package manager:
- APT for Debian/Ubuntu systems
- Future support: DNF, Pacman, Homebrew, etc.
- Verification after installation

## Configuration

### Application Configuration Example

```yaml
applications:
  development:
    - name: Eza
      description: 'A modern alternative to ls'
      category: 'System Utilities'
      default: true
      linux:
        install_method: 'apt'
        install_command: 'eza'
        platform_requirements:
          - os: 'debian'
            dependencies:
              - 'curl'
              - 'gnupg'
          - os: 'ubuntu'
            dependencies:
              - 'curl'
              - 'gnupg'
        apt_sources:
          - key_source: 'https://raw.githubusercontent.com/eza-community/eza/main/deb.asc'
            key_name: '/etc/apt/keyrings/gierens.gpg'
            source_repo: 'deb [signed-by=/etc/apt/keyrings/gierens.gpg] http://deb.gierens.de stable main'
            source_name: '/etc/apt/sources.list.d/gierens.list'
            require_dearmor: true
```

### Platform Requirements Structure

```yaml
platform_requirements:
  - os: 'debian'                    # Match against distribution
    dependencies:                   # Required packages
      - 'curl'
      - 'gnupg'
      - 'ca-certificates'
  - os: 'ubuntu'                    # Alternative platform
    dependencies:
      - 'curl'
      - 'gnupg2'                    # Different package name
```

## Supported Platforms

### Currently Supported
- **Debian**: APT package manager
- **Ubuntu**: APT package manager

### Planned Support
- **Fedora/RHEL**: DNF package manager
- **Arch Linux**: Pacman package manager
- **macOS**: Homebrew package manager
- **Windows**: Winget/Chocolatey package managers

## Workflow Sequence

1. **Application Installation Request**
   ```
   devex install eza
   ```

2. **Platform Detection**
   ```
   Detected platform: OS=linux, Distribution=debian, Architecture=amd64
   ```

3. **Dependency Analysis**
   ```
   Found platform requirements: os=debian, dependencies=[curl, gnupg]
   Platform dependencies identified: count=2, dependencies=[curl, gnupg]
   ```

4. **Security Validation**
   ```
   Validating package names for security compliance...
   ✅ curl - valid
   ✅ gnupg - valid
   ```

5. **Parallel Dependency Checking**
   ```
   Missing platform dependency: gnupg
   Platform dependency available: curl
   ```

6. **Automatic Installation**
   ```
   Installing missing platform dependencies: [gnupg]
   Using package manager: apt
   ✅ Successfully installed platform dependencies: [gnupg]
   ```

7. **Main Application Installation**
   ```
   Installing eza using apt...
   ✅ GPG operations now succeed because gnupg is available
   ```

## Error Handling

### Common Scenarios

#### Missing Package Manager
```
Error: package manager apt is not available on this system
```
**Solution**: Ensure you're on a supported platform

#### Invalid Package Names
```
Error: invalid dependency package name: test;rm -rf / (contains invalid characters)
```
**Solution**: Review configuration for malicious or invalid package names

#### Network Issues
```
Error: failed to install platform dependencies [gnupg]: failed to update APT package lists
```
**Solution**: Check network connectivity and repository availability

#### Permission Issues
```
Error: failed to install packages [gnupg]: permission denied
```
**Solution**: Ensure sudo access or run with appropriate privileges

## Troubleshooting

### Debug Mode
Enable verbose logging to see detailed dependency workflow:
```bash
devex install --verbose eza
```

### Manual Dependency Installation
If automatic installation fails, install dependencies manually:
```bash
# Debian/Ubuntu
sudo apt-get update && sudo apt-get install -y curl gnupg

# Then retry DevEx installation
devex install eza
```

### Skipping Dependency Checks
Currently not supported, but you can:
1. Manually install required dependencies
2. Remove `platform_requirements` from configuration (not recommended)

### Platform Detection Issues
Check detected platform:
```bash
devex system info  # Shows detected platform details
```

If platform detection is incorrect:
1. Check `/etc/os-release` file
2. Verify distribution-specific files exist
3. Report as bug if platform should be supported

## Architecture Details

### Key Components

#### DependencyChecker
- **File**: `pkg/utils/dependency_checker.go`
- **Purpose**: Core dependency validation and installation logic
- **Key Methods**:
  - `CheckAndInstallPlatformDependencies()`: Main workflow
  - `checkDependenciesParallel()`: Parallel checking
  - `validatePackageName()`: Security validation

#### PackageManager Interface
- **File**: `pkg/utils/dependency_checker.go`
- **Purpose**: Abstraction for different package managers
- **Implementations**:
  - `APTInstaller`: Debian/Ubuntu support

#### StreamingInstaller Integration
- **File**: `pkg/tui/installer.go`
- **Purpose**: Integration with main installation workflow
- **Method**: `checkAndInstallDependencies()`

### Security Features

1. **Package Name Validation**
   - Regex pattern: `^[a-zA-Z0-9][a-zA-Z0-9\-\+\.\_]*$`
   - Length limits: 1-255 characters
   - Prevents injection attacks

2. **Context Cancellation**
   - All operations respect context cancellation
   - Graceful handling of interruptions

3. **Parallel Safety**
   - Thread-safe dependency checking
   - Proper goroutine management

## Performance Optimizations

### Parallel Dependency Checking
- Multiple dependencies checked simultaneously
- Significant speedup for apps with many dependencies
- Proper context handling for cancellation

### Implemented Optimizations
- **Caching**: In-memory caching of dependency check results with TTL expiration
- **Parallel Checking**: Multiple dependencies checked simultaneously
- **Cache Invalidation**: Automatic cache updates after installations

### Future Optimizations
- **Batch Installation**: Install multiple missing dependencies in single command
- **Smart Ordering**: Install dependencies in optimal order
- **Cross-Session Persistence**: Optional disk-based cache persistence

## Testing

### Running Tests
```bash
# Run all dependency checker tests
ginkgo run --focus="DependencyChecker" ./pkg/utils/

# Run specific test category
ginkgo run --focus="Package Name Validation" ./pkg/utils/
```

### Test Coverage
- ✅ Package name validation (valid/invalid cases)
- ✅ Platform matching logic
- ✅ Context cancellation handling
- ✅ Dry-run mode behavior
- ✅ Error scenarios with enhanced error messages
- ✅ Dependency caching (cache hits/misses, invalidation, eviction)
- ✅ Cache expiration and TTL behavior
- ✅ Custom cache configuration
- ✅ Metrics collection (cache statistics, timing, package counts)
- ✅ Metrics reset and summary functionality

### Adding New Tests
See `pkg/utils/dependency_checker_test.go` for examples.

## Dependency Caching

### Cache Features
- **TTL-Based Expiration**: Default 5-minute cache lifetime
- **LRU Eviction**: Automatic eviction of oldest entries when cache is full
- **Thread-Safe**: Concurrent access with RWMutex protection
- **Configurable**: Custom TTL and cache size via `NewDependencyCheckerWithCache()`
- **Smart Invalidation**: Cache entries removed after successful installations

### Cache Performance
- **Parallel Processing**: Uncached dependencies checked concurrently
- **Early Return**: Fully cached dependency lists return immediately
- **Memory Efficient**: Default limit of 100 cached entries
- **Logging**: Cache hit/miss statistics in verbose mode

### Cache Management
```go
// Default cache (5 minutes TTL, 100 entries)
checker := utils.NewDependencyChecker(packageManager, platform)

// Custom cache settings
checker := utils.NewDependencyCheckerWithCache(packageManager, platform, 10*time.Minute, 200)

// Clear cache manually
checker.ClearCache()

// Invalidate specific entries
checker.InvalidateCacheEntries([]string{"curl", "git"})
```

## Metrics Collection

### Performance Metrics
The dependency system now collects comprehensive metrics for monitoring and optimization:

- **Cache Performance**: Hit/miss rates and cache efficiency
- **Installation Timing**: Time spent installing packages
- **Validation Timing**: Time spent validating dependencies
- **Package Counts**: Total packages installed and checked

### Metrics API
```go
// Get current metrics (thread-safe)
metrics := checker.Metrics.GetMetrics()

// Reset metrics (for testing)
checker.Metrics.Reset()

// Log detailed metrics summary
checker.LogMetricsSummary()
```

### Metrics Structure
```go
type DependencyMetrics struct {
    CacheHits         int64         // Total cache hits
    CacheMisses       int64         // Total cache misses  
    TotalChecks       int64         // Total dependency checks
    InstallTime       time.Duration // Total installation time
    ValidationTime    time.Duration // Total validation time
    LastInstallTime   time.Time     // Last installation timestamp
    PackagesInstalled int64         // Total packages installed
}
```

### Enhanced Error Messages
Error messages now provide consistent context and specific package information:

```go
// Before
"invalid package name: test;rm -rf / (contains invalid characters)"

// After  
"dependency validation failed for package 'test;rm -rf /': contains invalid characters"
```

## Future Enhancements

### Planned Features
1. **More Package Managers**: DNF, Pacman, Homebrew support
2. **Version Requirements**: Specify minimum package versions
3. **Dependency Conflicts**: Handle conflicting package requirements
4. **Rollback Capability**: Undo dependency installations if main install fails
5. **Cross-Session Persistence**: Optional disk-based cache persistence

### Contributing
When adding new package manager support:
1. Implement `PackageManager` interface
2. Add platform detection logic
3. Update `checkAndInstallDependencies()` switch statement
4. Add comprehensive tests
5. Update this documentation

## Related Files

- `apps/cli/pkg/utils/dependency_checker.go` - Core implementation
- `apps/cli/pkg/utils/dependency_checker_test.go` - Test suite
- `apps/cli/pkg/installers/apt/apt.go` - APT package manager
- `apps/cli/pkg/tui/installer.go` - Integration point
- `apps/cli/config/applications.yaml` - Example configurations
