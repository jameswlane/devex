package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("ExecutableManager", func() {
	var (
		manager    *sdk.ExecutableManager
		tempDir    string
		pluginPath string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "plugin-manager-test-*")
		Expect(err).NotTo(HaveOccurred())

		manager = sdk.NewExecutableManager(tempDir)

		// Create a mock plugin executable for testing
		pluginPath = filepath.Join(tempDir, "devex-plugin-test-plugin")
		if runtime.GOOS == "windows" {
			pluginPath += ".exe"
		}
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("NewExecutableManager", func() {
		It("should create a new manager with correct plugin directory", func() {
			Expect(manager).NotTo(BeNil())
			Expect(manager.GetPluginDir()).To(Equal(tempDir))
		})
	})

	Describe("DiscoverPlugins", func() {
		Context("when plugin directory is empty", func() {
			It("should discover no plugins", func() {
				err := manager.DiscoverPlugins()
				Expect(err).NotTo(HaveOccurred())

				plugins := manager.ListPlugins()
				Expect(plugins).To(BeEmpty())
			})
		})

		Context("when plugin directory contains valid plugins", func() {
			BeforeEach(func() {
				// Create a mock plugin that outputs valid JSON
				createMockPlugin(pluginPath, sdk.PluginInfo{
					Name:        "test-plugin",
					Version:     "1.0.0",
					Description: "Test plugin",
					Commands: []sdk.PluginCommand{
						{
							Name:        "test",
							Description: "Test command",
							Usage:       "Run a test",
						},
					},
				})
			})

			It("should discover and load plugins", func() {
				err := manager.DiscoverPlugins()
				Expect(err).NotTo(HaveOccurred())

				plugins := manager.ListPlugins()
				Expect(plugins).To(HaveLen(1))
				Expect(plugins).To(HaveKey("test-plugin"))

				pluginInfo := plugins["test-plugin"]
				Expect(pluginInfo.Name).To(Equal("test-plugin"))
				Expect(pluginInfo.Version).To(Equal("1.0.0"))
				Expect(pluginInfo.Description).To(Equal("Test plugin"))
				Expect(pluginInfo.Commands).To(HaveLen(1))
				Expect(pluginInfo.Commands[0].Name).To(Equal("test"))
			})
		})

		Context("when plugin directory contains invalid files", func() {
			BeforeEach(func() {
				// Create a non-plugin file
				invalidFile := filepath.Join(tempDir, "not-a-plugin.txt")
				err := os.WriteFile(invalidFile, []byte("invalid content"), 0644)
				Expect(err).NotTo(HaveOccurred())

				// Create a plugin file that doesn't start with correct prefix
				wrongPrefix := filepath.Join(tempDir, "wrong-prefix-plugin")
				err = os.WriteFile(wrongPrefix, []byte("#!/bin/bash\necho invalid"), 0755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should ignore invalid files", func() {
				err := manager.DiscoverPlugins()
				Expect(err).NotTo(HaveOccurred())

				plugins := manager.ListPlugins()
				Expect(plugins).To(BeEmpty())
			})
		})

		Context("when plugin returns invalid JSON", func() {
			BeforeEach(func() {
				// Create a plugin that returns invalid JSON
				content := createMockPluginScript("invalid json output")
				err := os.WriteFile(pluginPath, []byte(content), 0755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create fallback plugin entry", func() {
				err := manager.DiscoverPlugins()
				Expect(err).NotTo(HaveOccurred())

				plugins := manager.ListPlugins()
				Expect(plugins).To(HaveLen(1))
				Expect(plugins).To(HaveKey("test-plugin"))

				// Verify fallback metadata
				pluginInfo := plugins["test-plugin"]
				Expect(pluginInfo.Name).To(Equal("test-plugin"))
				Expect(pluginInfo.Version).To(Equal("unknown"))
				Expect(pluginInfo.Description).To(Equal("DevEx plugin: test-plugin"))
				Expect(pluginInfo.Commands).To(BeEmpty())
			})
		})
	})

	Describe("ExecutePlugin", func() {
		var pluginInfo sdk.PluginInfo

		BeforeEach(func() {
			pluginInfo = sdk.PluginInfo{
				Name:        "test-plugin",
				Version:     "1.0.0",
				Description: "Test plugin",
				Commands: []sdk.PluginCommand{
					{
						Name:        "hello",
						Description: "Say hello",
						Usage:       "Print hello message",
					},
				},
			}
			createMockPlugin(pluginPath, pluginInfo)

			err := manager.DiscoverPlugins()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when plugin exists", func() {
			It("should execute plugin with correct arguments", func() {
				err := manager.ExecutePlugin("test-plugin", []string{"hello", "world"})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when plugin does not exist", func() {
			It("should return an error", func() {
				err := manager.ExecutePlugin("nonexistent-plugin", []string{"test"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin nonexistent-plugin is not installed"))
			})
		})
	})

	Describe("InstallPlugin", func() {
		Context("when installing from valid source", func() {
			var sourcePath string

			BeforeEach(func() {
				// Create a source plugin file
				sourcePath = filepath.Join(tempDir, "source-plugin")
				pluginInfo := sdk.PluginInfo{
					Name:        "installed-plugin",
					Version:     "1.0.0",
					Description: "Installed plugin",
					Commands:    []sdk.PluginCommand{},
				}
				createMockPlugin(sourcePath, pluginInfo)
			})

			It("should install plugin successfully", func() {
				err := manager.InstallPlugin(sourcePath, "installed-plugin")
				Expect(err).NotTo(HaveOccurred())

				plugins := manager.ListPlugins()
				Expect(plugins).To(HaveKey("installed-plugin"))

				// Verify the plugin file exists
				expectedPath := filepath.Join(tempDir, "devex-plugin-installed-plugin")
				if runtime.GOOS == "windows" {
					expectedPath += ".exe"
				}
				Expect(expectedPath).To(BeAnExistingFile())
			})
		})

		Context("when source file does not exist", func() {
			It("should return an error", func() {
				err := manager.InstallPlugin("/nonexistent/path", "test-plugin")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to open source plugin"))
			})
		})
	})

	Describe("RemovePlugin", func() {
		var removablePluginPath string

		BeforeEach(func() {
			// Create a plugin with filename that matches the expected name
			removablePluginPath = filepath.Join(tempDir, "devex-plugin-removable-plugin")
			if runtime.GOOS == "windows" {
				removablePluginPath += ".exe"
			}

			pluginInfo := sdk.PluginInfo{
				Name:        "removable-plugin",
				Version:     "1.0.0",
				Description: "Plugin to remove",
				Commands:    []sdk.PluginCommand{},
			}
			createMockPlugin(removablePluginPath, pluginInfo)

			err := manager.DiscoverPlugins()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when plugin exists", func() {
			It("should remove plugin successfully", func() {
				// Verify plugin exists
				plugins := manager.ListPlugins()
				Expect(plugins).To(HaveKey("removable-plugin"))

				err := manager.RemovePlugin("removable-plugin")
				Expect(err).NotTo(HaveOccurred())

				// Verify plugin is removed from manager
				plugins = manager.ListPlugins()
				Expect(plugins).NotTo(HaveKey("removable-plugin"))

				// Verify plugin file is deleted
				Expect(removablePluginPath).NotTo(BeAnExistingFile())
			})
		})

		Context("when plugin does not exist", func() {
			It("should return an error", func() {
				err := manager.RemovePlugin("nonexistent-plugin")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin nonexistent-plugin is not installed"))
			})
		})
	})
})

// createMockPlugin creates a mock plugin executable that returns plugin info as JSON
func createMockPlugin(path string, info sdk.PluginInfo) {
	content := createMockPluginScript("")

	// If we want to return specific plugin info, create a script that outputs it
	if info.Name != "" {
		jsonOutput, _ := json.Marshal(info)
		content = createMockPluginScriptWithJSON(string(jsonOutput))
	}

	err := os.WriteFile(path, []byte(content), 0755)
	Expect(err).NotTo(HaveOccurred())
}

// createMockPluginScript creates a basic mock plugin script
func createMockPluginScript(output string) string {
	// If output contains "invalid json", make the plugin return invalid JSON for --plugin-info
	pluginInfoOutput := `{"name":"test-plugin","version":"1.0.0","description":"Test plugin","commands":[]}`
	if strings.Contains(output, "invalid json") {
		pluginInfoOutput = "invalid json output"
	}

	if runtime.GOOS == "windows" {
		return `@echo off
if "%1"=="--plugin-info" (
    echo ` + pluginInfoOutput + `
) else (
    echo ` + output + `
)`
	}

	return `#!/bin/bash
if [ "$1" = "--plugin-info" ]; then
    echo '` + pluginInfoOutput + `'
else
    echo "` + output + `"
fi`
}

// createMockPluginScriptWithJSON creates a mock plugin script that returns specific JSON
func createMockPluginScriptWithJSON(jsonOutput string) string {
	if runtime.GOOS == "windows" {
		return `@echo off
if "%1"=="--plugin-info" (
    echo ` + jsonOutput + `
) else (
    echo Mock plugin executed with args: %*
)`
	}

	return `#!/bin/bash
if [ "$1" = "--plugin-info" ]; then
    echo '` + jsonOutput + `'
else
    echo "Mock plugin executed with args: $@"
fi`
}
