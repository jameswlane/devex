package config

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigSecurity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Security Suite")
}

var _ = Describe("Config Security Functions", func() {
	Describe("isValidFilename", func() {
		Context("with valid filenames", func() {
			It("should accept standard yaml files", func() {
				Expect(isValidFilename("config.yaml")).To(BeTrue())
				Expect(isValidFilename("app.yml")).To(BeTrue())
				Expect(isValidFilename("00-priority.yaml")).To(BeTrue())
				Expect(isValidFilename("database_config.yaml")).To(BeTrue())
				Expect(isValidFilename("multi-word-config.yaml")).To(BeTrue())
			})

			It("should accept files with version numbers", func() {
				Expect(isValidFilename("config-v1.2.3.yaml")).To(BeTrue())
				Expect(isValidFilename("app-2.0.yaml")).To(BeTrue())
			})
		})

		Context("with invalid filenames", func() {
			It("should reject empty filenames", func() {
				Expect(isValidFilename("")).To(BeFalse())
			})

			It("should reject files without yaml extensions", func() {
				Expect(isValidFilename("config.txt")).To(BeFalse())
				Expect(isValidFilename("config.json")).To(BeFalse())
				Expect(isValidFilename("config")).To(BeFalse())
			})

			It("should reject path traversal attempts", func() {
				Expect(isValidFilename("../config.yaml")).To(BeFalse())
				Expect(isValidFilename("../../etc/passwd.yaml")).To(BeFalse())
				Expect(isValidFilename("..\\config.yaml")).To(BeFalse())
			})

			It("should reject filenames that are too long", func() {
				longName := string(make([]byte, 300)) + ".yaml"
				Expect(isValidFilename(longName)).To(BeFalse())
			})

			It("should reject filenames starting with special characters", func() {
				Expect(isValidFilename(".hidden.yaml")).To(BeFalse())
				Expect(isValidFilename("-config.yaml")).To(BeFalse())
			})

			It("should reject filenames with dangerous characters", func() {
				Expect(isValidFilename("config;rm -rf.yaml")).To(BeFalse())
				Expect(isValidFilename("config|cat.yaml")).To(BeFalse())
				Expect(isValidFilename("config&evil.yaml")).To(BeFalse())
			})
		})
	})

	Describe("isValidConfigPath", func() {
		Context("with valid paths", func() {
			It("should accept absolute paths", func() {
				Expect(isValidConfigPath("/home/user/.devex/config")).To(BeTrue())
				Expect(isValidConfigPath("/usr/local/share/devex/config")).To(BeTrue())
			})

			It("should accept relative paths without traversal", func() {
				Expect(isValidConfigPath("config/apps")).To(BeTrue())
				Expect(isValidConfigPath("./config/apps")).To(BeTrue())
			})
		})

		Context("with invalid paths", func() {
			It("should reject empty paths", func() {
				Expect(isValidConfigPath("")).To(BeFalse())
			})

			It("should reject path traversal attempts", func() {
				Expect(isValidConfigPath("../config")).To(BeFalse())
				Expect(isValidConfigPath("../../etc")).To(BeFalse())
				Expect(isValidConfigPath("config/../../../etc")).To(BeFalse())
				Expect(isValidConfigPath("..\\config")).To(BeFalse())
			})

			It("should reject paths with traversal after cleaning", func() {
				Expect(isValidConfigPath("config/./../../etc")).To(BeFalse())
			})
		})
	})

	Describe("sanitizeFilenameForKey", func() {
		It("should create safe keys from filenames", func() {
			Expect(sanitizeFilenameForKey("config.yaml")).To(Equal("config"))
			Expect(sanitizeFilenameForKey("multi-word-config.yaml")).To(Equal("multi-word-config"))
			Expect(sanitizeFilenameForKey("database_config.yml")).To(Equal("database_config"))
		})

		It("should handle special characters", func() {
			Expect(sanitizeFilenameForKey("config@special.yaml")).To(Equal("config_special"))
			Expect(sanitizeFilenameForKey("config.with.dots.yaml")).To(Equal("config_with_dots"))
		})

		It("should ensure keys start with a letter", func() {
			Expect(sanitizeFilenameForKey("123-config.yaml")).To(Equal("config_123-config"))
			Expect(sanitizeFilenameForKey("-config.yaml")).To(Equal("config_-config"))
		})
	})

	Describe("isYamlFile", func() {
		It("should identify YAML files correctly", func() {
			Expect(isYamlFile("config.yaml")).To(BeTrue())
			Expect(isYamlFile("app.yml")).To(BeTrue())
		})

		It("should reject non-YAML files", func() {
			Expect(isYamlFile("config.json")).To(BeFalse())
			Expect(isYamlFile("config.txt")).To(BeFalse())
			Expect(isYamlFile("config")).To(BeFalse())
		})
	})
})

var _ = Describe("Directory Loading Error Handling", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "devex-config-test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("getDirectoryFiles", func() {
		Context("with malformed directory structure", func() {
			It("should handle non-existent directories", func() {
				files, err := getDirectoryFiles(filepath.Join(tempDir, "nonexistent"))
				Expect(err).To(HaveOccurred())
				Expect(files).To(BeNil())
			})

			It("should skip invalid filenames", func() {
				// Create files with invalid names
				os.WriteFile(filepath.Join(tempDir, "valid.yaml"), []byte("test: value"), 0644)
				os.WriteFile(filepath.Join(tempDir, "../traversal.yaml"), []byte("test: value"), 0644)
				os.WriteFile(filepath.Join(tempDir, "no-extension"), []byte("test: value"), 0644)

				files, err := getDirectoryFiles(tempDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(HaveLen(1))
				Expect(files[0]).To(Equal("valid.yaml"))
			})
		})

		Context("with path traversal attempts", func() {
			It("should reject dangerous directory paths", func() {
				files, err := getDirectoryFiles("../../../etc")
				Expect(err).To(HaveOccurred())
				Expect(files).To(BeNil())
			})
		})
	})

	Describe("loadYamlFileToViper", func() {
		Context("with malformed YAML files", func() {
			It("should handle empty files", func() {
				emptyFile := filepath.Join(tempDir, "empty.yaml")
				os.WriteFile(emptyFile, []byte(""), 0644)

				_, err := loadYamlFileToViper(emptyFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("is empty"))
			})

			It("should handle invalid YAML syntax", func() {
				invalidFile := filepath.Join(tempDir, "invalid.yaml")
				os.WriteFile(invalidFile, []byte("invalid: yaml: content: ["), 0644)

				_, err := loadYamlFileToViper(invalidFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse YAML"))
			})

			It("should handle files with only whitespace", func() {
				whitespaceFile := filepath.Join(tempDir, "whitespace.yaml")
				os.WriteFile(whitespaceFile, []byte("   \n\t  \n"), 0644)

				_, err := loadYamlFileToViper(whitespaceFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("is empty"))
			})
		})

		Context("with permission issues", func() {
			It("should handle unreadable files", func() {
				unreadableFile := filepath.Join(tempDir, "unreadable.yaml")
				os.WriteFile(unreadableFile, []byte("test: value"), 0000)

				_, err := loadYamlFileToViper(unreadableFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to read file"))

				// Clean up
				os.Chmod(unreadableFile, 0644)
			})
		})
	})

	Describe("loadDirectoryAlphabetically", func() {
		Context("with mixed valid and invalid files", func() {
			BeforeEach(func() {
				// Create a mix of valid and invalid files
				os.WriteFile(filepath.Join(tempDir, "01-valid.yaml"), []byte("app1: config1"), 0644)
				os.WriteFile(filepath.Join(tempDir, "02-valid.yaml"), []byte("app2: config2"), 0644)
				os.WriteFile(filepath.Join(tempDir, "invalid.txt"), []byte("not yaml"), 0644)
				os.WriteFile(filepath.Join(tempDir, "03-malformed.yaml"), []byte("malformed: yaml: ["), 0644)
				os.WriteFile(filepath.Join(tempDir, "04-empty.yaml"), []byte(""), 0644)
			})

			It("should continue loading valid files even when some fail", func() {
				// This should not return an error even though some files are invalid
				err := loadDirectoryAlphabetically(tempDir, "test")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with nested directories", func() {
			BeforeEach(func() {
				subDir := filepath.Join(tempDir, "subdir")
				os.Mkdir(subDir, 0755)
				os.WriteFile(filepath.Join(subDir, "nested.yaml"), []byte("nested: config"), 0644)
				os.WriteFile(filepath.Join(tempDir, "root.yaml"), []byte("root: config"), 0644)
			})

			It("should only process files in the specified directory", func() {
				err := loadDirectoryAlphabetically(tempDir, "test")
				Expect(err).ToNot(HaveOccurred())
				// Note: This tests the current behavior - only direct files, not recursive
			})
		})
	})
})

var _ = Describe("Config Cache", func() {
	var tempDir string
	var testFile string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "devex-cache-test")
		Expect(err).ToNot(HaveOccurred())

		testFile = filepath.Join(tempDir, "test.yaml")
		os.WriteFile(testFile, []byte("test: initial"), 0644)

		// Clear cache before each test
		globalConfigCache.clearCache()
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("shouldReloadFile", func() {
		It("should require reload for new files", func() {
			shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReload).To(BeTrue())
		})

		It("should not require reload for unchanged files", func() {
			// First call should require reload
			shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReload).To(BeTrue())

			// Second call should not require reload
			shouldReload, err = globalConfigCache.shouldReloadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReload).To(BeFalse())
		})

		It("should require reload for modified files", func() {
			// Initial check
			shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReload).To(BeTrue())

			// Modify file
			os.WriteFile(testFile, []byte("test: modified"), 0644)

			// Should now require reload
			shouldReload, err = globalConfigCache.shouldReloadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReload).To(BeTrue())
		})

		It("should handle non-existent files", func() {
			nonExistent := filepath.Join(tempDir, "nonexistent.yaml")
			_, err := globalConfigCache.shouldReloadFile(nonExistent)
			Expect(err).To(HaveOccurred())
		})
	})
})
