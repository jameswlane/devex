# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DevEx is a monorepo containing:

1. **CLI Tool** (`apps/cli/`) - A Go CLI tool for streamlining development environment setup. It manages application installations, programming language configurations, themes, and GNOME settings through YAML configuration files.

2. **Website** (`apps/web/`) - A Next.js website hosted at devex.sh for project information and documentation.

3. **Documentation** (`apps/docs/`) - Technical documentation site built with MDX and Next.js.

## Monorepo Structure

When working in this repository, understand the workspace structure:
- Use `pnpm` for workspace management and Node.js dependencies
- Use `task` for CLI development tasks (in `apps/cli/`)
- Each app has its own development workflow and dependencies

## Development Commands

### CLI Development (apps/cli/)
All CLI development should be done in the `apps/cli/` directory:

```bash
cd apps/cli

# Default development workflow (lint + test)
task

# Install and build locally
task install

# Run specific test types
task test          # Standard Go tests
task test:ginkgo   # Ginkgo BDD tests
task test:testify  # Testify tests

# Build commands
task build         # Production build to bin/devex
task build:local   # Local development build

# Code quality
task lint          # Run golangci-lint
task lint:fix      # Auto-fix linting issues
task lint:staticcheck  # Run staticcheck
task gocritic      # Advanced Go analysis

# Security and dependencies
task vulncheck     # Check for vulnerabilities
task mod           # Download and tidy Go modules
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

### Workspace Commands (Root Level)
```bash
# Install all workspace dependencies
pnpm install

# Format code across all apps
pnpm biome:format

# Lint code across all apps
pnpm biome:lint

# Check formatting and linting
pnpm biome:check
```

### Testing Commands (CLI)
```bash
cd apps/cli

# Run individual test suites
go test ./pkg/datastore/...           # Test specific package
ginkgo run ./pkg/commands/            # Run Ginkgo tests for commands
go test -run TestSpecificFunction     # Run specific test function

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Architecture

### CLI Tool Architecture (apps/cli/)

**Main Entry Point**: `apps/cli/cmd/main.go`
- Initializes logger, loads configurations, validates dependencies
- Creates SQLite database connection and repository
- Executes Cobra CLI commands

**Configuration System**: `apps/cli/pkg/config/`
- Uses Viper for YAML configuration loading
- Supports default configs in `~/.local/share/devex/config/` and overrides in `~/.devex/`
- Configuration types defined in `apps/cli/pkg/types/types.go`

**Data Layer**: `apps/cli/pkg/datastore/`
- SQLite database with schema migrations in `apps/cli/migrations/`
- Repository pattern with interfaces in `apps/cli/pkg/types/types.go`
- App repository handles application installation tracking

**Command Structure**: `apps/cli/pkg/commands/`
- Cobra-based CLI with root command in `root.go`
- Subcommands: install, system, completion
- All commands accept `--verbose` and `--dry-run` flags

**Installation System**: `apps/cli/pkg/installers/`
- Multiple installer types: apt, flatpak, brew, docker, pip, mise, etc.
- Each installer implements the `Installer` interface from `apps/cli/pkg/types/types.go`

### Key Configuration Files

**Default Configurations** (in `apps/cli/config/`):
- `apps.yaml` - Application definitions with install methods
- `programming_languages.yaml` - Language-specific tools
- `themes.yaml` - UI theme configurations
- `gnome_settings.yaml` - GNOME desktop settings
- `databases.yaml` - Database applications

### Website Architecture (apps/web/)

**Framework**: Next.js with TypeScript
- React components in `apps/web/app/components/`
- Pages in `apps/web/app/` directory
- Styling with Tailwind CSS
- Static assets in `apps/web/public/`

### Documentation Architecture (apps/docs/)

**Framework**: MDX with Next.js
- MDX pages in `apps/docs/pages/`
- Components in `apps/docs/pages/components/`
- Configuration in `apps/docs/pages/_app.js`

### Testing Framework (CLI)

The CLI project uses both Ginkgo and standard Go testing:
- **Ginkgo**: BDD-style tests with `*_suite_test.go` files in `apps/cli/pkg/`
- **Standard Go tests**: Traditional tests with `*_test.go` files
- **Mocks**: Generated mocks in `apps/cli/pkg/mocks/` using gomock

### Database Schema (CLI)

SQLite database with migration system:
- Schema versions tracked in `schema_migrations` table
- Migration files in `apps/cli/migrations/` directory (up/down SQL files)
- Apps table tracks installed applications

## Development Guidelines

### Working with the Monorepo
1. Always change to the appropriate app directory before development
2. Use `pnpm` for workspace-level commands
3. Use app-specific package managers and tools within each app
4. Consider cross-app dependencies and shared code

### Adding New CLI Installers
1. Create new installer in `apps/cli/pkg/installers/[method]/`
2. Implement `Installer` interface from `apps/cli/pkg/types/types.go`
3. Register in `apps/cli/pkg/installers/installers.go`
4. Add tests following existing patterns

### CLI Configuration Changes
1. Update type definitions in `apps/cli/pkg/types/types.go`
2. Add validation methods if needed
3. Update default configs in `apps/cli/config/` directory
4. Test with both default and override scenarios

### Testing New Features
1. **CLI**: Write Ginkgo BDD tests for complex behavior, standard Go tests for unit testing
2. **Website/Docs**: Follow Next.js testing conventions
3. Generate mocks for external dependencies
4. Test both dry-run and actual execution modes for CLI features

### Cross-App Development
When changes affect multiple apps:
1. Update relevant documentation in each app
2. Test all affected applications
3. Consider versioning implications
4. Update workspace-level configurations if needed
