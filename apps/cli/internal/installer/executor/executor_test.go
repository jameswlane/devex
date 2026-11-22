package executor_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installer/executor"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
)

func TestExecutor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Executor Suite")
}

var _ = BeforeEach(func() {
	home := GinkgoT().TempDir()
	testhelper.SetProfileEnv(GinkgoT(), home)
	GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", filepath.Join(home, "team"))
	previous := security.SystemOrgPolicyPath
	security.SystemOrgPolicyPath = filepath.Join(home, "system-policy.yaml")
	DeferCleanup(func() { security.SystemOrgPolicyPath = previous })
})

var _ = Describe("Executor", func() {
	var (
		ctx  context.Context
		exec executor.Executor
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Default Executor", func() {
		BeforeEach(func() {
			exec = executor.New()
		})

		Context("when validating commands", func() {
			It("should accept safe commands", func() {
				err := exec.Validate("ls -la")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept package manager commands", func() {
				err := exec.Validate("apt-get update")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject dangerous commands", func() {
				err := exec.Validate("rm -rf /")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dangerous"))
			})

			It("should reject fork bombs", func() {
				err := exec.Validate(":(){ :|:& };:")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when executing commands", func() {
			It("should execute simple commands", func() {
				cmd, err := exec.Execute(ctx, "echo test")
				Expect(err).ToNot(HaveOccurred())
				Expect(cmd).ToNot(BeNil())
				Expect(cmd.Args).To(ContainElement("echo"))
			})

			It("should handle commands with pipes", func() {
				cmd, err := exec.Execute(ctx, "echo test | grep test")
				Expect(err).ToNot(HaveOccurred())
				Expect(cmd).ToNot(BeNil())
				Expect(cmd.Args[0]).To(Equal("bash"))
				Expect(cmd.Args[1]).To(Equal("-c"))
			})

			It("should reject dangerous commands during execution", func() {
				_, err := exec.Execute(ctx, "rm -rf /")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("validation failed"))
			})

			It("should handle context cancellation", func() {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel() // Cancel immediately

				cmd, err := exec.Execute(cancelCtx, "sleep 10")
				Expect(err).ToNot(HaveOccurred()) // Command creation succeeds
				Expect(cmd).ToNot(BeNil())
			})
		})
	})

	Describe("Secure Executor", func() {
		BeforeEach(func() {
			exec = executor.NewSecure(security.SecurityLevelStrict, nil)
		})

		Context("when validating with strict security", func() {
			It("should be more restrictive than default", func() {
				// A command that might pass moderate but fail strict
				err := exec.Validate("rm /tmp/test")
				Expect(err).To(HaveOccurred())
			})

			It("should still allow explicitly whitelisted commands", func() {
				err := exec.Validate("apt-get update")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("ParseCommand", func() {
		It("should identify simple commands", func() {
			exe, args, needsShell := executor.ParseCommand("ls -la")
			Expect(exe).To(Equal("ls"))
			Expect(args).To(Equal([]string{"-la"}))
			Expect(needsShell).To(BeFalse())
		})

		It("should identify commands needing shell", func() {
			exe, args, needsShell := executor.ParseCommand("echo test | grep test")
			Expect(exe).To(Equal("bash"))
			Expect(args).To(Equal([]string{"-c", "echo test | grep test"}))
			Expect(needsShell).To(BeTrue())
		})

		It("should handle empty commands", func() {
			exe, args, needsShell := executor.ParseCommand("")
			Expect(exe).To(Equal(""))
			Expect(args).To(BeNil())
			Expect(needsShell).To(BeFalse())
		})

		It("should handle whitespace-only commands", func() {
			exe, args, needsShell := executor.ParseCommand("   ")
			Expect(exe).To(Equal(""))
			Expect(args).To(BeNil())
			Expect(needsShell).To(BeFalse())
		})

		It("should detect various shell operators", func() {
			testCases := []string{
				"cmd1 && cmd2",
				"cmd1 || cmd2",
				"cmd1; cmd2",
				"cmd > file",
				"cmd < file",
				"cmd >> file",
				"cmd 2> error",
				"cmd &",
			}

			for _, tc := range testCases {
				_, _, needsShell := executor.ParseCommand(tc)
				Expect(needsShell).To(BeTrue(), "Failed for: %s", tc)
			}
		})
	})

	Describe("Command Execution Integration", func() {
		BeforeEach(func() {
			exec = executor.New()
		})

		It("should execute and return output", func() {
			command := "echo Hello World"
			if runtime.GOOS == "windows" {
				command = "cmd /c echo Hello World"
			}
			cmd, err := exec.Execute(ctx, command)
			Expect(err).ToNot(HaveOccurred())

			output, err := cmd.Output()
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.TrimSpace(string(output))).To(Equal("Hello World"))
		})

		It("should handle command timeouts", func() {
			timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			command := "sleep 5"
			if runtime.GOOS == "windows" {
				command = "ping -n 6 127.0.0.1"
			}
			cmd, err := exec.Execute(timeoutCtx, command)
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(timeoutCtx.Err()).To(Equal(context.DeadlineExceeded))
			if runtime.GOOS != "windows" {
				Expect(err.Error()).To(ContainSubstring("signal: killed"))
			}
		})
	})
})
