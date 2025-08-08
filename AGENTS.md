# Repository Guidelines

## Project Structure & Module Organization
- Monorepo apps:
  - `apps/cli/` (Go): `cmd/` entrypoints, `pkg/` core logic, `config/` consolidated YAML (`applications.yaml`, `environment.yaml`, `desktop.yaml`, `system.yaml`), `assets/`, `migrations/`.
  - `apps/web/` (Next.js): `app/` routes/components, `public/` assets.
  - `apps/docs/` (Fumadocs + Next.js): `content/docs/` MDX, `app/` components.
- Shared workspace config at root: `pnpm-workspace.yaml`, CI in `.github/`.
- Tests live next to code (e.g., `apps/cli/pkg/<pkg>/*_test.go`).

## Build, Test, and Development Commands
- Workspace tooling:
  - `pnpm biome:lint` / `pnpm biome:check` / `pnpm biome:format` – TS/JS lint, check, format.
- CLI (Go):
  - `cd apps/cli && task build` – build CLI.
  - `cd apps/cli && task test` – run Ginkgo/std Go tests.
  - `cd apps/cli && task lint` – gofmt/golint and static checks.
- Web/Docs:
  - `cd apps/web && pnpm dev|build|start` – Next.js app.
  - `cd apps/docs && pnpm dev|build|start` – Docs site.

## Coding Style & Naming Conventions
- TypeScript/React: Biome-enforced; prefer camelCase for vars, PascalCase for components; 2-space indentation.
- Go: idiomatic Go (gofmt), package names lower_snake, exported names PascalCase; keep files focused per package.
- Keep cross-platform logic aligned with `pkg/platform` and types in `pkg/types`.

## Testing Guidelines
- CLI: Write unit tests in `apps/cli/pkg/...` using Ginkgo or `testing`. Name files `*_test.go` and table-test where practical.
- Web/Docs: Use framework conventions if configured. Keep tests colocated.
- Run: `cd apps/cli && task test` or `pnpm test` in web/docs apps. Aim for meaningful coverage around business logic and installers.

## Commit & Pull Request Guidelines
- Conventional Commits with app scope: `[cli] feat: add uninstall command`, `[web] fix: navigation bug`, `[workspace] chore: update deps`.
- PRs: clear description, scope, linked issues, screenshots where UI changes, updated docs, and verification steps (lint + tests). CI must pass before merge.

## Agent-Specific & Security Notes
- Work within the target app dir; prefer `apps/cli/pkg/` for logic and `apps/cli/cmd/` for entrypoints.
- Never commit secrets; follow MFA. User overrides live in `~/.devex/` and take precedence.
- Validate with: `pnpm biome:check`, `cd apps/cli && task lint && task test` before submitting.

