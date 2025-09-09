# Automated Changesets System

This repo uses **automated** [Changesets](https://github.com/changesets/changesets) to manage versioning and releases.

## How It Works (Fully Automated!)

1. **Push to main** - Any push to main with changes in `packages/**` or `apps/cli/**`
2. **Auto-changeset creation** - The system automatically detects what changed and creates appropriate changesets
3. **Version PR created** - Changesets Action creates a "🚀 Release" PR with version bumps
4. **Merge to release** - When you merge the PR, releases are automatically created with proper tags
5. **GoReleaser triggered** - The tags automatically trigger GoReleaser to build and publish

## What's Managed vs Excluded

### ✅ Managed by Changesets
- `@devex/cli` - The main DevEx CLI (creates tags like `v1.2.3`)
- `@devex/plugin-sdk` - Plugin SDK (creates tags like `plugin-sdk/v1.2.3`) 
- All plugins in `packages/` (creates tags like `plugin-package-manager-apt-v1.2.3`)

### ❌ Excluded (Vercel handles these)
- `@devex/docs` - Documentation site
- `@devex/web` - Marketing website
- `@devex/registry` - Plugin registry

## Semantic Versioning (Automatic Detection)

The system automatically determines version bumps based on commit messages:

- **Major (breaking)**: Commits with `BREAKING CHANGE`, `breaking:`, or `!:`
- **Minor (feature)**: Commits starting with `feat`
- **Patch (default)**: All other commits

## Tag Format

- **CLI**: `v1.2.3`
- **Plugin SDK**: `plugin-sdk/v1.2.3`  
- **Plugins**: `plugin-{name}-v1.2.3`

## Workflows

### 1. `auto-changeset.yml`
- Triggers on pushes to main with relevant changes
- Detects which packages changed
- Automatically creates changesets with appropriate version bumps
- Commits and pushes the changeset

### 2. `release.yml` 
- Triggers on all pushes to main
- Uses Changesets Action to create/update version PRs
- When changesets exist: Creates "🚀 Release" PR
- When version PR is merged: Creates tags and triggers GoReleaser

### 3. `goreleaser.yml` (existing)
- Triggers on tag pushes
- Builds and publishes actual releases

## Manual Override (if needed)

If you need to manually create a changeset:

```bash
pnpm changeset
```

The system will skip auto-generation if changesets already exist.

## Developer Experience

As a developer, you just:

1. Make your changes to CLI or plugins
2. Push to main  
3. Everything else is automated!

The system will:
- Detect your changes
- Create appropriate version bumps
- Generate changelog entries
- Create release PRs
- Build and publish on merge

No manual intervention required! 🚀
