# DevEx Documentation Index

Welcome to the complete documentation for DevEx CLI and its ecosystem. This index helps you find the right documentation for your needs.

## 📚 Quick Navigation

| I want to... | Read this document |
|--------------|-------------------|
| Get started quickly | [README.md](README.md) → [USAGE.md](USAGE.md) |
| Learn the CLI basics | [USAGE.md](USAGE.md) |
| Understand package managers | [docs/PACKAGE_MANAGERS.md](docs/PACKAGE_MANAGERS.md) |
| Configure development tools | [docs/TOOL_PLUGINS.md](docs/TOOL_PLUGINS.md) |
| Fix problems | [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) |
| Use a specific plugin | [Plugin-specific docs](#plugin-documentation) |
| Contribute to the project | [README.md#contributing](README.md#contributing) |

## 📖 Core Documentation

### 1. [README.md](README.md) - Project Overview
- **What it covers**: Project introduction, features, quick start
- **When to read**: First time learning about DevEx
- **Key sections**: Quick start, features, architecture, development setup

### 2. [USAGE.md](USAGE.md) - Complete CLI Guide
- **What it covers**: Comprehensive CLI usage with practical examples
- **When to read**: When you need to understand how to use DevEx effectively
- **Key sections**: 
  - Core commands and workflows
  - Configuration management
  - Template usage
  - Common development scenarios
  - Integration examples

### 3. [docs/PACKAGE_MANAGERS.md](docs/PACKAGE_MANAGERS.md) - Package Manager Plugins
- **What it covers**: Detailed guide for all package manager plugins
- **When to read**: When working with package installation and management
- **Key sections**:
  - APT (Ubuntu/Debian)
  - Docker (containers and images)
  - Pip (Python packages)
  - Flatpak (universal applications)
  - Cross-platform usage patterns

### 4. [docs/TOOL_PLUGINS.md](docs/TOOL_PLUGINS.md) - Development Tool Configuration
- **What it covers**: Configuration tools for development environments
- **When to read**: When setting up Git, shell, or analyzing project stacks
- **Key sections**:
  - Git configuration and aliases
  - Shell setup and switching
  - Stack detection and analysis
  - Team synchronization

### 5. [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) - Problem Resolution
- **What it covers**: Comprehensive troubleshooting guide
- **When to read**: When encountering issues or errors
- **Key sections**:
  - Quick diagnostics
  - Common issues and solutions
  - Debug information collection
  - Emergency recovery procedures

## 🔌 Plugin Documentation

### Package Manager Plugins

| Plugin | Documentation | Platform | Status |
|--------|---------------|----------|--------|
| [APT](packages/package-manager-apt/USAGE.md) | Complete usage guide | Ubuntu/Debian | ✅ Production |
| [Docker](packages/package-manager-docker/USAGE.md) | Container management | Cross-platform | ✅ Production |
| DNF | *Coming soon* | Fedora/RHEL | 🚧 Development |
| Flatpak | *Part of main docs* | Universal Linux | ✅ Production |
| Pip | *Part of main docs* | Cross-platform | ✅ Production |

### Tool Plugins

| Plugin | Documentation | Purpose | Status |
|--------|---------------|---------|--------|
| [Git Tool](packages/tool-git/USAGE.md) | Git configuration guide | Version control setup | ✅ Production |
| Shell Tool | *Part of main docs* | Terminal environment | ✅ Production |
| Stack Detector | *Part of main docs* | Project analysis | ✅ Production |

### Desktop Environment Plugins

Desktop environment plugins are documented in the main [PACKAGE_MANAGERS.md](docs/PACKAGE_MANAGERS.md) document. Individual plugin documentation will be added as these plugins mature.

## 🎯 Usage Scenarios

### For New Users
1. **Start here**: [README.md](README.md) - Get an overview
2. **Then read**: [USAGE.md](USAGE.md) - Learn basic commands
3. **Try examples**: Follow the quick start guides
4. **If problems**: [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)

### For Package Management
1. **Overview**: [docs/PACKAGE_MANAGERS.md](docs/PACKAGE_MANAGERS.md)
2. **Specific manager**: Plugin-specific documentation
3. **Cross-platform**: Configuration examples in main guide
4. **Issues**: Troubleshooting section for package managers

### For Development Environment Setup
1. **Tool configuration**: [docs/TOOL_PLUGINS.md](docs/TOOL_PLUGINS.md)
2. **Git setup**: [packages/tool-git/USAGE.md](packages/tool-git/USAGE.md)
3. **Complete workflows**: [USAGE.md](USAGE.md) - Common workflows section
4. **Team setup**: Template and configuration examples

### For Contributors
1. **Project overview**: [README.md](README.md) - Development section
2. **Architecture**: [CLAUDE.md](CLAUDE.md) - Technical implementation details
3. **Plugin development**: Plugin SDK documentation
4. **Testing**: Examples in existing plugin documentation

## 📋 Documentation Standards

All DevEx documentation follows these standards:

### Structure
- **Table of Contents** for navigation
- **Quick examples** before detailed explanations
- **Practical use cases** with real-world scenarios
- **Troubleshooting sections** for common issues
- **Cross-references** to related documentation

### Code Examples
- **Copy-pasteable** commands and scripts
- **Platform-specific** examples where relevant
- **Error handling** and recovery procedures
- **Best practices** and recommendations

### Coverage
- **Installation and setup** procedures
- **Basic usage** patterns
- **Advanced features** and configuration
- **Integration** with other tools
- **Troubleshooting** and debugging

## 🔄 Documentation Updates

Documentation is updated with:
- **Feature releases**: New functionality and capabilities
- **Bug fixes**: Corrections and clarifications
- **User feedback**: Community suggestions and improvements
- **Platform changes**: New OS and package manager support

### Contributing to Documentation

When contributing to DevEx, please:

1. **Update relevant docs** for code changes
2. **Add examples** for new features
3. **Test instructions** before submitting
4. **Follow existing style** and structure
5. **Cross-reference** related documentation

## 🆘 Getting Help

If you can't find what you need in the documentation:

1. **Search existing docs** using your browser's find function
2. **Check the troubleshooting guide** for common issues
3. **Look at plugin-specific docs** for detailed features
4. **Ask in discussions**: [GitHub Discussions](https://github.com/jameswlane/devex/discussions)
5. **Report doc issues**: [GitHub Issues](https://github.com/jameswlane/devex/issues) with "documentation" label

## 📈 Documentation Roadmap

Planned documentation improvements:

- **Video tutorials** for common workflows
- **Interactive examples** with web-based terminals
- **Plugin development guides** with step-by-step examples
- **Migration guides** from other tools
- **Advanced configuration** patterns and best practices

---

**Happy DevExing!** 🚀

*This documentation index is maintained alongside the project. Last updated with the comprehensive usage examples creation.*
