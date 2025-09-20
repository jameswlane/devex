# Standardized Error Handling and Background Validation Example

This document demonstrates the new standardized error handling and background validation system implemented for installer consistency.

## Background Validation Usage

### System Validation
```go
// Create validator with timeout
validator := utilities.NewBackgroundValidator(30 * time.Second)

// Add system validation suite for a specific installer
validator.AddSuite(utilities.CreateSystemValidationSuite("apt"))

// Add network validation suite
validator.AddSuite(utilities.CreateNetworkValidationSuite())

// Run all validations concurrently
ctx := context.Background()
if err := validator.RunValidations(ctx); err != nil {
    // Handle critical validation failures
    return utilities.WrapError(err, utilities.ErrorTypeSystem, "install", packageName, "apt")
}
```

### Custom Validation Suites
```go
customSuite := utilities.ValidationSuite{
    Name: "custom-checks",
    Checks: []utilities.ValidationCheck{
        {
            Name:        "disk-space",
            Description: "Check available disk space",
            Validator: func(ctx context.Context) error {
                // Your validation logic here
                return nil
            },
            Timeout:  10 * time.Second,
            Critical: true, // Blocks installation if fails
        },
    },
}
validator.AddSuite(customSuite)
```

## Standardized Error Handling

### Creating Structured Errors

#### System Errors
```go
// For system validation failures
err := utilities.NewSystemError("apt", "APT not available", underlyingErr)
// Automatically includes suggestions like:
// - "Ensure apt is installed and available in PATH"
// - "Check system requirements and dependencies"
// - "Verify user has necessary permissions"
```

#### Package Errors
```go
// For package operation failures
err := utilities.NewPackageError("install", "nginx", "apt", underlyingErr)
// Automatically includes install-specific suggestions like:
// - "Update package metadata/cache"
// - "Check package name spelling"
// - "Verify package exists in configured repositories"
// - "Check available disk space"

err := utilities.NewPackageError("uninstall", "nginx", "apt", underlyingErr)
// Includes uninstall-specific suggestions like:
// - "Verify package is actually installed"
// - "Check for dependent packages that prevent removal"
```

#### Repository Errors
```go
// For database/repository operations
err := utilities.NewRepositoryError("add", "nginx", "apt", underlyingErr)
// Includes suggestions like:
// - "Check database connectivity"
// - "Verify repository configuration"
// - "Try operation again after a short delay"
```

#### Network Errors
```go
// For download/connectivity issues
err := utilities.NewNetworkError("download", "package.deb", "apt", underlyingErr)
// Includes suggestions like:
// - "Check internet connectivity"
// - "Verify repository URLs are accessible"
// - "Try again later if servers are temporarily unavailable"
// - "Check firewall and proxy settings"
```

### Error Wrapping
```go
// Wrap existing errors with installer context
err := utilities.WrapError(originalErr, utilities.ErrorTypePackage, "install", packageName, "apt")
```

### Error Checking
```go
// Check error types
if errors.Is(err, utilities.ErrPackageNotFound) {
    // Handle package not found specifically
}

if errors.Is(err, utilities.ErrSystemValidation) {
    // Handle system validation failures
}

// Check if error is recoverable
if installerErr, ok := err.(*utilities.InstallerError); ok {
    if installerErr.Recoverable {
        // Suggest retry or alternative approach
        log.Info("Operation failed but may succeed on retry", "suggestions", installerErr.Suggestions)
    }
}
```

## Updated Installer Pattern

Here's how the DNF installer now uses both systems:

```go
func (d *DnfInstaller) Install(command string, repo types.Repository) error {
    log.Debug("DNF Installer: Starting installation", "command", command)

    // 1. Background validation runs concurrently
    validator := utilities.NewBackgroundValidator(30 * time.Second)
    validator.AddSuite(utilities.CreateSystemValidationSuite("dnf"))
    validator.AddSuite(utilities.CreateNetworkValidationSuite())

    ctx := context.Background()
    if err := validator.RunValidations(ctx); err != nil {
        return utilities.WrapError(err, utilities.ErrorTypeSystem, "install", command, "dnf")
    }

    // 2. Package operations use structured errors
    isInstalled, err := d.isPackageInstalled(command)
    if err != nil {
        return utilities.NewPackageError("install-check", command, "dnf", err)
    }

    if isInstalled {
        log.Info("Package already installed", "command", command)
        return nil // Success condition
    }

    // 3. Installation with proper error handling
    if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
        return utilities.NewPackageError("install", command, "dnf", err)
    }

    // 4. Repository operations
    if err := repo.AddApp(command); err != nil {
        return utilities.NewRepositoryError("add", command, "dnf", err)
    }

    return nil
}
```

## Benefits

### Performance Improvements
- **Concurrent validation**: System and network checks run in parallel
- **Early failure detection**: Critical validations prevent wasted operations
- **Timeout management**: Individual checks have configurable timeouts
- **Background execution**: Validations don't block each other

### Error Handling Consistency
- **Structured errors**: All installers use the same error types and patterns
- **Actionable suggestions**: Each error includes specific recovery guidance
- **Error categorization**: System, package, repository, network, permission, configuration
- **Recoverability analysis**: Errors indicate if retry is likely to succeed
- **Proper error chaining**: Underlying errors are preserved with context

### Developer Experience
- **Consistent patterns**: Same error handling across all installers
- **Rich error information**: Package name, installer type, operation context
- **Testing support**: Structured errors are easier to test and mock
- **Debugging assistance**: Clear error categorization aids troubleshooting
