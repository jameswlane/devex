# Release Workflow Improvements

## Overview

This document outlines the comprehensive improvements made to the DevEx release workflow to address reliability, maintainability, and debugging issues with the previous implementation.

## Previous Issues

### 1. GoReleaser Failures Due to Dirty Git State
**Problem**: GoReleaser would fail with "git is in a dirty state" errors because old `dist/` files from previous builds remained in the repository.

**Root Cause**: The workflow didn't clean up artifacts between releases, causing file conflicts.

### 2. Complex, Fragile Shell Scripting
**Problem**: 160+ lines of complex bash scripting with poor error handling, making it difficult to debug and maintain.

**Issues**:
- Silent failures with `|| echo "warning"`
- Complex conditional logic hard to follow
- No proper error propagation
- Difficult to test individual components

### 3. Inefficient Tag Creation Process
**Problem**: Tags were created before verifying GoReleaser would succeed, leading to orphaned tags without corresponding releases.

### 4. Poor Error Visibility
**Problem**: Failures were masked or logged as warnings, making it impossible to diagnose issues.

## New Architecture: Multi-Job Workflow

The improved workflow uses GitHub Actions best practices with separate, focused jobs:

### Job 1: `detect-changes`
**Purpose**: Intelligently detect what packages need releasing and calculate versions.

**Improvements**:
- ✅ Uses Node.js for reliable version calculation instead of bash arithmetic
- ✅ Proper conventional commit detection
- ✅ Precise file change analysis
- ✅ Clear output variables for downstream jobs
- ✅ Support for forced releases via workflow dispatch

### Job 2: `release-cli`
**Purpose**: Build and release the CLI package with GoReleaser.

**Improvements**:
- ✅ **Clean workspace**: Removes `dist/` files before building
- ✅ **Atomic operations**: Tag creation and release happen together
- ✅ **Proper error handling**: Fails fast on any error
- ✅ **Isolated environment**: No interference from other package builds

### Job 3: `release-plugin-sdk`
**Purpose**: Build and release the Plugin SDK.

**Improvements**:
- ✅ **Independent execution**: Doesn't depend on CLI release success
- ✅ **Graceful handling**: Skips binary release if no GoReleaser config exists
- ✅ **Atomic tag creation**: Tags and releases are consistent

### Job 4: `release-plugins`
**Purpose**: Build and release individual plugins using matrix strategy.

**Improvements**:
- ✅ **Parallel execution**: Multiple plugins can be released simultaneously
- ✅ **Fail-safe**: One plugin failure doesn't stop others
- ✅ **Dynamic detection**: Only releases plugins that actually changed
- ✅ **Scalable**: Easy to add new plugins to the matrix

### Job 5: `update-registry`
**Purpose**: Update the plugin registry after all releases complete.

**Improvements**:
- ✅ **Dependency management**: Only runs after release jobs complete
- ✅ **Wait mechanism**: Allows time for releases to become available
- ✅ **Proper error handling**: Registry update failures are visible

### Job 6: `notify`
**Purpose**: Provide clear success/failure notifications.

**Improvements**:
- ✅ **Always runs**: Provides status regardless of job outcomes
- ✅ **Clear messaging**: Shows exactly what was released
- ✅ **Failure detection**: Aggregates failures from all jobs

## Key Technical Improvements

### 1. Clean Git State Management
```yaml
- name: Clean workspace
  run: |
    # Remove any existing dist files to prevent dirty git state
    rm -rf apps/cli/dist/
    git status
```

**Impact**: Eliminates the primary cause of GoReleaser failures.

### 2. Atomic Tag and Release Operations
```yaml
# Create and push tag
git tag -a "$VERSION" -m "Release @devex/cli $VERSION"
git push origin "$VERSION"

# Run GoReleaser immediately after
goreleaser release --clean -f .goreleaser.yml
```

**Impact**: Ensures tags always have corresponding releases.

### 3. Robust Version Calculation
```javascript
const [major, minor, patch] = '$CURRENT_VERSION'.split('.').map(Number);
const bumpType = '$BUMP_TYPE';
let newVersion;
if (bumpType === 'major') newVersion = `${major + 1}.0.0`;
else if (bumpType === 'minor') newVersion = `${major}.${minor + 1}.0`;
else newVersion = `${major}.${minor}.${patch + 1}`;
console.log(newVersion);
```

**Impact**: Eliminates bash arithmetic errors and edge cases.

### 4. Parallel Plugin Releases
```yaml
strategy:
  matrix:
    package: ${{ fromJson('["@devex/package-manager-apt", "@devex/tool-git"]') }}
  fail-fast: false
```

**Impact**: Faster releases and isolated failures.

### 5. Proper Error Propagation
```yaml
- name: Notify failure
  if: needs.update-registry.result == 'failure' || contains(needs.*.result, 'failure')
  run: |
    echo "❌ Release failed!"
    exit 1
```

**Impact**: Clear failure detection and debugging information.

## Migration Strategy

### Phase 1: Testing (Recommended)
1. Keep existing workflow as `release.yml`
2. Deploy improved workflow as `release-improved.yml`
3. Test with manual triggers using `workflow_dispatch`
4. Validate releases work correctly

### Phase 2: Gradual Rollout
1. Rename existing workflow to `release-legacy.yml`
2. Rename improved workflow to `release.yml`
3. Monitor first few automatic releases
4. Remove legacy workflow once confident

### Phase 3: Cleanup
1. Update documentation to reflect new workflow
2. Remove old shell scripts if no longer needed
3. Add monitoring and alerting for release failures

## Manual Release Process

The improved workflow supports manual releases via GitHub UI:

1. Go to Actions → Release (Improved)
2. Click "Run workflow"
3. Select branch (usually `main`)
4. Check "Force release" if needed
5. Click "Run workflow"

## Testing Recommendations

### Before Deployment
1. **Test in fork**: Deploy to a fork and test with sample releases
2. **Validate GoReleaser configs**: Ensure all `.goreleaser.yml` files are valid
3. **Check permissions**: Verify all required secrets are configured
4. **Test matrix strategy**: Ensure plugin matrix includes all packages

### After Deployment
1. **Monitor first release**: Watch logs closely for any issues
2. **Verify GitHub releases**: Check that releases are created correctly
3. **Test download links**: Ensure binary artifacts are accessible
4. **Validate registry updates**: Confirm plugin registry reflects new versions

## Troubleshooting Guide

### Common Issues and Solutions

#### "No GoReleaser config found"
**Cause**: Plugin package missing `.goreleaser.yml`
**Solution**: Either add config file or remove package from matrix

#### "Tag already exists"
**Cause**: Previous failed run created tag but not release
**Solution**: Delete the orphaned tag and re-run workflow

#### "Registry update failed"
**Cause**: GitHub API rate limiting or network issues
**Solution**: Re-run the `update-registry` job manually

#### "Matrix package not found"
**Cause**: Package name in matrix doesn't match actual package structure
**Solution**: Update matrix to reflect correct package names

## Benefits Summary

| Aspect | Before | After |
|--------|--------|-------|
| **Reliability** | ❌ Frequent failures | ✅ Robust error handling |
| **Debugging** | ❌ Silent failures | ✅ Clear error messages |
| **Maintainability** | ❌ Complex bash scripts | ✅ Structured YAML jobs |
| **Performance** | ❌ Sequential releases | ✅ Parallel execution |
| **Scalability** | ❌ Hard to add packages | ✅ Simple matrix updates |
| **Testing** | ❌ All-or-nothing | ✅ Manual trigger support |

## Future Enhancements

### Potential Additions
1. **Slack/Discord notifications** for release events
2. **Automated changelog generation** from conventional commits
3. **Release candidate** support for pre-releases
4. **Cross-repository** plugin releases
5. **Rollback mechanisms** for failed releases

### Monitoring
1. **Release success rates** dashboard
2. **Time-to-release** metrics
3. **Failed release** alerting
4. **Download statistics** tracking

---

*This improved workflow addresses all known issues with the previous release system and provides a foundation for reliable, maintainable releases at scale.*