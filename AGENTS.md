# Repository guidance

## Project layout

DevEx automates development environment setup. This is a pnpm workspace with
Turborepo orchestration, independent Go modules, and Next.js applications.

- `apps/cli`: Go CLI using Cobra and Viper; entry point is `cmd/main.go`.
  Application code lives almost entirely under `internal` (commands, config,
  installer, bootstrap, datastore, tui, security, and more). Embedded YAML
  configuration lives in `config`; database migrations live in `migrations`;
  the container-based E2E suite lives in `test/e2e`.
- `packages/plugin-sdk`: shared Go plugin framework plus `sdktest` helpers.
- `packages/package-manager-*`, `packages/desktop-*`, `packages/tool-*`, and
  `packages/system-setup`: independently built Go plugins.
- `apps/web`: Next.js marketing site and application catalog.
- `apps/docs`: Next.js documentation site using Fumadocs and MDX.
- `apps/registry`: Next.js plugin registry API with Prisma and PostgreSQL.
- `apps/get`: static installation scripts (`public/install`, `install.ps1`).
- `scripts/release` and `.github/workflows`: release automation and CI.

Use current manifests, task files, and source code to verify commands and paths;
some older documentation describes a previous layout. `CLAUDE.md` contains
useful design guidance but also references a `pkg/` directory structure
(`pkg/commands`, `pkg/installers`, `pkg/types`) that no longer exists — those
packages now live under `apps/cli/internal`. Trust `CLAUDE.md` for patterns
(Cobra/Viper conventions, Ginkgo style) but verify file locations against the
tree.

## Architecture: how the pieces fit together

- The CLI does not link plugins statically. Each package under `packages/` is
  its own Go module whose `main` package builds a standalone executable named
  `devex-plugin-<name>`. The CLI discovers, downloads (from the registry at
  `registry.devex.sh`), checksum-verifies, and executes these binaries.
- Every plugin declares `metadata.yaml` (name, type, platforms, capabilities,
  priority, SDK/API versions). Lower `priority` numbers win when multiple
  plugins can handle a task. The CLI reads plugin metadata over the boundary
  (e.g. invoking the binary with `--plugin-info`).
- Both the CLI and every plugin consume the SDK through local `replace`
  directives pointing at `packages/plugin-sdk`, so SDK edits take effect in
  dependent modules immediately — but that also means SDK changes require
  checking CLI and plugin consumers, not only SDK tests.
- Embedded CLI configuration in `apps/cli/config` (applications, environments,
  desktop, system, security, templates) feeds both the CLI and, through the
  Turbo `generate` task, the web catalog: `pnpm generate` regenerates
  `apps/web/app/generated` from `apps/cli/config/applications/**/*.yaml`.
  Never edit `apps/web/app/generated` by hand.
- Releases are per-component. Tags follow `vX.Y.Z` for the CLI and
  `packages/<name>/vX.Y.Z` for plugins and the SDK; `scripts/release/release-plan.mjs`
  derives versions from conventional commits touching each component. Plugin
  `.goreleaser.yml` files are generated — run `pnpm release:generate` /
  `pnpm release:check` rather than editing them. See `RELEASING.md`.

## Setup and commands

- Install workspace dependencies with `pnpm install`; preserve `pnpm-lock.yaml`.
- Use the pnpm version declared in root `package.json`. `.mise.toml` declares
  development tools, including Node 22 and Go 1.26; individual `go.mod` files
  declare module requirements. The mise pnpm pin currently differs from the
  package manifest; do not change either pin incidentally.
- Run Go commands from the relevant module directory. There is no root Go
  module; CLI and plugins use local `replace` directives for the SDK.
- Root `pnpm build`, `pnpm test`, and `pnpm lint` orchestrate workspace tasks.
  Turbo's test task depends on builds, so prefer focused checks while iterating.

Useful focused commands from the repository root:

```sh
pnpm --filter @devex/cli build
pnpm --filter @devex/cli test
pnpm --filter @devex/cli lint
pnpm --filter @devex/plugin-sdk test
pnpm --filter @devex/package-manager-apt test
pnpm --filter @devex/web test -- --runInBand
pnpm --filter @devex/registry test -- --runInBand
pnpm --filter @devex/registry typecheck
pnpm --filter @devex/docs build
pnpm release:test
pnpm release:check
```

Replace the example plugin name with the affected workspace. In `apps/cli`,
`task` runs lint and Ginkgo tests, `task build:local` builds the CLI, and
`task test` runs Ginkgo recursively while excluding `test/e2e`. The `task`
binary and the Ginkgo CLI are not provided by mise — install them separately,
matching the Ginkgo version in `apps/cli/go.mod` (see `apps/cli/TESTING.md`):

```sh
go install github.com/go-task/task/v3/cmd/task@latest
go install github.com/onsi/ginkgo/v2/ginkgo@v2.33.1
```

Inspect each plugin's task file before using it; task coverage varies between
plugins (typical tasks: `build`, `build:single`, `test`, `lint`,
`install:local`, `clean`).

## Testing

- Go tests use Ginkgo/Gomega alongside standard Go tests; suites follow the
  `<package>_suite_test.go` convention. `task test:short` skips specs labeled
  `slow`; `task test:coverage` produces `coverage.out`/`coverage.html`.
- Reuse SDK test helpers and mock command execution instead of installing
  software on the host.
- E2E tests (`apps/cli/test/e2e`) run the real CLI inside a disposable
  Ubuntu/Debian container: `task e2e:docker DISTRO=ubuntu-24.04` (Podman works
  with `CONTAINER_ENGINE=podman`). The suite refuses to run outside its
  sandbox (`DEVEX_E2E_SANDBOX=1` is mandatory) because it replaces `$HOME`,
  and destructive specs (which use sudo) only run in CI. See `TESTING.md` for
  the full variable list, including testing a released binary via
  `E2E_DEVEX_BINARY`.
- Files with a `.dangerous` suffix (e.g. `plugin_test.go.dangerous`) are
  disabled tests kept in-tree; without a `.go` extension they are not
  compiled. Do not re-enable or delete them casually.

## Gotchas

- Pre-commit hooks (lefthook) auto-fix staged files: `yamlfmt` for YAML,
  Biome for JSON/JS/TS, and `gofmt`/`goimports`/golangci-lint scoped per
  module root. Expect your staged files to be rewritten and re-staged.
  Commit messages must pass commitlint (Conventional Commits, lowercase
  subject).
- Pre-push hooks build, lint, and test only the packages changed since
  `HEAD~1`, so a first push after branch work can surface failures from any
  commit in the branch.
- The pnpm version pin in `.mise.toml` differs from `packageManager` in root
  `package.json`; both are intentional — do not reconcile them incidentally.
- Generated outputs to never hand-edit: `apps/web/app/generated` (via
  `pnpm generate`), plugin `.goreleaser.yml` (via `pnpm release:generate`),
  and CLI docs under `apps/cli/docs` (via the docgen tool).
- Environment variables use the `DEVEX_` prefix with `.`/`-` mapped to `_`.
  Configuration precedence is flags > environment > config file > defaults.

## Implementation conventions

- Follow existing code and tests in the affected package. Keep changes focused
  and avoid unrelated dependency upgrades or repository-wide formatting.
- Format Go with `gofmt`; use the module's golangci-lint configuration.
- For CLI work, preserve configuration precedence: flags, environment variables,
  configuration files, then defaults. Propagate command contexts and return
  errors through Cobra's `RunE` pattern.
- Keep shared plugin behavior in the SDK where appropriate. SDK changes require
  checking affected CLI and plugin consumers, not only SDK tests.
- Use Biome for supported frontend files; `biome.json` configures tabs and double
  quotes. `.editorconfig` supplies general whitespace defaults. Root
  `biome:format`, `biome:lint`, and `biome:check` scripts write changes; scope
  formatting to touched files, or use `pnpm exec biome check <files>` to inspect.
- Update relevant documentation and configuration examples when behavior changes.
  Regenerate derived files through their existing scripts rather than editing
  generated output manually.
- Keep secrets out of source and logs. Use local environment configuration for
  registry services; inspect its README and Prisma scripts before database work.
  Registry schema changes must be additive and migration-safe (see the
  migration rules in `CLAUDE.md`) — never reset or force-push a production
  database.

## Validation and delivery

Run checks appropriate to the affected component and add regression coverage for
behavior changes. For documentation-only changes, check accuracy and diff
whitespace; application tests are unnecessary. Report checks run and any blockers
without claiming unexecuted checks passed.

Use Conventional Commits with lowercase subjects, as configured in
`commitlint.config.js`, for example `fix(cli): preserve config overrides`.
Consult `RELEASING.md` for release changes and `SECURITY.md` for vulnerability
reporting. Leave changes reviewable for the human review described in
`AI_USAGE.md`; initializing or editing code does not imply publishing a release.
