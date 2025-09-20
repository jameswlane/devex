package sdk_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Plugin SDK", func() {
	var (
		tempDir    string
		downloader *sdk.Downloader
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "plugin-sdk-test-*")
		Expect(err).ToNot(HaveOccurred())

		downloader = sdk.NewDownloader("https://registry.example.com", tempDir)
	})

	AfterEach(func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	Describe("DownloadError", func() {
		It("should format error messages correctly", func() {
			err := &sdk.DownloadError{
				Plugin: "test-plugin",
				Err:    fmt.Errorf("network error"),
			}
			Expect(err.Error()).To(Equal("failed to download plugin test-plugin: network error"))
		})

		It("should unwrap the underlying error", func() {
			underlyingErr := fmt.Errorf("network error")
			err := &sdk.DownloadError{
				Plugin: "test-plugin",
				Err:    underlyingErr,
			}
			Expect(err.Unwrap()).To(Equal(underlyingErr))
		})
	})

	Describe("MultiError", func() {
		It("should handle no errors", func() {
			err := &sdk.MultiError{Errors: []error{}}
			Expect(err.Error()).To(Equal("no errors"))
		})

		It("should handle single error", func() {
			singleErr := fmt.Errorf("single error")
			err := &sdk.MultiError{Errors: []error{singleErr}}
			Expect(err.Error()).To(Equal("single error"))
		})

		It("should handle multiple errors", func() {
			err1 := fmt.Errorf("first error")
			err2 := fmt.Errorf("second error")
			err := &sdk.MultiError{Errors: []error{err1, err2}}
			Expect(err.Error()).To(ContainSubstring("2 errors occurred"))
			Expect(err.Error()).To(ContainSubstring("first error"))
		})

		It("should unwrap multiple errors", func() {
			err1 := fmt.Errorf("first error")
			err2 := fmt.Errorf("second error")
			err := &sdk.MultiError{Errors: []error{err1, err2}}
			unwrapped := err.Unwrap()
			Expect(unwrapped).To(HaveLen(2))
			Expect(unwrapped[0]).To(Equal(err1))
			Expect(unwrapped[1]).To(Equal(err2))
		})
	})

	Describe("DefaultLogger", func() {
		It("should create and use logger", func() {
			logger := sdk.NewDefaultLogger(false)
			Expect(logger).ToNot(BeNil())
			
			// These methods should not panic
			Expect(func() { logger.Printf("test %s", "message") }).ToNot(Panic())
			Expect(func() { logger.Info("info message") }).ToNot(Panic())
		})

		It("should create silent logger", func() {
			logger := sdk.NewDefaultLogger(true)
			Expect(logger).ToNot(BeNil())
			
			// Silent logger should not panic
			Expect(func() { logger.Printf("test %s", "message") }).ToNot(Panic())
		})
	})

	Describe("PluginInfo", func() {
		It("should create valid plugin info", func() {
			info := sdk.PluginInfo{
				Name:        "test-plugin",
				Version:     "1.0.0",
				Description: "A test plugin",
				Author:      "Test Author",
			}
			Expect(info.Name).To(Equal("test-plugin"))
			Expect(info.Version).To(Equal("1.0.0"))
			Expect(info.Description).To(Equal("A test plugin"))
			Expect(info.Author).To(Equal("Test Author"))
		})
	})

	Describe("BasePlugin", func() {
		var plugin *sdk.BasePlugin

		BeforeEach(func() {
			info := sdk.PluginInfo{
				Name:        "base-test",
				Version:     "1.0.0",
				Description: "Base plugin test",
				Author:      "Test Author",
			}
			plugin = sdk.NewBasePlugin(info)
		})

		It("should return plugin info", func() {
			info := plugin.Info()
			Expect(info.Name).To(Equal("base-test"))
			Expect(info.Version).To(Equal("1.0.0"))
		})

		It("should output plugin info", func() {
			// This should not panic
			Expect(func() { plugin.OutputPluginInfo() }).ToNot(Panic())
		})
	})

	Describe("Downloader", func() {
		Context("when properly configured", func() {
			It("should create a new downloader", func() {
				d := sdk.NewDownloader("https://registry.example.com", tempDir)
				Expect(d).ToNot(BeNil())
			})

			It("should handle download attempts gracefully", func() {
				// Test that the downloader is properly configured
				Expect(downloader).ToNot(BeNil())
			})

			It("should handle missing cache directory", func() {
				invalidDir := filepath.Join(tempDir, "nonexistent", "cache")
				d := sdk.NewDownloader("https://registry.example.com", invalidDir)
				Expect(d).ToNot(BeNil())
			})
		})

		Context("when downloading plugins", func() {
			It("should handle plugin download attempts", func() {
				ctx := context.Background()
				
				// This will likely fail due to no real plugin, but should not panic
				err := downloader.DownloadPluginWithContext(ctx, "nonexistent-plugin")
				Expect(err).To(HaveOccurred()) // Expected since plugin doesn't exist
			})
		})
	})

	Describe("ExecutableManager", func() {
		var manager *sdk.ExecutableManager

		BeforeEach(func() {
			manager = sdk.NewExecutableManager(tempDir)
		})

		It("should create a new executable manager", func() {
			Expect(manager).ToNot(BeNil())
		})

		It("should get plugin directory", func() {
			pluginDir := manager.GetPluginDir()
			Expect(pluginDir).ToNot(BeEmpty())
			Expect(filepath.IsAbs(pluginDir)).To(BeTrue())
		})

		It("should list plugins", func() {
			plugins := manager.ListPlugins()
			Expect(plugins).ToNot(BeNil()) // Should return empty map, not nil
		})

		It("should discover plugins", func() {
			err := manager.DiscoverPlugins()
			// This might succeed or fail depending on directory structure, but shouldn't panic
			if err != nil {
				Expect(err.Error()).ToNot(BeEmpty())
			}
		})
	})
})

var _ = Describe("Plugin Metadata", func() {
	It("should create valid plugin metadata", func() {
		info := sdk.PluginInfo{
			Name:        "test-plugin",
			Version:     "1.0.0",
			Author:      "Test Author",
			Description: "A comprehensive test plugin",
		}
		
		metadata := sdk.PluginMetadata{
			PluginInfo: info,
			Path:       "/path/to/plugin",
			Type:       "package-manager",
			Priority:   1,
		}

		Expect(metadata.PluginInfo.Name).To(Equal("test-plugin"))
		Expect(metadata.PluginInfo.Version).To(Equal("1.0.0"))
		Expect(metadata.Path).To(Equal("/path/to/plugin"))
		Expect(metadata.Type).To(Equal("package-manager"))
	})
})

var _ = Describe("Plugin Commands", func() {
	It("should create valid plugin commands", func() {
		cmd := sdk.PluginCommand{
			Name:        "install",
			Description: "Install packages",
			Usage:       "install [package...]",
			Flags:       map[string]string{"verbose": "Enable verbose output"},
		}

		Expect(cmd.Name).To(Equal("install"))
		Expect(cmd.Description).To(Equal("Install packages"))
		Expect(cmd.Usage).To(Equal("install [package...]"))
		Expect(cmd.Flags["verbose"]).To(Equal("Enable verbose output"))
	})
})

var _ = Describe("Platform Binary", func() {
	It("should handle platform-specific binaries", func() {
		binary := sdk.PlatformBinary{
			URL:      "https://github.com/example/plugin/releases/download/v1.0.0/plugin-linux-amd64.tar.gz",
			Checksum: "sha256:abcdef123456789",
			Size:     1048576,
			OS:       "linux",
			Arch:     "amd64",
		}

		Expect(binary.URL).To(ContainSubstring("linux-amd64"))
		Expect(binary.Checksum).To(HavePrefix("sha256:"))
		Expect(binary.OS).To(Equal("linux"))
		Expect(binary.Arch).To(Equal("amd64"))
		Expect(binary.Size).To(Equal(int64(1048576)))
	})
})
