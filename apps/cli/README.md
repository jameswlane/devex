# DevEx CLI

[![Go Version](https://img.shields.io/github/go-mod/go-version/jameswlane/devex/apps/cli)](https://golang.org/)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/jameswlane/devex/ci.yml?branch=main)](https://github.com/jameswlane/devex/actions)

Enterprise-grade CLI tool for streamlined development environment setup and management across Linux, macOS, and Windows. Built with modern Go patterns using Cobra + Viper architecture.

## 🚀 Features

- **🔌 Plugin Architecture**: 36 specialized plugins for package managers and desktop environments
- **⚡ Cross-Platform**: Native support for Linux distributions, macOS, and Windows
- **🏢 Enterprise Patterns**: Cobra CLI framework with Viper configuration management
- **🎯 Smart Detection**: Automatic platform, distribution, and desktop environment detection
- **📋 12-Factor Config**: Hierarchical configuration (flags > env > config > defaults)
- **🛡️ Quality Gates**: Comprehensive testing with Ginkgo BDD and security scanning

## 📦 Supported Package Managers

### Linux Package Managers
- **Debian/Ubuntu**: apt, deb
- **Fedora/RHEL**: dnf, rpm  
- **Arch Linux**: pacman, yay
- **openSUSE**: zypper
- **Gentoo**: emerge
- **Void Linux**: xbps
- **Solus**: eopkg
- **Alpine**: apk

### Universal Package Managers
- **flatpak**: Cross-distribution app distribution
- **snap**: Ubuntu's universal packages
- **appimage**: Portable application format
- **docker**: Containerized applications
- **pip**: Python package manager
- **mise**: Multi-language version manager

### Cross-Platform
- **brew**: macOS and Linux package manager
- **curlpipe**: Direct download installer

### Nix Ecosystem
- **nixpkgs**: Nix package manager
- **nixflake**: Nix flakes for reproducible builds

## 🖥️ Desktop Environment Support

- **GNOME**: Extensions, themes, and configuration
- **KDE Plasma**: Widgets, themes, and settings
- **XFCE**: Lightweight desktop customization
- **MATE**: Traditional desktop environment
- **Cinnamon**: Modern desktop with classic paradigms
- **LXQt**: Lightweight Qt desktop
- **Budgie**: Modern, elegant desktop
- **Pantheon**: Elementary OS desktop
- **COSMIC**: System76's new Rust-based desktop

## 🏗️ Architecture

### CLI Framework
```
apps/cli/
├── cmd/                 # Cobra command definitions
│   ├── root.go         # Root command with PersistentPreRunE
│   ├── install.go      # Installation commands
│   ├── system.go       # System configuration
│   └── list.go         # List available packages
├── pkg/                # Public packages
│   ├── commands/       # Command implementations
│   ├── installers/     # Package manager interfaces
│   ├── types/          # Core data structures
│   └── config/         # Configuration management
├── internal/           # Private application code
│   ├── cli/           # CLI-specific logic
│   └── config/        # Internal configuration
├── config/            # Default YAML configurations
│   ├── applications.yaml
│   ├── environment.yaml
│   ├── desktop.yaml
│   └── system.yaml
└── Taskfile.yml       # Development automation
```

### Configuration System
DevEx follows 12-Factor App configuration principles:

1. **Command-line flags** (highest precedence)
2. **Environment variables** (`DEVEX_*`)
3. **Configuration files** (`~/.devex/config.yaml`)
4. **Default values** (lowest precedence)

## 🚀 Quick Start

### Installation
```bash
# One-line installation
curl -fsSL https://devex.sh/install | bash

# Or install locally for development
cd apps/cli
task install
```

### Basic Usage
```bash
# Install development environment
devex install

# Install specific categories
devex install --categories development,databases

# List available applications
devex list apps

# Configure desktop environment
devex system apply

# Show current configuration
devex config show
```

## 💻 Development

### Prerequisites
- **Go**: Version 1.24+
- **Task**: Task runner (`go install github.com/go-task/task/v3/cmd/task@latest`)
- **golangci-lint**: Linting (`curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin`)

### Development Workflow
```bash
# Default workflow (lint + test)
task

# Development commands
task build:local    # Build for local testing
task test          # Run all tests
task test:ginkgo   # Run Ginkgo BDD tests
task lint          # Run golangci-lint
task lint:fix      # Auto-fix linting issues
task vulncheck     # Security vulnerability check

# Install locally
task install
```

### Testing Framework
DevEx uses **Ginkgo BDD** exclusively for testing:

```go
var _ = Describe("Install Command", func() {
    var (
        installer *mocks.MockInstaller
        cmd       *cobra.Command
    )
    
    BeforeEach(func() {
        installer = mocks.NewMockInstaller()
        cmd = NewInstallCommand(installer)
    })
    
    Context("when installing applications", func() {
        It("should install default applications", func() {
            err := cmd.Execute()
            Expect(err).ToNot(HaveOccurred())
            Expect(installer.InstallCalls).To(HaveLen(3))
        })
    })
})
```

### Adding New Commands
Use cobra-cli for consistent command generation:

```bash
# Install cobra-cli
go install github.com/spf13/cobra-cli@latest

# Generate new command
cobra-cli add [command]

# Generate child command
cobra-cli add [child] -p [parent]
```

## 🔌 Plugin Development

DevEx uses a modular plugin system with 36 specialized plugins:

### Plugin Interface
```go
type Installer interface {
    Install(ctx context.Context, app types.CrossPlatformApp) error
    Uninstall(ctx context.Context, app types.CrossPlatformApp) error
    IsInstalled(ctx context.Context, app types.CrossPlatformApp) (bool, error)
    GetName() string
    GetPriority() int
    CanInstall(app types.CrossPlatformApp) bool
}
```

### Plugin Commands
```bash
# Check plugin changes
lefthook run plugin-check

# Build all changed plugins
lefthook run plugin-build

# Test specific plugin
lefthook run plugin-test [plugin-name]
```

## 🛡️ Quality & Security

### Quality Gates
- **golangci-lint**: Comprehensive Go linting
- **Ginkgo BDD**: Behavior-driven testing
- **govulncheck**: Security vulnerability scanning
- **lefthook**: Git hooks for quality enforcement

### Security Best Practices
- Context-aware command execution
- Input validation for shell commands
- Minimal but essential security patterns
- Secure configuration file handling

## 📚 Configuration Examples

### Application Configuration (`~/.devex/applications.yaml`)
```yaml
applications:
  - name: "Visual Studio Code"
    description: "Modern code editor"
    categories: ["development", "editors"]
    linux:
      apt:
        package: "code"
        repository: "https://packages.microsoft.com/repos/code"
    macos:
      brew:
        cask: "visual-studio-code"
```

### System Configuration (`~/.devex/system.yaml`)
```yaml
git:
  user_name: "Your Name"
  user_email: "your.email@example.com"
  default_branch: "main"

shell:
  preferred: "zsh"
  plugins: ["oh-my-zsh", "powerlevel10k"]
```

## 🚀 Release Management

DevEx uses automated releases with semantic versioning:

- **GoReleaser**: Multi-platform binary builds
- **GitHub Actions**: Automated testing and deployment
- **Semantic Commits**: Conventional commit-based versioning

### Commit Conventions
```bash
# Feature (minor version)
git commit -m "feat: add new package manager support"

# Bug fix (patch version)
git commit -m "fix: resolve installation issue on Ubuntu"

# Breaking change (major version)
git commit -m "feat!: redesign configuration system"
```

## 📖 Documentation

- **[Main Documentation](https://docs.devex.sh)**: Comprehensive user guides
- **[API Reference](https://pkg.go.dev/github.com/jameswlane/devex/apps/cli)**: Go package documentation
- **[Plugin Development](../../docs/PLUGIN_DEVELOPMENT.md)**: Plugin creation guide
- **[Contributing](../../.github/CONTRIBUTING.md)**: Development guidelines

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Make your changes following our coding standards
4. Run tests: `task test && task lint`
5. Commit with conventional commits: `git commit -m "feat: your feature"`
6. Push and create a Pull Request

## 📄 License

This project is licensed under the [Apache-2.0 License](../../LICENSE).

---

<div align="center">

**[Install DevEx](https://devex.sh)** • **[Documentation](https://docs.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
