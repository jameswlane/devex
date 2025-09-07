# Centralized Logging System

DevEx now uses a centralized logging system that eliminates test noise and provides consistent logging across the codebase.

## Key Features

### 🔇 Silent Test Mode
- Automatically suppresses all log output during tests
- No more cluttered test output or log noise
- Tests run clean and focus on actual failures

### 🔧 Flexible Logging Modes
- **Production Mode**: Logs to files with system information
- **Debug Mode**: Logs to both files and console  
- **Test Mode**: Silent operation (discards all output)
- **Silent Mode**: Completely suppress all output

### 🎯 Consistent API
All logging goes through `internal/log` with these functions:
- `log.Print()`, `log.Printf()`, `log.Println()` - General output
- `log.Success()` - Success messages with ✅
- `log.Warning()` - Warning messages with ⚠️  
- `log.ErrorMsg()` - Error messages with ❌
- `log.Info()`, `log.Warn()`, `log.Error()`, `log.Debug()` - Structured logging

## Usage

### In Application Code
```go
import "github.com/jameswlane/devex/apps/cli/internal/log"

// Replace fmt.Printf calls with:
log.Printf("Processing %d items", count)
log.Success("Operation completed successfully")
log.Warning("Configuration file not found, using defaults")
log.ErrorMsg("Failed to connect to database")
```

### In Tests
Tests automatically use silent logging via `testhelper.SuppressLogs()`:
```go
var _ = BeforeEach(func() {
    testhelper.SuppressLogs()  // Already in test suites
})
```

### For Plugin Development
The plugin SDK provides a Logger interface that integrates with the centralized system:

```go
// Plugin SDK automatically configures logging based on test mode
downloader := sdk.NewDownloader(registryURL, pluginDir)
// In test mode, downloader is automatically set to silent

// Custom logger implementation
type MyLogger struct {
    silent bool
}

func (l *MyLogger) Printf(format string, args ...any) {
    if !l.silent {
        fmt.Printf(format, args...)
    }
}

// Set custom logger
downloader.SetLogger(MyLogger{silent: false})
```

## Plugin Logging Architecture

### For Plugin Authors
Plugins should use the SDK Logger interface instead of direct fmt.Printf:

```go
// ❌ Don't do this:
fmt.Printf("Installing package %s\n", packageName)

// ✅ Do this instead:
logger.Success("Installing package %s", packageName)
```

### Plugin Logger Interface
```go
type Logger interface {
    Printf(format string, args ...any)
    Println(msg string, args ...any) 
    Success(msg string, args ...any)
    Warning(msg string, args ...any)
    ErrorMsg(msg string, args ...any)
    Info(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
    Error(msg string, err error, keyvals ...any)
    Debug(msg string, keyvals ...any)
}
```

### Automatic Test Mode Detection
The system automatically detects when running in test mode and configures all components accordingly:

```go
// In plugin bootstrap
if log.IsTestMode() {
    downloader.SetSilent(true)  // Suppress plugin SDK output
}
```

## Benefits

### ✅ Clean Test Output
- No more printf statements cluttering test results
- Easy to spot actual test failures
- Consistent test experience across the codebase

### ✅ Consistent Logging
- All output goes through the same system
- Proper log levels and formatting
- Centralized configuration

### ✅ Plugin Compatibility  
- Plugin SDK respects the centralized logging system
- Silent mode works across all plugin operations
- Easy to add logging to new plugins

### ✅ Development Workflow
- Debug mode shows logs in console during development
- Production mode logs to files for troubleshooting
- No need to manually suppress logs in tests anymore

## Migration Guide

### Replacing fmt.Printf Calls
```go
// Before
fmt.Printf("Processing %s...\n", filename)
fmt.Printf("✅ Success: %s\n", message)
fmt.Printf("⚠️ Warning: %s\n", warning)
fmt.Printf("❌ Error: %s\n", errorMsg)

// After  
log.Printf("Processing %s...", filename)
log.Success("Success: %s", message)  
log.Warning("Warning: %s", warning)
log.ErrorMsg("Error: %s", errorMsg)
```

### Plugin Integration
```go
// In plugin main function
func main() {
    // Create logger (will be silent in test mode)
    logger := sdk.NewDefaultLogger(false)
    
    // Use throughout plugin
    logger.Success("Plugin initialized")
    logger.Printf("Processing %d items", count)
}
```

## Testing

The logging system includes comprehensive tests:
```bash
go test ./internal/log -v      # Test logger functionality
go test ./internal/... -v     # Verify silent test mode works
```

Tests verify:
- Silent mode suppresses all output
- Different log levels work correctly
- Test mode detection works properly
- Plugin SDK integration functions correctly

This centralized logging system eliminates the recurring issue of test noise while providing a robust foundation for consistent logging across DevEx and all its plugins.
