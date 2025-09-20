package security_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installer/security"
)

func TestSecurity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Security Suite")
}

var _ = Describe("Security", func() {
	Describe("SecureString", func() {
		It("should store and retrieve strings securely", func() {
			ss := security.NewSecureString("secret123")
			Expect(ss.String()).To(Equal("secret123"))
		})

		It("should clear data from memory", func() {
			ss := security.NewSecureString("secret123")
			ss.Clear()
			Expect(ss.String()).To(Equal(""))
		})

		It("should handle multiple clears safely", func() {
			ss := security.NewSecureString("secret123")
			ss.Clear()
			ss.Clear() // Should not panic
			Expect(ss.String()).To(Equal(""))
		})
	})

	Describe("URLValidator", func() {
		var validator *security.URLValidator

		BeforeEach(func() {
			validator = security.NewURLValidator([]string{"example.com", "trusted.org"})
		})

		It("should accept HTTPS URLs from trusted domains", func() {
			err := validator.ValidateURL("https://example.com/path")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject HTTP URLs", func() {
			err := validator.ValidateURL("http://example.com/path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("only HTTPS"))
		})

		It("should reject untrusted domains", func() {
			err := validator.ValidateURL("https://evil.com/script")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not in trusted domains"))
		})

		It("should reject empty URLs", func() {
			err := validator.ValidateURL("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty URL"))
		})

		It("should reject invalid URLs", func() {
			err := validator.ValidateURL("not-a-url")
			Expect(err).To(HaveOccurred())
			// The error could be either "invalid URL" or "only HTTPS" depending on parsing
			Expect(err.Error()).To(SatisfyAny(
				ContainSubstring("invalid URL"),
				ContainSubstring("only HTTPS"),
			))
		})

		It("should allow adding trusted domains", func() {
			validator.AddTrustedDomain("new.domain.com")
			domains := validator.GetTrustedDomains()
			Expect(domains).To(ContainElement("new.domain.com"))

			err := validator.ValidateURL("https://new.domain.com/path")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("PathValidator", func() {
		var validator *security.PathValidator

		BeforeEach(func() {
			validator = security.NewPathValidator()
		})

		It("should validate safe temp paths", func() {
			tempDir := os.TempDir()
			safePath := filepath.Join(tempDir, "test-file.txt")
			err := validator.ValidateTempPath(safePath)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject relative paths", func() {
			err := validator.ValidateTempPath("../etc/passwd")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must be absolute"))
		})

		It("should reject directory traversal attempts", func() {
			tempDir := os.TempDir()
			maliciousPath := filepath.Join(tempDir, "../../../etc/passwd")
			err := validator.ValidateTempPath(maliciousPath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("directory traversal"))
		})

		It("should reject empty paths", func() {
			err := validator.ValidateTempPath("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty path"))
		})

		It("should reject dangerous config paths", func() {
			dangerousPaths := []string{
				"/etc/passwd",
				"/etc/shadow",
				"/etc/sudoers",
				"/root/test",
				"/sys/test",
				"/proc/test",
				"/dev/test",
			}

			for _, path := range dangerousPaths {
				err := validator.ValidateConfigPath(path)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dangerous path"))
			}
		})

		It("should allow safe config paths", func() {
			safePaths := []string{
				"/home/user/.config/app/config.json",
				"/opt/app/config.yaml",
				"/usr/local/share/app/settings.conf",
			}

			for _, path := range safePaths {
				err := validator.ValidateConfigPath(path)
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})

	Describe("InputSanitizer", func() {
		var sanitizer *security.InputSanitizer

		BeforeEach(func() {
			sanitizer = security.NewInputSanitizer()
		})

		It("should remove null bytes", func() {
			input := "test\x00malicious"
			result := sanitizer.SanitizeUserInput(input)
			Expect(result).To(Equal("testmalicious"))
		})

		It("should remove control characters but preserve tabs and newlines", func() {
			input := "test\x01\x02\ttab\nline\x03\x04"
			result := sanitizer.SanitizeUserInput(input)
			Expect(result).To(Equal("test\ttab\nline"))
		})

		It("should trim whitespace", func() {
			input := "  test input  "
			result := sanitizer.SanitizeUserInput(input)
			Expect(result).To(Equal("test input"))
		})

		It("should sanitize passwords and return SecureString", func() {
			password := "  secret\x00pass  "
			securePass := sanitizer.SanitizePassword(password)
			Expect(securePass.String()).To(Equal("secretpass"))
			securePass.Clear()
		})
	})

	Describe("TempFileManager", func() {
		var manager *security.TempFileManager

		BeforeEach(func() {
			manager = security.NewTempFileManager()
		})

		It("should create secure temporary files", func() {
			tmpFile, err := manager.CreateSecureTempFile("", "test-*.txt")
			Expect(err).ToNot(HaveOccurred())
			defer tmpFile.Close()
			defer manager.CleanupTempFile(tmpFile.Name())

			// Check file exists and has secure permissions
			info, err := os.Stat(tmpFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
		})

		It("should clean up temporary files safely", func() {
			tmpFile, err := manager.CreateSecureTempFile("", "test-*.txt")
			Expect(err).ToNot(HaveOccurred())
			tmpFile.Close()

			path := tmpFile.Name()

			// File should exist
			_, err = os.Stat(path)
			Expect(err).ToNot(HaveOccurred())

			// Clean up
			err = manager.CleanupTempFile(path)
			Expect(err).ToNot(HaveOccurred())

			// File should not exist
			_, err = os.Stat(path)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("should refuse to clean up non-temp files", func() {
			err := manager.CleanupTempFile("/etc/passwd")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("refusing to remove"))
		})

		It("should handle empty path cleanup gracefully", func() {
			err := manager.CleanupTempFile("")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("ContentValidator", func() {
		var validator *security.ContentValidator

		BeforeEach(func() {
			validator = security.NewContentValidator()
		})

		It("should accept safe script content", func() {
			content := `#!/bin/bash
echo "Installing application..."
apt-get update
apt-get install -y myapp
echo "Installation complete"`

			err := validator.ValidateScriptContent(content, 1024*1024) // 1MB limit
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject scripts with dangerous patterns", func() {
			// Test individual patterns to see which ones are working
			dangerousScripts := map[string]string{
				"rm -rf /":        "#!/bin/bash\nrm -rf /",
				"dd if=/dev/zero": "#!/bin/bash\ndd if=/dev/zero of=/dev/sda",
				":(){ :|:& };:":   "#!/bin/bash\n:(){ :|:& };:",
				"/etc/passwd":     "#!/bin/bash\ncat /etc/passwd",
				"chmod 777 /":     "#!/bin/bash\nchmod 777 /",
			}

			for pattern, script := range dangerousScripts {
				err := validator.ValidateScriptContent(script, 1024*1024)
				Expect(err).To(HaveOccurred(), "Pattern '%s' should have been detected in script: %s", pattern, script)
				Expect(err.Error()).To(ContainSubstring("dangerous pattern"))
			}
		})

		It("should reject empty scripts", func() {
			err := validator.ValidateScriptContent("", 1024*1024)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("script is empty"))
		})

		It("should reject scripts that are too large", func() {
			largeScript := strings.Repeat("a", 1000)
			err := validator.ValidateScriptContent(largeScript, 100) // 100 byte limit
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("too large"))
		})
	})

	Describe("SecurityManager", func() {
		var manager *security.SecurityManager

		BeforeEach(func() {
			manager = security.NewSecurityManager(security.DefaultTrustedDomains())
		})

		It("should provide access to all security components", func() {
			Expect(manager.URLValidator).ToNot(BeNil())
			Expect(manager.PathValidator).ToNot(BeNil())
			Expect(manager.InputSanitizer).ToNot(BeNil())
			Expect(manager.TempFileManager).ToNot(BeNil())
			Expect(manager.ContentValidator).ToNot(BeNil())
		})

		It("should have default trusted domains", func() {
			domains := security.DefaultTrustedDomains()
			Expect(domains).To(ContainElement("mise.run"))
			Expect(domains).To(ContainElement("get.docker.com"))
			Expect(len(domains)).To(BeNumerically(">", 0))
		})

		It("should allow comprehensive validation workflow", func() {
			// URL validation
			err := manager.URLValidator.ValidateURL("https://mise.run/install.sh")
			Expect(err).ToNot(HaveOccurred())

			// Input sanitization
			cleanInput := manager.InputSanitizer.SanitizeUserInput("test\x00input")
			Expect(cleanInput).To(Equal("testinput"))

			// Content validation
			content := "#!/bin/bash\necho 'safe script'"
			err = manager.ContentValidator.ValidateScriptContent(content, 1024)
			Expect(err).ToNot(HaveOccurred())

			// Temp file management
			tmpFile, err := manager.TempFileManager.CreateSecureTempFile("", "test-*.sh")
			Expect(err).ToNot(HaveOccurred())
			defer tmpFile.Close()
			defer manager.TempFileManager.CleanupTempFile(tmpFile.Name())
		})
	})
})
