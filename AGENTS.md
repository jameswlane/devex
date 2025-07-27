# Contributor & Agent Guide

Welcome! This guide covers how human contributors and AI agents (including ChatGPT, Claude, Gemini, and CODEX) should work within this repository. Please read it fully before contributing or using automated agents.

---

## Project Layout & Key Files

**Monorepo Structure:**
- `apps/cli/` – DevEx CLI tool (Go)
    - `apps/cli/cmd/` – CLI entrypoints
    - `apps/cli/pkg/` – Core libraries, business logic
    - `apps/cli/config/` – 4 consolidated configuration files (applications.yaml, environment.yaml, desktop.yaml, system.yaml)
    - `apps/cli/assets/` – Static resources (themes, defaults)
    - `apps/cli/migrations/` – Database schema migrations
- `apps/web/` – Website (Next.js)
    - `apps/web/app/` – Next.js app router pages and components
    - `apps/web/public/` – Static assets
- `apps/docs/` – Documentation site (Fumadocs + Next.js)
    - `apps/docs/content/docs/` – MDX documentation pages
    - `apps/docs/app/` – Next.js app components
- `packages/` – Shared packages (future use)

**Root-level files:**
- `README.md` – Main project overview
- `CLAUDE.md` – Claude Code specific documentation and architecture guide
- `ROADMAP.md` – Project roadmap and development priorities (consolidated from IMPROVEMENTS.md)
- `pnpm-workspace.yaml` – Workspace configuration
- `.github/` – CI/CD and workflow configuration

**Important URLs:**
- **GitHub**: https://github.com/jameswlane/devex
- **Website**: https://devex.sh/
- **Documentation**: https://docs.devex.sh/
- **One-line installer**: `wget -qO- https://devex.sh/install | bash`

> **Agents:** Always work within the appropriate app directory (`apps/cli/`, `apps/web/`, or `apps/docs/`). For CLI development, prefer contributing to `apps/cli/pkg/` for business logic or `apps/cli/cmd/` for entrypoints.

---

## Contribution & Style Guidelines

**Code Style Enforcement:**
- **Root/Workspace Level**: Biome for formatting and linting (`pnpm biome:format`, `pnpm biome:lint`)
- **CLI (Go)**: GoLint/GoFmt via Task (`cd apps/cli && task lint`)
- **Web/Docs (TypeScript/React)**: Biome for TypeScript, app-specific configurations

**Development Commands:**
- **CLI**: Use Task commands (`cd apps/cli && task <command>`)
- **Web/Docs**: Use pnpm commands (`cd apps/web && pnpm <command>`)
- **Workspace**: Use pnpm workspace commands from root (`pnpm <command>`)

**Testing and Documentation:**
- Always update/add tests in the appropriate app directory
- CLI tests: `apps/cli/pkg/` subdirectories using Ginkgo or standard Go tests
- Web/Docs tests: Follow Next.js testing conventions
- Documentation must be updated if the code change warrants it

**Commit Guidelines:**
- Use Conventional Commits via commitlint
- Format: `[<app_name>] <type>: <description>` (e.g., `[cli] feat: add uninstall command`)
- For cross-app changes: `[workspace] <type>: <description>`

---

## Recent Major Changes (2025-01)

The codebase has undergone significant improvements:

1. **Configuration Consolidation**: Reduced 11 config files to 4 structured files in `apps/cli/config/`
2. **Cross-Platform Architecture**: Modern type system supporting Linux, macOS, Windows
3. **Code Cleanup**: Removed dead code, obsolete test files, improved maintainability  
4. **Enhanced CLI**: Added uninstall command, comprehensive dry-run support
5. **Documentation**: Complete configuration guides at https://docs.devex.sh/
6. **One-Line Installer**: Production-ready installer at https://devex.sh/install

**Current Focus**: DNF installer implementation for Red Hat-based Linux systems

**Agents:** Always work within the appropriate app directory (`apps/cli/`, `apps/web/`, `apps/docs/`). For CLI development, understand the consolidated configuration system and cross-platform architecture.

---

## Change Validation

**Linting:**
- **Root/Workspace**: `pnpm biome:lint` and `pnpm biome:check`
- **CLI**: `cd apps/cli && task lint`
- **Web**: `cd apps/web && pnpm lint` (if configured)
- **Docs**: `cd apps/docs && pnpm lint` (if configured)

**Unit Tests:**
- **CLI**: `cd apps/cli && task test` (includes Ginkgo and standard Go tests)
- **Web**: `cd apps/web && pnpm test` (if configured)
- **Docs**: `cd apps/docs && pnpm test` (if configured)

**CI:**
All PRs are checked via workflows in `.github/workflows`.
**Agents:** Never merge if test suite or linters are failing.

---

## How Agents Should Work

- **File Scope:** Work within the appropriate app directory. Respect app-specific configurations and workflows.
- **Exploration:** When exploring for relevant code context:
  - **CLI Development**: Look in `apps/cli/pkg/`, `apps/cli/cmd/`, or `apps/cli/test/`
  - **Website Development**: Look in `apps/web/app/`, `apps/web/components/`
  - **Documentation**: Look in `apps/docs/content/docs/`, `apps/docs/app/components/`
- **Documentation:** Update or create docs in `apps/docs/content/docs/` for user-facing documentation, or update `CLAUDE.md`/`ROADMAP.md` for development documentation.
- **PRs:**
    - Use clear, conventional commit titles (`[app_name] type: short summary`)
    - Examples: `[cli] feat: add uninstall command`, `[web] fix: navigation bug`, `[workspace] chore: update dependencies`
    - Summarize all changes with context in the PR body
    - List verification steps (lint, test) and confirm status
- **Formatting:**
  - Use Biome for workspace-level and TypeScript/React formatting
  - Use GoFmt/Task for CLI Go code
  - Follow app-specific conventions
- **Safety:** Never leak secrets or credentials.
  Log only non-sensitive build/test output in commit comments and PRs.

---

## Prompting Agent Tools (e.g., CODEX, ChatGPT, Claude, Gemini)

**Give clear code pointers:**
Include full filenames, stack traces, or snippet blocks where possible to focus the search.

**Include verification:**
State expected outcomes, how to validate (tests, linters), and setup steps if needed.

**Customize approach:**
Specify if special commit templates, logs, or forbidden/required commands should be used:
- Use `pnpm` not `npm` for workspace management
- Use `task` for CLI development commands
- Always specify which app directory to work in (`apps/cli/`, `apps/web/`, `apps/docs/`)

**Split complex work:**
Large tasks should be broken into smaller, self-testable PRs or commits.

**Leverage debugging:**
Paste errors or logs for root cause analysis, citing relevant files/folders.

**Try open-ended improvements:**
Ask agents to refactor, find bugs, or brainstorm solutions for tricky code.

---

## Security Best Practices

- **Account Security:**
  Agents and contributors should always use MFA for source code platform accounts.
- **Access:**
  API keys and secrets must **never** be pushed to the repository.
- **SSO:**
  If using SSO, org admins must enforce MFA for all users.
- **Multiple Login:**
  If any login is with email+password, MFA is mandatory.

---

*For organization-wide or agent-specific rules, see additional docs like `CLAUDE.md` or `.currsorrules` if present.*

---

---

## Key Configuration Files (CLI)

Since the major consolidation, understand these 4 core configuration files in `apps/cli/config/`:

1. **applications.yaml** - All application definitions with cross-platform support
   - Organized by: development, databases, system_tools, optional
   - Each app can define linux/macos/windows specific configurations
   - Supports multiple install methods: apt, dnf, pacman, brew, winget, etc.

2. **environment.yaml** - Programming languages, fonts, shell configurations
   - Uses mise for language version management (Node.js, Python, Go, etc.)
   - Font installation and management
   - Shell customization (zsh, oh-my-zsh, etc.)

3. **desktop.yaml** - Desktop environment settings by type
   - GNOME: themes, extensions, keybindings, settings
   - KDE: plasma configuration
   - macOS: dock and system defaults

4. **system.yaml** - Git, SSH, terminal configurations
   - Git aliases and global settings
   - SSH host configurations
   - Terminal profiles and preferences

**Configuration System Features:**
- User overrides in `~/.devex/` take priority over defaults
- Built-in validation via `pkg/config/validation.go`
- Cross-platform type system in `pkg/types/types.go`
- Platform detection in `pkg/platform/platform.go`

---

_Last updated: 2025-01-27. Please keep this file up to date as conventions or automation changes!_
