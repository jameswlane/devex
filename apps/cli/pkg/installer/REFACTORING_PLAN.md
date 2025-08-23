# Installer Package Refactoring Plan

## Problem
The `pkg/tui/installer.go` file is 1817 lines long, making it difficult to maintain, test, and understand. It has multiple responsibilities that should be separated.

## Solution
Break down the installer into focused, testable packages with clear responsibilities.

## Completed Packages

### 1. executor package (`pkg/installer/executor/`)
**Responsibility:** Command execution with security validation
- `Executor` interface for command execution abstraction
- `Default` executor with moderate security
- `Secure` executor with strict validation
- Platform-specific process attributes
- Command parsing for shell vs direct execution
- **Tests:** ✅ Complete with 17 passing specs

### 2. theme package (`pkg/installer/theme/`)
**Responsibility:** Theme selection and application
- Theme preference management (global and per-app)
- Theme file copying and application
- Theme validation
- Path expansion and directory creation
- **Tests:** ✅ Complete with 15 passing specs

### 3. script package (`pkg/installer/script/`)
**Responsibility:** Script download, validation, and management
- URL validation against trusted domains
- Secure script downloading with size limits
- Content validation for dangerous patterns
- Temporary file management
- Cleanup utilities
- **Tests:** ✅ Complete with comprehensive coverage

## Remaining Packages to Create

### 4. packagemanager package (`pkg/installer/packagemanager/`)
**Responsibility:** Package manager specific operations
- APT source and GPG key management
- DNF/YUM operations
- Pacman, Zypper, Brew operations
- Package manager update caching
- Dependency resolution

### 5. stream package (`pkg/installer/stream/`)
**Responsibility:** Output streaming and logging
- Terminal output cleaning (ANSI sequences)
- Carriage return handling for progress indicators
- Log level management
- Password prompt detection and handling
- Progress tracking integration

### 6. security package (`pkg/installer/security/`)
**Responsibility:** Security validation helpers
- Input sanitization
- Path validation
- Temporary file security
- SecureString for password handling

## Integration Plan

### Phase 1: Update installer.go
1. Import new packages
2. Replace inline code with package calls
3. Update StreamingInstaller to use new abstractions
4. Maintain backward compatibility

### Phase 2: Simplify installer.go structure
```go
// Simplified StreamingInstaller
type StreamingInstaller struct {
    executor    executor.Executor
    theme       *theme.Manager
    script      *script.Manager
    stream      *stream.Manager
    packageMgr  *packagemanager.Manager
    // ... other fields
}
```

### Phase 3: Testing
1. Ensure all existing tests pass
2. Add integration tests for package interactions
3. Benchmark performance improvements

## Benefits

1. **Improved Maintainability**
   - Each package has a single responsibility
   - Easier to understand and modify
   - Clear interfaces between components

2. **Better Testing**
   - Each package has focused unit tests
   - Mocking is simpler with clear interfaces
   - Higher test coverage possible

3. **Reusability**
   - Packages can be used independently
   - Other parts of the codebase can leverage these utilities
   - Clear API for each functionality

4. **Performance**
   - Smaller compilation units
   - Better code organization for compiler optimizations
   - Easier to identify and fix bottlenecks

## Migration Strategy

1. **Create packages incrementally** (In Progress)
2. **Write comprehensive tests for each package** (In Progress)
3. **Update installer.go to use packages one at a time**
4. **Verify no regression with existing functionality**
5. **Remove old code from installer.go**
6. **Document new package APIs**

## File Size Targets

- Original: `installer.go` - 1817 lines
- Target: `installer.go` - ~500 lines (core orchestration only)
- Each package: 200-400 lines of focused code

## Next Steps

1. Complete remaining packages (packagemanager, stream, security)
2. Update installer.go to use all new packages
3. Run full test suite
4. Performance benchmarking
5. Documentation updates
