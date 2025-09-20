package bootstrap_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
)

var _ = Describe("PluginBootstrap", func() {
	var (
		pluginBootstrap *bootstrap.PluginBootstrap
		tempHomeDir     string
		ctx             context.Context
	)

	BeforeEach(func() {
		var err error
		tempHomeDir, err = os.MkdirTemp("", "plugin-bootstrap-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Set HOME environment variable to temp directory
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHomeDir)

		// Clean up in AfterEach
		DeferCleanup(func() {
			os.Setenv("HOME", originalHome)
			os.RemoveAll(tempHomeDir)
		})

		ctx = context.Background()
	})

	Describe("NewPluginBootstrap", func() {
		Context("when skipDownload is false", func() {
			It("should create bootstrap instance with download enabled", func() {
				bootstrap, err := bootstrap.NewPluginBootstrap(false)
				Expect(err).NotTo(HaveOccurred())
				Expect(bootstrap).NotTo(BeNil())

				// Verify plugin directory was created
				pluginDir := filepath.Join(tempHomeDir, ".devex", "plugins")
				Expect(pluginDir).To(BeADirectory())
			})
		})

		Context("when skipDownload is true", func() {
			It("should create bootstrap instance with download disabled", func() {
				bootstrap, err := bootstrap.NewPluginBootstrap(true)
				Expect(err).NotTo(HaveOccurred())
				Expect(bootstrap).NotTo(BeNil())
			})
		})

		Context("when home directory is not accessible", func() {
			BeforeEach(func() {
				os.Setenv("HOME", "/nonexistent/directory")
			})

			It("should return an error", func() {
				_, err := bootstrap.NewPluginBootstrap(false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get user home directory"))
			})
		})
	})

	Describe("Initialize", func() {
		BeforeEach(func() {
			var err error
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true) // Skip download for tests
			Expect(err).NotTo(HaveOccurred())
		})

		It("should initialize successfully", func() {
			err := pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should detect platform information", func() {
			err := pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())

			platform := pluginBootstrap.GetPlatform()
			Expect(platform).NotTo(BeNil())
			Expect(platform.OS).NotTo(BeEmpty())
			Expect(platform.Architecture).NotTo(BeEmpty())
		})

		It("should create plugin manager", func() {
			err := pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())

			manager := pluginBootstrap.GetManager()
			Expect(manager).NotTo(BeNil())
		})
	})

	Describe("Platform Detection", func() {
		BeforeEach(func() {
			var err error
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true)
			Expect(err).NotTo(HaveOccurred())

			err = pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should detect the current operating system", func() {
			platform := pluginBootstrap.GetPlatform()

			// OS should be one of the supported platforms
			Expect(platform.OS).To(BeElementOf([]string{"linux", "darwin", "windows"}))
		})

		It("should detect available package managers", func() {
			platform := pluginBootstrap.GetPlatform()

			// Should have detected at least one package manager or none if in test environment
			Expect(platform.PackageManagers).To(BeAssignableToTypeOf([]string{}))
		})

		It("should provide platform string representation", func() {
			platform := pluginBootstrap.GetPlatform()
			platformStr := platform.String()

			Expect(platformStr).NotTo(BeEmpty())
			Expect(platformStr).To(ContainSubstring(platform.OS))
		})
	})

	Describe("Plugin Management Commands", func() {
		var mockRootCmd *cobra.Command

		BeforeEach(func() {
			var err error
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true)
			Expect(err).NotTo(HaveOccurred())

			err = pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Create a mock root command
			mockRootCmd = &cobra.Command{
				Use:   "devex",
				Short: "DevEx CLI",
			}
		})

		It("should register plugin management commands", func() {
			pluginBootstrap.RegisterCommands(mockRootCmd)

			// Verify plugin command was added
			pluginCmd, _, err := mockRootCmd.Find([]string{"plugin"})
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginCmd.Name()).To(Equal("plugin"))
		})

		It("should include subcommands for plugin management", func() {
			pluginBootstrap.RegisterCommands(mockRootCmd)

			pluginCmd, _, err := mockRootCmd.Find([]string{"plugin"})
			Expect(err).NotTo(HaveOccurred())

			// Check for expected subcommands
			subcommands := []string{"list", "search", "install", "remove", "update", "info"}
			for _, subcmd := range subcommands {
				found := false
				for _, cmd := range pluginCmd.Commands() {
					if cmd.Name() == subcmd {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Expected subcommand '%s' not found", subcmd)
			}
		})
	})

	Describe("Error Handling", func() {
		It("should handle plugin directory creation errors gracefully", func() {
			// Create a file where the plugin directory should be
			pluginDirPath := filepath.Join(tempHomeDir, ".devex", "plugins")
			parentDir := filepath.Dir(pluginDirPath)
			err := os.MkdirAll(parentDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Create a file with the same name as the directory
			err = os.WriteFile(pluginDirPath, []byte("blocking file"), 0644)
			Expect(err).NotTo(HaveOccurred())

			_, err = bootstrap.NewPluginBootstrap(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create plugin directory"))
		})

		It("should handle plugin loading errors gracefully", func() {
			var err error
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true)
			Expect(err).NotTo(HaveOccurred())

			// Initialize should succeed even if plugin loading fails
			err = pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
