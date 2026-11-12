# DevEx Stack Detector Tool Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Detection](https://img.shields.io/badge/Stack-Detection-FF6B6B?logo=detective)](https://github.com/jameswlane/devex)

Automatic technology stack detection plugin for DevEx. Intelligently detects project technologies, frameworks, and tools to provide tailored development environment setup recommendations.

## 🚀 Features

- **🔍 Automatic Detection**: Scan projects to identify technologies and frameworks
- **🎯 Smart Recommendations**: Suggest appropriate tools and packages based on stack
- **📊 Multi-Language Support**: Detect Node.js, Python, Go, Rust, Java, and more
- **🔧 Framework Recognition**: Identify React, Vue, Django, Rails, Spring, etc.
- **📦 Tool Suggestions**: Recommend development tools and extensions
- **⚡ Fast Scanning**: Efficient project analysis with minimal overhead

## 📦 Supported Technologies

### Programming Languages
- **JavaScript/TypeScript**: Node.js, Deno, Bun runtime detection
- **Python**: Version detection, virtual environment setup
- **Go**: Module and version management
- **Rust**: Cargo project detection and toolchain setup
- **Java**: Maven, Gradle, Spring Boot detection
- **PHP**: Composer, Laravel, Symfony detection

### Frameworks & Tools
- **Frontend**: React, Vue, Angular, Svelte, Next.js, Nuxt
- **Backend**: Express, FastAPI, Django, Rails, Spring
- **Databases**: PostgreSQL, MySQL, MongoDB, Redis
- **DevOps**: Docker, Kubernetes, CI/CD configurations

## 🚀 Quick Start

Use the CLI's detect commands; they install this plugin when needed:

```bash
# Detect the current project's stack
devex detect

# Full analysis of a directory
devex detect analyze /path/to/project

# DevEx applications that match the project
devex detect suggest

# Add them to your configuration (preview first)
devex detect apply --dry-run
```

## Plugin commands

| Command | Arguments | Output |
|---------|-----------|--------|
| `detect` | `[directory]` | Human-readable list of detected technologies |
| `analyze` | `[directory] [--verbose]` | Human-readable analysis |
| `report` | `[directory] [--path DIR] [--format text\|json\|yaml] [--output FILE]` | The stack report |

`report --format json` writes only the JSON report to stdout (progress goes to stderr). The report has `project_path`, `technologies` (`name`, `category`, `confidence` 1-10, `description`, `files`), `dependencies`, `project_size`, `recommendations`, `issues` and `summary`. `devex detect` reads this report.

## 🚀 Platform Support

- **Universal**: All platforms with project file access
- **Languages**: 25+ programming languages and frameworks
- **Project Types**: Web, mobile, desktop, CLI, and server applications

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).
