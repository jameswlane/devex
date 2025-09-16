# DevEx Plugin Architecture

DevEx uses a modular plugin architecture to keep the core CLI lightweight while supporting extensive platform-specific functionality. This document describes the plugin system architecture, development process, and distribution.

## Overview

The plugin system moves OS-specific and desktop environment-specific code out of the main binary into standalone executable plugins that are dynamically downloaded based on platform detection.

### Benefits

- **Lightweight Core**: Main binary is ~5MB instead of ~50MB
- **Platform-Specific**: Only downloads plugins needed for your platform
- **Extensible**: Easy to add new package managers and desktop environments
- **Independent Versioning**: Plugins can be updated independently
- **Offline Support**: Downloaded plugins are cached locally

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   DevEx Core    │    │  Plugin System  │    │ Plugin Registry │
│                 │    │                 │    │  (Vercel API)   │
│ • Platform      │───►│ • Detection     │───►│                 │
│   Detection     │    │ • Download      │    │ • Plugin Index  │
│ • Plugin        │    │ • Management    │    │ • Metadata      │
│   Bootstrap     │    │ • Execution     │    │ • Binaries      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │     Plugins     │
                    │                 │
                    │ Package Mgr:    │
                    │ • apt, dnf      │
                    │ • brew, choco   │
                    │                 │
                    │ Desktop:        │
                    │ • gnome, kde    │
                    │ • macos, win    │
                    └─────────────────┘
```

## Plugin Types

### 1. Package Manager Plugins

Handle installation and management of software packages.

**Examples**: `package-manager-apt`, `package-manager-brew`, `package-manager-choco`

**Commands**: `install`, `remove`, `update`, `search`, `list`

### 2. Desktop Environment Plugins

Configure desktop-specific settings, themes, and extensions.

**Examples**: `desktop-gnome`, `desktop-kde`, `desktop-macos`, `desktop-windows`

**Commands**: `configure`, `set-background`, `install-extensions`, `apply-theme`

### 3. Platform System Plugins

Handle OS-specific system configuration.

**Examples**: `system-linux`, `system-macos`, `system-windows`

**Commands**: `configure-shell`, `setup-environment`, `install-fonts`

## Plugin Discovery and Loading

### 1. Platform Detection

On startup, DevEx detects:
- Operating system (linux, darwin, windows)
- Distribution (ubuntu, fedora, arch, etc.)
- Desktop environment (gnome, kde, xfce, etc.)
- Available package managers

### 2. Plugin Resolution

Based on platform detection, DevEx determines required plugins:

```go
// Linux Ubuntu with GNOME
requiredPlugins := []string{
    "package-manager-apt",
    "package-manager-flatpak",
    "desktop-gnome",
    "system-linux",
    "distro-ubuntu",
}
```

### 3. Download Process

```
1. Check ~/.devex/plugins/ for existing plugins
2. Query registry for missing plugins
3. Download platform-specific binaries
4. Verify checksums
5. Make executable and cache locally
```

### 4. Command Registration

Plugins register their commands with the main CLI:

```bash
# Plugin commands are available as subcommands
devex package-manager-apt install firefox
devex desktop-gnome set-background ~/wallpaper.jpg
devex system-linux configure-shell zsh
```

## Plugin Development

### Creating a New Plugin

1. **Initialize Plugin Structure**:

```bash
mkdir packages/plugins/my-plugin
cd packages/plugins/my-plugin

# Create go.mod
go mod init github.com/jameswlane/devex/packages/plugins/my-plugin

# Add SDK dependency
go mod edit -require github.com/jameswlane/devex/packages/shared/plugin-sdk@latest
go mod edit -replace github.com/jameswlane/devex/packages/shared/plugin-sdk=../../shared/plugin-sdk
```

2. **Implement Plugin Interface**:

```go
package main

import (
    "fmt"
    "os"
    sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

type MyPlugin struct {
    *sdk.BasePlugin
}

func NewMyPlugin() *MyPlugin {
    info := sdk.PluginInfo{
        Name:        "my-plugin",
        Version:     version,
        Description: "My custom plugin",
        Commands: []sdk.PluginCommand{
            {
                Name:        "hello",
                Description: "Say hello",
                Usage:       "Print a greeting message",
            },
        },
    }

    return &MyPlugin{
        BasePlugin: sdk.NewBasePlugin(info),
    }
}

func (p *MyPlugin) Execute(command string, args []string) error {
    switch command {
    case "hello":
        fmt.Println("Hello from my plugin!")
        return nil
    default:
        return fmt.Errorf("unknown command: %s", command)
    }
}

func main() {
    plugin := NewMyPlugin()
    sdk.HandleArgs(plugin, os.Args[1:])
}
```

3. **Add Metadata** (`package.json`):

```json
{
    "name": "@devex/plugin-my-plugin",
    "version": "1.0.0",
    "description": "My custom DevEx plugin",
    "keywords": ["devex", "plugin"],
    "author": "Your Name",
    "license": "Apache-2.0",
    "devex": {
        "plugin": {
            "type": "utility",
            "platforms": ["linux", "darwin", "windows"],
            "dependencies": [],
            "priority": 5
        }
    }
}
```

4. **Add Build Configuration** (`Taskfile.yml`):

```yaml
version: '3'

vars:
  PLUGIN_NAME: devex-plugin-my-plugin

tasks:
  build:
    desc: Build plugin for all platforms
    cmds:
      - mkdir -p dist
      - GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/{{.PLUGIN_NAME}}-linux-amd64 .
      - GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/{{.PLUGIN_NAME}}-linux-arm64 .
      - GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/{{.PLUGIN_NAME}}-darwin-amd64 .
      - GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/{{.PLUGIN_NAME}}-darwin-arm64 .
      - GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/{{.PLUGIN_NAME}}-windows-amd64.exe .

  test:
    desc: Run tests
    cmds:
      - go test -v ./...
```

### Testing Plugins

Use Ginkgo BDD framework for comprehensive testing:

```go
// my_plugin_suite_test.go
package main_test

import (
    "testing"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestMyPlugin(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "My Plugin Suite")
}

// my_plugin_test.go
var _ = Describe("My Plugin", func() {
    Describe("Plugin Information", func() {
        It("should return valid plugin info", func() {
            // Test plugin info command
        })
    })

    Describe("Command Execution", func() {
        It("should execute hello command", func() {
            // Test hello command
        })
    })
})
```

## Plugin Distribution

### Build System Integration

Plugins are built using GoReleaser with the main CLI:

```yaml
# .goreleaser.yml
builds:
  - id: plugin-my-plugin
    main: ./packages/plugins/my-plugin
    binary: devex-plugin-my-plugin
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
```

### Registry Structure

The plugin registry (`https://registry.devex.sh/v1/registry.json`) contains:

```json
{
  "base_url": "https://github.com/jameswlane/devex/releases/download",
  "last_updated": "2025-01-01T12:00:00Z",
  "plugins": {
    "package-manager-apt": {
      "name": "package-manager-apt",
      "version": "1.2.0",
      "description": "APT package manager support",
      "platforms": {
        "linux-amd64": {
          "url": "https://github.com/.../devex-plugin-package-manager-apt_linux_amd64.tar.gz",
          "checksum": "sha256:abc123...",
          "size": 5242880
        }
      },
      "tags": ["package-manager", "apt", "debian", "ubuntu"]
    }
  }
}
```

### Release Process

1. **Automatic Detection**: CI detects plugin changes using Turborepo
2. **Build**: GoReleaser compiles plugins for all platforms
3. **Release**: Binaries are published to GitHub releases
4. **Registry Update**: Script generates new registry.json
5. **Deployment**: Registry deployed to Vercel

## Plugin Management Commands

### User Commands

```bash
# List installed plugins
devex plugin list

# Search available plugins
devex plugin search gnome

# Install specific plugin
devex plugin install desktop-gnome

# Update all plugins
devex plugin update

# Show plugin information
devex plugin info desktop-gnome

# Remove plugin
devex plugin remove desktop-gnome
```

### Development Commands

```bash
# Build plugin locally
cd packages/plugins/my-plugin
task build

# Install for local testing
task install:local

# Run tests
task test

# Lint code
task lint
```

## Plugin SDK Reference

### Core Interfaces

```go
// Plugin - Basic plugin interface
type Plugin interface {
    Info() PluginInfo
    Execute(command string, args []string) error
}

// DesktopPlugin - Desktop environment plugins
type DesktopPlugin interface {
    Plugin
    IsAvailable() bool
    GetDesktopEnvironment() string
}
```

### Helper Functions

```go
// Command utilities
sdk.CommandExists(cmd string) bool
sdk.ExecCommand(useSudo bool, name string, args ...string) error
sdk.RunCommand(name string, args ...string) (string, error)

// File utilities
sdk.FileExists(path string) bool

// System utilities
sdk.RequireSudo() bool
sdk.IsRoot() bool
sdk.GetEnv(key, defaultValue string) string
```

### Plugin Types

```go
// Base plugin for all plugins
type BasePlugin struct {
    info PluginInfo
}

// Package manager plugin with common functionality
type PackageManagerPlugin struct {
    *BasePlugin
    managerCommand string
}
```

## Best Practices

### 1. Error Handling

- Provide helpful error messages with suggestions
- Gracefully handle missing dependencies
- Use proper exit codes

### 2. Platform Compatibility

- Check for required tools before executing
- Provide meaningful error messages when unavailable
- Handle different OS behaviors appropriately

### 3. Security

- Validate all inputs
- Use exec.Command instead of shell execution where possible
- Check file permissions before operations

### 4. Performance

- Cache expensive operations
- Minimize startup time
- Use efficient algorithms for bulk operations

### 5. User Experience

- Provide progress feedback for long operations
- Use consistent command naming
- Include helpful examples in usage text

## Examples

### Package Manager Plugin

See `packages/plugins/package-manager-apt/` for a complete example of a package manager plugin with:
- Installation and removal commands
- Package search and listing
- Security validation
- Comprehensive error handling

### Desktop Environment Plugin

See `packages/plugins/desktop-gnome/` for a complete example of a desktop plugin with:
- Configuration management
- Theme and wallpaper setting
- Extension installation
- Backup and restore functionality

## Troubleshooting

### Plugin Not Found

- Check platform compatibility in plugin metadata
- Verify internet connection for downloads
- Use `--offline` flag to run with cached plugins only

### Plugin Execution Errors

- Ensure required dependencies are installed
- Check plugin permissions (should be executable)
- Use `--verbose` flag for detailed debugging

### Development Issues

- Verify SDK import paths are correct
- Run `task build` to compile plugin locally
- Use `task install:local` for testing
- Check plugin implements all required interface methods

## Future Enhancements

### Planned Features

1. **Plugin Sandboxing**: Restrict plugin file system and network access
2. **Plugin Signing**: Cryptographic verification of plugin authenticity
3. **Hot Reloading**: Update plugins without CLI restart
4. **Plugin Marketplace**: Community plugin discovery and sharing
5. **Configuration Plugins**: Custom YAML schema support

### Contributing

1. Follow the plugin development guide above
2. Include comprehensive Ginkgo tests
3. Document all commands and flags
4. Submit PR with plugin in `packages/plugins/` directory
5. Update this documentation for new plugin types
