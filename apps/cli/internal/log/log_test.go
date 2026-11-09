package log_test

import (
	"bytes"
	"fmt"
	"io"
	"os"

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

	It("does not write file-only errors to a non-file logger", func() {
		buffer := &bytes.Buffer{}
		log.InitDefaultLogger(buffer)

		log.ErrorToFile("File Only Error", fmt.Errorf("an error occurred"))
		Expect(buffer.String()).To(BeEmpty())
	})

	It("writes file-only errors to the log file but not stderr in debug mode", func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())

		originalStderr := os.Stderr
		r, w, err := os.Pipe()
		Expect(err).ToNot(HaveOccurred())
		os.Stderr = w
		initErr := log.InitFileLogger(true)
		os.Stderr = originalStderr
		Expect(initErr).ToNot(HaveOccurred())

		log.ErrorToFile("File Only Error", fmt.Errorf("an error occurred"))
		logFile := log.GetLogFile()
		Expect(log.Close()).To(Succeed())
		Expect(w.Close()).To(Succeed())

		stderrOutput, err := io.ReadAll(r)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(stderrOutput)).ToNot(ContainSubstring("File Only Error"))

		content, err := os.ReadFile(logFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("File Only Error"))
		Expect(string(content)).To(ContainSubstring("an error occurred"))
	})

	It("logs fatal messages", func() {
		// Skip this test as Fatal() calls os.Exit() which terminates the test process
		// The Fatal function is tested through integration tests instead
		Skip("Fatal() calls os.Exit() - tested in integration tests")
	})
})

var _ = Describe("Warning output stream", func() {
	capture := func(target **os.File, fn func()) string {
		original := *target
		reader, writer, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		*target = writer
		fn()
		*target = original
		Expect(writer.Close()).To(Succeed())
		data, err := io.ReadAll(reader)
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}

	It("writes warnings to stderr so stdout stays machine-readable", func() {
		log.InitDefaultLogger(&bytes.Buffer{})
		var stderr string
		stdout := capture(&os.Stdout, func() {
			stderr = capture(&os.Stderr, func() {
				log.Warning("plugin %s unavailable", "package-manager-yum")
			})
		})
		Expect(stdout).To(BeEmpty())
		Expect(stderr).To(ContainSubstring("⚠️  plugin package-manager-yum unavailable"))
	})
})

var _ = Describe("Runtime output policy", func() {
	AfterEach(log.InitTestLogger)
	capture := func(target **os.File, fn func()) string {
		original := *target
		reader, writer, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		defer reader.Close()
		*target = writer
		defer func() { *target = original }()
		fn()
		Expect(writer.Close()).To(Succeed())
		data, err := io.ReadAll(reader)
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}
	It("suppresses routine output in quiet mode but preserves actionable errors on stderr", func() {
		log.InitDefaultLogger(&bytes.Buffer{})
		log.Configure("debug", true)
		var stderr string
		stdout := capture(&os.Stdout, func() {
			stderr = capture(&os.Stderr, func() {
				log.Print("routine progress")
				log.Success("done")
				log.Warning("routine warning")
				log.ErrorMsg("repair configuration")
			})
		})
		Expect(stdout).To(BeEmpty())
		Expect(stderr).To(ContainSubstring("repair configuration"))
		Expect(stderr).NotTo(ContainSubstring("routine"))
	})

	It("keeps error diagnostics off stdout and filters warning helpers at error level", func() {
		log.InitDefaultLogger(&bytes.Buffer{})
		log.Configure("error", false)
		var stderr string
		stdout := capture(&os.Stdout, func() {
			stderr = capture(&os.Stderr, func() {
				log.Warning("suppressed-warning")
				log.ErrorMsg("actionable-error")
			})
		})
		Expect(stdout).To(BeEmpty())
		Expect(stderr).To(ContainSubstring("actionable-error"))
		Expect(stderr).NotTo(ContainSubstring("suppressed-warning"))
	})
	It("mirrors resolved debug logs to stderr and stops mirroring when quiet is enabled", func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		Expect(log.InitFileLogger(false)).To(Succeed())
		defer log.Close()
		stderr := capture(&os.Stderr, func() {
			log.Configure("debug", false)
			log.Debug("console-debug-marker")
			log.Configure("debug", true)
			log.Info("quiet-info-marker")
		})
		Expect(stderr).To(ContainSubstring("console-debug-marker"))
		Expect(stderr).NotTo(ContainSubstring("quiet-info-marker"))
	})
	DescribeTable("filters file logging", func(level string, quiet bool, expected []string) {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		Expect(log.InitFileLogger(false)).To(Succeed())
		log.Configure(level, quiet)
		log.Debug("file-debug-marker")
		log.Info("file-info-marker")
		log.Warn("file-warn-marker")
		log.Error("file-error-marker", nil)
		path := log.GetLogFile()
		Expect(log.Close()).To(Succeed())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		for _, name := range []string{"debug", "info", "warn", "error"} {
			if containsLevel(expected, name) {
				Expect(string(data)).To(ContainSubstring("file-" + name + "-marker"))
			} else {
				Expect(string(data)).NotTo(ContainSubstring("file-" + name + "-marker"))
			}
		}
	},
		Entry("debug", "debug", false, []string{"debug", "info", "warn", "error"}),
		Entry("info", "info", false, []string{"info", "warn", "error"}),
		Entry("warn", "warn", false, []string{"warn", "error"}),
		Entry("error", "error", false, []string{"error"}),
		Entry("quiet", "debug", true, []string{"error"}),
	)
})

func containsLevel(levels []string, target string) bool {
	for _, level := range levels {
		if level == target {
			return true
		}
	}
	return false
}
