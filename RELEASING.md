# DevEx Release Process

Every component of the monorepo is released on its own with GoReleaser Pro, routed by tag prefix:

| Component | Tag | Config |
| --- | --- | --- |
| CLI | `vX.Y.Z` | `apps/cli/.goreleaser.yml` |
| Plugin SDK | `packages/plugin-sdk/vX.Y.Z` | `packages/plugin-sdk/.goreleaser.yml` |
| Plugin | `packages/<name>/vX.Y.Z` | `packages/<name>/.goreleaser.yml` (generated) |

Only CLI releases become GitHub's "Latest" release; plugin and SDK configs set `make_latest: false`.

## How a release flows

1. **Tag**: `release components` (`.github/workflows/release-components.yml`) runs
   `scripts/release/release-plan.mjs`. For each component it finds the last tag, looks at
   conventional commits touching that component's directory since then, and picks the next version
   (`feat` = minor, `fix` and others = patch, `!` or `BREAKING CHANGE` = major, or minor while on 0.x).
2. **Build and publish**: `goreleaser` (`.github/workflows/goreleaser.yml`) checks out the tag and runs
   GoReleaser Pro with the component's config. It signs the checksum file with keyless cosign, publishes
   the GitHub release, then runs GoReleaser's `verify` step against the published assets.
3. **Registry**: for plugin tags, the workflow dispatches `Sync Registry Database` with the tag. The sync
   records the release's binaries and checksums in the registry, never moving a plugin to an older
   version. A nightly run re-scans every plugin tag to repair anything missed.
4. **CLI download**: the CLI asks the registry for the plugin, downloads the asset, verifies its checksum,
   and installs the executable (extracting it when the asset is an archive).

Each plugin release contains, per platform, a `tar.gz` (`zip` on Windows) and a raw executable. The registry
points at the raw executable, which every CLI version can install.

## Releasing

### From GitHub (recommended)

Run **Actions → release components → Run workflow**:

- `dry_run` (default on) only prints the plan.
- `only` limits the run to some components, e.g. `cli,package-manager-apt`.

With `dry_run` off, it tags `main` and releases each tag, up to 4 at a time.

### By hand

```bash
pnpm release:plan                                # what would be released
node scripts/release/release-plan.mjs --only package-manager-apt --tag
git push origin packages/package-manager-apt/v0.0.2
```

A pushed tag starts `goreleaser` on its own.

### Re-running a failed release

Run **Actions → goreleaser → Run workflow** with the existing tag (e.g. `v0.0.4`). The plugin's registry
entry is refreshed when the run succeeds.

## Plugin GoReleaser configs are generated

Do not edit `packages/<name>/.goreleaser.yml`. They are written by
`scripts/release/generate-goreleaser.mjs` from each plugin's `metadata.yaml`, whose `platforms` decide the
build targets:

```yaml
platforms:
  linux:
    architectures: [amd64, arm64]
  darwin:
    architectures: [amd64, arm64]
```

After changing `metadata.yaml` or the generator:

```bash
pnpm release:generate   # rewrite configs
pnpm release:check      # what CI runs: fails if any config is stale
```

A new plugin needs a `metadata.yaml` with `name` equal to its directory name; then run `pnpm release:generate`.

## Local testing

GoReleaser must run from the **repository root**, with `-f` pointing at the component's config. Snapshot
builds need no Pro key.

```bash
goreleaser check -f apps/cli/.goreleaser.yml
goreleaser release --snapshot --clean --skip=sign,verify -f apps/cli/.goreleaser.yml
goreleaser release --snapshot --clean --skip=sign,verify -f packages/package-manager-apt/.goreleaser.yml
```

The `release config` workflow runs the same checks on pull requests.

## Troubleshooting

- **Wrong previous tag or changelog**: every config needs a `monorepo.tag_prefix`, so GoReleaser only
  compares tags of the same component.
- **Missing files in the CLI archive**: with `monorepo.dir` set, archive `files` are relative to that
  directory (`apps/cli`).
- **Plugin missing in the registry**: check the `Sync Registry Database` run for the tag. Re-run it with the
  tag as input.
- **CGO**: every build uses `CGO_ENABLED=0`. The CLI uses pure-Go SQLite (`modernc.org/sqlite`); keep new
  dependencies CGO-free.

## Resources

- [GoReleaser Pro monorepo](https://goreleaser.com/customization/monorepo/)
- [GoReleaser verify](https://goreleaser.com/customization/verify/)
- [Semantic Versioning](https://semver.org/)
