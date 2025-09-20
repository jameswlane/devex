# Plugin Release System

## Overview

The DevEx project uses an automated release system based on **conventional commits** and **package change detection**. This provides **zero-maintenance** releases that trigger automatically when changes are made to plugins, apps, or the CLI.

## Current Release Architecture

The system has evolved through multiple iterations to achieve the current streamlined approach:

1. **Changesets** → Too complex, required manual changeset creation
2. **Individual Plugin Releases** → Over-engineered, created too many releases
3. **Automated Conventional Commits** → Current solution: Simple, automated, scalable

## How It Works

### Automated Release Trigger
1. **Conventional Commit**: Developer makes commit with conventional format (`feat:`, `fix:`, `BREAKING CHANGE:`)
2. **Change Detection**: GitHub Actions detects which packages changed based on file paths
3. **Version Calculation**: Determines version bump type (major/minor/patch) from commit types
4. **Tag Creation**: Creates versioned tags for changed packages (`@devex/package@version`)
5. **GoReleaser Trigger**: Individual GoReleaser workflows build and release changed packages

### Supported Commit Types
- `feat:` → **minor** version bump (new features)
- `fix:` → **patch** version bump (bug fixes) 
- `BREAKING CHANGE:` or `!:` → **major** version bump (breaking changes)
- `chore:`, `docs:`, `style:`, `refactor:` → No release triggered

### Package Detection
The system automatically detects changes in:
- **CLI**: `apps/cli/` → `@devex/cli`
- **Plugin SDK**: `packages/plugin-sdk/` → `@devex/plugin-sdk`
- **Plugins**: `packages/*` → `@devex/{package-name}`
- **Apps**: Docs and web apps (versioned but not released)

## File Structure

```
devex/
├── .autorc                           # Auto configuration for conventional commits
├── .github/workflows/
│   └── release.yml                   # Main automated release workflow
├── apps/
│   ├── cli/                         # DevEx CLI application
│   │   ├── package.json             # Version tracking
│   │   └── .goreleaser.yml          # CLI binary releases
│   └── docs/                        # Documentation site
├── packages/
│   ├── plugin-sdk/                  # Plugin SDK
│   │   ├── package.json             # Version tracking
│   │   └── .goreleaser.yml          # SDK releases
│   ├── package-manager-*/           # Package manager plugins
│   │   ├── package.json             # Version tracking
│   │   └── .goreleaser.yml          # Individual plugin releases
│   └── [other plugins]/             # Desktop, tool plugins, etc.
└── scripts/
    └── generate-goreleaser.js        # GoReleaser config generator
```

## Release Workflow Details

### Automated Release Process

#### 1. Commit Analysis
```bash
# Get commits since last tag
COMMITS=$(git log $LAST_TAG..HEAD --pretty=format:"%s" --no-merges | grep -E "^(feat|fix|perf|refactor)(\([^)]+\))?: ")

# Determine version bump type
if echo "$COMMITS" | grep -q "BREAKING CHANGE\|!:"; then
  BUMP_TYPE="major"
elif echo "$COMMITS" | grep -q "^feat"; then
  BUMP_TYPE="minor"
else
  BUMP_TYPE="patch"
fi
```

#### 2. Package Change Detection
```bash
# Check file changes since last tag
CHANGED_FILES=$(git diff --name-only $LAST_TAG..HEAD)

# Map file paths to package names
if echo "$CHANGED_FILES" | grep -q "^apps/cli/"; then
  PACKAGES_TO_RELEASE="$PACKAGES_TO_RELEASE @devex/cli"
fi

if echo "$CHANGED_FILES" | grep -q "^packages/plugin-sdk/"; then
  PACKAGES_TO_RELEASE="$PACKAGES_TO_RELEASE @devex/plugin-sdk"
fi

# Check each plugin package
for dir in packages/*/; do
  if echo "$CHANGED_FILES" | grep -q "^$dir"; then
    PACKAGE_NAME=$(basename "$dir")
    PACKAGES_TO_RELEASE="$PACKAGES_TO_RELEASE @devex/$PACKAGE_NAME"
  fi
done
```

#### 3. Version Calculation & Tagging
```bash
# Calculate new version from current version + bump type
NEW_VERSION="$major.$((minor + 1)).0"  # Example for minor bump

# Create GitHub tag
TAG="@devex/cli@$NEW_VERSION"
git tag -a "$TAG" -m "Release DevEx CLI $NEW_VERSION"
git push origin "$TAG"

# Update package.json
jq ".version = \"$NEW_VERSION\"" apps/cli/package.json > apps/cli/package.json.tmp
mv apps/cli/package.json.tmp apps/cli/package.json
```

## Package Types & GoReleaser Configuration

### DevEx CLI (`apps/cli/`)
- **Platforms**: Windows, macOS, Linux (multiple architectures)
- **Output**: Single `devex` binary per platform
- **Tag Pattern**: `@devex/cli@*`
- **Release Naming**: `DevEx CLI v{version}`

### Plugin SDK (`packages/plugin-sdk/`)
- **Type**: Go library (no binaries)
- **Tag Pattern**: `@devex/plugin-sdk@*`
- **Purpose**: Shared code for plugin development

### Desktop Environment Plugins
- **Platforms**: Linux only
- **Examples**: `desktop-gnome`, `desktop-kde`, `desktop-xfce`
- **Output**: `devex-plugin-{name}` binary
- **Tag Pattern**: `@devex/desktop-{name}@*`

### Package Manager Plugins
- **Platform Specific**: `package-manager-apt` (Linux), `package-manager-brew` (macOS/Linux)
- **Universal**: `package-manager-docker`, `package-manager-curlpipe`, `package-manager-mise`
- **Output**: `devex-plugin-package-manager-{name}` binary
- **Tag Pattern**: `@devex/package-manager-{name}@*`

### Tool Plugins
- **Platforms**: Cross-platform (Windows, macOS, Linux)
- **Examples**: `tool-git`, `tool-shell`, `tool-stackdetector`
- **Output**: `devex-plugin-tool-{name}` binary
- **Tag Pattern**: `@devex/tool-{name}@*`

## Adding New Plugins

### 1. Create Plugin Structure
```bash
# Create plugin directory
mkdir packages/my-new-plugin
cd packages/my-new-plugin

# Create package.json for version tracking
cat > package.json <<EOF
{
  "name": "@devex/my-new-plugin",
  "version": "1.0.0",
  "description": "Description of my plugin",
  "private": true,
  "scripts": {
    "build": "go build -o devex-plugin-my-new-plugin .",
    "test": "ginkgo run .",
    "lint": "golangci-lint run"
  }
}
EOF

# Create main.go with plugin implementation
cat > main.go <<EOF
package main

import (
    "github.com/jameswlane/devex/packages/plugin-sdk"
)

func main() {
    // Plugin implementation
}
EOF
```

### 2. Generate GoReleaser Config
```bash
# Generate .goreleaser.yml automatically
node scripts/generate-goreleaser.js
```

### 3. Test Plugin
```bash
# Test build
cd packages/my-new-plugin
go build -o devex-plugin-my-new-plugin .

# Test GoReleaser config
goreleaser check
goreleaser build --snapshot --clean --single-target
```

### 4. Commit and Release
```bash
# Commit with conventional commit message
git add .
git commit -m "feat: add my-new-plugin for enhanced functionality"
git push

# The automated release system will:
# 1. Detect changes in packages/my-new-plugin/
# 2. Create tag @devex/my-new-plugin@1.0.0
# 3. Trigger GoReleaser to build and release
```

## Testing the Release System

### Test Conventional Commit Parsing
```bash
# Test different commit types
git commit --allow-empty -m "feat: test feature commit"  # Should trigger minor bump
git commit --allow-empty -m "fix: test bug fix"         # Should trigger patch bump  
git commit --allow-empty -m "feat!: breaking change"    # Should trigger major bump
git commit --allow-empty -m "docs: update readme"       # Should not trigger release
```

### Test Package Change Detection
```bash
# Test CLI changes
echo "# Test change" >> apps/cli/README.md
git commit -am "feat: test CLI change detection"
# Should create @devex/cli@ tag

# Test plugin changes
echo "# Test change" >> packages/package-manager-apt/README.md
git commit -am "fix: test plugin change detection"
# Should create @devex/package-manager-apt@ tag
```

### Test GoReleaser Configurations
```bash
# Test individual plugin builds
cd packages/package-manager-apt
goreleaser check                              # Validate config
goreleaser build --snapshot --clean          # Test build process

# Test CLI build
cd apps/cli
goreleaser build --snapshot --clean --single-target
```

### Monitor Release Workflow
```bash
# Check workflow status
gh run list --workflow=release.yml

# View logs for specific run
gh run view --log

# Check created tags
git tag --sort=-creatordate | head -10
```

## Manual Release Process

While the system is designed to be fully automated, you can manually trigger releases if needed:

### Method 1: Conventional Commit (Recommended)
```bash
# Make changes to a package
echo "# Manual change" >> packages/package-manager-apt/README.md

# Commit with conventional format
git add .
git commit -m "fix: manual update to apt package manager"
git push

# The automated system will handle the rest
```

### Method 2: Direct Tag Creation (Not Recommended)
```bash
# Create tag manually (bypasses conventional commit parsing)
git tag @devex/package-manager-apt@1.2.3
git push origin @devex/package-manager-apt@1.2.3

# This will trigger GoReleaser directly
```

### Emergency Release Process
```bash
# For urgent releases, you can manually run GoReleaser
cd packages/package-manager-apt
GITHUB_TOKEN=$YOUR_TOKEN goreleaser release --clean
```

## Benefits of Current System

### Automated & Efficient
- ✅ **Zero Manual Work**: Releases trigger automatically from conventional commits
- ✅ **Smart Change Detection**: Only releases packages that actually changed
- ✅ **Proper Version Bumping**: Semantic versioning based on commit types
- ✅ **Monorepo Aware**: Handles 35+ packages intelligently

### Developer Experience
- ✅ **Simple Workflow**: Just use conventional commits
- ✅ **Fast Feedback**: Releases happen within minutes of commit
- ✅ **No Configuration**: Adding new packages requires no release setup
- ✅ **Clear History**: Each release linked to specific commits

### Scalability & Maintenance
- ✅ **Self-Maintaining**: System scales automatically with new packages
- ✅ **Failure Isolation**: One package failure doesn't block others
- ✅ **Parallel Execution**: Multiple packages can release simultaneously
- ✅ **Resource Efficient**: Only builds what changed

### User Experience
- ✅ **Frequent Updates**: Packages update as soon as fixes/features are ready
- ✅ **Semantic Versioning**: Clear understanding of change impact
- ✅ **Individual Releases**: Download only needed components
- ✅ **Consistent Release Notes**: Generated from conventional commits

## Configuration Files

### Auto Configuration (`.autorc`)
```json
{
  "plugins": ["conventional-commits"],
  "owner": "jameswlane",
  "repo": "devex",
  "labels": {
    "patch": {"releaseType": "patch"},
    "minor": {"releaseType": "minor"},
    "major": {"releaseType": "major"}
  }
}
```

### Release Workflow (`.github/workflows/release.yml`)
- **Trigger**: Push to main branch
- **Steps**: 
  1. Analyze conventional commits
  2. Detect changed packages
  3. Calculate version bumps
  4. Create tags
  5. Update package.json files
  6. Trigger GoReleaser workflows

## Troubleshooting

### Release Not Triggered
1. **Check Commit Format**: Must use conventional commits (`feat:`, `fix:`, etc.)
2. **Check File Changes**: Must have actual changes in `apps/` or `packages/` directories
3. **Check Workflow Logs**: Look for parsing errors in GitHub Actions

### Wrong Version Bump
1. **feat:** commits create minor releases
2. **fix:** commits create patch releases  
3. **BREAKING CHANGE:** or **!:** creates major releases
4. Other commit types don't trigger releases

### Package Not Released
1. **Check Package Detection**: File changes must be in correct directory structure
2. **Check package.json**: Each package needs package.json for version tracking
3. **Check GoReleaser Config**: Each package needs `.goreleaser.yml`

## System Status

- ✅ **Automated Release System**: Fully operational with conventional commits
- ✅ **Package Change Detection**: Handles 35+ packages intelligently
- ✅ **Version Management**: Semantic versioning with automatic bumps
- ✅ **GoReleaser Integration**: Individual configs for each package type
- ✅ **Documentation**: Complete system documentation
- ✅ **Testing**: Validated with multiple release scenarios

The DevEx release system is **production-ready**, **fully automated**, and **zero-maintenance**. It represents the evolution from complex manual processes to a streamlined conventional commit workflow that scales automatically with the project.
