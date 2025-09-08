# Individual Plugin Release System

## Overview

The DevEx project has been transformed from a monolithic GoReleaser configuration requiring manual maintenance to an automated individual plugin release system. This provides **zero-maintenance** plugin releases that scale automatically with new plugins.

## What Changed

### Before (Problems)
- ❌ **800+ line monolithic `.goreleaser.yml`** with 35+ plugin builds
- ❌ **Manual maintenance required** for every new plugin
- ❌ **Path mismatches** (`packages/plugins/` vs `packages/`)
- ❌ **Complex release scripts** with overlapping functionality
- ❌ **All-or-nothing releases** - one plugin failure blocked everything
- ❌ **Slow releases** due to building all plugins together

### After (Solutions)
- ✅ **Individual `.goreleaser.yml`** in each plugin directory (35 files)
- ✅ **Zero maintenance** - new plugins work automatically
- ✅ **Correct paths** - all plugins use `packages/<plugin-name>/`
- ✅ **Simplified workflow** - automatic change detection
- ✅ **Independent releases** - each plugin releases separately
- ✅ **Fast parallel releases** - only changed plugins are released

## File Structure

```
devex/
├── templates/
│   └── plugin-goreleaser.yml          # Template for new plugins
├── .github/workflows/
│   ├── plugin-release.yml             # New individual plugin releases
│   └── release.yml                    # CLI-only releases (updated)
├── packages/
│   ├── package-manager-apt/
│   │   └── .goreleaser.yml            # Individual config
│   ├── tool-git/
│   │   └── .goreleaser.yml            # Individual config
│   └── [34 other plugins]/
│       └── .goreleaser.yml            # Each has its own config
└── scripts/
    ├── test-plugin-release.sh         # Test individual plugin releases
    └── generate-plugin-goreleaser.sh  # Generate configs (if needed)
```

## Release Workflows

### Automatic Releases (Push to main)
1. **Change Detection**: GitHub Actions detects which plugins changed
2. **Matrix Build**: Releases only the changed plugins in parallel
3. **Independent Versioning**: Each plugin gets its own version number
4. **Tagged Releases**: Creates tags like `plugin-apt-v1.0.3`

### Manual Releases (workflow_dispatch)
1. **Plugin Selection**: Specify which plugin to release
2. **Version Control**: Optional custom version number
3. **On-Demand**: Release any plugin at any time

## Plugin Configuration

Each plugin has its own optimized GoReleaser configuration:

### Linux-Only Plugins
- All desktop environment plugins (`desktop-*`)
- Most package managers (`package-manager-apt`, `package-manager-dnf`, etc.)

### Cross-Platform Plugins
- Tool plugins (`tool-git`, `tool-shell`, `tool-stackdetector`)
- Universal package managers (`package-manager-curlpipe`, `package-manager-docker`, `package-manager-mise`, `package-manager-pip`)
- System setup (`system-setup`)

### Platform-Specific Plugins
- `package-manager-brew`: Darwin + Linux
- `package-manager-nixflake`, `package-manager-nixpkgs`: Darwin + Linux

## Adding New Plugins

### Automatic Method (Recommended)
1. Create new plugin directory: `packages/my-new-plugin/`
2. Add `main.go` with plugin implementation
3. Copy template: `cp templates/plugin-goreleaser.yml packages/my-new-plugin/.goreleaser.yml`
4. Customize the template placeholders
5. Commit - the plugin will automatically be included in releases!

### Generated Method
1. Create the plugin directory and `main.go`
2. Run: `./scripts/generate-plugin-goreleaser.sh` (regenerates all configs)

## Testing Plugin Releases

```bash
# Test a specific plugin's release configuration
./scripts/test-plugin-release.sh package-manager-apt

# Test the build process
cd packages/package-manager-apt
goreleaser build --snapshot --clean --single-target

# Validate configuration
goreleaser check
```

## Release Commands

### Manual Plugin Release
```bash
# From plugin directory
cd packages/package-manager-apt
git tag plugin-package-manager-apt-v1.2.3
git push origin plugin-package-manager-apt-v1.2.3
goreleaser release --clean
```

### GitHub Actions Manual Trigger
1. Go to Actions → Individual Plugin Releases
2. Click "Run workflow"
3. Enter plugin name (e.g., `package-manager-apt`)
4. Optionally specify version (e.g., `v1.2.3`)

## Benefits Achieved

### For Developers
- **Zero Maintenance**: New plugins work automatically
- **Fast Iterations**: Only changed plugins are released
- **Independent Development**: Plugin teams can release independently
- **Clear Ownership**: Each plugin has its own configuration
- **Easy Testing**: Individual plugin testing

### For Users
- **Faster Releases**: No waiting for unrelated plugin builds
- **More Frequent Updates**: Plugins can be updated as needed
- **Better Release Notes**: Plugin-specific changelogs
- **Smaller Downloads**: Download only needed plugins

### For CI/CD
- **Parallel Processing**: Multiple plugins released simultaneously  
- **Failure Isolation**: One plugin failure doesn't block others
- **Reduced Build Times**: Only build what changed
- **Better Resource Usage**: More efficient CI/CD resource utilization

## Migration Complete

- ✅ **35 individual GoReleaser configs** generated and tested
- ✅ **GitHub Actions workflow** implemented and ready
- ✅ **Path issues fixed** in main `.goreleaser.yml`
- ✅ **Template system** created for future plugins
- ✅ **Testing framework** implemented and validated
- ✅ **Documentation** complete

The DevEx plugin release system is now **fully automated**, **maintainable**, and **scalable**. Adding new plugins requires zero configuration changes to the build system!
