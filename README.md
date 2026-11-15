# DevEx

[![License](https://img.shields.io/github/license/jameswlane/devex)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/jameswlane/devex)](https://github.com/jameswlane/devex/releases)

DevEx is a powerful, enterprise-grade CLI tool designed to streamline the setup and management of development environments across Linux, macOS, and Windows. With support for 36+ package managers and desktop environments, DevEx automates application installation, system configuration, and development workflow setup.

## 🏗️ Monorepo Architecture

This is a modern monorepo containing:

- **🔧 CLI Tool** (`apps/cli/`) - Enterprise Go CLI with Cobra + Viper architecture
- **🌐 Website** (`apps/web/`) - Marketing site at [devex.sh](https://devex.sh)
- **📚 Documentation** (`apps/docs/`) - Technical documentation with MDX
- **🔌 Plugin System** (`packages/plugins/`) - 36 modular plugins for package managers and desktop environments
- **📦 Shared SDK** (`packages/shared/`) - Common plugin development framework

## ✨ Key Features

- **🔌 Plugin Architecture**: 36 specialized plugins for package managers (apt, dnf, pacman, brew, etc.) and desktop environments
- **🏢 Enterprise-Ready**: Built with Cobra CLI framework, Viper configuration, and 12-Factor App methodology
- **⚡ Cross-Platform**: Native support for Linux distributions, macOS, and Windows
- **🎯 Smart Detection**: Automatic platform, distribution, and desktop environment detection
- **📋 Configuration Management**: YAML-based configuration with hierarchical overrides
- **🚀 Rapid Development**: 6-day sprint methodology with automated testing and deployment

## 🚀 Quick Start

### One-Line Installation

```bash
# Install DevEx CLI
curl -fsSL https://get.devex.sh/install | bash
```

### Basic Usage

```bash
# Install development environment
devex install

# Install specific categories
devex install --categories development,databases

# Configure desktop environment
devex system apply

# List available applications
devex list apps
```

## 🔌 Plugin System

DevEx features a comprehensive plugin architecture with 36 specialized plugins:

### Package Manager Plugins
- **Linux**: apt, dnf, pacman, yay, zypper, emerge, eopkg, xbps, apk, rpm, deb
- **Universal**: flatpak, snap, appimage, mise, docker, pip, curlpipe
- **Cross-platform**: brew (macOS/Linux)
- **Nix**: nixpkgs, nixflake

### Desktop Environment Plugins
- **GNOME**, **KDE Plasma**, **XFCE**, **MATE**, **Cinnamon**
- **LXQt**, **Budgie**, **Pantheon**, **COSMIC**
- **Themes & Fonts**: Centralized theming and font management

### System Plugins
- **Git Configuration**: Automated Git setup and credential management
- **Shell Setup**: Bash, Zsh, Fish configuration
- **System Setup**: Core system configuration and optimization
- **Stack Detection**: Automatic project stack detection and setup

## 🏢 Enterprise Architecture

DevEx follows enterprise patterns and best practices:

### CLI Framework
- **Cobra**: Command structure and flag management
- **Viper**: Hierarchical configuration (flags > env > config > defaults)
- **12-Factor App**: Configuration, logging, and process management
- **Context Propagation**: Proper cancellation and timeout handling

### Testing & Quality
- **Ginkgo BDD**: Behavior-driven development testing
- **golangci-lint**: Comprehensive Go linting
- **lefthook**: Git hooks for quality gates
- **govulncheck**: Security vulnerability scanning

### Development Workflow
- **6-Day Sprints**: Rapid iteration and delivery cycles
- **Automated Testing**: Comprehensive test coverage with CI/CD
- **Semantic Versioning**: Automated releases with semantic-release
- **Plugin Versioning**: Independent plugin release management

## 💻 Development Setup

### Prerequisites

- **Go**: Version 1.26+ (CLI development)
- **Node.js**: Version 20.9+ (website and docs)
- **pnpm**: Version 9+ (workspace management)
- **Task**: Task runner for CLI development
- **lefthook**: Git hooks (installed via pnpm)

### Development Installation

#### Full Monorepo Setup

```bash
# Clone repository
git clone https://github.com/jameswlane/devex.git
cd devex

# Install workspace dependencies
pnpm install

# Install CLI locally for testing
cd apps/cli
task install
```

#### CLI Development Workflow

```bash
cd apps/cli

# Default workflow (lint + test)
task

# Development commands
task build:local    # Build for local testing
task test          # Run all tests
task test:ginkgo   # Run Ginkgo BDD tests
task lint          # Run golangci-lint
task lint:fix      # Auto-fix linting issues
task vulncheck     # Security vulnerability check

# Plugin development
lefthook run plugin-check        # Check plugin changes
lefthook run plugin-build       # Build all changed plugins
lefthook run plugin-test        # Test specific plugin
```

---

## Configuration

### Custom Configuration Files

Custom configurations are stored under `~/.devex/`:

```plaintext
~/.devex/
    ├── apps.yaml
    ├── gnome_extensions.yaml
    ├── programming_languages.yaml
    ├── config/
    │   └── additional_configs.yaml
    └── themes.yaml
```

### Default vs Custom Configuration

DevEx prioritizes custom configurations in `~/.devex/`. If not found, it falls back to defaults in the `assets/` directory.

#### Example: `apps.yaml`

```yaml
apps:
   - name: "Visual Studio Code"
     description: "Code editor from Microsoft"
     category: "Editors"
     install_method: "apt"
     install_command: "code"
     dependencies:
        - "gnome-shell"
        - "git"
```

### Formatting

To format configuration files, run:

```bash
prettier --write "**/*.{yaml,md}"
```

---

## 📁 Project Structure

```
devex/
├── apps/
│   ├── cli/                    # DevEx CLI tool (Go)
│   │   ├── cmd/               # Cobra command definitions
│   │   ├── pkg/               # Public packages
│   │   ├── internal/          # Private application code
│   │   ├── config/            # Default YAML configurations
│   │   └── Taskfile.yml       # CLI-specific tasks
│   ├── web/                   # Marketing website (Next.js)
│   └── docs/                  # Documentation site (MDX)
├── packages/
│   ├── plugins/               # Plugin system (36 plugins)
│   │   ├── package-manager-*/ # Package manager plugins
│   │   ├── desktop-*/         # Desktop environment plugins
│   │   ├── tool-*/           # Development tool plugins
│   │   └── system-setup/     # System configuration plugin
│   └── shared/
│       └── plugin-sdk/       # Common plugin development framework
├── scripts/                   # Build and release automation
├── .github/                   # GitHub Actions workflows
├── lefthook.yml              # Git hooks configuration
├── pnpm-workspace.yaml       # Workspace configuration
└── package.json              # Root dependencies and scripts
```

## Development Commands

### CLI Development (apps/cli/)

The CLI uses `Task` for automation:

```bash
cd apps/cli

# Default development workflow (lint + test)
task

# Build and install locally
task install

# Run tests
task test          # Standard Go tests
task test:ginkgo   # Ginkgo BDD tests
task test:testify  # Testify tests

# Code quality
task lint          # Run golangci-lint
task lint:fix      # Auto-fix linting issues
task vulncheck     # Check for vulnerabilities
```

### Website Development (apps/web/)

```bash
cd apps/web

# Install dependencies
pnpm install

# Start development server
pnpm dev

# Build for production
pnpm build
```

### Documentation Development (apps/docs/)

```bash
cd apps/docs

# Install dependencies
pnpm install

# Start development server
pnpm start

# Build static site
pnpm build
```

### Workspace Commands (Root)

```bash
# Install all dependencies
pnpm install

# Format all code
pnpm biome:format

# Lint all code
pnpm biome:lint

# Check formatting and linting
pnpm biome:check
```

---

## Development

### Workspace task orchestration

Run `pnpm build`, `pnpm lint`, and `pnpm test` for the workspace, or use Turbo's
real package task names for focused checks, for example
`pnpm turbo:run run build --filter=@devex/cli`. The root `build:go`, `lint:go`,
`test:go`, and `test:ginkgo` commands are convenience filters over `build`,
`lint`, and `test`; they are not separate package tasks. CLI `task turbo:build`,
`task turbo:test`, and `task turbo:lint` use those same real tasks.
`pnpm typecheck` runs the registry's `typecheck` script.

`pnpm generate` creates `apps/web/app/generated/tools.json` from the application
YAML in `apps/cli/config/applications`, using
`apps/web/scripts/generate-tools.mjs`. This application catalog supports registry
seeding; the website's live catalog API reads the registry. Generation validates
all sources before replacing the catalog and fails on malformed, duplicate, or
empty input. Generated catalogs are ignored by Git. Turbo hashes both the YAML
and generator script and restores the output on cache hits. The web Turbo build
runs generation first; direct package builds (`pnpm --filter @devex/web build`)
bypass that orchestration, so run `pnpm generate` first when using them.

Registry Turbo `build`, `test`, `dev`, and `typecheck` run the registry's own
`db:generate` first. Prisma generation does not require a database connection;
commands that access the database still need their usual environment settings.

Go packages extend `@devex/turbo-go`; Next.js apps extend `@devex/turbo-next`.
These configuration-only workspaces hold shared settings, while package configs
retain local generation dependencies and output overrides. Turbo 2.11.11 supports
extending named workspace configurations after `"//"` (the installed schema's
`extends` description still says only `"//"`; the executable and regression suite
verify named inheritance).

Go and Vitest tests compile from source and do not depend on application builds.
Registry tests retain Prisma generation because they import generated runtime
values. Go test/lint/build inputs include SDK sources, manifests and fixtures,
module defaults (including YAML and test data), shared task scripts, and root mise
pins. SDK changes therefore invalidate consumers without a build prerequisite.
The root Biome config affects JavaScript linting. Plugin build caches retain the
existing executable paths and exclude matching `.go` source files.

Root Turbo commands use `scripts/turbo/run.mjs` to measure the active tools before
hashing. It queries effective `go env` settings in executable Go modules in the
selected task graph, including dependencies. This honors module-selected toolchains,
persistent Go environment settings, and local/CI overrides; the sorted set of
fingerprints affects Go tasks only. `golangci-lint version` affects Go lint tasks only. Next-only graphs
need neither tool. Use `pnpm turbo:run run <task> [filters/options]` for custom
cached Go runs. Raw `pnpm exec turbo` bypasses measurement; Go task scripts reject
uncached runs without matching measured fingerprints. Direct package commands
(`pnpm --filter @devex/cli test`) continue to run normally without Turbo caching.
No versions are installed or tool pins changed by the runner.

Environment files are scoped to each Next app and task mode, rather than hashed
globally. Registry database, Redis, signing, GitHub and application settings are
listed on its runtime tasks, including test and dev, for Turbo's strict environment
mode. Next public variables and standard Next runtime settings are available to
web/docs tasks. Database/Redis settings and Next `.env*` files do not invalidate
Go tasks. Root `.env*` files are not inputs: Next loads env files from each app.

`pnpm vulncheck` scans all Go modules; use
`pnpm turbo:run run vulncheck --filter=@devex/cli` for a focused scan, or
`task vulncheck` from the CLI directory. The runner uses
`go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` without a global install.
Turbo scanning is always uncached, and the CLI Taskfile has no source freshness
shortcut: every invocation reads the current vulnerability database. Scanner
findings and network/tool failures fail the command.

Check task graphs, hash isolation/inheritance, measured tool versions, repeated
scanning, cache restoration, and catalog generation with `pnpm turbo:test`
(requires installed workspace dependencies, Git, and `tar` with Zstandard support).

Unused Turbo-only aliases were removed: use `build`, `lint`, `test`, and
`typecheck` as task names. The root `build:go`, `lint:go`, `test:go` and
`test:ginkgo` scripts remain convenience filters. Taskfile operations such as
local builds and installation remain available in the relevant Go module. The
root `clean`, `lint:fix`, and `release:detect`/`release:version`/`release:changelog`/
`release:build` wrappers had no matching scripts and were removed. Release
commands are documented in [RELEASING.md](RELEASING.md).

### Testing

Run CLI tests:

```bash
cd apps/cli
task test
```

### Linting

Run linting across the monorepo:

```bash
# Root level (Biome for JS/TS)
pnpm biome:lint

# CLI specific (Go)
cd apps/cli
task lint
```

### Building

Build individual applications:

```bash
# CLI tool
cd apps/cli
task build

# Website
cd apps/web
pnpm build

# Documentation
cd apps/docs
pnpm build
```

---

## 🚀 Release Management

DevEx uses comprehensive automated release management:

### CLI Releases
- **GoReleaser**: Automated binary builds for multiple platforms
- **GitHub Releases**: Automatic changelog generation and asset publishing
- **Semantic Versioning**: Conventional commit-based version bumping

### Plugin Releases
- **Individual Versioning**: Each of the 36 plugins versions independently
- **Parallel Builds**: Multi-threaded plugin compilation
- **Automated Registry**: Plugin registry updates with version tracking

### Commit Conventions
- `feat:` - New features (minor version bump)
- `fix:` - Bug fixes (patch version bump)
- `feat!:` or `BREAKING CHANGE:` - Breaking changes (major version bump)
- `docs:`, `style:`, `refactor:`, `test:`, `chore:` - No version bump

### Release Triggers
```bash
# Feature release (minor version)
git commit -m "feat: add new plugin system"

# Bug fix release (patch version)
git commit -m "fix: resolve installation issue"

# Breaking change release (major version)
git commit -m "feat!: redesign configuration system"
```

---

## 🌟 Platform Support

### Operating Systems
- **Linux**: Ubuntu, Debian, Fedora, CentOS, Arch, openSUSE, Gentoo, Void, Alpine
- **macOS**: Intel and Apple Silicon support
- **Windows**: Native Windows support (planned)

### Package Managers
- **Linux**: apt, dnf, pacman, yay, zypper, emerge, eopkg, xbps, apk
- **Universal**: flatpak, snap, appimage, docker, pip, mise
- **Cross-platform**: brew (macOS/Linux)
- **Nix**: nixpkgs, nixflake
- **Binary**: curlpipe, rpm, deb

### Desktop Environments
- **GNOME**, **KDE Plasma**, **XFCE**, **MATE**, **Cinnamon**
- **LXQt**, **Budgie**, **Pantheon**, **COSMIC**

## 🤝 Community & Support

### 📞 Getting Help
- **[Issues](https://github.com/jameswlane/devex/issues)**: Bug reports and feature requests
- **[Discussions](https://github.com/jameswlane/devex/discussions)**: Community Q&A and ideas
- **[Documentation](https://docs.devex.sh)**: Comprehensive guides and API docs
- **[Website](https://devex.sh)**: Official website with tutorials and updates

### 🏗️ Development Resources
- **[Projects](https://github.com/jameswlane/devex/projects)**: Roadmap and progress tracking
- **[Wiki](https://github.com/jameswlane/devex/wiki)**: In-depth technical documentation
- **[Security](https://github.com/jameswlane/devex/security)**: Vulnerability reporting
- **[Pulse](https://github.com/jameswlane/devex/pulse)**: Project activity and metrics

---

## 🤝 Contributing

We welcome contributions! DevEx follows enterprise development practices:

### Getting Started
1. **Fork** the repository
2. **Clone** your fork: `git clone https://github.com/yourusername/devex.git`
3. **Install** dependencies: `pnpm install`
4. **Create** a feature branch: `git checkout -b feat/your-feature`
5. **Develop** using our CLI workflow: `cd apps/cli && task`
6. **Test** thoroughly: `task test && task lint`
7. **Commit** with conventional commits: `git commit -m "feat: add new feature"`
8. **Push** and create a **Pull Request**

### Development Standards
- **Quality Gates**: All commits must pass linting, testing, and security checks
- **Conventional Commits**: Required for automated versioning
- **Test Coverage**: Maintain comprehensive test coverage with Ginkgo BDD
- **Documentation**: Update docs for user-facing changes

### Plugin Development
```bash
# Check plugin changes
lefthook run plugin-check

# Build and test plugins
lefthook run plugin-build
lefthook run plugin-test [plugin-name]

# Plugin versioning
node scripts/determine-plugin-version.js update [plugin-name]
```

Refer to the [Contributing Guide](.github/CONTRIBUTING.md) for detailed guidelines.

---

## 📄 Legal & Security

### License
DevEx is licensed under the [Apache-2.0 License](LICENSE), ensuring it remains free and open source.

### Security Policy
For security vulnerabilities, please refer to our [Security Policy](SECURITY.md) and report issues privately.

### Code of Conduct
We maintain a welcoming community following our [Code of Conduct](CODE_OF_CONDUCT.md).

---

## 🤖 AI-Assisted Development

This project leverages AI-assisted development tools to enhance productivity and code quality while maintaining human oversight and decision-making. [Learn more about our AI usage](./AI_USAGE.md).

---

<div align="center">

### 🚀 Ready to streamline your development environment?

**[Install DevEx](https://devex.sh)** • **[Browse Docs](https://docs.devex.sh)** • **[Join Discussions](https://github.com/jameswlane/devex/discussions)**

</div>
