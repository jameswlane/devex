# GitHub Issues for Future Platform Support

This document contains issue templates for tracking macOS and Windows support. Copy these to GitHub Issues when ready to work on them.

---

## 🍎 macOS Support Issues

### Issue: Add Homebrew Package Manager Support
**Labels:** `enhancement`, `macos`, `package-manager`
**Priority:** High
**Milestone:** macOS Support

**Description:**
Implement Homebrew package manager support for macOS applications.

**Requirements:**
- [ ] Create `pkg/installers/brew/` installer (expand existing)
- [ ] Support both `brew install` and `brew install --cask`
- [ ] Handle Homebrew taps (custom repositories)
- [ ] Add tap management for additional repositories
- [ ] Support for Mac App Store via `mas` integration

**Example Configuration:**
```yaml
macos:
  install_method: "brew"
  install_command: "visual-studio-code"
  brew_cask: true
  brew_tap: "homebrew/cask-fonts"
```

**Acceptance Criteria:**
- [ ] Homebrew packages install successfully
- [ ] Cask applications install correctly
- [ ] Custom taps are added automatically
- [ ] Error handling for missing Homebrew
- [ ] Integration tests pass on macOS

---

### Issue: Add macOS System Preferences Configuration
**Labels:** `enhancement`, `macos`, `system-config`
**Priority:** Medium
**Milestone:** macOS Support

**Description:**
Enable configuration of macOS system preferences via `defaults` commands.

**Requirements:**
- [ ] Implement `defaults` command execution
- [ ] Support for different value types (bool, string, int, array)
- [ ] Dock configuration management
- [ ] Finder preferences setup
- [ ] System appearance settings

**Example Configuration:**
```yaml
macos:
  system_preferences:
    - domain: "com.apple.dock"
      key: "autohide"
      value: true
    - domain: "NSGlobalDomain"
      key: "AppleInterfaceStyle"
      value: "Dark"
```

---

### Issue: Add Mac App Store (mas) Integration
**Labels:** `enhancement`, `macos`, `app-store`
**Priority:** Medium
**Milestone:** macOS Support

**Description:**
Support installing applications from the Mac App Store using `mas` CLI tool.

**Requirements:**
- [ ] Install `mas` as dependency via Homebrew
- [ ] Search and install App Store applications
- [ ] Handle authentication requirements
- [ ] Support for app ID-based installation

---

## 🪟 Windows Support Issues

### Issue: Add Windows Package Manager (winget) Support
**Labels:** `enhancement`, `windows`, `package-manager`
**Priority:** High
**Milestone:** Windows Support

**Description:**
Implement Windows Package Manager (winget) support for Windows 10/11.

**Requirements:**
- [ ] Create `pkg/installers/winget/` installer
- [ ] Support package installation and search
- [ ] Handle Windows Store and community packages
- [ ] Version specification support
- [ ] Silent installation options

**Example Configuration:**
```yaml
windows:
  install_method: "winget"
  install_command: "Microsoft.VisualStudioCode"
  winget_source: "msstore"  # optional
  winget_scope: "machine"   # user or machine
```

---

### Issue: Add Chocolatey Package Manager Support
**Labels:** `enhancement`, `windows`, `package-manager`
**Priority:** Medium
**Milestone:** Windows Support

**Description:**
Add support for Chocolatey as an alternative Windows package manager.

**Requirements:**
- [ ] Create `pkg/installers/chocolatey/` installer
- [ ] Auto-install Chocolatey if missing
- [ ] Support for community packages
- [ ] Handle package parameters and features

---

### Issue: Add Scoop Package Manager Support
**Labels:** `enhancement`, `windows`, `package-manager`
**Priority:** Low
**Milestone:** Windows Support

**Description:**
Add support for Scoop package manager, popular among developers.

**Requirements:**
- [ ] Create `pkg/installers/scoop/` installer
- [ ] Support bucket management
- [ ] Handle portable applications
- [ ] PowerShell execution policies

---

### Issue: Add Windows Registry Configuration
**Labels:** `enhancement`, `windows`, `system-config`
**Priority:** Medium
**Milestone:** Windows Support

**Description:**
Enable Windows system configuration via registry modifications.

**Requirements:**
- [ ] Registry key creation and modification
- [ ] Support for different value types (DWORD, String, Binary)
- [ ] Backup and restore functionality
- [ ] Safe registry operation validation

**Example Configuration:**
```yaml
windows:
  registry_settings:
    - key: "HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced"
      value_name: "ShowFileExtensions"
      value: 1
      type: "DWORD"
```

---

### Issue: Add PowerShell Profile Configuration
**Labels:** `enhancement`, `windows`, `shell-config`
**Priority:** Medium
**Milestone:** Windows Support

**Description:**
Configure PowerShell profiles and modules for development.

**Requirements:**
- [ ] PowerShell profile creation and management
- [ ] Module installation (PSReadLine, etc.)
- [ ] Execution policy configuration
- [ ] Theme and prompt customization

---

### Issue: Add Windows Terminal Configuration
**Labels:** `enhancement`, `windows`, `terminal`
**Priority:** Low
**Milestone:** Windows Support

**Description:**
Configure Windows Terminal settings, themes, and profiles.

**Requirements:**
- [ ] Settings.json file management
- [ ] Theme installation and configuration
- [ ] Profile creation for different shells
- [ ] Font and appearance settings

---

## 🔧 Cross-Platform Enhancement Issues

### Issue: Add Desktop Environment Auto-Detection
**Labels:** `enhancement`, `cross-platform`, `detection`
**Priority:** High
**Milestone:** Desktop Integration

**Description:**
Enhance platform detection to identify desktop environments across Linux distributions.

**Requirements:**
- [ ] Detect GNOME, KDE, XFCE, Cinnamon, etc.
- [ ] Version detection for DE-specific features
- [ ] Fallback strategies for unknown environments
- [ ] Configuration adaptation based on DE

---

### Issue: Add Configuration Import/Export
**Labels:** `enhancement`, `cross-platform`, `config-management`
**Priority:** Medium
**Milestone:** User Experience

**Description:**
Allow users to import/export their DevEx configurations.

**Requirements:**
- [ ] Export current configuration to file
- [ ] Import configuration from file or URL
- [ ] Selective import/export (apps only, themes only, etc.)
- [ ] Configuration validation before import

---

### Issue: Add Interactive Setup Mode
**Labels:** `enhancement`, `cross-platform`, `user-experience`
**Priority:** Low
**Milestone:** User Experience

**Description:**
Provide an interactive setup wizard for new users.

**Requirements:**
- [ ] Platform detection and explanation
- [ ] App category selection interface
- [ ] Theme preview and selection
- [ ] Configuration file generation
- [ ] Installation preview before execution

---

## 📱 Mobile/Container Platform Issues

### Issue: Add Container Development Support
**Labels:** `enhancement`, `containers`, `development`
**Priority:** Low
**Milestone:** Container Support

**Description:**
Support development environment setup within containers.

**Requirements:**
- [ ] Dockerfile generation for development environments
- [ ] Dev container configuration
- [ ] Volume mounting for persistent configurations
- [ ] Multi-stage build optimization

---

### Issue: Add WSL (Windows Subsystem for Linux) Support
**Labels:** `enhancement`, `windows`, `wsl`
**Priority:** Medium
**Milestone:** Windows Support

**Description:**
Special handling for WSL environments on Windows.

**Requirements:**
- [ ] WSL detection and version identification
- [ ] Windows-Linux interop configuration
- [ ] File system permission handling
- [ ] WSL-specific optimizations

---

## 🎯 How to Use These Issues

1. **Copy to GitHub Issues** when ready to implement
2. **Add appropriate labels** and milestones
3. **Assign to team members** or leave open for contributors
4. **Reference in pull requests** when implementing
5. **Update with implementation details** as work progresses

This ensures all future platform work is properly tracked and organized!
