# DevEx Improvement Roadmap

Based on codebase analysis, here's a prioritized list of improvements, optimizations, and fixes to make DevEx production-ready.

## 🚨 Critical Issues (Fix First)

### 2. **Configuration Loading Issues** - `cmd/main.go:45`
- Uses deprecated `config.LoadConfigs` instead of newer `config.LoadSettings`
- Configuration loading is not aligned with the Settings struct
**Fix:** Update to use `config.LoadSettings(homeDir)`

### 3. **Repository Interface Mismatch** - `pkg/types/types.go:187-195`
Repository interface doesn't match actual usage:
```go
// Current interface expects AppConfig objects, but implementation only handles strings
GetApp(name string) (*AppConfig, error)    // Returns *AppConfig
ListApps() ([]AppConfig, error)           // Returns []AppConfig

// But app_repository.go actually works with strings
GetApp(appName string) (bool, error)      // Returns bool
ListAllApps() ([]string, error)          // Returns []string
```
**Fix:** Align interface with implementation or vice versa.

### 4. **Missing Database Directory Creation** - `cmd/main.go:82`
```go
dbPath := filepath.Join(homeDir, ".devex/datastore.db")
```
**Issue:** `.devex` directory may not exist
**Fix:** Create directory before initializing database.

## ⚠️ High Priority Issues

### 5. **Shell Command Execution Bug** - `pkg/installers/installers.go:127-134`
```go
if cmd.Shell != "" {
    processedCommand := utils.ReplacePlaceholders(cmd.UpdateShellConfig, map[string]string{})
    //                                           ^^^ Wrong field! Should be cmd.Shell
}
```

### 6. **Incomplete Installer Implementations** - `pkg/installers/installers.go:187-226`
Critical functions are stubbed out:
- `processConfigFiles()` - Not implemented
- `processThemes()` - Not implemented
- `validateSystemRequirements()` - Not implemented
- `backupExistingFiles()` - Not implemented
- `setupEnvironment()` - Not implemented
- `cleanupAfterInstall()` - Not implemented

### 7. **System Command Placeholder** - `pkg/commands/system.go`
Current system command only prints user info. Needs actual functionality.

### 8. **Missing Rollback/Recovery**
No mechanism to undo installations or recover from failures.

### 9. **Missing Dry-Run Implementation**
While `--dry-run` flag exists, many operations don't respect it.

## 🔧 Architecture Improvements

### 10. **Add Installation State Management**
Track installation progress, handle interruptions, resume capability.

### 11. **Improve Error Handling**
- Add structured error types
- Better error messages for users
- Graceful failure recovery

### 12. **Add Configuration Validation**
- Validate YAML configs on load
- Check for circular dependencies
- Validate installer method compatibility

### 13. **Add Installer Interface Consistency**
Standardize all installers to follow same pattern:
```go
type Installer interface {
    Install(app AppConfig) error
    Uninstall(app AppConfig) error
    IsInstalled(app AppConfig) (bool, error)
    Validate(app AppConfig) error
}
```

### 14. **Database Schema Management**
- Implement proper migrations
- Add version tracking
- Schema validation

## 🚀 Feature Additions

### 15. **Add Uninstall Command**
```bash
devex uninstall --app curl
devex uninstall --category "System Tools"
```

### 16. **Add List/Status Commands**
```bash
devex list installed
devex list available
devex status --app curl
```

### 17. **Add Update/Upgrade Commands**
```bash
devex update        # Update package lists
devex upgrade       # Upgrade installed packages
```

### 18. **Add Backup/Restore**
```bash
devex backup --file my-setup.tar.gz
devex restore --file my-setup.tar.gz
```

### 19. **Add Profile Management**
```bash
devex profile create work
devex profile switch personal
devex profile list
```

### 20. **Implement Missing Core Features**
- **Theme Management**: Apply desktop/terminal themes
- **GNOME Settings**: Configure desktop environment
- **Fonts Installation**: Install development fonts
- **Git Configuration**: Setup git aliases and settings
- **Shell Configuration**: Configure zsh/bash

## 🛡️ Security & Safety

### 21. **Add Command Validation**
Validate all shell commands before execution, prevent injection.

### 22. **Add Privilege Escalation Control**
Clear separation of operations requiring sudo vs user permissions.

### 23. **Add Download Verification**
Verify checksums/signatures for downloaded packages.

### 24. **Add Sandbox Mode**
Run installations in isolated environments first.

## 📊 Performance & Reliability

### 25. **Add Parallel Installation**
Install independent packages concurrently.

### 26. **Add Installation Caching**
Cache downloaded packages and validation results.

### 27. **Add Progress Reporting**
Show progress bars and status updates.

### 28. **Add Retry Logic**
Retry failed operations with exponential backoff.

## 🧪 Testing Infrastructure

### 29. **Add Integration Tests**
Test actual installations in containers.

### 30. **Add Configuration Tests**
Validate all config files automatically.

### 31. **Add Installer Tests**
Mock-test all installer implementations.

### 32. **Add CLI Tests**
Test all command combinations.

## 📝 User Experience

### 33. **Improve Help System**
- Better command descriptions
- Usage examples
- Troubleshooting guides

### 34. **Add Interactive Mode**
```bash
devex install --interactive
# Shows menu of available apps with descriptions
```

### 35. **Add Configuration Generator**
```bash
devex init
# Generates starter config files
```

### 36. **Add Verbose Logging Levels**
```bash
devex install --log-level debug
devex install --quiet
```

## 🔄 Quick Wins (Start Here)

1. **Fix double command execution** (5 min)
2. **Fix shell command bug** (10 min)
3. **Add directory creation** (10 min)
4. **Align repository interface** (30 min)
5. **Fix configuration loading** (30 min)
6. **Implement dry-run respect** (1 hour)
7. **Add basic uninstall command** (2 hours)
8. **Add list commands** (2 hours)

## 📋 Implementation Priority

**Phase 1 (Stability):**
- Fix all critical issues (#1-4)
- Complete installer implementations (#6)
- Add proper error handling (#11)

**Phase 2 (Core Features):**
- Add uninstall/list commands (#15-16)
- Implement theme/GNOME features (#20)
- Add configuration validation (#12)

**Phase 3 (Advanced Features):**
- Add profile management (#19)
- Implement backup/restore (#18)
- Add parallel installation (#25)

**Phase 4 (Polish):**
- Interactive mode (#34)
- Advanced security features (#21-23)
- Performance optimizations (#25-28)

This roadmap will transform DevEx from a prototype into a robust, production-ready development environment automation tool!
