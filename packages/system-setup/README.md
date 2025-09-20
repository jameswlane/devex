# DevEx System Setup Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![System](https://img.shields.io/badge/System-Setup-28A745?logo=linux)](https://github.com/jameswlane/devex)

Core system configuration and optimization plugin for DevEx. Provides essential system setup, security hardening, and performance optimization for development environments.

## 🚀 Features

- **⚙️ System Configuration**: Essential system settings and optimizations
- **🛡️ Security Hardening**: Firewall, user permissions, and security policies
- **🚀 Performance Tuning**: Memory management, I/O optimization, and caching
- **🔧 Development Tools**: Core development utilities and dependencies
- **📊 System Monitoring**: Health checks and performance monitoring
- **🔄 Service Management**: System service configuration and management

## 📦 System Components

### Core Configuration
- **User Management**: Developer user setup with appropriate permissions
- **System Limits**: File descriptors, process limits, and memory settings
- **File System**: Mount points, permissions, and directory structure
- **Network**: DNS, hosts file, and network optimization
- **Time & Locale**: Timezone, locale, and system clock configuration

### Security Features
- **Firewall Setup**: UFW, iptables configuration for development
- **SSH Hardening**: Secure SSH configuration and key management
- **User Permissions**: Sudo configuration and group memberships
- **Service Security**: Disable unnecessary services and ports

### Performance Optimization
- **Memory Management**: Swap configuration and memory optimization
- **I/O Performance**: Disk scheduling and file system tuning
- **CPU Scaling**: Power management and performance governors
- **Cache Optimization**: System and application cache tuning

## 🚀 Quick Start

```bash
# Run complete system setup
devex system-setup configure

# Security hardening
devex system-setup security --enable-firewall --harden-ssh

# Performance optimization
devex system-setup performance --optimize-memory --tune-io

# Install development essentials
devex system-setup essentials --build-tools --version-control
```

## 🔧 Configuration

```yaml
# ~/.devex/system-setup.yaml
system:
  user:
    groups: ["docker", "sudo", "wheel"]
    shell: "/bin/zsh"
    
  security:
    firewall: true
    ssh_hardening: true
    fail2ban: true
    
  performance:
    swappiness: 10
    vm_dirty_ratio: 15
    io_scheduler: "mq-deadline"
    
  services:
    enable: ["docker", "ssh"]
    disable: ["cups", "bluetooth"]
```

## 🚀 Platform Support

- **Linux**: Ubuntu, Debian, Fedora, Arch, openSUSE, CentOS
- **macOS**: System preferences and Homebrew integration
- **Container**: Docker and Podman environment setup

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).
