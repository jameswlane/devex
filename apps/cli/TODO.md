# DevEx CLI TODO List for 1.0 Release

This document tracks all outstanding TODOs, improvements, and missing implementations that need to be completed before the 1.0 release.

## 🔧 Core Implementation TODOs

### Shell Management
- [ ] **Integrate with installers.InstallCrossPlatformApp** (`pkg/shell/manager.go:102`)
  - Replace simplified shell installation with proper installer integration
  - Ensure consistent installation patterns across all shell types

### Platform Detection & Configuration
- [ ] **Add platform detection for version and arch** (`pkg/types/types.go:427`)
  - Implement automatic version and architecture detection
  - Enhance cross-platform compatibility
- [ ] **Add actual distribution detection** (`pkg/types/types.go:438`)
  - Replace stub implementation with real distribution detection
  - Support for major Linux distributions, macOS, and Windows

### Configuration System
- [ ] **Implement proper desktop environment config conversion** (`pkg/config/config.go:428`)
  - Add support for GNOME, KDE, XFCE, and other desktop environments
  - Implement proper configuration mapping and validation
- [ ] **Implement auto-fix logic** (`pkg/commands/config.go:577`)
  - Add automatic configuration repair functionality
  - Handle common configuration errors gracefully

### Version Management
- [ ] **Get version from build info** (`pkg/commands/config.go:1631`)
  - Replace hardcoded version with build-time version information
  - Implement proper version tracking across all components
- [ ] **Get version from build info** (`pkg/commands/template_custom_cmd.go:151`)
  - Ensure consistent version handling in template system

### Installation & Package Management
- [ ] **Implement app lookup by name** (`pkg/commands/install.go:129`)
  - Add fuzzy search and name resolution for applications
  - Support for multiple naming conventions and aliases
- [ ] **Implement category filtering** (`pkg/commands/install.go:135`)
  - Add robust category-based application filtering
  - Support for nested categories and tags

### Configuration Merging
- [ ] **Implement smart merge logic for complex cases** (`pkg/commands/config.go:2126`)
  - Handle complex configuration merging scenarios
  - Resolve conflicts intelligently

## 🧪 Testing & Quality TODOs

### Test Completions
- [ ] **Add full concurrent test implementation** (`pkg/tui/installer_theme_test.go:338`)
  - Complete concurrent installation testing
  - Ensure thread safety in TUI operations
- [ ] **Convert remaining test functions to Ginkgo format** (`pkg/tui/validation_test.go:198`)
  - Migrate legacy test functions to Ginkgo BDD format
  - Ensure consistent testing patterns
- [ ] **Convert remaining test functions to Ginkgo format** (`pkg/commands/setup_theme_test.go:100`)
  - Complete migration to Ginkgo for setup theme tests

### TUI Error Handling
- [ ] **Handle TUI error - could log or send to error channel** (`pkg/tui/progress_runner.go:44`)
- [ ] **Handle TUI error - could log or send to error channel** (`pkg/tui/progress_runner.go:62`)
- [ ] **Handle TUI error - could log or send to error channel** (`pkg/tui/progress_runner.go:84`)
- [ ] **Handle TUI error - could log or send to error channel** (`pkg/tui/progress_runner.go:108`)
- [ ] **Handle TUI error - could log or send to error channel** (`pkg/tui/progress_runner.go:130`)
  - Implement proper error handling for TUI operations
  - Add error channels or logging for better debugging

## 🔌 Template System TODOs

### Template Installation Methods
- [ ] **Implement Git-based template installation** (`pkg/templates/custom.go:668`)
  - Support for installing templates from Git repositories
  - Handle authentication and versioning
- [ ] **Implement HTTP-based template installation** (`pkg/templates/custom.go:673`)
  - Support for downloading templates from HTTP endpoints
  - Implement validation and security checks
- [ ] **Implement registry-based template installation** (`pkg/templates/custom.go:710`)
  - Create template registry system
  - Support for publishing and discovering templates

## 🔄 Context & Architecture TODOs

### Context Propagation
- [ ] **Update tui.StartInstallation to accept context parameter** (`pkg/commands/install.go:154`)
- [ ] **Update runGuidedSetup to accept context parameter** (`pkg/commands/setup.go:122`)
- [ ] **Update runAutomatedSetup to accept context parameter** (`pkg/commands/setup.go:138`)
  - Implement proper context propagation throughout the application
  - Enable proper cancellation and timeout handling

### Detection Logic
- [ ] **Implement actual detection logic based on system state** (`pkg/commands/list_filters.go:291`)
  - Replace placeholder with real system state detection
  - Improve application status detection accuracy

## 📋 Documentation & Help TODOs

### System Commands
- [ ] **Complete system command implementation** (`pkg/commands/system.go:28`)
  - The system command is currently a placeholder
  - Implement full system configuration and detection functionality

## 🔍 Code Quality Improvements

### Warning Resolution
While not blocking for 1.0, these warnings should be addressed:
- Multiple "Warning: Failed to..." messages throughout the codebase
- Improve error handling to reduce warning frequency
- Add better fallback mechanisms

### Security Enhancements
- Review and test the security command validator patterns
- Ensure all dangerous command patterns are properly blocked
- Add comprehensive security testing

## 📊 Priority Classification

### 🚨 Critical (Must-Have for 1.0)
- Shell management installer integration
- Platform detection implementation  
- Context propagation fixes
- Template installation methods
- Version management from build info

### ⚠️ High Priority
- Configuration auto-fix logic
- TUI error handling improvements
- Test completion and migration
- Desktop environment config conversion

### 📝 Medium Priority  
- Smart config merge logic
- App lookup and category filtering improvements
- System command completion

### 🔧 Nice-to-Have
- Enhanced detection logic
- Warning reduction
- Additional security enhancements

## 🎯 Implementation Strategy

1. **Phase 1**: Complete critical TODOs (shell, platform, context)
2. **Phase 2**: Implement high-priority items (config, TUI, tests)
3. **Phase 3**: Address medium-priority improvements
4. **Phase 4**: Polish with nice-to-have features

## 📈 Progress Tracking

- **Total TODOs**: 26
- **Critical**: 8
- **High Priority**: 8  
- **Medium Priority**: 6
- **Nice-to-Have**: 4

---

*Last updated: 2025-08-21*  
*Auto-generated from codebase analysis*

## 🤝 Contributing

When working on these TODOs:
1. Update this document when completing items
2. Add comprehensive tests for new implementations
3. Follow the existing code patterns and conventions
4. Update related documentation
5. Ensure backward compatibility where possible

Each TODO item includes the file location and line number for easy navigation.
