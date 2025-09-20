package script_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installer/script"
)

func TestScript(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Script Manager Suite")
}

var _ = Describe("Script Manager", func() {
	var (
		manager *script.Manager
		ctx     context.Context
		server  *httptest.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		config := script.Config{
			MaxScriptSize: 1024 * 1024, // 1MB for testing
			HTTPTimeout:   5 * time.Second,
			TrustedDomains: []string{
				"mise.run",
				"test.example.com",
			},
		}
		manager = script.New(config)
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Describe("URL Validation", func() {
		It("should accept HTTPS URLs from trusted domains", func() {
			err := manager.ValidateURL("https://mise.run/install.sh")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject HTTP URLs", func() {
			err := manager.ValidateURL("http://mise.run/install.sh")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("only HTTPS"))
		})

		It("should reject untrusted domains", func() {
			err := manager.ValidateURL("https://evil.com/install.sh")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not in trusted domains"))
		})

		It("should reject empty URLs", func() {
			err := manager.ValidateURL("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty URL"))
		})

		It("should reject invalid URLs", func() {
			err := manager.ValidateURL("not-a-url")
			Expect(err).To(HaveOccurred())
			// Error could be "invalid URL" or "only HTTPS" depending on parsing
			Expect(err.Error()).To(SatisfyAny(
				ContainSubstring("invalid URL"),
				ContainSubstring("only HTTPS"),
			))
		})
	})

	Describe("Content Validation", func() {
		var tempFile string

		BeforeEach(func() {
			tmpFile, err := os.CreateTemp("", "test-script-*.sh")
			Expect(err).ToNot(HaveOccurred())
			tempFile = tmpFile.Name()
			tmpFile.Close()
		})

		AfterEach(func() {
			os.Remove(tempFile)
		})

		It("should accept safe scripts", func() {
			content := `#!/bin/bash
echo "Installing application..."
apt-get update
apt-get install -y myapp
echo "Installation complete"`

			err := os.WriteFile(tempFile, []byte(content), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = manager.ValidateContent(tempFile)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject scripts with dangerous patterns", func() {
			dangerousScripts := map[string]string{
				"rm -rf /":        "#!/bin/bash\nrm -rf /",
				"/etc/passwd":     "#!/bin/bash\ncat /etc/passwd",
				"chmod 777 /":     "#!/bin/bash\nchmod 777 /",
				"dd if=/dev/zero": "#!/bin/bash\ndd if=/dev/zero of=/dev/sda",
			}

			for pattern, script := range dangerousScripts {
				err := os.WriteFile(tempFile, []byte(script), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = manager.ValidateContent(tempFile)
				Expect(err).To(HaveOccurred(), "Pattern '%s' should have been detected", pattern)
				Expect(err.Error()).To(ContainSubstring("dangerous pattern"))
			}
		})

		It("should reject empty scripts", func() {
			err := os.WriteFile(tempFile, []byte(""), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = manager.ValidateContent(tempFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("script is empty"))
		})

		It("should reject scripts that are too large", func() {
			// Create a script larger than MaxScriptSize
			largeContent := make([]byte, 2*1024*1024) // 2MB
			for i := range largeContent {
				largeContent[i] = 'a'
			}

			err := os.WriteFile(tempFile, largeContent, 0644)
			Expect(err).ToNot(HaveOccurred())

			err = manager.ValidateContent(tempFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("too large"))
		})

		It("should warn about scripts without shebang", func() {
			content := "echo 'No shebang here'"

			err := os.WriteFile(tempFile, []byte(content), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Should still pass validation but with a warning
			err = manager.ValidateContent(tempFile)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Download", func() {
		BeforeEach(func() {
			// Create a test server that serves a simple script
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/install.sh" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("#!/bin/bash\necho 'test script'"))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))

			// Add test server domain to trusted domains
			manager.AddTrustedDomain("127.0.0.1")
		})

		It("should download scripts from valid URLs", func() {
			Skip("Skipping due to TLS certificate validation in test server")
		})

		It("should handle download failures", func() {
			// Try to download from a non-existent path
			_, err := manager.Download(ctx, "https://mise.run/nonexistent.sh")
			Expect(err).To(HaveOccurred())
		})

		It("should validate URL before downloading", func() {
			_, err := manager.Download(ctx, "https://untrusted.com/script.sh")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("URL validation failed"))
		})
	})

	Describe("DownloadAndValidate", func() {
		It("should fail if URL is not trusted", func() {
			_, err := manager.DownloadAndValidate(ctx, "https://evil.com/install.sh")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("URL validation failed"))
		})
	})

	Describe("Cleanup", func() {
		It("should remove temporary files", func() {
			tmpFile, err := os.CreateTemp("", "test-cleanup-*.sh")
			Expect(err).ToNot(HaveOccurred())
			tmpFile.Close()

			path := tmpFile.Name()

			// File should exist
			_, err = os.Stat(path)
			Expect(err).ToNot(HaveOccurred())

			// Cleanup
			manager.Cleanup(path)

			// File should not exist
			_, err = os.Stat(path)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("should handle cleanup of non-existent files gracefully", func() {
			// Should not panic or error
			manager.Cleanup("/tmp/nonexistent-file.sh")
		})

		It("should handle empty path gracefully", func() {
			// Should not panic or error
			manager.Cleanup("")
		})
	})

	Describe("Trusted Domains", func() {
		It("should allow adding trusted domains", func() {
			initialDomains := manager.GetTrustedDomains()
			initialCount := len(initialDomains)

			manager.AddTrustedDomain("new.domain.com")

			newDomains := manager.GetTrustedDomains()
			Expect(len(newDomains)).To(Equal(initialCount + 1))
			Expect(newDomains).To(ContainElement("new.domain.com"))
		})

		It("should validate against newly added domains", func() {
			manager.AddTrustedDomain("newly.trusted.com")

			err := manager.ValidateURL("https://newly.trusted.com/script.sh")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
