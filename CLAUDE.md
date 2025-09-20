# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the DevEx CLI codebase, following enterprise Go CLI best practices.

## Project Overview

DevEx is a monorepo containing a production-ready CLI tool built with modern Go patterns:

1. **CLI Tool** (`apps/cli/`) - Enterprise-grade Go CLI using Cobra + Viper for streamlined development environment setup
2. **Website** (`apps/web/`) - Next.js marketing site hosted at devex.sh  
3. **Documentation** (`apps/docs/`) - Technical documentation built with MDX

## CLI Architecture & Best Practices

### Enterprise CLI Patterns

The DevEx CLI follows 12-Factor App methodology and enterprise patterns:

**Configuration Hierarchy** (highest to lowest precedence):
1. Command-line flags (`--port 3000`)
2. Environment variables (`DEVEX_PORT=9000`) 
3. Configuration files (`~/.devex/config.yaml`)
4. Sensible defaults

**Project Structure** (follows Cobra enterprise patterns):
```
apps/cli/
├── cmd/                    # Cobra command definitions
│   ├── root.go            # Root command with PersistentPreRunE
│   ├── install.go         # Feature commands
│   └── system.go          
├── internal/              # Private application code
│   ├── cli/               # CLI-specific logic
│   └── config/            # Configuration management
├── pkg/                   # Public packages
│   ├── commands/          # Command implementations
│   ├── installers/        # Platform-specific installers
│   ├── types/             # Core data structures
│   └── config/            # Viper configuration
├── config/                # Default YAML configurations
├── migrations/            # Database schema migrations
└── docs/                  # Auto-generated CLI documentation
```

### Command Design Patterns

**Standard Command Pattern**:
```go
package cmd

import (
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var installCmd = &cobra.Command{
    Use:   "install [apps...]",
    Short: "Install development applications",
    Long:  "Install applications with cross-platform package manager support",
    Example: `  # Install default applications
  devex install

  # Install specific applications  
  devex install docker git vscode`,
    Args: cobra.ArbitraryArgs,
    RunE: func(cmd *cobra.Command, args []string) error {
        // Get config from Viper (not flags directly)
        verbose := viper.GetBool("verbose")
        dryRun := viper.GetBool("dry-run")
        
        return executeInstall(cmd.Context(), args, verbose, dryRun)
    },
}

func init() {
    // Define flags
    installCmd.Flags().Bool("dry-run", false, "show what would be installed")
    installCmd.Flags().StringSlice("categories", nil, "install apps from categories")
    
    // Bind flags to Viper for hierarchical config
    viper.BindPFlag("dry-run", installCmd.Flags().Lookup("dry-run"))
    viper.BindPFlag("categories", installCmd.Flags().Lookup("categories"))
    
    rootCmd.AddCommand(installCmd)
}
```

**Configuration Integration** (in `cmd/root.go`):
```go
var rootCmd = &cobra.Command{
    Use: "devex",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        return initializeConfig(cmd)
    },
}

func initializeConfig(cmd *cobra.Command) error {
    // 1. Environment variables
    viper.SetEnvPrefix("DEVEX")
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
    viper.AutomaticEnv()
    
    // 2. Configuration file
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, _ := os.UserHomeDir()
        viper.AddConfigPath(".")
        viper.AddConfigPath(home + "/.devex")
        viper.AddConfigPath(home + "/.local/share/devex/config")
        viper.SetConfigName("config")
        viper.SetConfigType("yaml")
    }
    
    // 3. Read config file (ignore if not found)
    if err := viper.ReadInConfig(); err != nil {
        var configFileNotFoundError viper.ConfigFileNotFoundError
        if !errors.As(err, &configFileNotFoundError) {
            return err
        }
    }
    
    // 4. Bind all flags to Viper
    return viper.BindPFlags(cmd.Flags())
}
```

### Development Workflow

All CLI development should be done in the `apps/cli/` directory:

```bash
cd apps/cli

# Development workflow
task                    # Default: lint + test
task build             # Production build to bin/devex
task build:local       # Development build
task install           # Install locally

# Testing (Ginkgo BDD framework)
task test              # All tests
task test:ginkgo       # Ginkgo BDD tests only
ginkgo run ./pkg/commands/  # Specific package tests

# Code quality  
task lint              # golangci-lint
task lint:fix          # Auto-fix issues
task vulncheck         # Security vulnerability scan

# Command Generation (cobra-cli)
cobra-cli add [command]               # Generate new command
cobra-cli add [child] -p [parent]     # Generate child command  
cobra-cli add config --config         # Add config command

# Documentation generation
go run ./internal/tools/docgen -out ./docs/cli -format markdown
```

### Configuration System

**4 Core Configuration Files** (in `apps/cli/config/`):
- `applications.yaml` - Cross-platform application definitions
- `environment.yaml` - Programming languages, fonts, shells  
- `desktop.yaml` - Desktop environment settings (GNOME, KDE, macOS)
- `system.yaml` - Git, SSH, terminal configurations

**Configuration Features**:
- **12-Factor compliant**: Environment variable overrides
- **Cross-platform**: Linux, macOS, Windows support
- **User overrides**: `~/.devex/` overrides defaults
- **Validation**: Built-in YAML schema validation
- **Auto-discovery**: Platform and desktop environment detection

### Installation System Architecture

**Installer Interface** (from `pkg/types/types.go`):
```go
type Installer interface {
    Install(ctx context.Context, app types.CrossPlatformApp) error
    Uninstall(ctx context.Context, app types.CrossPlatformApp) error
    IsInstalled(ctx context.Context, app types.CrossPlatformApp) (bool, error)
    GetName() string
    GetPriority() int
    CanInstall(app types.CrossPlatformApp) bool
}
```

**Platform Installers** (in `pkg/installers/`):
- **Linux**: apt, dnf, pacman, zypper, flatpak, snap
- **Cross-platform**: mise, curlpipe, docker, pip
- **macOS**: brew, mas (Mac App Store)
- **Windows**: winget, chocolatey, scoop (planned)

**Installer Priority System**:
1. Platform-specific package managers (highest priority)
2. Universal package managers (flatpak, snap)
3. Language-specific managers (mise, pip)
4. Direct download methods (curlpipe)

### Error Handling & Observability

**Error Handling Patterns**:
```go
// Use RunE for proper error propagation
var myCmd = &cobra.Command{
    RunE: func(cmd *cobra.Command, args []string) error {
        ctx := cmd.Context()
        
        if err := validateInputs(args); err != nil {
            return fmt.Errorf("invalid inputs: %w", err)
        }
        
        return executeOperation(ctx, args)
    },
}

func init() {
    // Prevent usage spam on runtime errors
    myCmd.SilenceUsage = true
}
```

**Context & Tracing** (for observability):
```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("devex")

func executeInstall(ctx context.Context, apps []string) error {
    ctx, span := tracer.Start(ctx, "install_command",
        trace.WithAttributes(
            attribute.StringSlice("apps", apps),
            attribute.String("platform", runtime.GOOS),
        ),
    )
    defer span.End()
    
    for _, app := range apps {
        if err := installApp(ctx, app); err != nil {
            span.RecordError(err)
            return fmt.Errorf("failed to install %s: %w", app, err)
        }
    }
    
    span.SetStatus(codes.Ok, "Installation completed")
    return nil
}
```

### Testing Framework (Ginkgo BDD)

**CRITICAL**: Use Ginkgo BDD exclusively - no mixing with standard Go tests:

```go
// pkg/commands/install_suite_test.go
package commands_test

import (
    "testing"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestCommands(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Commands Suite")
}

// pkg/commands/install_test.go  
var _ = Describe("Install Command", func() {
    var (
        installer *mocks.MockInstaller
        cmd       *cobra.Command
    )
    
    BeforeEach(func() {
        installer = mocks.NewMockInstaller()
        cmd = NewInstallCommand(installer)
    })
    
    Context("when installing applications", func() {
        It("should install default applications", func() {
            err := cmd.Execute()
            Expect(err).ToNot(HaveOccurred())
            Expect(installer.InstallCalls).To(HaveLen(3))
        })
        
        It("should handle installation failures gracefully", func() {
            installer.ShouldFail = true
            err := cmd.Execute()
            Expect(err).To(HaveOccurred())
            Expect(err.Error()).To(ContainSubstring("installation failed"))
        })
    })
})
```

### Security & Validation

**Input Validation** (minimal but essential):
```go
func validateCommand(command string) error {
    // Only block obviously destructive commands
    dangerousPatterns := []*regexp.Regexp{
        regexp.MustCompile(`\brm\s+(-[rfRi]*\s+)*(/|/home|/usr|/var|/etc)\s*$`),
        regexp.MustCompile(`\bdd\s+.*\bof=/dev/(sd[a-z]|hd[a-z]|nvme\d+n\d+)\b`),
        regexp.MustCompile(`\bmkfs\b.*\b/dev/`),
        regexp.MustCompile(`:\(\)\{.*:\|:&.*\};:`), // fork bombs
    }
    
    for _, pattern := range dangerousPatterns {
        if pattern.MatchString(command) {
            return fmt.Errorf("command contains potentially dangerous pattern")
        }
    }
    
    return nil // Allow by default - focus on functionality
}
```

**Context-Aware Execution**:
```go
// Always use CommandContext for cancellation support
func executeShellCommand(ctx context.Context, command string) error {
    if err := validateCommand(command); err != nil {
        return err
    }
    
    cmd := exec.CommandContext(ctx, "sh", "-c", command)
    return cmd.Run()
}
```

### Auto-Generated Documentation

**CLI Documentation Generation**:
```go
// internal/tools/docgen/main.go
package main

import (
    "github.com/spf13/cobra/doc"
    "example.com/devex/cmd"
)

func main() {
    root := cmd.Root()
    root.DisableAutoGenTag = true // Stable, reproducible files
    
    // Generate Markdown docs (LLM-friendly)
    err := doc.GenMarkdownTree(root, "./docs/cli")
    if err != nil {
        log.Fatal(err)
    }
}
```

Run documentation generation:
```bash
go run ./internal/tools/docgen -out ./docs/cli -format markdown
```

### Flag Management Best Practices

**Flag Design Patterns**:
```go
func init() {
    // Local flags (command-specific)
    installCmd.Flags().StringSlice("categories", nil, "install from categories")
    installCmd.Flags().Bool("dry-run", false, "show what would be installed") 
    
    // Persistent flags (inherited by subcommands)
    rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
    rootCmd.PersistentFlags().String("config", "", "config file path")
    
    // Required flags
    installCmd.MarkFlagRequired("categories")
    
    // Flag validation in PreRunE
    installCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
        categories, _ := cmd.Flags().GetStringSlice("categories")
        for _, cat := range categories {
            if !isValidCategory(cat) {
                return fmt.Errorf("invalid category: %s", cat)
            }
        }
        return nil
    }
    
    // Bind to Viper for config hierarchy
    viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
    viper.BindPFlag("dry-run", installCmd.Flags().Lookup("dry-run"))
}
```

### Pre-Commit & Quality Gates

**🚨 MANDATORY: Never Skip Quality Checks 🚨**

```bash
# Pre-commit requirements (enforced by lefthook)
task lint          # Must pass golangci-lint
task test          # All tests must pass  
task build         # Must build successfully

# NEVER use these bypass flags:
# git commit --no-verify
# git push --no-verify
```

**If quality checks fail:**
1. Fix the issues (don't bypass)
2. Run `task lint:fix` for auto-fixes
3. Address test failures properly
4. Only commit after all checks pass

### Cross-Platform Support

**Platform Detection**:
```go
// pkg/platform/platform.go
type PlatformInfo struct {
    OS           string // linux, darwin, windows
    Distribution string // ubuntu, fedora, arch, etc.
    Version      string
    Desktop      string // gnome, kde, xfce, etc.
}

func DetectPlatform() PlatformInfo {
    // Auto-detect current platform
}

// Configuration selection
func (app *CrossPlatformApp) GetOSConfig() OSConfig {
    switch runtime.GOOS {
    case "linux":
        return app.Linux
    case "darwin": 
        return app.MacOS
    case "windows":
        return app.Windows
    default:
        return app.AllPlatforms // Fallback
    }
}
```

### Database & Migration System

**SQLite Schema Management**:
```sql
-- migrations/001_initial_schema.up.sql
CREATE TABLE apps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    version TEXT,
    install_method TEXT,
    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Repository Pattern**:
```go
type Repository interface {
    CreateApp(ctx context.Context, app *types.App) error
    GetApp(ctx context.Context, name string) (*types.App, error)
    ListApps(ctx context.Context) ([]*types.App, error)
    DeleteApp(ctx context.Context, name string) error
}
```

## Monorepo Structure & Development

### Workspace Management
- Use `pnpm` for workspace management and Node.js dependencies
- Use `task` for CLI development tasks (in `apps/cli/`)
- Each app has its own development workflow and dependencies

### Website Development (apps/web/)
```bash
cd apps/web
pnpm install    # Install dependencies
pnpm dev        # Development server
pnpm build      # Production build
```

### Documentation Development (apps/docs/)
```bash
cd apps/docs
pnpm install    # Install dependencies
pnpm start      # Development server  
pnpm build      # Static site build
```

### Workspace Commands (Root Level)
```bash
pnpm install         # Install all workspace dependencies
pnpm biome:format    # Format code across all apps
pnpm biome:lint      # Lint code across all apps
pnpm biome:check     # Check formatting and linting
```

## Quick Start & Key Files

### One-Line Setup
```bash
# Install DevEx
wget -qO- https://devex.sh/install | bash

# Development setup
git clone https://github.com/jameswlane/devex.git
cd devex/apps/cli
task install
```

### Essential Files to Understand
1. **`cmd/root.go`** - Cobra root command with PersistentPreRunE configuration
2. **`pkg/types/types.go`** - Core data structures and interfaces
3. **`pkg/config/`** - Viper configuration management 
4. **`config/*.yaml`** - Default application configurations
5. **`pkg/installers/`** - Platform-specific installer implementations
6. **`pkg/commands/`** - Business logic for CLI commands
7. **`internal/cli/`** - CLI-specific internal packages

### Current CLI Usage Examples

```bash
# Install development environment
devex install

# Install specific categories
devex install --categories development,databases

# System configuration
devex system apply

# List available applications
devex list apps

# Show configuration
devex config show

# Generate shell completion
devex completion bash > /etc/bash_completion.d/devex
```

## Platform Development Status

**Current Priority** (Linux-first approach):
1. ✅ **Debian/Ubuntu** (APT) - Production ready
2. 🚧 **Fedora/RHEL** (DNF) - In development  
3. 📋 **Arch Linux** (Pacman) - Planned
4. 📋 **SUSE** (Zypper) - Planned
5. 📋 **macOS** (Homebrew) - Planned
6. 📋 **Windows** (winget/chocolatey) - Future

## Adding New Features & Installers

### Adding New CLI Commands

**Using cobra-cli Generator (Recommended)**:
```bash
cd apps/cli

# Install cobra-cli if not present
go install github.com/spf13/cobra-cli@latest

# Generate a new command
cobra-cli add [command]

# Generate a child command  
cobra-cli add [child] -p [parent]

# Examples
cobra-cli add backup           # Creates backup command
cobra-cli add restore -p backup  # Creates backup restore subcommand
cobra-cli add config --config   # Add config management command
```

**Generated Command Benefits**:
- ✅ Follows enterprise Cobra patterns automatically
- ✅ Includes proper license headers and imports
- ✅ Pre-configured with RunE for error handling
- ✅ Placeholder help text and examples
- ✅ Consistent file structure and naming

**Post-Generation Steps**:
1. Implement business logic in the RunE function
2. Add appropriate flags with Viper bindings
3. Update help text and examples
4. Add comprehensive Ginkgo BDD tests
5. Register command in parent (if not done automatically)

**Manual Command Creation** (if needed):
1. Create new command in `apps/cli/pkg/commands/[command].go`
2. Implement proper Cobra command structure with RunE
3. Add comprehensive flag definitions and Viper bindings
4. Register in `apps/cli/pkg/commands/root.go`
5. Add robust error handling with actionable messages
6. Write Ginkgo BDD tests in `apps/cli/pkg/commands/[command]_test.go`
7. Include comprehensive examples and help text

### Adding New CLI Installers
1. Create new installer in `apps/cli/pkg/installers/[method]/`
2. Implement `Installer` interface from `apps/cli/pkg/types/types.go`
3. Register in `apps/cli/pkg/installers/installers.go`
4. Add robust error handling with actionable messages
5. Implement service/dependency validation before operations
6. Add fallback verification mechanisms for installation checks
7. Include specific guidance for common setup issues
8. Add comprehensive Ginkgo tests covering success and failure scenarios

### CLI Configuration Changes
1. Update type definitions in `apps/cli/pkg/types/types.go`
2. Add validation methods if needed
3. Update default configs in `apps/cli/config/` directory
4. Test with both default and override scenarios

### Testing New Features
1. **CLI**: Write Ginkgo BDD tests for complex behavior
2. **Website/Docs**: Follow Next.js testing conventions
3. Generate mocks for external dependencies
4. Test both dry-run and actual execution modes for CLI features

## Resources & Documentation

- **Live CLI docs**: https://docs.devex.sh/
- **Website**: https://devex.sh/
- **Configuration examples**: See `config/*.yaml` files
- **Testing examples**: See `pkg/commands/*_test.go` files
- **Installer examples**: See `pkg/installers/*/` directories

## Security & Quality Standards

### Security Requirements
- Use `exec.CommandContext` instead of `exec.Command` for all command execution
- Validate user inputs with minimal but essential security patterns
- Prevent directory traversal attacks in file operations
- Focus on functionality over restrictive security validation

### Error Handling Standards
- Provide actionable error messages with specific guidance
- Implement fallback mechanisms for critical operations
- Include hints for common resolution steps in error messages
- Log errors with structured context using the log package
- Validate system requirements before attempting operations

### Code Quality Requirements
- All code must pass `golangci-lint` with zero issues
- Follow Go naming conventions and documentation standards
- Use dependency injection for better testability
- Implement robust input validation for all public functions
- Structure error messages following Go conventions

## Cobra-CLI Setup & Configuration

**Initial Setup**:
```bash
cd apps/cli

# Install cobra-cli globally
go install github.com/spf13/cobra-cli@latest

# Initialize cobra-cli configuration (if not done)
cobra-cli init --pkg-name github.com/jameswlane/devex/apps/cli

# Configure license and author (optional)
cobra-cli config set license apache
cobra-cli config set author "James Lane <email@example.com>"
```

**Project Integration**:
- Generated commands automatically follow DevEx patterns
- License headers match project standards  
- Import paths use correct module structure
- Commands integrate with existing Viper configuration
- Follows established error handling patterns

**Best Practices with cobra-cli**:
- Generate commands first, then implement business logic
- Keep generated structure, customize implementation
- Add comprehensive tests after generation
- Update help text and examples for user clarity
- Use descriptive command and flag names

## Registry Database Migration Best Practices

### 🚨 CRITICAL: Production-Safe Database Migrations

When the registry goes to production, all database changes MUST be migration-safe and preserve existing data. Follow these rules:

#### Migration Safety Rules

**❌ NEVER DO IN PRODUCTION:**
- `prisma migrate reset` (wipes all data)
- `prisma db push --accept-data-loss` (can lose data)
- Dropping columns without migration strategy
- Changing column types without data conversion

**✅ ALWAYS DO FOR PRODUCTION:**
```bash
# 1. Create migration (never reset)
pnpm prisma migrate dev --name descriptive_name

# 2. Review migration SQL before applying
cat prisma/migrations/[timestamp]_descriptive_name/migration.sql

# 3. Test migration on staging database first
pnpm prisma migrate deploy --preview-feature

# 4. Apply to production only after staging validation
pnpm prisma migrate deploy
```

#### Safe Schema Change Patterns

**Adding New Columns:**
```sql
-- ✅ Safe: Add nullable column with default
ALTER TABLE plugins ADD COLUMN sdk_version VARCHAR(50);

-- ✅ Safe: Add non-null column with default value
ALTER TABLE plugins ADD COLUMN api_version VARCHAR(50) DEFAULT 'v1';
```

**Removing Columns (3-Step Process):**
```sql
-- Step 1: Deploy code that doesn't use the column (one release)
-- Step 2: Mark column as deprecated in schema comments (one release)
-- Step 3: Drop column after confirming no usage (next release)
ALTER TABLE plugins DROP COLUMN deprecated_field;
```

**JSON Schema Evolution:**
```sql
-- ✅ Safe: JSON field changes are backwards compatible
-- Adding new JSON fields doesn't break existing code
UPDATE plugins SET binaries = binaries || '{"checksums_v2": {}}'
WHERE binaries IS NOT NULL;
```

#### Registry Migration Checklist

Before deploying registry changes:

1. **Schema Changes**
   - [ ] Migration creates only additive changes
   - [ ] Default values provided for new non-null columns
   - [ ] Indexes added for new query patterns
   - [ ] No data loss operations

2. **Data Migration**
   - [ ] Existing data preserved and converted correctly
   - [ ] Migration tested on staging database copy
   - [ ] Rollback strategy documented

3. **API Compatibility**
   - [ ] Registry API endpoints remain backwards compatible
   - [ ] CLI registry.json format unchanged or versioned
   - [ ] Download URLs continue to work during migration

**Remember:** In production, database migrations are permanent and irreversible. Always err on the side of caution and test thoroughly.

# Important Reminders

- **Use cobra-cli for new commands** - ensures consistency and best practices
- **Use Ginkgo BDD for all tests** - no mixing with standard Go tests
- **Follow 12-Factor configuration patterns** - flags > env > config > defaults  
- **Always use RunE for commands** - proper error handling and exit codes
- **Implement context propagation** - use `cmd.Context()` throughout
- **Generate documentation automatically** - keep CLI docs in sync
- **Never skip quality gates** - pre-commit hooks are mandatory
- **Design for cross-platform** - Linux first, but plan for macOS/Windows
