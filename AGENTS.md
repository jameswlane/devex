# Contributor & Agent Guide

Welcome! This guide covers how human contributors and AI agents (including ChatGPT, Claude, Gemini, and CODEX) should work within this repository. Please read it fully before contributing or using automated agents.

---

## Project Layout & Key Files

**Monorepo Structure:**
- `apps/cli/` – DevEx CLI tool (Go)
    - `apps/cli/cmd/` – CLI entrypoints
    - `apps/cli/pkg/` – Core libraries, business logic
    - `apps/cli/config/` – Default configurations
    - `apps/cli/assets/` – Static resources (themes, defaults)
    - `apps/cli/test/` – Test utilities and fixtures
- `apps/web/` – Website (Next.js)
    - `apps/web/app/` – Next.js app router pages and components
    - `apps/web/public/` – Static assets
- `apps/docs/` – Documentation site (MDX + Next.js)
    - `apps/docs/pages/` – MDX documentation pages
- `packages/` – Shared packages (future use)

**Root-level files:**
- `README.md` – Main project overview
- `CLAUDE.md`, `IMPROVEMENTS.md` – LLM/automation-focused documentation
- `pnpm-workspace.yaml` – Workspace configuration
- `.github/` – CI/CD and workflow configuration

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

## Codebase Migration

The codebase has been refactored into a monorepo structure. See `IMPROVEMENTS.md` for ongoing/refactor efforts.
**Agents:** Always work within the appropriate app directory (`apps/cli/`, `apps/web/`, `apps/docs/`). For CLI development, prefer working within the `apps/cli/pkg/` structure & modern idioms when adding or modifying features.

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
  - **Documentation**: Look in `apps/docs/pages/`, `apps/docs/components/`
- **Documentation:** Update or create docs in the appropriate app directory or root-level docs.
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

_Last updated: 2025-07-26. Please keep this file up to date as conventions or automation changes!_
