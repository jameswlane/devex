# DevEx Shell Tool Plugin

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Shell](https://img.shields.io/badge/Shell-Configuration-4EAA25?logo=gnubash)](https://www.gnu.org/software/bash/)

Shell configuration and management plugin for DevEx. Provides comprehensive setup for Bash, Zsh, Fish, and other shells with framework integration and optimization.

## 🚀 Features

- **🐚 Multi-Shell Support**: Bash, Zsh, Fish, and PowerShell configuration
- **🎨 Framework Integration**: Oh My Zsh, Prezto, Fish frameworks
- **⚡ Performance Optimization**: Fast startup and efficient configurations
- **🔧 Plugin Management**: Shell plugin installation and management
- **🎯 Prompt Customization**: Modern, informative shell prompts
- **🔄 Environment Management**: PATH, aliases, and environment variables

## 📦 Supported Shells

### Shell Types
- **Bash**: Traditional Unix shell with modern enhancements
- **Zsh**: Extended shell with powerful features and plugins  
- **Fish**: User-friendly shell with smart defaults
- **PowerShell**: Cross-platform PowerShell Core
- **Dash**: Lightweight POSIX shell for scripts

### Framework Support
- **Oh My Zsh**: Popular Zsh framework with plugins and themes
- **Prezto**: Fast Zsh configuration framework
- **Oh My Fish**: Fish shell framework
- **Starship**: Cross-shell prompt customization

## 🚀 Quick Start

```bash
# Configure default shell
devex tool shell setup --shell zsh --framework oh-my-zsh

# Install shell plugins
devex tool shell plugins --install "syntax-highlighting,autosuggestions"

# Set up prompt
devex tool shell prompt --theme "starship" --style "pastel"

# Configure aliases
devex tool shell aliases --development --git --system
```

## 🚀 Platform Support

- **Linux**: All distributions
- **macOS**: 10.15+, 11+, 12+, 13+, 14+  
- **Windows**: PowerShell Core, WSL

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).
