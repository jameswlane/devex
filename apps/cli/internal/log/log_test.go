package log_test

import (
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

var _ = Describe("InitDefaultLogger", func() {
	It("initializes the default logger", func() {
		buffer := &bytes.Buffer{}
		log.InitDefaultLogger(buffer)

		log.Info("Test Info")
		Expect(buffer.String()).To(ContainSubstring("Test Info"))
	})
})

var _ = Describe("SetLevel", func() {
	It("sets the log level dynamically", func() {
		buffer := &bytes.Buffer{}
		log.InitDefaultLogger(buffer)

		// Set log level to WarnLevel
		log.SetLevel(log.WarnLevel)

		log.Info("This should not be logged")
		log.Warn("This should be logged")
		Expect(buffer.String()).To(ContainSubstring("This should be logged"))
		Expect(buffer.String()).ToNot(ContainSubstring("This should not be logged"))
	})
})

var _ = Describe("WithContext", func() {
	It("adds contextual metadata to log messages", func() {
		buffer := &bytes.Buffer{}
		log.InitDefaultLogger(buffer)

		log.WithContext(map[string]any{
			"request_id": "12345",
		})

		log.Info("Log with context")
		Expect(buffer.String()).To(ContainSubstring("request_id=12345"))
		Expect(buffer.String()).To(ContainSubstring("Log with context"))
	})
})

var _ = Describe("Test Mode", func() {
	It("should suppress output in test mode", func() {
		log.InitTestLogger()

		// These should not produce any output
		log.Print("This should be silent")
		log.Printf("This should also be silent: %s", "test")
		log.Success("Success message")
		log.Warning("Warning message")
		log.ErrorMsg("Error message")
	})

	It("should detect test mode correctly", func() {
		log.InitTestLogger()
		Expect(log.IsTestMode()).To(BeTrue())
		Expect(log.IsSilentMode()).To(BeTrue())

		buffer := &bytes.Buffer{}
		log.InitDefaultLogger(buffer)
		Expect(log.IsTestMode()).To(BeFalse())
		Expect(log.IsSilentMode()).To(BeFalse())
	})
})

var _ = Describe("Log Levels", func() {
	It("logs informational messages", func() {
		buffer := &bytes.Buffer{}
		log.InitDefaultLogger(buffer)

		log.Info("Test Info")
		Expect(buffer.String()).To(ContainSubstring("INFO")) // Match exact level capitalization
		Expect(buffer.String()).To(ContainSubstring("Test Info"))
	})

	It("logs warning messages", func() {
		buffer := &bytes.Buffer{}
		log.InitDefaultLogger(buffer)

		log.Warn("Test Warn")
		Expect(buffer.String()).To(ContainSubstring("WARN")) // Match exact level capitalization
		Expect(buffer.String()).To(ContainSubstring("Test Warn"))
	})

	It("logs error messages", func() {
		buffer := &bytes.Buffer{}
		log.InitDefaultLogger(buffer)

		log.Error("Test Error", fmt.Errorf("an error occurred"))
		Expect(buffer.String()).To(ContainSubstring("ERRO")) // Match logger's exact output
		Expect(buffer.String()).To(ContainSubstring("Test Error"))
		Expect(buffer.String()).To(ContainSubstring("an error occurred"))
	})

	It("logs fatal messages", func() {
		// Skip this test as Fatal() calls os.Exit() which terminates the test process
		// The Fatal function is tested through integration tests instead
		Skip("Fatal() calls os.Exit() - tested in integration tests")
	})
})
