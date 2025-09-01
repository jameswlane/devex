package testhelper

import (
	"bytes"
	"io"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/onsi/ginkgo/v2"
)

// LogCapture provides a way to capture logs during tests
type LogCapture struct {
	buffer *bytes.Buffer
}

// NewLogCapture creates a new log capture instance
func NewLogCapture() *LogCapture {
	return &LogCapture{
		buffer: &bytes.Buffer{},
	}
}

// GetOutput returns the captured log output
func (lc *LogCapture) GetOutput() string {
	return lc.buffer.String()
}

// Clear clears the captured output
func (lc *LogCapture) Clear() {
	lc.buffer.Reset()
}

// Writer returns the underlying writer
func (lc *LogCapture) Writer() io.Writer {
	return lc.buffer
}

// SuppressLogs sets up log suppression for tests
// This should be called in BeforeEach blocks
func SuppressLogs() {
	// Initialize test logger that discards all output
	log.InitTestLogger()
}

// CaptureLogsTo initializes logging to capture output
// This should be called when you want to test log output
func CaptureLogsTo(capture *LogCapture) {
	log.InitDefaultLogger(capture.Writer())
}

// SetupTestLoggingWithCapture sets up logging with capture capability
// Returns a LogCapture instance that can be used to inspect logs
func SetupTestLoggingWithCapture() *LogCapture {
	var capture *LogCapture

	ginkgo.BeforeEach(func() {
		capture = NewLogCapture()
		CaptureLogsTo(capture)
	})

	return capture
}
