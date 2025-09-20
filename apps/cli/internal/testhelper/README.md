# Test Helper Package

This package provides utilities for managing test logging and output during Ginkgo test runs.

## Purpose

The testhelper package standardizes log suppression across all test suites to:
- Reduce test noise and improve readability
- Make it easier to spot actual test failures
- Provide consistent test output formatting
- Allow selective log capture when needed for debugging

## Usage

### Suppressing Logs in Test Suites

In your `*_suite_test.go` files, add the following:

```go
import (
    "testing"
    
    "github.com/jameswlane/devex/pkg/testhelper"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestYourPackage(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Your Package Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
    testhelper.SuppressLogs()
})
```

### Capturing Logs for Testing

When you need to test log output:

```go
var _ = Describe("Log Testing", func() {
    var capture *testhelper.LogCapture
    
    BeforeEach(func() {
        capture = testhelper.NewLogCapture()
        testhelper.CaptureLogsTo(capture)
    })
    
    It("should log the expected message", func() {
        log.Info("test message")
        
        output := capture.GetOutput()
        Expect(output).To(ContainSubstring("test message"))
    })
})
```

### Using SetupTestLogging Helper

For simpler setup, you can use the convenience function:

```go
// This will automatically set up log suppression in BeforeEach
var _ = testhelper.SetupTestLogging()
```

## Implementation Details

- Uses the charmbracelet/log package's writer interface
- Redirects all log output to `io.Discard` by default
- Allows capture to a buffer when needed for testing
- Thread-safe for parallel test execution

## Migration Guide

To migrate existing test suites:

1. Add the testhelper import
2. Add the BeforeEach block with `testhelper.SuppressLogs()`
3. Remove any manual log suppression code
4. Test suites will now run with clean output

## Benefits

- **Cleaner test output**: Only see test results, not application logs
- **Faster test runs**: No I/O overhead from logging
- **Better CI/CD integration**: Cleaner logs in CI pipelines
- **Easier debugging**: Can selectively enable logs when needed
