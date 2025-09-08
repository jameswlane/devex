# DevEx Pip Package Manager Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Python](https://img.shields.io/badge/Python-Package%20Installer-3776AB?logo=python)](https://pip.pypa.io/)

Python package management plugin for DevEx. Provides comprehensive Python package installation, virtual environment management, and dependency resolution using pip and the Python Package Index (PyPI).

## 🚀 Features

- **🐍 Python Packages**: Install from PyPI and private repositories
- **🌐 Virtual Environments**: Isolated Python environments with venv/virtualenv
- **📋 Requirements Management**: Handle requirements.txt and setup.py files
- **🔒 Security Scanning**: Vulnerability detection with pip-audit integration
- **📦 Wheel Support**: Fast binary package installation when available
- **🚀 Development Tools**: Install development dependencies and tools

## 🚀 Quick Start

```bash
# Install Python packages
devex install jupyter pandas numpy

# Create virtual environment
devex package-manager pip venv create myproject

# Install from requirements file
devex package-manager pip install -r requirements.txt

# List installed packages
devex package-manager pip list
```

## 🚀 Platform Support

- **Cross-Platform**: Linux, macOS, Windows
- **Python Versions**: 3.8+, 3.9+, 3.10+, 3.11+, 3.12+
- **Ubuntu**: 18.04+, 20.04+, 22.04+, 24.04+
- **Debian**: 10+, 11+, 12+
- **Fedora**: 35+, 36+, 37+, 38+, 39+, 40+
- **macOS**: 10.15+, 11+, 12+, 13+, 14+

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
