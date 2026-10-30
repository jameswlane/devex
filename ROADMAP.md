# DevEx Platform Roadmap

This document outlines the development roadmap for expanding DevEx across platforms and features.

Open items link to their GitHub issues, which hold the current context and definition of done. Finished and obsolete items have been removed; see the issues labeled [`roadmap-review`](https://github.com/jameswlane/devex/issues?q=label%3Aroadmap-review) for the full list.

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
- [ ] **Incomplete Installer Implementations** - Install steps still missing:
    - `processConfigFiles()` - Not implemented ([#97](https://github.com/jameswlane/devex/issues/97))
    - `validateSystemRequirements()` - Not implemented ([#98](https://github.com/jameswlane/devex/issues/98))
- [ ] **Missing Rollback/Recovery** - No mechanism to undo installations or recover from failures ([#99](https://github.com/jameswlane/devex/issues/99))
- [ ] **Missing Dry-Run Implementation** - Many operations don't respect `--dry-run` flag ([#101](https://github.com/jameswlane/devex/issues/101))

#### Error Handling & Validation
- [ ] **Add Configuration Validation** - Validate YAML configs on load, check for circular dependencies, validate installer method compatibility ([#102](https://github.com/jameswlane/devex/issues/102))
- [ ] **Add Command Validation** - Validate all shell commands before execution, prevent injection ([#103](https://github.com/jameswlane/devex/issues/103))

### High Priority Linux Features

#### Package Manager Support Status

**🟢 Priority 1: Debian-based Linux - READY FOR PRODUCTION**
- [x] **Snap** - ✅ Basic support (Ubuntu integration) ([#187](https://github.com/jameswlane/devex/issues/187))

**🟡 Priority 2: Red Hat-based Linux - CRITICAL MISSING INSTALLER**
- [ ] **DNF (Fedora/RHEL/CentOS)** - ❌ **MISSING** - Blocks RHEL family support ([#174](https://github.com/jameswlane/devex/issues/174))
- [ ] **YUM (Legacy RHEL/CentOS)** - ❌ Missing ([#104](https://github.com/jameswlane/devex/issues/104))
- [ ] **RPM (Manual packages)** - ❌ Missing ([#105](https://github.com/jameswlane/devex/issues/105))

**🟡 Priority 3: Arch-based Linux - CRITICAL MISSING INSTALLER**
- [ ] **Pacman (Arch Linux/Manjaro)** - ❌ **MISSING** - Blocks Arch family support ([#106](https://github.com/jameswlane/devex/issues/106))
- [ ] **YAY (AUR helper)** - ❌ Missing (for AUR packages) ([#107](https://github.com/jameswlane/devex/issues/107))

**🟡 Priority 4: SUSE-based Linux - MISSING INSTALLER**
- [ ] **Zypper (openSUSE/SLES)** - ❌ **MISSING** - Blocks SUSE family support ([#108](https://github.com/jameswlane/devex/issues/108))

**🔴 Priority 5: macOS - PARTIAL SUPPORT**
- [x] **Homebrew** - ✅ Basic implementation (needs testing) ([#109](https://github.com/jameswlane/devex/issues/109))
- [ ] **Mac App Store (mas)** - ❌ Missing ([#110](https://github.com/jameswlane/devex/issues/110))

**🔴 Priority 6: Windows - NO SUPPORT**
- [ ] **Windows Package Manager (winget)** - ❌ Missing ([#111](https://github.com/jameswlane/devex/issues/111))
- [ ] **Chocolatey** - ❌ Missing ([#112](https://github.com/jameswlane/devex/issues/112))
- [ ] **Scoop** - ❌ Missing ([#113](https://github.com/jameswlane/devex/issues/113))

#### Core Installation Features
- [ ] **Configuration Files** - Copy and process app configs ([#97](https://github.com/jameswlane/devex/issues/97))
- [ ] **Shell Setup** - Zsh/Bash configuration with Oh My Zsh ([#114](https://github.com/jameswlane/devex/issues/114))
- [ ] **Git Configuration** - Aliases, user settings, SSH keys ([#115](https://github.com/jameswlane/devex/issues/115))
- [ ] **Installation State Management** - Track installation progress, handle interruptions, resume capability ([#116](https://github.com/jameswlane/devex/issues/116))
- [ ] **Enhance NeoVim configuration** - Additional plugins and config ([#175](https://github.com/jameswlane/devex/issues/175))

#### Desktop Environment Support
- [ ] **GNOME Extensions** - Install and configure extensions ([#117](https://github.com/jameswlane/devex/issues/117))

#### Essential Commands
- [ ] **Uninstall Command** - Remove apps and dependencies ([#100](https://github.com/jameswlane/devex/issues/100))
- [ ] **Status Command** - Check installation status (`devex status --app curl`) ([#118](https://github.com/jameswlane/devex/issues/118))
- [ ] **Update Command** - Update package lists and apps (`devex update`, `devex upgrade`) ([#119](https://github.com/jameswlane/devex/issues/119))

#### Database & Storage
- [ ] **Database Schema Management** - Implement proper migrations, add version tracking, schema validation ([#120](https://github.com/jameswlane/devex/issues/120))

### Distribution Testing Priority
1. **Ubuntu** (20.04, 22.04, 24.04)
2. **Debian** (11, 12)
3. **Fedora** (39, 40) ([#121](https://github.com/jameswlane/devex/issues/121))
4. **Arch Linux** ([#122](https://github.com/jameswlane/devex/issues/122))
5. **CentOS Stream/RHEL** ([#121](https://github.com/jameswlane/devex/issues/121))
6. **openSUSE Tumbleweed/Leap** ([#123](https://github.com/jameswlane/devex/issues/123))

---

## 🍎 Phase 2: macOS Support (Priority 5)

**Current Status:** 🟡 Basic Homebrew support implemented, needs testing and Mac App Store integration

### Package Managers Status
- [x] **Homebrew** - ✅ Basic implementation **NEEDS TESTING** ([#109](https://github.com/jameswlane/devex/issues/109))
- [ ] **MacPorts** - Alternative package manager (lower priority) ([#124](https://github.com/jameswlane/devex/issues/124))
- [ ] **Mac App Store (mas)** - App Store installations **HIGH PRIORITY** ([#110](https://github.com/jameswlane/devex/issues/110))

### macOS Implementation Milestones

**Milestone 2.1: Core macOS Package Management**
- [ ] **Test and fix Homebrew installer** - Ensure compatibility with Intel/Apple Silicon Macs ([#109](https://github.com/jameswlane/devex/issues/109))
- [ ] **Implement mas (Mac App Store) installer** - Essential for GUI applications ([#110](https://github.com/jameswlane/devex/issues/110))
- [ ] **Add Homebrew Cask support** - GUI applications via Homebrew ([#109](https://github.com/jameswlane/devex/issues/109))
- [ ] **Handle macOS permissions** - Admin privileges and security ([#126](https://github.com/jameswlane/devex/issues/126))

### macOS-Specific Features
- [ ] **Dock Management** - Add/remove dock items ([#127](https://github.com/jameswlane/devex/issues/127))
- [ ] **Launchpad Organization** - App grouping ([#176](https://github.com/jameswlane/devex/issues/176))
- [ ] **Spotlight Configuration** - Search preferences ([#128](https://github.com/jameswlane/devex/issues/128))
- [ ] **Git Installation** - Ensure git is available by default ([#129](https://github.com/jameswlane/devex/issues/129))
- [ ] **Sleep Prevention** - Prevent macOS from sleeping during installs ([#130](https://github.com/jameswlane/devex/issues/130))

### Development Tools
- [ ] **Xcode Command Line Tools** - Essential dev tools ([#129](https://github.com/jameswlane/devex/issues/129))
- [ ] **iTerm2 Configuration** - Terminal setup ([#131](https://github.com/jameswlane/devex/issues/131))
- [ ] **Desktop & Screensaver** - Theme management ([#132](https://github.com/jameswlane/devex/issues/132))

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
- [ ] **Windows Package Manager (winget)** - ❌ **MISSING** **HIGHEST PRIORITY** ([#111](https://github.com/jameswlane/devex/issues/111))
- [ ] **Chocolatey** - ❌ **MISSING** **HIGH PRIORITY** ([#112](https://github.com/jameswlane/devex/issues/112))
- [ ] **Scoop** - ❌ **MISSING** **MEDIUM PRIORITY** ([#113](https://github.com/jameswlane/devex/issues/113))

### Windows Implementation Milestones

**Milestone 3.1: Core Windows Package Management**
- [ ] **Implement winget installer** - Microsoft's official package manager (Windows 10 v1809+/Windows 11) ([#111](https://github.com/jameswlane/devex/issues/111))
- [ ] **Implement Chocolatey installer** - Community package manager (broader Windows support) ([#112](https://github.com/jameswlane/devex/issues/112))
- [ ] **Implement Scoop installer** - User-scope package manager ([#113](https://github.com/jameswlane/devex/issues/113))
- [ ] **Handle Windows UAC** - Admin elevation and user-scope installations ([#133](https://github.com/jameswlane/devex/issues/133))

### Windows-Specific Features
- [ ] **Windows Features** - Enable/disable optional features ([#134](https://github.com/jameswlane/devex/issues/134))
- [ ] **PowerShell Configuration** - Profile and modules ([#135](https://github.com/jameswlane/devex/issues/135))
- [ ] **Windows Terminal** - Configuration and themes ([#136](https://github.com/jameswlane/devex/issues/136))

### Development Environment
- [ ] **Visual Studio Integration** - Extensions and settings ([#137](https://github.com/jameswlane/devex/issues/137))
- [ ] **Windows SDK** - Development tools ([#137](https://github.com/jameswlane/devex/issues/137))
- [ ] **Docker Desktop** - Containerization ([#138](https://github.com/jameswlane/devex/issues/138))
- [ ] **Git for Windows** - Version control ([#139](https://github.com/jameswlane/devex/issues/139))
- [ ] **Windows Defender Exclusions** - Development folders ([#140](https://github.com/jameswlane/devex/issues/140))

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
- [x] **`devex init`** - Interactive configuration wizard
    - Configuration validation and conflict detection ([#102](https://github.com/jameswlane/devex/issues/102))

- [x] **`devex add`** - Interactive application/tool addition
    - Smart dependency resolution and conflict checking ([#177](https://github.com/jameswlane/devex/issues/177))
    - Custom install methods and commands configuration ([#141](https://github.com/jameswlane/devex/issues/141))
    - Validation before saving to configuration ([#177](https://github.com/jameswlane/devex/issues/177))

- [x] **`devex config`** - Configuration management utilities
    - Validate configuration files (`devex config validate`) ([#102](https://github.com/jameswlane/devex/issues/102))
    - Merge configurations from different sources ([#142](https://github.com/jameswlane/devex/issues/142))

#### Advanced Features:
- [x] **Template System** - Pre-built configuration templates
    - Mobile Development (React Native, Flutter, iOS, Android) ([#143](https://github.com/jameswlane/devex/issues/143))
    - DevOps (Docker, Kubernetes, Terraform, AWS) ([#143](https://github.com/jameswlane/devex/issues/143))
    - Data Science (Python, R, Jupyter, MLflow) ([#143](https://github.com/jameswlane/devex/issues/143))
    - Game Development (Unity, Unreal, Godot) ([#143](https://github.com/jameswlane/devex/issues/143))

- [x] **Interactive Wizards** - Smart configuration assistance
    - Technology stack detection and recommendations ([#144](https://github.com/jameswlane/devex/issues/144))
    - Guided dependency resolution ([#178](https://github.com/jameswlane/devex/issues/178))
    - Conflict resolution with user choices ([#179](https://github.com/jameswlane/devex/issues/179))

#### Technical Implementation:
- YAML schema validation with comprehensive error messages ([#102](https://github.com/jameswlane/devex/issues/102))
- Plugin architecture for custom configuration types ([#180](https://github.com/jameswlane/devex/issues/180))

#### Benefits:
- **Customization**: Tailor DevEx to specific workflows and preferences
- **Team Collaboration**: Share and maintain consistent team configurations
- **Onboarding**: New team members can adopt team standards instantly
- **Flexibility**: Support for custom tools and non-standard setups
- **Reproducibility**: Consistent environments across different machines

### Security & Safety
- [ ] **Privilege Escalation Control** - Clear separation of operations requiring sudo vs user permissions ([#125](https://github.com/jameswlane/devex/issues/125))
- [ ] **Download Verification** - Verify checksums/signatures for downloaded packages ([#145](https://github.com/jameswlane/devex/issues/145))
- [ ] **Sandbox Mode** - Run installations in isolated environments first ([#181](https://github.com/jameswlane/devex/issues/181))

### Performance & Reliability
- [ ] **Parallel Installation** - Install independent packages concurrently ([#146](https://github.com/jameswlane/devex/issues/146))
- [ ] **Installation Caching** - Cache downloaded packages and validation results ([#147](https://github.com/jameswlane/devex/issues/147))
- [ ] **Retry Logic** - Retry failed operations with exponential backoff ([#148](https://github.com/jameswlane/devex/issues/148))

### User Experience Enhancements
- [ ] **Verbose Logging Levels** - Better debug output (`devex install --log-level debug`, `devex install --quiet`) ([#149](https://github.com/jameswlane/devex/issues/149))
- [ ] **Profile Management** - Multiple environment profiles (`devex profile create work`, `devex profile switch personal`) ([#150](https://github.com/jameswlane/devex/issues/150))
- [ ] **Container Development Support** - DevContainer and containerized environments ([#151](https://github.com/jameswlane/devex/issues/151))

### Cloud Integration
- [ ] **Dotfiles Sync** - GitHub/GitLab dotfiles integration ([#152](https://github.com/jameswlane/devex/issues/152))
- [ ] **Config Backup** - Cloud storage for configurations ([#153](https://github.com/jameswlane/devex/issues/153))
- [ ] **Remote Installation** - SSH-based setup ([#154](https://github.com/jameswlane/devex/issues/154))

### Enterprise Features
- [ ] **Policy Management** - Corporate environment compliance ([#182](https://github.com/jameswlane/devex/issues/182))
- [ ] **Audit Logging** - Track all installations ([#155](https://github.com/jameswlane/devex/issues/155))
- [ ] **Approval Workflows** - IT approval integration ([#183](https://github.com/jameswlane/devex/issues/183))
- [ ] **Inventory Management** - Asset tracking ([#184](https://github.com/jameswlane/devex/issues/184))

### Testing Infrastructure
- [ ] **Configuration Tests** - Validate all config files automatically ([#156](https://github.com/jameswlane/devex/issues/156))
- [ ] **Installer Tests** - Mock-test all installer implementations ([#157](https://github.com/jameswlane/devex/issues/157))

---

## 📋 Implementation Strategy

### Phase 1 Milestones (Linux Focus)

**Milestone 1.0: Critical Bug Fixes (Immediate)**
- [ ] Add proper dry-run support across all operations ([#101](https://github.com/jameswlane/devex/issues/101))
- [ ] Implement rollback/recovery mechanisms ([#99](https://github.com/jameswlane/devex/issues/99))

**Milestone 1.1: Priority 2 - Red Hat Linux Support (Blocks 🟡)**
- [ ] **Implement DNF installer** - Core package management for Fedora/RHEL/CentOS/Rocky/Alma ([#174](https://github.com/jameswlane/devex/issues/174))
- [ ] **Add YUM fallback** - Legacy RHEL/CentOS 7 support ([#104](https://github.com/jameswlane/devex/issues/104))
- [ ] **Implement RPM installer** - Manual .rpm package support ([#105](https://github.com/jameswlane/devex/issues/105))
- [ ] **Test on Fedora 40+, RHEL 9, CentOS Stream 9** ([#121](https://github.com/jameswlane/devex/issues/121))

**Milestone 1.2: Priority 3 - Arch Linux Support (Blocks 🟡)**
- [ ] **Implement Pacman installer** - Core package management for Arch/Manjaro/EndeavourOS ([#106](https://github.com/jameswlane/devex/issues/106))
- [ ] **Add YAY/AUR support** - Access to Arch User Repository ([#107](https://github.com/jameswlane/devex/issues/107))
- [ ] **Handle Arch-specific configurations** - Rolling release considerations ([#185](https://github.com/jameswlane/devex/issues/185))
- [ ] **Test on Arch Linux, Manjaro, EndeavourOS** ([#122](https://github.com/jameswlane/devex/issues/122))

**Milestone 1.3: Priority 4 - SUSE Linux Support (Blocks 🟡)**
- [ ] **Implement Zypper installer** - Core package management for openSUSE/SLES ([#108](https://github.com/jameswlane/devex/issues/108))
- [ ] **Add pattern support** - SUSE software patterns ([#108](https://github.com/jameswlane/devex/issues/108))
- [ ] **Test on openSUSE Tumbleweed/Leap, SLES** ([#123](https://github.com/jameswlane/devex/issues/123))

**Milestone 1.4: Essential Commands (Cross-Platform)**
- [ ] Add uninstall command (`devex uninstall --app curl`) ([#100](https://github.com/jameswlane/devex/issues/100))
- [ ] Add status command (`devex status --app curl`) ([#118](https://github.com/jameswlane/devex/issues/118))
- [ ] Add update commands (`devex update`, `devex upgrade`) ([#119](https://github.com/jameswlane/devex/issues/119))

**Milestone 1.5: Desktop Environment Integration (Linux)**
- [ ] Implement GNOME extension management ([#117](https://github.com/jameswlane/devex/issues/117))

**Milestone 1.6: Core Feature Completion (Linux)**
- [ ] Add shell configuration management ([#114](https://github.com/jameswlane/devex/issues/114))
- [ ] Implement Git setup automation ([#115](https://github.com/jameswlane/devex/issues/115))
- [ ] Add configuration validation system ([#102](https://github.com/jameswlane/devex/issues/102))

### Testing Strategy
1. **Unit Tests** - Individual installer components ([#157](https://github.com/jameswlane/devex/issues/157))
2. **Distribution Tests** - Test matrix across Linux distros
3. **VM Tests** - Full desktop environment testing ([#186](https://github.com/jameswlane/devex/issues/186))

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
- 🔧 **Quick fixes** - Address critical bugs in existing functionality

**🟡 Priority 2: Unlock Red Hat Linux (Estimated: 1-2 weeks)**
1. **DNF installer implementation** (8-16 hours) - Core blocker ([#174](https://github.com/jameswlane/devex/issues/174))
2. **YUM fallback support** (4-8 hours) - Legacy compatibility ([#104](https://github.com/jameswlane/devex/issues/104))
3. **RPM installer** (4-6 hours) - Manual packages ([#105](https://github.com/jameswlane/devex/issues/105))
4. **Testing on Fedora/RHEL** (4-8 hours) ([#121](https://github.com/jameswlane/devex/issues/121))

**🟡 Priority 3: Unlock Arch Linux (Estimated: 1-2 weeks)**
1. **Pacman installer implementation** (8-16 hours) - Core blocker ([#106](https://github.com/jameswlane/devex/issues/106))
2. **YAY/AUR support** (6-12 hours) - Community packages ([#107](https://github.com/jameswlane/devex/issues/107))
3. **Arch-specific handling** (2-4 hours) - Rolling release ([#185](https://github.com/jameswlane/devex/issues/185))
4. **Testing on Arch/Manjaro** (4-8 hours) ([#122](https://github.com/jameswlane/devex/issues/122))

**🟡 Priority 4: Unlock SUSE Linux (Estimated: 1 week)**
1. **Zypper installer implementation** (6-12 hours) - Core blocker ([#108](https://github.com/jameswlane/devex/issues/108))
2. **Pattern support** (2-4 hours) - SUSE software patterns ([#108](https://github.com/jameswlane/devex/issues/108))
3. **Testing on openSUSE** (2-4 hours) ([#123](https://github.com/jameswlane/devex/issues/123))

**🔴 Priority 5: Enable macOS (Estimated: 2-3 weeks)**
1. **Test existing Homebrew installer** (2-4 hours) ([#109](https://github.com/jameswlane/devex/issues/109))
2. **Implement mas installer** (8-12 hours) - App Store ([#110](https://github.com/jameswlane/devex/issues/110))
3. **Testing on Intel/Apple Silicon** (8-16 hours) ([#109](https://github.com/jameswlane/devex/issues/109))

**🔴 Priority 6: Enable Windows (Estimated: 3-4 weeks)**
1. **winget installer** (12-20 hours) - Primary package manager ([#111](https://github.com/jameswlane/devex/issues/111))
2. **Chocolatey installer** (8-16 hours) - Community favorite ([#112](https://github.com/jameswlane/devex/issues/112))
3. **Scoop installer** (6-12 hours) - User-scope ([#113](https://github.com/jameswlane/devex/issues/113))

### ⚡ **Critical Bug Fixes (All Platforms)**

**30-60 Minute Fixes:**
1. Implement proper dry-run support in core installers ([#101](https://github.com/jameswlane/devex/issues/101))
2. Add basic uninstall command structure ([#100](https://github.com/jameswlane/devex/issues/100))
3. Add configuration validation for YAML files ([#102](https://github.com/jameswlane/devex/issues/102))

**2-4 Hour Features:**
1. Add rollback/recovery mechanism for failed installations ([#99](https://github.com/jameswlane/devex/issues/99))
2. Add status command for checking app installation state ([#118](https://github.com/jameswlane/devex/issues/118))

**High-Impact Features (1-2 Days):**
1. **Implement configuration validation** - Ensure custom configs are valid before use ([#102](https://github.com/jameswlane/devex/issues/102))

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
