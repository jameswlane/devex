package main_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/shell"
)

var _ = Describe("Shell Manager", func() {
	var (
		tempDir   string
		assetsDir string
		manager   *shell.ShellManager
	)

	BeforeEach(func() {
		// Create temporary directories for testing
		var err error
		tempDir, err = os.MkdirTemp("", "shell-test")
		Expect(err).ToNot(HaveOccurred())

		assetsDir = filepath.Join(tempDir, "assets")
		err = os.MkdirAll(assetsDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		// Create test shell manager
		configDir := filepath.Join(tempDir, ".config", "devex")
		err = os.MkdirAll(configDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		manager = shell.NewShellManagerSimple(tempDir, assetsDir, configDir)
		Expect(manager).ToNot(BeNil())
	})

	AfterEach(func() {
		// Clean up temporary directory
		err := os.RemoveAll(tempDir)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Shell Detection", func() {
		Context("when SHELL environment variable is set", func() {
			BeforeEach(func() {
				os.Setenv("SHELL", "/bin/bash")
			})

			AfterEach(func() {
				os.Unsetenv("SHELL")
			})

			It("should detect bash correctly", func() {
				shellType := shell.DetectUserShell()
				Expect(shellType).To(Equal(shell.Bash))
			})
		})

		Context("when SHELL points to zsh", func() {
			BeforeEach(func() {
				os.Setenv("SHELL", "/usr/local/bin/zsh")
			})

			AfterEach(func() {
				os.Unsetenv("SHELL")
			})

			It("should detect zsh correctly", func() {
				shellType := shell.DetectUserShell()
				Expect(shellType).To(Equal(shell.Zsh))
			})
		})

		Context("when SHELL points to fish", func() {
			BeforeEach(func() {
				os.Setenv("SHELL", "/usr/bin/fish")
			})

			AfterEach(func() {
				os.Unsetenv("SHELL")
			})

			It("should detect fish correctly", func() {
				shellType := shell.DetectUserShell()
				Expect(shellType).To(Equal(shell.Fish))
			})
		})

		Context("when SHELL contains invalid shell name", func() {
			BeforeEach(func() {
				os.Setenv("SHELL", "/usr/bin/some-other-bash-tool")
			})

			AfterEach(func() {
				os.Unsetenv("SHELL")
			})

			It("should default to bash", func() {
				shellType := shell.DetectUserShell()
				Expect(shellType).To(Equal(shell.Bash))
			})
		})

		Context("when SHELL is empty", func() {
			BeforeEach(func() {
				os.Unsetenv("SHELL")
			})

			It("should default to bash", func() {
				shellType := shell.DetectUserShell()
				Expect(shellType).To(Equal(shell.Bash))
			})
		})
	})

	Describe("DeployShellModules", func() {
		Context("when deploying bash modules", func() {
			BeforeEach(func() {
				// Create bash assets directory with double subdirectory structure
				bashDir := filepath.Join(assetsDir, "bash", "bash")
				err := os.MkdirAll(bashDir, 0755)
				Expect(err).ToNot(HaveOccurred())

				// Create test module files (matching expected bash modules)
				moduleFiles := []string{"aliases", "extra", "init", "oh-my-bash", "prompt", "rc", "shell"}
				for _, file := range moduleFiles {
					err := os.WriteFile(filepath.Join(bashDir, file), []byte("# Test "+file+" content\n"), 0644)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should deploy all bash modules successfully", func() {
				err := manager.DeployShellModules("bash")
				Expect(err).ToNot(HaveOccurred())

				// Check that files were deployed to defaults directory
				defaultsDir := filepath.Join(tempDir, ".local", "share", "devex", "defaults", "bash")
				moduleFiles := []string{"aliases", "extra", "init", "oh-my-bash", "prompt", "rc", "shell"}

				for _, file := range moduleFiles {
					destPath := filepath.Join(defaultsDir, file)
					Expect(destPath).To(BeARegularFile())

					// Verify content was copied
					content, err := os.ReadFile(destPath)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(ContainSubstring("Test " + file + " content"))
				}
			})
		})

		Context("when deploying zsh modules", func() {
			BeforeEach(func() {
				// Create zsh assets with single subdirectory structure
				zshDir := filepath.Join(assetsDir, "zsh")
				err := os.MkdirAll(zshDir, 0755)
				Expect(err).ToNot(HaveOccurred())

				// Create test module files (matching expected zsh modules)
				moduleFiles := []string{"aliases", "extra", "init", "oh-my-zsh", "prompt", "rc", "shell", "zplug"}
				for _, file := range moduleFiles {
					err := os.WriteFile(filepath.Join(zshDir, file), []byte("# Test zsh "+file+" content\n"), 0644)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should deploy zsh modules from fallback path", func() {
				err := manager.DeployShellModules("zsh")
				Expect(err).ToNot(HaveOccurred())

				// Check that files were deployed
				defaultsDir := filepath.Join(tempDir, ".local", "share", "devex", "defaults", "zsh")
				moduleFiles := []string{"aliases", "extra", "init", "oh-my-zsh", "prompt", "rc", "shell", "zplug"}

				for _, file := range moduleFiles {
					destPath := filepath.Join(defaultsDir, file)
					Expect(destPath).To(BeARegularFile())

					// Verify content was copied
					content, err := os.ReadFile(destPath)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(ContainSubstring("Test zsh " + file + " content"))
				}
			})
		})

		Context("when some module files are missing", func() {
			BeforeEach(func() {
				// Create bash assets with only some files
				bashDir := filepath.Join(assetsDir, "bash", "bash")
				err := os.MkdirAll(bashDir, 0755)
				Expect(err).ToNot(HaveOccurred())

				// Only create some of the expected files
				err = os.WriteFile(filepath.Join(bashDir, "aliases"), []byte("# Test aliases\n"), 0644)
				Expect(err).ToNot(HaveOccurred())
				err = os.WriteFile(filepath.Join(bashDir, "extra"), []byte("# Test extra\n"), 0644)
				Expect(err).ToNot(HaveOccurred())
				// Missing: init, oh-my-bash, prompt, rc, shell
			})

			It("should deploy available files and skip missing ones", func() {
				err := manager.DeployShellModules("bash")
				Expect(err).ToNot(HaveOccurred())

				defaultsDir := filepath.Join(tempDir, ".local", "share", "devex", "defaults", "bash")

				// These should exist
				Expect(filepath.Join(defaultsDir, "aliases")).To(BeARegularFile())
				Expect(filepath.Join(defaultsDir, "extra")).To(BeARegularFile())

				// These should not exist (missing files)
				Expect(filepath.Join(defaultsDir, "init")).ToNot(BeARegularFile())
				Expect(filepath.Join(defaultsDir, "oh-my-bash")).ToNot(BeARegularFile())
				Expect(filepath.Join(defaultsDir, "prompt")).ToNot(BeARegularFile())
				Expect(filepath.Join(defaultsDir, "rc")).ToNot(BeARegularFile())
				Expect(filepath.Join(defaultsDir, "shell")).ToNot(BeARegularFile())
			})
		})

		Context("when deploying unsupported shell", func() {
			It("should return an error", func() {
				err := manager.DeployShellModules("unsupported-shell")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported shell for module deployment"))
			})
		})

		Context("when asset directory doesn't exist", func() {
			It("should handle missing assets gracefully", func() {
				// Don't create any asset files
				err := manager.DeployShellModules("bash")
				Expect(err).ToNot(HaveOccurred()) // Should not fail, just skip missing files

				defaultsDir := filepath.Join(tempDir, ".local", "share", "devex", "defaults", "bash")
				// Directory should be created but no files
				Expect(defaultsDir).To(BeADirectory())

				files, err := os.ReadDir(defaultsDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(BeEmpty()) // No files should be deployed
			})
		})
	})

	Describe("GetConfigPath", func() {
		Context("for different shell types", func() {
			It("should return correct bash config path", func() {
				path, err := manager.GetConfigPath(shell.Bash)
				Expect(err).ToNot(HaveOccurred())
				Expect(path).To(Equal(filepath.Join(tempDir, ".bashrc")))
			})

			It("should return correct zsh config path", func() {
				path, err := manager.GetConfigPath(shell.Zsh)
				Expect(err).ToNot(HaveOccurred())
				Expect(path).To(Equal(filepath.Join(tempDir, ".zshrc")))
			})

			It("should return correct fish config path", func() {
				path, err := manager.GetConfigPath(shell.Fish)
				Expect(err).ToNot(HaveOccurred())
				expected := filepath.Join(tempDir, ".config", "fish", "config.fish")
				Expect(path).To(Equal(expected))
			})
		})
	})

	Describe("HasMarker", func() {
		Context("when config file has marker", func() {
			BeforeEach(func() {
				configPath := filepath.Join(tempDir, ".bashrc")
				content := `# Some existing content
# DEVEX MANAGED SECTION - DO NOT EDIT MANUALLY
source ~/.local/share/devex/defaults/bash/rc
# END DEVEX MANAGED SECTION
# More content`
				err := os.WriteFile(configPath, []byte(content), 0644)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should detect the marker", func() {
				hasMarker, err := manager.HasMarker(shell.Bash, "DEVEX MANAGED SECTION")
				Expect(err).ToNot(HaveOccurred())
				Expect(hasMarker).To(BeTrue())
			})
		})

		Context("when config file doesn't have marker", func() {
			BeforeEach(func() {
				configPath := filepath.Join(tempDir, ".bashrc")
				content := `# Some existing content without marker
export PATH=$PATH:/usr/local/bin`
				err := os.WriteFile(configPath, []byte(content), 0644)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not detect the marker", func() {
				hasMarker, err := manager.HasMarker(shell.Bash, "DEVEX MANAGED SECTION")
				Expect(err).ToNot(HaveOccurred())
				Expect(hasMarker).To(BeFalse())
			})
		})

		Context("when config file doesn't exist", func() {
			It("should return false without error", func() {
				hasMarker, err := manager.HasMarker(shell.Bash, "DEVEX MANAGED SECTION")
				Expect(err).ToNot(HaveOccurred())
				Expect(hasMarker).To(BeFalse())
			})
		})
	})

	// Skip shell switching tests on certain CI environments
	PDescribe("Shell switching operations", func() {
		It("should be tested manually due to system dependencies", func() {
			Skip("Shell switching requires system modifications and should be tested manually")
		})
	})
})
