# Plugin Finalization Plan for Debian 12/13

## Overview
Following the APT plugin architecture as the reference implementation, we need to finalize the following plugins with consistent structure and functionality.

## Plugin Requirements

### 1. package-manager-deb
**Status**: Needs complete rewrite
**Requirements**:
- Handle .deb file installations (local files and URLs)
- Use `dpkg -i` for installation with proper dependency resolution
- Check dependencies with `dpkg-deb -f <file> Depends`
- Install missing dependencies using apt
- Implement proper validation and error handling
- Support for downloading .deb files from URLs

### 2. package-manager-flatpak
**Status**: Needs system install function
**Requirements**:
- All standard package management operations
- `ensure-installed` command to install flatpak on the system if missing
- `add-flathub` command to add Flathub repository
- Repository management (add/remove remotes)
- Application permission management
- Runtime management

### 3. package-manager-docker
**Status**: Needs system install function
**Requirements**:
- Container management operations (run, stop, remove, etc.)
- Image management (pull, push, build, tag)
- `ensure-installed` command to install Docker on the system
- Docker Compose support
- Volume and network management
- System configuration (add user to docker group)

### 4. package-manager-pip
**Status**: Needs enhancement
**Requirements**:
- Python package installation/removal
- Virtual environment support detection
- User vs system installation handling
- Requirements.txt support
- Version constraint handling
- pip upgrade functionality

### 5. package-manager-curlpipe
**Status**: Needs complete implementation
**Requirements**:
- Download and execute installation scripts
- Security validation (checksum, signature verification)
- Trusted source validation
- Support for common patterns (curl | sh, wget | bash)
- Dry-run capability to inspect scripts
- Caching of downloads

### 6. package-manager-appimage
**Status**: Needs path and permission handling
**Requirements**:
- Download AppImages to correct directories:
  - GUI apps: `$HOME/Applications/`
  - CLI tools: `$HOME/.local/bin/` or `$HOME/bin/`
- Make AppImages executable: `chmod +x`
- Desktop integration (optional)
- Update checking
- AppImage management (list, remove, update)

### 7. tool-git
**Status**: Needs review
**Requirements**:
- Git configuration management
- Repository operations
- Credential management
- SSH key setup
- Global gitignore management
- Hooks management

### 8. tool-shell
**Status**: Needs review
**Requirements**:
- Shell detection and configuration
- Profile management (.bashrc, .zshrc, etc.)
- Completion installation
- PATH management
- Alias and function management
- Theme/prompt configuration

### 9. tool-stackdetector
**Status**: Needs review
**Requirements**:
- Detect programming languages and frameworks
- Identify required tools and dependencies
- Generate installation recommendations
- Support for common stacks (Node.js, Python, Go, Rust, etc.)
- Project file analysis (package.json, go.mod, Cargo.toml, etc.)

### 10. package-manager-mise
**Status**: Needs system install function
**Requirements**:
- Language version management
- `ensure-installed` command to install mise on the system
- Plugin management for mise
- Version switching
- Global and local version configuration
- Legacy version file support (.nvmrc, .ruby-version, etc.)

## Common Patterns to Implement

### System Installation Functions
For tools that need to bootstrap themselves (flatpak, docker, mise):
```go
func (p *Plugin) EnsureSystemInstalled() error {
    // Check if tool is installed
    // If not, detect OS and install using appropriate method
    // Verify installation succeeded
}
```

### Path Management for AppImage
```go
func getAppImageInstallPath(appName string, isCliTool bool) string {
    if isCliTool {
        // Check for $HOME/.local/bin, then $HOME/bin
        // Create if necessary
    } else {
        // Use $HOME/Applications
        // Create if necessary
    }
}
```

### Security Validation for CurlPipe
```go
func validateScriptSource(url string) error {
    // Check against trusted sources
    // Verify SSL certificate
    // Check checksums if provided
}
```

## Implementation Priority
1. package-manager-deb (core functionality)
2. package-manager-mise (with system install)
3. package-manager-flatpak (with system install)
4. package-manager-docker (with system install)
5. package-manager-appimage (path handling)
6. package-manager-curlpipe (security critical)
7. package-manager-pip (enhancement)
8. tool-git (review and enhance)
9. tool-shell (review and enhance)
10. tool-stackdetector (review and enhance)

## Testing Requirements
- Each plugin needs comprehensive Ginkgo tests
- Test both success and failure scenarios
- Mock external commands where appropriate
- Validate error messages are helpful
- Test system installation functions in isolated environments
