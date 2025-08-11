# DevEx

DevEx is a powerful CLI tool designed to streamline the setup and management of development environments. It simplifies the installation of applications, configuration of programming languages, and customization of themes.

This is a monorepo containing:
- **CLI Tool** (`apps/cli/`) - The main DevEx CLI application
- **Website** (`apps/web/`) - The official website at [devex.sh](https://devex.sh)
- **Documentation** (`apps/docs/`) - Technical documentation site

---

## Features

- **Custom Configuration Management**: Tailor application, GNOME extension, and programming language setups with YAML files.
- **Automated Releases**: Leverage `commitizen`, `semantic-release`, and `goreleaser` for seamless versioning and publishing.
- **Task Automation**: Use `Taskfile` for efficient script execution and workflow management.
- **Community Support**: Engage with contributors through GitHub Issues, Discussions, and Wiki.
- **Biome Formatting**: Standardize code with Biome for consistent formatting and linting.
- **Comprehensive Website**: Access guides, documentation, and updates at [devex.sh](https://devex.sh).

---

## Getting Started

### Prerequisites

- **Go**: Version 1.23 or later (for CLI development)
- **Node.js**: Version 18.x or later (for website and docs)
- **pnpm**: Version 9.x or later (for workspace management)
- **Mise**: Install from the [Mise GitHub page](https://github.com/mise/mise)

### Installation

#### CLI Tool
To install the DevEx CLI:

```bash
cd apps/cli
task install
```

#### Development Setup (Full Monorepo)
To set up the entire development environment:

```bash
# Install workspace dependencies
pnpm install

# Build all projects
pnpm build

# Start development servers
pnpm dev
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

## Monorepo Structure

```
devex/
├── apps/
│   ├── cli/           # DevEx CLI tool (Go)
│   ├── web/           # Main website (Next.js)
│   └── docs/          # Documentation site (MDX)
├── packages/          # Shared packages (future use)
├── pnpm-workspace.yaml
└── package.json       # Root workspace configuration
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

## Automated Releases

DevEx uses `semantic-release` for automated versioning and releases. The monorepo structure allows for independent versioning of each application.

Releases are triggered automatically on the main branch when commits follow conventional commit patterns:
- `feat:` - new features (minor version bump)
- `fix:` - bug fixes (patch version bump)
- `BREAKING CHANGE:` - breaking changes (major version bump)

---

## Community and Support

### GitHub Features

- **[Issues](https://github.com/jameswlane/devex/issues)**: Report bugs or request features.
- **[Discussions](https://github.com/jameswlane/devex/discussions)**: Ask questions or share ideas.
- **[Docs](https://docs.devex.sh)**: View documentation.
- **[Projects](https://github.com/jameswlane/devex/projects)**: Track project progress.
- **[Wiki](https://github.com/jameswlane/devex/wiki)**: Access in-depth documentation.
- **[Security](https://github.com/jameswlane/devex/security)**: Report vulnerabilities.
- **[Pulse](https://github.com/jameswlane/devex/pulse)**: View project activity.

### Website

Visit the official website at [devex.sh](https://devex.sh) for documentation, guides, and updates.

---

## Contributing

Contributions are welcome! Refer to the [Contributing Guide](.github/CONTRIBUTING.md) for details.

### Code of Conduct

We expect all contributors to adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).

---

## License

DevEx is licensed under the [GNU GPL v3 License](LICENSE).

---

## Security

For security concerns, please refer to our [Security Policy](SECURITY.md).

---

**Note:** This project uses AI-assisted tools for certain tasks.  
[Learn more about our AI usage here.](./AI_USAGE.md)
