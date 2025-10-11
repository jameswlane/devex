# DevEx Release Process

This document describes how to release different components of the DevEx monorepo using GoReleaser.

## Architecture

DevEx uses the official [GoReleaser monorepo pattern](https://goreleaser.com/customization/monorepo/) with tag-based routing:

- **CLI**: `v*` → `apps/cli/.goreleaser.yml`
- **SDK**: `packages/plugin-sdk/v*` → `packages/plugin-sdk/.goreleaser.yml`
- **Plugins**: `packages/[name]/v*` → `packages/[name]/.goreleaser.yml`

## Quick Start

### CLI Release

```bash
# Tag and push
git tag -a v0.3.0 -m "Release v0.3.0"
git push origin v0.3.0

# GitHub Actions will automatically:
# 1. Build binaries for all platforms
# 2. Create archives
# 3. Generate changelog
# 4. Create GitHub release
# 5. Upload assets
```

### SDK Release

```bash
# Tag and push
git tag -a packages/plugin-sdk/v0.1.0 -m "SDK Release v0.1.0"
git push origin packages/plugin-sdk/v0.1.0

# GitHub Actions will create the release
# (SDK is a Go module, no binaries built)
```

### Plugin Release

```bash
# Tag and push
git tag -a packages/desktop-gnome/v0.1.0 -m "GNOME plugin v0.1.0"
git push origin packages/desktop-gnome/v0.1.0

# GitHub Actions will build plugin binaries and create release
```

## Local Testing

Before pushing tags, test locally:

### Test Build (No Publishing)

```bash
# CLI
cd apps/cli
goreleaser release --snapshot --clean

# SDK
cd packages/plugin-sdk
goreleaser release --snapshot --clean

# Plugin
cd packages/desktop-gnome
goreleaser release --snapshot --clean
```

### Test Single-Target Build

```bash
# Build for your current platform only (fast)
goreleaser build --snapshot --clean --single-target

# Build for specific platform
GOOS=linux GOARCH=amd64 goreleaser build --single-target
```

### Validate Config

```bash
# Check configuration is valid
goreleaser check
```

## Automation

### Lefthook Pre-Push Hook

When you push a tag, lefthook will detect it and show a reminder:

```
🏷️  Tag push detected: v0.3.0
📦 GitHub Actions will run goreleaser automatically
💡 Or run locally: goreleaser release --clean
```

### GitHub Actions Workflow

The `.github/workflows/goreleaser.yml` workflow:

1. Triggers on tag push matching patterns
2. Routes to appropriate `.goreleaser.yml` config
3. Runs GoReleaser with GitHub token and Pro license
4. Creates release and uploads assets

## Version Strategy

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### CLI Versioning

```bash
v1.0.0  # Major release
v1.1.0  # Minor update
v1.1.1  # Patch/bugfix
```

### SDK Versioning

```bash
packages/plugin-sdk/v0.1.0  # Initial release
packages/plugin-sdk/v0.2.0  # New features
packages/plugin-sdk/v0.2.1  # Bug fixes
```

### Plugin Versioning

```bash
packages/desktop-gnome/v0.1.0  # Plugin initial release
packages/desktop-gnome/v0.2.0  # New features
```

## Creating a New Plugin

1. Copy the template:

```bash
cp packages/.goreleaser.template.yml packages/my-plugin/.goreleaser.yml
```

2. Replace `PLUGIN_NAME` with your plugin name throughout the file

3. Customize build settings if needed

4. Validate:

```bash
cd packages/my-plugin
goreleaser check
```

## Troubleshooting

### Build Fails Locally

```bash
# Check Go version matches CI
go version

# Ensure dependencies are up to date
go mod tidy

# Check for uncommitted changes
git status
```

### Wrong Config Used

The workflow routes based on tag prefix. Ensure:

- CLI tags: `v*` (no prefix)
- SDK tags: `packages/plugin-sdk/v*`
- Plugin tags: `packages/[name]/v*`

### Release Already Exists

GoReleaser uses `mode: append` for CLI and SDK, `mode: replace` for plugins.

To force replace:

```bash
# Delete the release first
gh release delete v0.3.0

# Then re-run
goreleaser release --clean
```

### Schema Validation Errors

All configs include:

```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema-pro.json
```

This enables IDE validation. Run:

```bash
goreleaser check
```

## Manual Release Process

If CI is unavailable:

```bash
# 1. Set GitHub token
export GITHUB_TOKEN="your_token_here"
export GORELEASER_KEY="your_pro_key_here"

# 2. Create tag
git tag -a v0.3.0 -m "Release v0.3.0"

# 3. Run GoReleaser
goreleaser release --clean

# 4. Push tag (if successful)
git push origin v0.3.0
```

## Resources

- [GoReleaser Docs](https://goreleaser.com/)
- [GoReleaser Pro Monorepo Docs](https://goreleaser.com/customization/monorepo/)
- [Semantic Versioning](https://semver.org/)
- [DevEx Contributing Guide](CONTRIBUTING.md)

## Support

Questions? Issues?

- Open an issue: https://github.com/jameswlane/devex/issues
- Discussion: https://github.com/jameswlane/devex/discussions
