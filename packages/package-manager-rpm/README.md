# DevEx RPM Package Manager Plugin

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue?logo=go)](https://golang.org/)
[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![RPM](https://img.shields.io/badge/RPM-Package%20Manager-EE0000?logo=redhat)](https://rpm.org/)

Red Hat Package Manager (RPM) plugin for DevEx. Provides low-level package management for RPM-based distributions with direct package installation, verification, and system maintenance capabilities.

## 🚀 Features

- **📦 Direct RPM Management**: Install, remove, query RPM packages directly
- **🔍 Package Verification**: Integrity and authenticity checking with GPG
- **📊 Dependency Analysis**: Query package dependencies and conflicts
- **🛡️ Database Management**: RPM database maintenance and repair
- **🚀 Scriptlet Execution**: Pre/post installation script handling
- **🔧 Package Building**: Support for .spec files and source RPMs

## 🚀 Quick Start

```bash
# Install RPM packages directly
devex package-manager rpm install package.rpm

# Query installed packages
devex package-manager rpm -qa

# Verify package integrity
devex package-manager rpm -V package-name

# Extract package contents
devex package-manager rpm -ql package-name
```

## 🚀 Platform Support

- **Red Hat Enterprise Linux**: 7+, 8+, 9+
- **CentOS**: 7+, 8+, Stream 8+, Stream 9+
- **Fedora**: 35+, 36+, 37+, 38+, 39+, 40+
- **Rocky Linux**: 8+, 9+
- **AlmaLinux**: 8+, 9+
- **openSUSE**: Leap 15+, Tumbleweed

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
