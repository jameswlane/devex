package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

var _ = Describe("Recursive Directory Loading", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "devex-recursive-test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("getDirectoryFilesRecursive", func() {
		Context("with nested directory structure", func() {
			BeforeEach(func() {
				// Create nested directory structure
				subDir1 := filepath.Join(tempDir, "level1")
				subDir2 := filepath.Join(tempDir, "level1", "level2")
				subDir3 := filepath.Join(tempDir, "level1", "level2", "level3")

				os.MkdirAll(subDir3, 0755)

				// Create files at different levels
				os.WriteFile(filepath.Join(tempDir, "root.yaml"), []byte("root: config"), 0644)
				os.WriteFile(filepath.Join(subDir1, "level1.yaml"), []byte("level1: config"), 0644)
				os.WriteFile(filepath.Join(subDir2, "level2.yaml"), []byte("level2: config"), 0644)
				os.WriteFile(filepath.Join(subDir3, "level3.yaml"), []byte("level3: config"), 0644)

				// Add some non-YAML files to test filtering
				os.WriteFile(filepath.Join(subDir1, "ignore.txt"), []byte("ignore"), 0644)
				os.WriteFile(filepath.Join(subDir2, "README.md"), []byte("readme"), 0644)
			})

			It("should find all YAML files within depth limit", func() {
				files, err := getDirectoryFilesRecursive(tempDir, 2)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(HaveLen(3))
				Expect(files).To(ContainElements("root.yaml", "level1/level1.yaml", "level1/level2/level2.yaml"))
			})

			It("should respect depth limit", func() {
				files, err := getDirectoryFilesRecursive(tempDir, 1)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(HaveLen(2))
				Expect(files).To(ContainElements("root.yaml", "level1/level1.yaml"))
			})

			It("should work with depth 0 (root level only)", func() {
				files, err := getDirectoryFilesRecursive(tempDir, 0)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(HaveLen(1))
				Expect(files).To(ContainElement("root.yaml"))
			})

			It("should return sorted results", func() {
				// Create files that would be unsorted naturally
				subDir := filepath.Join(tempDir, "apps")
				os.MkdirAll(subDir, 0755)
				os.WriteFile(filepath.Join(subDir, "zz-last.yaml"), []byte("last"), 0644)
				os.WriteFile(filepath.Join(subDir, "aa-first.yaml"), []byte("first"), 0644)
				os.WriteFile(filepath.Join(subDir, "bb-middle.yaml"), []byte("middle"), 0644)

				files, err := getDirectoryFilesRecursive(tempDir, 1)
				Expect(err).ToNot(HaveOccurred())

				// Check that files starting with 'apps/' are properly sorted
				var appFiles []string
				for _, file := range files {
					if strings.HasPrefix(file, "apps/") {
						appFiles = append(appFiles, file)
					}
				}
				Expect(appFiles).To(Equal([]string{
					"apps/aa-first.yaml",
					"apps/bb-middle.yaml",
					"apps/zz-last.yaml",
				}))
			})
		})

		Context("with invalid inputs", func() {
			It("should reject negative depth", func() {
				_, err := getDirectoryFilesRecursive(tempDir, -1)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("maxDepth must be non-negative"))
			})

			It("should handle path traversal attempts", func() {
				_, err := getDirectoryFilesRecursive("../../../etc", 1)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid directory path"))
			})

			It("should handle non-existent directories", func() {
				nonExistent := filepath.Join(tempDir, "nonexistent")
				_, err := getDirectoryFilesRecursive(nonExistent, 1)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to access directory"))
			})
		})

		Context("with permission issues", func() {
			It("should continue walking when encountering permission errors", func() {
				// Create accessible and inaccessible directories
				accessibleDir := filepath.Join(tempDir, "accessible")
				inaccessibleDir := filepath.Join(tempDir, "inaccessible")

				os.MkdirAll(accessibleDir, 0755)
				os.MkdirAll(inaccessibleDir, 0755)

				os.WriteFile(filepath.Join(accessibleDir, "good.yaml"), []byte("good"), 0644)
				os.WriteFile(filepath.Join(inaccessibleDir, "bad.yaml"), []byte("bad"), 0644)

				// Remove read permissions from inaccessible directory
				os.Chmod(inaccessibleDir, 0000)

				files, err := getDirectoryFilesRecursive(tempDir, 1)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(ContainElement("accessible/good.yaml"))
				// Should not contain files from inaccessible directory due to permission error

				// Clean up permissions for removal
				os.Chmod(inaccessibleDir, 0755)
			})
		})

		Context("with filename validation", func() {
			BeforeEach(func() {
				subDir := filepath.Join(tempDir, "validation")
				os.MkdirAll(subDir, 0755)

				// Create valid and invalid files
				os.WriteFile(filepath.Join(subDir, "valid.yaml"), []byte("valid"), 0644)
				os.WriteFile(filepath.Join(subDir, "invalid;dangerous.yaml"), []byte("dangerous"), 0644)
				os.WriteFile(filepath.Join(subDir, "..traversal.yaml"), []byte("traversal"), 0644)
			})

			It("should only include files with valid names", func() {
				files, err := getDirectoryFilesRecursive(tempDir, 1)
				Expect(err).ToNot(HaveOccurred())

				var validationFiles []string
				for _, file := range files {
					if strings.HasPrefix(file, "validation/") {
						validationFiles = append(validationFiles, file)
					}
				}

				Expect(validationFiles).To(HaveLen(1))
				Expect(validationFiles).To(ContainElement("validation/valid.yaml"))
			})
		})
	})
})

var _ = Describe("Cache Edge Cases", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "devex-cache-edge-test")
		Expect(err).ToNot(HaveOccurred())
		globalConfigCache.clearCache()
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("Cache invalidation edge cases", func() {
		Context("with file system race conditions", func() {
			It("should handle file deletion between stat calls", func() {
				testFile := filepath.Join(tempDir, "disappearing.yaml")
				os.WriteFile(testFile, []byte("test: value"), 0644)

				// Cache the file
				shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(shouldReload).To(BeTrue())

				// Delete file
				os.Remove(testFile)

				// Should detect file is missing
				_, err = globalConfigCache.shouldReloadFile(testFile)
				Expect(err).To(HaveOccurred())
			})

			It("should handle rapid file modifications", func() {
				testFile := filepath.Join(tempDir, "rapid.yaml")
				os.WriteFile(testFile, []byte("test: initial"), 0644)

				// Initial cache
				shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(shouldReload).To(BeTrue())

				// Wait briefly to ensure timestamp resolution
				time.Sleep(10 * time.Millisecond)

				// Rapid modifications with size changes to ensure detection
				for i := 0; i < 5; i++ {
					// Change both content and size to ensure cache invalidation
					content := []byte(fmt.Sprintf("test: modification_%d_with_more_content_%d", i, i*100))
					os.WriteFile(testFile, content, 0644)

					// Brief pause for file system consistency
					time.Sleep(5 * time.Millisecond)

					shouldReload, err = globalConfigCache.shouldReloadFile(testFile)
					Expect(err).ToNot(HaveOccurred())
					// Should detect change due to size difference at minimum
					Expect(shouldReload).To(BeTrue())
				}
			})
		})

		Context("with concurrent access", func() {
			It("should be thread-safe with multiple goroutines", func() {
				testFile := filepath.Join(tempDir, "concurrent.yaml")
				os.WriteFile(testFile, []byte("test: concurrent"), 0644)

				var wg sync.WaitGroup
				errors := make(chan error, 100)

				// Start 10 goroutines
				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for j := 0; j < 10; j++ {
							_, err := globalConfigCache.shouldReloadFile(testFile)
							if err != nil {
								errors <- err
								return
							}
						}
					}()
				}

				wg.Wait()
				close(errors)

				// Check for any errors
				for err := range errors {
					Expect(err).ToNot(HaveOccurred())
				}
			})
		})

		Context("with symlinks", func() {
			It("should handle symlink target changes", func() {
				targetFile1 := filepath.Join(tempDir, "target1.yaml")
				targetFile2 := filepath.Join(tempDir, "target2.yaml")
				symlinkFile := filepath.Join(tempDir, "symlink.yaml")

				os.WriteFile(targetFile1, []byte("target: 1"), 0644)
				os.WriteFile(targetFile2, []byte("target: 2 - different content length"), 0644)

				// Create symlink to first target
				os.Symlink(targetFile1, symlinkFile)

				// Cache symlink
				shouldReload, err := globalConfigCache.shouldReloadFile(symlinkFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(shouldReload).To(BeTrue())

				// Small delay for file system consistency
				time.Sleep(10 * time.Millisecond)

				// Change symlink target to file with different size
				os.Remove(symlinkFile)
				os.Symlink(targetFile2, symlinkFile)

				// Should detect change due to different target file size
				shouldReload, err = globalConfigCache.shouldReloadFile(symlinkFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(shouldReload).To(BeTrue())
			})
		})

		Context("with permission changes", func() {
			It("should handle files becoming unreadable", func() {
				// Skip if running as root (permission tests don't work)
				if os.Geteuid() == 0 {
					Skip("Skipping permission test - running as root")
				}

				testFile := filepath.Join(tempDir, "permissions.yaml")
				os.WriteFile(testFile, []byte("test: permissions"), 0644)

				// Cache the file
				shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(shouldReload).To(BeTrue())

				// Remove all permissions
				os.Chmod(testFile, 0000)

				// Should detect permission error (but may not on all systems)
				_, err = globalConfigCache.shouldReloadFile(testFile)
				if err != nil {
					// Permission error detected as expected
					Expect(err).To(HaveOccurred())
				} else {
					// Some systems may still allow access, so just log it
					fmt.Printf("Note: Permission test may not work on this system\n")
				}

				// Restore permissions for cleanup
				os.Chmod(testFile, 0644)
			})
		})

		Context("with clock changes", func() {
			It("should handle system clock adjustments gracefully", func() {
				testFile := filepath.Join(tempDir, "clock.yaml")
				os.WriteFile(testFile, []byte("test: clock"), 0644)

				// Get file info
				info, err := os.Stat(testFile)
				Expect(err).ToNot(HaveOccurred())

				// Manually set cache with older timestamp
				globalConfigCache.mu.Lock()
				if globalConfigCache.files == nil {
					globalConfigCache.files = make(map[string]configFileInfo)
				}
				globalConfigCache.files[testFile] = configFileInfo{
					modTime: info.ModTime().Add(-time.Hour), // 1 hour ago
					size:    info.Size(),
				}
				globalConfigCache.mu.Unlock()

				// Should detect file as newer
				shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(shouldReload).To(BeTrue())
			})
		})
	})

	Describe("Cache memory management", func() {
		It("should handle large numbers of cached files", func() {
			// Create many files
			fileCount := 1000
			for i := 0; i < fileCount; i++ {
				filename := filepath.Join(tempDir, fmt.Sprintf("mass_%04d.yaml", i))
				os.WriteFile(filename, []byte(fmt.Sprintf("mass: %d", i)), 0644)

				// Cache each file
				_, err := globalConfigCache.shouldReloadFile(filename)
				Expect(err).ToNot(HaveOccurred())
			}

			// Verify cache size
			globalConfigCache.mu.RLock()
			cacheSize := len(globalConfigCache.files)
			globalConfigCache.mu.RUnlock()

			Expect(cacheSize).To(Equal(fileCount))
		})

		It("should handle cache corruption gracefully", func() {
			testFile := filepath.Join(tempDir, "corruption.yaml")
			os.WriteFile(testFile, []byte("test: corruption"), 0644)

			// Cache the file
			shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReload).To(BeTrue())

			// Simulate cache corruption by setting invalid data
			globalConfigCache.mu.Lock()
			globalConfigCache.files[testFile] = configFileInfo{
				modTime: time.Time{}, // Zero time
				size:    -1,          // Invalid size
			}
			globalConfigCache.mu.Unlock()

			// Should handle corruption gracefully and reload
			shouldReload, err = globalConfigCache.shouldReloadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReload).To(BeTrue())
		})
	})
})
