# DevEx Platform Roadmap

This document outlines the development roadmap for expanding DevEx across platforms and features.

## 🎯 **Platform Prioritization & Support Status**

Based on implementation complexity and market share, platforms are prioritized as follows:

**Priority 1:** Debian-based Linux (Ubuntu, Debian, Mint, etc.)
**Priority 2:** Red Hat-based Linux (RHEL, Fedora, CentOS, Rocky, Alma)
**Priority 3:** Arch-based Linux (Arch, Manjaro, EndeavourOS)
**Priority 4:** SUSE-based Linux (openSUSE, SLES)
**Priority 5:** macOS (Homebrew ecosystem)
**Priority 6:** Windows 10/11 (Multiple package managers)

## 🐧 Phase 1: Linux Feature Completion (Current Focus)

### 🚨 Critical Issues (Fix First)

#### Code Quality & Architecture
- [ ] **Shell Command Execution Bug** - `pkg/installers/installers.go:127-134` - Wrong field used in shell command processing
- [ ] **Incomplete Installer Implementations** - Critical functions are stubbed out:
    - `processConfigFiles()` - Not implemented
    - `processThemes()` - Not implemented
    - `validateSystemRequirements()` - Not implemented
    - `backupExistingFiles()` - Not implemented
    - `setupEnvironment()` - Not implemented
    - `cleanupAfterInstall()` - Not implemented
- [ ] **Missing Rollback/Recovery** - No mechanism to undo installations or recover from failures
- [ ] **Missing Dry-Run Implementation** - Many operations don't respect `--dry-run` flag

#### Error Handling & Validation
- [ ] **Improve Error Handling** - Add structured error types, better error messages, graceful failure recovery
- [ ] **Add Configuration Validation** - Validate YAML configs on load, check for circular dependencies, validate installer method compatibility
- [ ] **Add Command Validation** - Validate all shell commands before execution, prevent injection

### High Priority Linux Features

#### Package Manager Support Status

**🟢 Priority 1: Debian-based Linux - READY FOR PRODUCTION**
- [x] **APT (Ubuntu/Debian)** - ✅ **FULLY IMPLEMENTED** (Advanced GPG, repository management, conflict resolution)
- [x] **Flatpak** - ✅ Implemented (Universal Linux fallback)
- [x] **Snap** - ✅ Basic support (Ubuntu integration)

**🟡 Priority 2: Red Hat-based Linux - CRITICAL MISSING INSTALLER**
- [ ] **DNF (Fedora/RHEL/CentOS)** - ❌ **MISSING** - Blocks RHEL family support [#92](https://github.com/jameswlane/devex/issues/92)
- [ ] **YUM (Legacy RHEL/CentOS)** - ❌ Missing
- [ ] **RPM (Manual packages)** - ❌ Missing

**🟡 Priority 3: Arch-based Linux - CRITICAL MISSING INSTALLER**
- [ ] **Pacman (Arch Linux/Manjaro)** - ❌ **MISSING** - Blocks Arch family support [#93](https://github.com/jameswlane/devex/issues/93)
- [ ] **YAY (AUR helper)** - ❌ Missing (for AUR packages)

**🟡 Priority 4: SUSE-based Linux - MISSING INSTALLER**
- [ ] **Zypper (openSUSE/SLES)** - ❌ **MISSING** - Blocks SUSE family support [#94](https://github.com/jameswlane/devex/issues/94)

**🔴 Priority 5: macOS - PARTIAL SUPPORT**
- [x] **Homebrew** - ✅ Basic implementation (needs testing)
- [ ] **Mac App Store (mas)** - ❌ Missing [#55](https://github.com/jameswlane/devex/issues/55)

**🔴 Priority 6: Windows - NO SUPPORT**
- [ ] **Windows Package Manager (winget)** - ❌ Missing [#56](https://github.com/jameswlane/devex/issues/56)
- [ ] **Chocolatey** - ❌ Missing [#57](https://github.com/jameswlane/devex/issues/57)
- [ ] **Scoop** - ❌ Missing [#58](https://github.com/jameswlane/devex/issues/58)

#### Core Installation Features
- [ ] **Theme Processing** - GNOME/KDE theme application [#96](https://github.com/jameswlane/devex/issues/96)
- [ ] **Configuration Files** - Copy and process app configs
- [ ] **Font Installation** - System font management
- [ ] **Shell Setup** - Zsh/Bash configuration with Oh My Zsh
- [ ] **Git Configuration** - Aliases, user settings, SSH keys
- [ ] **Installation State Management** - Track installation progress, handle interruptions, resume capability
- [ ] **Add tldr utility** - Community-driven help pages [#7](https://github.com/jameswlane/devex/issues/7)
- [ ] **Enhance NeoVim configuration** - Additional plugins and config [#8](https://github.com/jameswlane/devex/issues/8)

#### Desktop Environment Support
- [ ] **GNOME Extensions** - Install and configure extensions [#3](https://github.com/jameswlane/devex/issues/3)
- [ ] **GNOME Settings** - Apply gsettings configurations
- [ ] **KDE Support** - Plasma themes and settings
- [ ] **Desktop Files** - Application launcher management
- [ ] **Desktop Environment Auto-Detection** - Enhanced DE detection [#62](https://github.com/jameswlane/devex/issues/62)

#### Essential Commands
- [ ] **Uninstall Command** - Remove apps and dependencies [#97](https://github.com/jameswlane/devex/issues/97)
- [ ] **List Command** - Show installed/available apps (`devex list installed`, `devex list available`) [#98](https://github.com/jameswlane/devex/issues/98)
- [ ] **Status Command** - Check installation status (`devex status --app curl`) [#99](https://github.com/jameswlane/devex/issues/99)
- [ ] **Update Command** - Update package lists and apps (`devex update`, `devex upgrade`)

#### Database & Storage
- [ ] **Database Schema Management** - Implement proper migrations, add version tracking, schema validation

### Distribution Testing Priority
1. **Ubuntu** (20.04, 22.04, 24.04)
2. **Debian** (11, 12)
3. **Fedora** (39, 40)
4. **Arch Linux**
5. **CentOS Stream/RHEL**
6. **openSUSE Tumbleweed/Leap**

---

## 🍎 Phase 2: macOS Support (Priority 5)

**Current Status:** 🟡 Basic Homebrew support implemented, needs testing and Mac App Store integration

### Package Managers Status
- [x] **Homebrew** - ✅ Basic implementation [#53](https://github.com/jameswlane/devex/issues/53) **NEEDS TESTING**
- [ ] **MacPorts** - Alternative package manager (lower priority)
- [ ] **Mac App Store (mas)** - App Store installations [#55](https://github.com/jameswlane/devex/issues/55) **HIGH PRIORITY**

### macOS Implementation Milestones

**Milestone 2.1: Core macOS Package Management**
- [ ] **Test and fix Homebrew installer** - Ensure compatibility with Intel/Apple Silicon Macs
- [ ] **Implement mas (Mac App Store) installer** - Essential for GUI applications
- [ ] **Add Homebrew Cask support** - GUI applications via Homebrew
- [ ] **Handle macOS permissions** - Admin privileges and security

**Milestone 2.2: macOS-Specific Features**

### macOS-Specific Features
- [ ] **System Preferences** - Configure via `defaults` commands [#54](https://github.com/jameswlane/devex/issues/54)
- [ ] **Dock Management** - Add/remove dock items
- [ ] **Launchpad Organization** - App grouping
- [ ] **Spotlight Configuration** - Search preferences
- [ ] **Touch Bar Customization** - For supported devices
- [ ] **Git Installation** - Ensure git is available by default [#6](https://github.com/jameswlane/devex/issues/6)
- [ ] **Sleep Prevention** - Prevent macOS from sleeping during installs [#4](https://github.com/jameswlane/devex/issues/4)

### Development Tools
- [ ] **Xcode Command Line Tools** - Essential dev tools
- [ ] **iTerm2 Configuration** - Terminal setup
- [ ] **Finder Preferences** - Show hidden files, etc.
- [ ] **Desktop & Screensaver** - Theme management

### Implementation Requirements
```yaml
# Example macOS app configuration
macos:
  install_method: "brew"
  install_command: "visual-studio-code"
  brew_cask: true
  post_install:
    - command: "defaults write com.microsoft.VSCode ApplePressAndHoldEnabled -bool false"
```

---

## 🪟 Phase 3: Windows Support (Priority 6)

**Current Status:** 🔴 No Windows support implemented - Requires ground-up development

### Package Managers Status
- [ ] **Windows Package Manager (winget)** - ❌ **MISSING** [#56](https://github.com/jameswlane/devex/issues/56) **HIGHEST PRIORITY**
- [ ] **Chocolatey** - ❌ **MISSING** [#57](https://github.com/jameswlane/devex/issues/57) **HIGH PRIORITY**
- [ ] **Scoop** - ❌ **MISSING** [#58](https://github.com/jameswlane/devex/issues/58) **MEDIUM PRIORITY**

### Windows Implementation Milestones

**Milestone 3.1: Core Windows Package Management**
- [ ] **Implement winget installer** - Microsoft's official package manager (Windows 10 v1809+/Windows 11)
- [ ] **Implement Chocolatey installer** - Community package manager (broader Windows support)
- [ ] **Implement Scoop installer** - User-scope package manager
- [ ] **Handle Windows UAC** - Admin elevation and user-scope installations

**Milestone 3.2: Windows-Specific Features**

### Windows-Specific Features
- [ ] **Registry Modifications** - System preferences [#59](https://github.com/jameswlane/devex/issues/59)
- [ ] **Windows Features** - Enable/disable optional features
- [ ] **PowerShell Configuration** - Profile and modules [#60](https://github.com/jameswlane/devex/issues/60)
- [ ] **Windows Terminal** - Configuration and themes [#61](https://github.com/jameswlane/devex/issues/61)
- [ ] **WSL Setup** - Windows Subsystem for Linux [#66](https://github.com/jameswlane/devex/issues/66)

### Development Environment
- [ ] **Visual Studio Integration** - Extensions and settings
- [ ] **Windows SDK** - Development tools
- [ ] **Docker Desktop** - Containerization
- [ ] **Git for Windows** - Version control
- [ ] **Windows Defender Exclusions** - Development folders

### Implementation Requirements
```yaml
# Example Windows app configuration
windows:
  install_method: "winget"
  install_command: "Microsoft.VisualStudioCode"
  alternatives:
    - install_method: "chocolatey"
      install_command: "vscode"
  post_install:
    - registry_key: "HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced"
      value_name: "ShowFileExtensions"
      value: 1
```

---

## 🚀 Phase 4: Advanced Features (Future)

### 🎯 Configuration Management System (High Priority)
**Timeline: Q1 2025 | Priority: High**

A comprehensive configuration management system that empowers users to create, customize, and manage their own development environment configurations.

#### Core Commands:
- [ ] **`devex init`** - Interactive configuration wizard [#95](https://github.com/jameswlane/devex/issues/95)
    - Guided setup for applications, languages, themes, and system settings
    - Platform detection and smart recommendations
    - Configuration validation and conflict detection
    - Custom configuration directory creation (`~/.devex/`)
    - Template selection (web dev, mobile dev, DevOps, data science, etc.)

- [ ] **`devex add`** - Interactive application/tool addition
    - Category-based addition (development, databases, optional, languages)
    - Smart dependency resolution and conflict checking
    - Custom install methods and commands configuration
    - Integration with existing package managers
    - Validation before saving to configuration

- [ ] **`devex remove`** - Safe configuration removal
    - Dependency checking before removal
    - Automatic backup creation before modification
    - Cascade removal options for related items
    - Undo capability for recent changes

- [ ] **`devex config`** - Configuration management utilities
    - Edit existing configurations (`devex config edit`)
    - View current configuration (`devex config show`)
    - Validate configuration files (`devex config validate`)
    - Import/export configurations (`devex config export`, `devex config import`)
    - Merge configurations from different sources
    - Configuration diff and comparison tools

#### Advanced Features:
- [ ] **Template System** - Pre-built configuration templates
    - Web Development (React, Vue, Angular, Node.js)
    - Mobile Development (React Native, Flutter, iOS, Android)
    - DevOps (Docker, Kubernetes, Terraform, AWS)
    - Data Science (Python, R, Jupyter, MLflow)
    - Game Development (Unity, Unreal, Godot)
    - Custom team templates

- [ ] **Configuration Inheritance** - Layered configuration system
    - Base configurations with overrides
    - Team/organization shared configurations
    - Personal customizations on top of team configs
    - Environment-specific configurations (dev, staging, prod)

- [ ] **Interactive Wizards** - Smart configuration assistance
    - Technology stack detection and recommendations
    - Guided dependency resolution
    - Conflict resolution with user choices
    - Performance impact warnings for heavy installations

#### Technical Implementation:
- Interactive CLI using `bubbletea` or `survey` libraries
- YAML schema validation with comprehensive error messages
- Configuration version control and migration system
- Plugin architecture for custom configuration types
- Integration with git for configuration tracking

#### Benefits:
- **Customization**: Tailor DevEx to specific workflows and preferences
- **Team Collaboration**: Share and maintain consistent team configurations
- **Onboarding**: New team members can adopt team standards instantly
- **Flexibility**: Support for custom tools and non-standard setups
- **Reproducibility**: Consistent environments across different machines

### Security & Safety
- [ ] **Privilege Escalation Control** - Clear separation of operations requiring sudo vs user permissions
- [ ] **Download Verification** - Verify checksums/signatures for downloaded packages
- [ ] **Sandbox Mode** - Run installations in isolated environments first

### Performance & Reliability
- [ ] **Parallel Installation** - Install independent packages concurrently
- [ ] **Installation Caching** - Cache downloaded packages and validation results
- [ ] **Progress Reporting** - Show progress bars and status updates
- [ ] **Retry Logic** - Retry failed operations with exponential backoff

### User Experience Enhancements
- [ ] **Interactive Mode** - Guided app selection with descriptions [#64](https://github.com/jameswlane/devex/issues/64)
- [ ] **Verbose Logging Levels** - Better debug output (`devex install --log-level debug`, `devex install --quiet`)
- [ ] **Profile Management** - Multiple environment profiles (`devex profile create work`, `devex profile switch personal`)
- [ ] **Configuration Import/Export** - Enhanced backup and restore configurations [#63](https://github.com/jameswlane/devex/issues/63)
- [ ] **Container Development Support** - DevContainer and containerized environments [#65](https://github.com/jameswlane/devex/issues/65)

### Cloud Integration
- [ ] **Dotfiles Sync** - GitHub/GitLab dotfiles integration
- [ ] **Config Backup** - Cloud storage for configurations
- [ ] **Team Profiles** - Shared team configurations
- [ ] **Remote Installation** - SSH-based setup

### Enterprise Features
- [ ] **Policy Management** - Corporate environment compliance
- [ ] **Audit Logging** - Track all installations
- [ ] **Approval Workflows** - IT approval integration
- [ ] **Inventory Management** - Asset tracking

### Testing Infrastructure
- [ ] **Integration Tests** - Test actual installations in containers
- [ ] **Configuration Tests** - Validate all config files automatically
- [ ] **Installer Tests** - Mock-test all installer implementations
- [ ] **CLI Tests** - Test all command combinations

---

## 📋 Implementation Strategy

### Phase 1 Milestones (Linux Focus)

**Milestone 1.0: Critical Bug Fixes (Immediate)**
- [ ] Fix shell command execution bug in installers
- [ ] Implement missing installer functions (processConfigFiles, processThemes, etc.)
- [ ] Add proper dry-run support across all operations
- [ ] Implement rollback/recovery mechanisms
- [ ] Add structured error handling

**Milestone 1.1: Priority 2 - Red Hat Linux Support (Blocks 🟡)**
- [ ] **Implement DNF installer** - Core package management for Fedora/RHEL/CentOS/Rocky/Alma
- [ ] **Add YUM fallback** - Legacy RHEL/CentOS 7 support
- [ ] **Implement RPM installer** - Manual .rpm package support
- [ ] **Test on Fedora 40+, RHEL 9, CentOS Stream 9**

**Milestone 1.2: Priority 3 - Arch Linux Support (Blocks 🟡)**
- [ ] **Implement Pacman installer** - Core package management for Arch/Manjaro/EndeavourOS
- [ ] **Add YAY/AUR support** - Access to Arch User Repository
- [ ] **Handle Arch-specific configurations** - Rolling release considerations
- [ ] **Test on Arch Linux, Manjaro, EndeavourOS**

**Milestone 1.3: Priority 4 - SUSE Linux Support (Blocks 🟡)**
- [ ] **Implement Zypper installer** - Core package management for openSUSE/SLES
- [ ] **Add pattern support** - SUSE software patterns
- [ ] **Test on openSUSE Tumbleweed/Leap, SLES**

**Milestone 1.4: Essential Commands (Cross-Platform)**
- [ ] Add uninstall command (`devex uninstall --app curl`)
- [ ] Add list commands (`devex list installed`, `devex list available`)
- [ ] Add status command (`devex status --app curl`)
- [ ] Add update commands (`devex update`, `devex upgrade`)

**Milestone 1.5: Desktop Environment Integration (Linux)**
- [ ] Implement GNOME extension management
- [ ] Add theme processing for GNOME
- [ ] Implement KDE Plasma support
- [ ] Add desktop file management

**Milestone 1.6: Core Feature Completion (Linux)**
- [ ] Implement font installation system
- [ ] Add shell configuration management
- [ ] Implement Git setup automation
- [ ] Add configuration validation system

### Testing Strategy
1. **Unit Tests** - Individual installer components
2. **Integration Tests** - Full installation workflows
3. **Distribution Tests** - Test matrix across Linux distros
4. **Container Tests** - Automated testing in Docker
5. **VM Tests** - Full desktop environment testing

### Documentation Requirements
- [ ] **Installation Guides** - Per-distribution setup
- [ ] **Configuration Examples** - Common use cases
- [ ] **Troubleshooting** - Common issues and solutions
- [ ] **API Documentation** - For developers extending DevEx

---

## 🎯 Success Metrics

### Phase 1 (Linux) Success Criteria
- ✅ Support for top 5 Linux distributions
- ✅ 90%+ success rate for default app installations
- ✅ Complete desktop environment theming
- ✅ Zero-config setup for developers
- ✅ Comprehensive test coverage

### Future Phase Success Criteria
- **Phase 2**: Full macOS Homebrew ecosystem support
- **Phase 3**: Windows development environment automation
- **Phase 4**: Enterprise-ready deployment capabilities

---

## 🤝 Community Contributions

### How to Contribute
1. **Pick a Distribution** - Test DevEx on your preferred Linux distro
2. **Add Package Managers** - Implement missing package manager support
3. **Create Configurations** - Share your app configurations
4. **Report Issues** - Help us improve compatibility
5. **Write Documentation** - Help others get started

### Contribution Priorities
1. **Package Manager Implementations** - DNF, Pacman, Zypper
2. **Desktop Environment Support** - KDE, XFCE, Cinnamon
3. **App Configurations** - Popular development tools
4. **Distribution Testing** - Verify compatibility
5. **Documentation** - Usage guides and examples

---

## ⚡ Quick Wins (Start Here)

These are high-impact, low-effort improvements that can be implemented quickly:

### 🚀 **Immediate Actions by Priority**

**🟢 Priority 1: Keep Debian Linux Production-Ready**
- ✅ **ALREADY COMPLETE** - APT installer fully functional
- ✅ **ALREADY COMPLETE** - Flatpak and Snap support
- 🔧 **Quick fixes** - Address critical bugs in existing functionality

**🟡 Priority 2: Unlock Red Hat Linux (Estimated: 1-2 weeks)**
1. **DNF installer implementation** (8-16 hours) - Core blocker
2. **YUM fallback support** (4-8 hours) - Legacy compatibility
3. **RPM installer** (4-6 hours) - Manual packages
4. **Testing on Fedora/RHEL** (4-8 hours)

**🟡 Priority 3: Unlock Arch Linux (Estimated: 1-2 weeks)**
1. **Pacman installer implementation** (8-16 hours) - Core blocker
2. **YAY/AUR support** (6-12 hours) - Community packages
3. **Arch-specific handling** (2-4 hours) - Rolling release
4. **Testing on Arch/Manjaro** (4-8 hours)

**🟡 Priority 4: Unlock SUSE Linux (Estimated: 1 week)**
1. **Zypper installer implementation** (6-12 hours) - Core blocker
2. **Pattern support** (2-4 hours) - SUSE software patterns
3. **Testing on openSUSE** (2-4 hours)

**🔴 Priority 5: Enable macOS (Estimated: 2-3 weeks)**
1. **Test existing Homebrew installer** (2-4 hours)
2. **Implement mas installer** (8-12 hours) - App Store
3. **macOS-specific features** (16-24 hours)
4. **Testing on Intel/Apple Silicon** (8-16 hours)

**🔴 Priority 6: Enable Windows (Estimated: 3-4 weeks)**
1. **winget installer** (12-20 hours) - Primary package manager
2. **Chocolatey installer** (8-16 hours) - Community favorite
3. **Scoop installer** (6-12 hours) - User-scope
4. **Windows-specific features** (20-30 hours)

### ⚡ **Critical Bug Fixes (All Platforms)**

**5-10 Minute Fixes:**
1. Fix shell command execution bug in `pkg/installers/installers.go:127-134`
2. Add missing directory creation before database initialization
3. Improve help command descriptions and examples

**30-60 Minute Fixes:**
4. Implement proper dry-run support in core installers
5. Add basic uninstall command structure
6. Add configuration validation for YAML files
7. Implement basic list commands (installed/available apps)

**2-4 Hour Features:**
8. Add rollback/recovery mechanism for failed installations
9. Implement structured error types and better error messages
10. Add status command for checking app installation state
11. Complete missing installer functions (processConfigFiles, processThemes, etc.)

**High-Impact Features (1-2 Days):**
12. **Implement basic `devex init` command** - Interactive configuration wizard foundation
13. **Add `devex add` command structure** - Allow users to add custom applications to their config
14. **Create configuration templates** - Pre-built configs for common development stacks
15. **Implement configuration validation** - Ensure custom configs are valid before use

---

## 🎯 Success Metrics

### Phase 1 (Linux) Success Criteria
- ✅ Support for top 5 Linux distributions
- ✅ 90%+ success rate for default app installations
- ✅ Complete desktop environment theming
- ✅ Zero-config setup for developers
- ✅ Comprehensive test coverage
- ✅ All critical bugs resolved
- ✅ Robust error handling and recovery

### Future Phase Success Criteria
- **Phase 2**: Full macOS Homebrew ecosystem support
- **Phase 3**: Windows development environment automation
- **Phase 4**: Enterprise-ready deployment capabilities

This roadmap ensures DevEx becomes the definitive development environment automation tool across all major platforms!
