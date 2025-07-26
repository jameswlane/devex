# Contributor & Agent Guide

Welcome! This guide covers how human contributors and AI agents (including ChatGPT, Claude, Gemini, and CODEX) should work within this repository. Please read it fully before contributing or using automated agents.

---

## Project Layout & Key Files

- Most code is under:
    - `/cmd` – App/service entrypoints
    - `/pkg` – Core libraries, business logic
    - `/config` – Configuration files
    - `/assets` – Static resources (themes, images)
    - `/test` – Test utilities and fixtures
- Docs and contribution guidelines:
    - `README.md` – Main project overview
    - `CLAUDE.md`, `IMPROVEMENTS.md` – LLM/automation-focused documentation and custom rules for Claude
    - `.github/` – CI/CD and workflow configuration

> **Agents:** Work only within the relevant folders for the assigned feature/bug. Prefer contributing to `/pkg` for business logic or `/cmd` for app entrypoints unless directed elsewhere.

---

## Contribution & Style Guidelines

- Follow code style enforced by:
    - ESLint (`pnpm lint`)
    - Prettier (`pnpm format`)
    - GoLint/GoFmt for Go (`make lint` or as per CI)
- Commit messages should use Conventional Commits via commitlint.
- Always update/add tests (`/test` or relevant package subdir) for any behavior change.
- Documentation must be updated if the code change warrants it.
- Follow the structure of PR titles: `[<project|package_name>] <Title>`

---

## Codebase Migration

Parts of the codebase may be migrating from legacy layouts or languages. See `IMPROVEMENTS.md` for ongoing/refactor efforts.
**Agents:** Prefer working within the new `/pkg` structure & modern idioms when adding or modifying features.

---

## Change Validation

- **Lint:**
  Run `pnpm lint --filter <project>` or `make lint` for Go code.
- **Unit Tests:**
    - JS/TS: `pnpm test --filter <project>`
    - Go: `go test ./...` from the relevant directory
- **CI:**
  All PRs are checked via workflows in `.github/workflows`.
  **Agents:** Never merge if test suite or linters are failing.

---

## How Agents Should Work

- **File Scope:** Respect the most specific (nested) `AGENTS.md` if multiple exist. Default to the closest root.
- **Exploration:** When exploring for relevant code context, look for matching terms in `/pkg`, `/cmd`, or `/test` based on the prompt.
- **Documentation:** Update or create docs in `/docs` or near the code changed, as appropriate.
- **PRs:**
    - Use clear, conventional commit titles (`[package] Short summary`)
    - Summarize all changes with context in the PR body
    - List verification steps (lint, test) and confirm status, e.g. "All tests pass locally"
- **Formatting:** Adhere to Prettier/ESLint for JS/TS; GoFmt for Go code.
- **Safety:** Never leak secrets or credentials.
  Log only non-sensitive build/test output in commit comments and PRs.

---

## Prompting Agent Tools (e.g., CODEX, ChatGPT, Claude, Gemini)

**Give clear code pointers:**
Include full filenames, stack traces, or snippet blocks where possible to focus the search.

**Include verification:**
State expected outcomes, how to validate (tests, linters), and setup steps if needed.

**Customize approach:**
Specify if special commit templates, logs, or forbidden/required commands (e.g., use `pnpm` not `npm`) should be used.

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
