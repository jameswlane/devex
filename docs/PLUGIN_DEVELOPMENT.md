# Plugin Development Guide

This guide covers the automated tools available for plugin development in the DevEx project.

## Pre-Commit Automation

The following tasks are automated via lefthook when you commit plugin changes:

### Automatic Formatting
- **Go files**: `gofmt` and `goimports` are run on all staged Go files in plugins
- **Dependencies**: `go mod tidy` is automatically run when `go.mod` or `main.go` files change
- **JSON validation**: `package.json` files are validated for syntax and required fields

### What Gets Checked
- Large file detection
- Go code formatting and imports
- Plugin dependencies are up-to-date
- Package.json structure validation

## Available Commands

You can run these commands using direct script calls (lefthook manual commands aren't working as expected):

### Plugin Status & Analysis
```bash
# Get comprehensive plugin development status
./scripts/plugin-dev-status.sh

# Check which plugins have changes since last release
./scripts/check-plugin-changes.sh

# Show verbose output with recent commits
./scripts/check-plugin-changes.sh -v

# Show all plugins (changed and unchanged)  
./scripts/check-plugin-changes.sh -a
```

### Plugin Building & Testing
```bash
# Build all changed plugins
./scripts/release-plugin-individual.sh build

# Test a specific plugin
./scripts/release-plugin-individual.sh test <plugin-name>

# Build specific plugins
./scripts/release-plugin-individual.sh build <plugin1> <plugin2>
```

### Plugin Versioning
```bash
# Check what version a plugin should have based on commits
node scripts/determine-plugin-version.js check <plugin-name>

# Update plugin version based on conventional commits
node scripts/determine-plugin-version.js update <plugin-name>

# Process multiple plugins
node scripts/determine-plugin-version.js batch <plugin1> <plugin2>
```

### Maintenance Commands
```bash
# Run 'go mod tidy' on all plugins
find packages/plugins -name go.mod -execdir go mod tidy \;

# Format all plugin Go files
find packages/plugins -name "*.go" -exec gofmt -w {} \;
find packages/plugins -name "*.go" -exec goimports -w {} \;
```

## Pre-Push Validation

Before you push changes, lefthook automatically:
- Runs tests for the CLI
- Checks for security vulnerabilities
- Builds web and docs
- **Checks plugin changes and builds changed plugins**

## Post-Commit Feedback

After you commit changes to plugins, you'll see helpful tips about:
- Which plugins were affected
- How to check plugin status
- How to build plugins locally

## Plugin Release Workflow

1. **Develop**: Make changes to plugins following conventional commits
2. **Commit**: Pre-commit hooks format code and update dependencies
3. **Check**: Run `lefthook run plugin-check` to see what's ready for release
4. **Test**: Run `lefthook run plugin-build` to build locally
5. **Push**: Push to main branch to trigger automatic release workflow

## Conventional Commits for Plugins

Use these commit prefixes for automatic semantic versioning:

- `fix(plugin-name):` → patch version bump (1.0.0 → 1.0.1)
- `feat(plugin-name):` → minor version bump (1.0.0 → 1.1.0)  
- `feat(plugin-name)!:` → major version bump (1.0.0 → 2.0.0)
- `fix(plugin-name)!:` → major version bump
- Include `BREAKING CHANGE:` in commit body → major version bump

### Examples
```bash
# Bug fix - patch version
git commit -m "fix(package-manager-apt): handle missing gpg keys gracefully"

# New feature - minor version  
git commit -m "feat(desktop-gnome): add support for custom themes"

# Breaking change - major version
git commit -m "feat(package-manager-docker)!: change plugin interface to support authentication"
```

## Plugin Structure

Each plugin should have:
- `main.go` - Plugin entry point
- `go.mod` - Go module file  
- `package.json` - Plugin metadata with name, version, description
- `Taskfile.yml` - Task definitions for build/test (optional)

The package.json should include DevEx-specific metadata:
```json
{
  "name": "@devex/plugin-example",
  "version": "1.0.0", 
  "description": "Example plugin description",
  "devex": {
    "plugin": {
      "type": "package-manager",
      "platforms": ["linux"],
      "priority": 10,
      "supports": {
        "install": true,
        "remove": true
      }
    }
  }
}
```

## Troubleshooting

### Common Issues

**Dependencies not resolving**: Run `lefthook run plugin-tidy-all` to update all plugin dependencies.

**Formatting issues**: Run `lefthook run plugin-format-all` to format all plugin Go files.

**Build failures**: Check that your plugin implements the required interface and has proper dependencies.

### Getting Help

- Check plugin changes: `./scripts/check-plugin-changes.sh -v`
- Test individual plugin: `./scripts/release-plugin-individual.sh test <plugin-name>`
- Check version logic: `node scripts/determine-plugin-version.js check <plugin-name>`

