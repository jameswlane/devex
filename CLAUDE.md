# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DevEx is a Go CLI tool for streamlining development environment setup. It manages application installations, programming language configurations, themes, and GNOME settings through YAML configuration files.

## Development Commands

### Primary Development Flow
```bash
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
```

### Code Quality and Analysis
```bash
# Linting
task lint          # Run golangci-lint
task lint:fix      # Auto-fix linting issues
task lint:staticcheck  # Run staticcheck
task gocritic      # Advanced Go analysis

# Security and dependencies
task vulncheck     # Check for vulnerabilities
task mod           # Download and tidy Go modules
```

### Testing Commands
```bash
# Run individual test suites
go test ./pkg/datastore/...           # Test specific package
ginkgo run ./pkg/commands/            # Run Ginkgo tests for commands
go test -run TestSpecificFunction     # Run specific test function

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Architecture

### Core Components

**Main Entry Point**: `cmd/main.go`
- Initializes logger, loads configurations, validates dependencies
- Creates SQLite database connection and repository
- Executes Cobra CLI commands

**Configuration System**: `pkg/config/`
- Uses Viper for YAML configuration loading
- Supports default configs in `~/.local/share/devex/config/` and overrides in `~/.devex/`
- Configuration types defined in `pkg/types/types.go`

**Data Layer**: `pkg/datastore/`
- SQLite database with schema migrations in `migrations/`
- Repository pattern with interfaces in `pkg/types/types.go`
- App repository handles application installation tracking

**Command Structure**: `pkg/commands/`
- Cobra-based CLI with root command in `root.go`
- Subcommands: install, system, completion
- All commands accept `--verbose` and `--dry-run` flags

**Installation System**: `pkg/installers/`
- Multiple installer types: apt, flatpak, brew, docker, pip, mise, etc.
- Each installer implements the `Installer` interface from `pkg/types/types.go`

### Key Configuration Files

**Default Configurations** (in `config/`):
- `apps.yaml` - Application definitions with install methods
- `programming_languages.yaml` - Language-specific tools
- `themes.yaml` - UI theme configurations
- `gnome_settings.yaml` - GNOME desktop settings
- `databases.yaml` - Database applications

### Testing Framework

The project uses both Ginkgo and standard Go testing:
- **Ginkgo**: BDD-style tests with `*_suite_test.go` files
- **Standard Go tests**: Traditional tests with `*_test.go` files
- **Mocks**: Generated mocks in `pkg/mocks/` using gomock

### Database Schema

SQLite database with migration system:
- Schema versions tracked in `schema_migrations` table
- Migration files in `migrations/` directory (up/down SQL files)
- Apps table tracks installed applications

## Development Guidelines

### Adding New Installers
1. Create new installer in `pkg/installers/[method]/`
2. Implement `Installer` interface from `pkg/types/types.go`
3. Register in `pkg/installers/installers.go`
4. Add tests following existing patterns

### Configuration Changes
1. Update type definitions in `pkg/types/types.go`
2. Add validation methods if needed
3. Update default configs in `config/` directory
4. Test with both default and override scenarios

### Testing New Features
1. Write Ginkgo BDD tests for complex behavior
2. Use standard Go tests for unit testing
3. Generate mocks for external dependencies
4. Test both dry-run and actual execution modes
