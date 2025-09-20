# DevEx Plugin SDK

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![SDK Version](https://img.shields.io/badge/Version-0.1.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Plugin SDK](https://img.shields.io/badge/Plugin-SDK-8A2BE2?logo=go)](https://github.com/jameswlane/devex)

The official Software Development Kit for creating DevEx plugins. Provides common interfaces, utilities, and tools for building consistent, high-quality plugins across the DevEx ecosystem.

## 🚀 Features

- **🔌 Plugin Interface**: Standard plugin contract and lifecycle management
- **⚙️ Configuration System**: Unified configuration loading and validation
- **📊 Logging Framework**: Structured logging with multiple output formats
- **🧪 Testing Utilities**: Mock helpers and test frameworks for plugin development
- **🛡️ Error Handling**: Consistent error types and handling patterns
- **📦 Package Management**: Common utilities for different package managers

## 🏗️ SDK Architecture

### Core Components
```
packages/shared/plugin-sdk/
├── pkg/
│   ├── interfaces/         # Plugin contracts and interfaces
│   ├── config/            # Configuration management
│   ├── logging/           # Logging and telemetry
│   ├── errors/            # Error types and handling
│   ├── testing/           # Testing utilities and mocks
│   ├── utils/             # Common utilities
│   └── types/             # Shared data structures
├── examples/              # Example plugin implementations
├── docs/                  # SDK documentation
└── go.mod                # Module definition
```

### Plugin Interface
```go
// Plugin defines the core plugin contract
type Plugin interface {
    // GetInfo returns plugin metadata
    GetInfo() PluginInfo

    // IsCompatible checks if plugin can run on current system
    IsCompatible() bool

    // Execute runs the plugin with given command and arguments
    Execute(ctx context.Context, command string, args []string) error

    // GetCommands returns list of supported commands
    GetCommands() []Command
}

// PluginInfo contains plugin metadata
type PluginInfo struct {
    Name        string            `json:"name"`
    Version     string            `json:"version"`
    Description string            `json:"description"`
    Author      string            `json:"author"`
    Website     string            `json:"website"`
    License     string            `json:"license"`
    Tags        []string          `json:"tags"`
    Platforms   []Platform        `json:"platforms"`
    Commands    []Command         `json:"commands"`
    Config      ConfigSchema      `json:"config"`
}
```

## 🔧 Plugin Development

### Creating a New Plugin
```go
package main

import (
    "context"
    "github.com/jameswlane/devex/packages/shared/plugin-sdk/pkg/interfaces"
    "github.com/jameswlane/devex/packages/shared/plugin-sdk/pkg/config"
    "github.com/jameswlane/devex/packages/shared/plugin-sdk/pkg/logging"
)

type MyPlugin struct {
    config *config.Config
    logger logging.Logger
}

func (p *MyPlugin) GetInfo() interfaces.PluginInfo {
    return interfaces.PluginInfo{
        Name:        "my-plugin",
        Version:     "1.0.0",
        Description: "Example plugin implementation",
        Author:      "Your Name",
        License:     "Apache-2.0",
        Platforms:   []interfaces.Platform{interfaces.Linux, interfaces.MacOS},
        Commands: []interfaces.Command{
            {
                Name:        "install",
                Description: "Install packages",
                Usage:       "install [packages...]",
            },
        },
    }
}

func (p *MyPlugin) IsCompatible() bool {
    // Check if plugin can run on current system
    return p.checkSystemRequirements()
}

func (p *MyPlugin) Execute(ctx context.Context, command string, args []string) error {
    switch command {
    case "install":
        return p.handleInstall(ctx, args)
    case "remove":
        return p.handleRemove(ctx, args)
    default:
        return interfaces.ErrCommandNotFound
    }
}

func main() {
    plugin := &MyPlugin{
        config: config.Load("my-plugin"),
        logger: logging.New("my-plugin"),
    }

    interfaces.RunPlugin(plugin)
}
```

### Configuration Management
```go
// Load plugin configuration
cfg := config.Load("plugin-name")

// Get configuration values with defaults
port := cfg.GetIntWithDefault("server.port", 8080)
debug := cfg.GetBoolWithDefault("debug", false)
hosts := cfg.GetStringSlice("allowed.hosts")

// Validate configuration
if err := cfg.Validate(schema); err != nil {
    return fmt.Errorf("invalid configuration: %w", err)
}
```

### Logging
```go
// Create structured logger
logger := logging.New("plugin-name")

// Log with context
logger.Info("Starting plugin",
    logging.String("version", "1.0.0"),
    logging.Int("port", 8080),
)

logger.Error("Operation failed",
    logging.Error(err),
    logging.String("operation", "install"),
)

// Debug logging (respects debug configuration)
logger.Debug("Processing package",
    logging.String("package", "example"),
    logging.Duration("elapsed", elapsed),
)
```

### Error Handling
```go
import "github.com/jameswlane/devex/packages/shared/plugin-sdk/pkg/errors"

// Standard error types
var (
    ErrPackageNotFound = errors.New("package not found")
    ErrInvalidConfig   = errors.New("invalid configuration")
    ErrSystemIncompatible = errors.New("system not compatible")
)

// Create contextual errors
func (p *Plugin) installPackage(name string) error {
    if !p.packageExists(name) {
        return errors.Wrapf(ErrPackageNotFound, "package %s", name)
    }

    if err := p.download(name); err != nil {
        return errors.Wrapf(err, "failed to download %s", name)
    }

    return nil
}
```

## 🧪 Testing

### Testing Utilities
```go
import (
    "testing"
    "github.com/jameswlane/devex/packages/shared/plugin-sdk/pkg/testing"
)

func TestPluginInstall(t *testing.T) {
    // Create test plugin instance
    plugin := testing.NewMockPlugin()

    // Set up test expectations
    plugin.ExpectCommand("install").
        WithArgs("example-package").
        Returns(nil)

    // Execute test
    err := plugin.Execute(context.Background(), "install", []string{"example-package"})
    assert.NoError(t, err)

    // Verify expectations
    plugin.AssertExpectations(t)
}

func TestConfigurationLoading(t *testing.T) {
    // Create temporary config for testing
    configPath := testing.CreateTempConfig(t, map[string]interface{}{
        "debug": true,
        "timeout": 30,
    })
    defer os.Remove(configPath)

    // Load and test configuration
    cfg := config.LoadFromPath(configPath)
    assert.True(t, cfg.GetBool("debug"))
    assert.Equal(t, 30, cfg.GetInt("timeout"))
}
```

### Mock Implementations
```go
// Mock plugin for testing
type MockPlugin struct {
    testing.MockPlugin
    commands map[string]func([]string) error
}

// Mock configuration for testing
cfg := testing.NewMockConfig(map[string]interface{}{
    "api_key": "test-key",
    "endpoint": "https://api.example.com",
})
```

## 📚 Plugin Types

### Package Manager Plugins
```go
type PackageManager interface {
    Plugin

    // Package operations
    Install(ctx context.Context, packages []string) error
    Remove(ctx context.Context, packages []string) error
    Update(ctx context.Context) error
    Search(ctx context.Context, query string) ([]Package, error)

    // Repository management
    AddRepository(ctx context.Context, repo Repository) error
    RemoveRepository(ctx context.Context, name string) error
}
```

### Desktop Environment Plugins
```go
type DesktopEnvironment interface {
    Plugin

    // Theme and appearance
    ApplyTheme(ctx context.Context, theme Theme) error
    SetWallpaper(ctx context.Context, path string) error

    // Configuration
    SetSetting(ctx context.Context, key, value string) error
    GetSetting(ctx context.Context, key string) (string, error)
}
```

### System Tool Plugins
```go
type SystemTool interface {
    Plugin

    // Tool-specific operations
    Configure(ctx context.Context, config ToolConfig) error
    GetStatus(ctx context.Context) (ToolStatus, error)

    // Service management
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
}
```

## 🚀 Quick Start

### Plugin Development Template
```bash
# Create new plugin from template
devex create-plugin --name my-plugin --type package-manager

# Directory structure created:
my-plugin/
├── main.go              # Plugin entry point
├── config/             # Default configurations
├── internal/           # Internal implementation
├── go.mod             # Go module
├── go.sum             # Dependencies
├── README.md          # Plugin documentation
└── Taskfile.yml       # Build tasks
```

### Build and Test
```bash
# Build plugin
task build

# Run tests
task test

# Run integration tests
task test:integration

# Install locally for testing
task install

# Package for distribution
task package
```

## 🔧 Configuration Schema

### Plugin Metadata
```yaml
# plugin.yaml
name: "my-plugin"
version: "1.0.0"
description: "Example plugin"
author: "Your Name"
license: "Apache-2.0"
website: "https://example.com"

platforms:
  - linux
  - macos

commands:
  - name: "install"
    description: "Install packages"
    usage: "install [packages...]"
    flags:
      - name: "force"
        description: "Force installation"
        type: "bool"

config_schema:
  type: "object"
  properties:
    debug:
      type: "boolean"
      default: false
    timeout:
      type: "integer"
      default: 30
```

## 📊 Versioning and Compatibility

### SDK Versioning
- **Major**: Breaking API changes
- **Minor**: New features, backward compatible
- **Patch**: Bug fixes and improvements

### Plugin Compatibility Matrix
```yaml
sdk_version: "0.1.0"
compatible_devex_versions:
  - ">=2.0.0"
  - "<3.0.0"

minimum_requirements:
  go_version: "1.24"
  os_support: ["linux", "macos", "windows"]
```

## 🤝 Contributing

### Development Guidelines
1. **Follow Go conventions**: Use standard Go code style
2. **Write tests**: Maintain >80% test coverage
3. **Document APIs**: Include comprehensive documentation
4. **Version carefully**: Follow semantic versioning
5. **Consider backward compatibility**: Avoid breaking changes

### Plugin Submission
1. Implement required interfaces
2. Add comprehensive tests
3. Include documentation and examples
4. Submit plugin for review
5. Maintain plugin after acceptance

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Plugin Documentation](https://docs.devex.sh/plugins)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
