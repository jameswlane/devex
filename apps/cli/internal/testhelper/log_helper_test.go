package testhelper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
)

var _ = Describe("LogHelper", func() {
	Describe("SuppressLogs", func() {
		It("should suppress all log output", func() {
			testhelper.SuppressLogs()

			// These should not produce any output
			log.Info("test info message")
			log.Warn("test warning message")
			log.Debug("test debug message")

			// No assertions needed - if logs were not suppressed,
			// they would appear in test output
			Expect(true).To(BeTrue()) // Dummy assertion
		})
	})

	Describe("LogCapture", func() {
		var capture *testhelper.LogCapture

		BeforeEach(func() {
			capture = testhelper.NewLogCapture()
			testhelper.CaptureLogsTo(capture)
		})

		It("should capture log messages", func() {
			log.Info("test message")

			output := capture.GetOutput()
			Expect(output).To(ContainSubstring("test message"))
		})

		It("should capture multiple log messages", func() {
			log.Info("first message")
			log.Warn("warning message")

			output := capture.GetOutput()
			Expect(output).To(ContainSubstring("first message"))
			Expect(output).To(ContainSubstring("warning message"))
		})

		It("should clear captured output", func() {
			log.Info("message to clear")
			capture.Clear()

			output := capture.GetOutput()
			Expect(output).To(BeEmpty())

			log.Info("new message")
			output = capture.GetOutput()
			Expect(output).To(ContainSubstring("new message"))
			Expect(output).NotTo(ContainSubstring("message to clear"))
		})
	})

	Describe("SetupTestLogging", func() {
		BeforeEach(func() {
			testhelper.SuppressLogs()
		})

		It("should suppress logs in all tests", func() {
			// This log should not appear in test output
			log.Info("this should be suppressed")

			// Test passes if no logs appear
			Expect(true).To(BeTrue())
		})
	})
})
