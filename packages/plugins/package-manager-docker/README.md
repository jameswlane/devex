# DevEx Docker Package Manager Plugin

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Container%20Platform-2496ED?logo=docker)](https://www.docker.com/)

Container-based application management plugin for DevEx. Provides isolated development environments, microservices deployment, and containerized application delivery using Docker containers and images.

## 🚀 Features

- **🐳 Container Management**: Deploy applications in isolated containers
- **📦 Image Registry**: Access to Docker Hub and private registries
- **🔒 Isolation**: Complete environment separation and security
- **🚀 Fast Deployment**: Instant application startup and scaling
- **📊 Resource Control**: CPU, memory, and network resource limits  
- **🔄 Version Management**: Multiple application versions side-by-side

## 🚀 Quick Start

```bash
# Install containerized applications
devex install postgres redis nginx

# List running containers
devex package-manager docker ps

# Pull latest images
devex package-manager docker pull node:20

# Container health monitoring
devex package-manager docker health
```

## 🚀 Platform Support

- **Linux**: All distributions with Docker Engine support
- **Ubuntu**: 18.04+, 20.04+, 22.04+, 24.04+
- **Debian**: 10+, 11+, 12+
- **CentOS/RHEL**: 7+, 8+, 9+
- **Fedora**: 35+, 36+, 37+, 38+, 39+, 40+
- **Arch Linux**: Rolling release

## 📄 License

Licensed under the [GNU GPL v3 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
