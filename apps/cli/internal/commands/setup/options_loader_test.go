package setup_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands/setup"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("OptionsLoader", func() {
	var (
		loader           *setup.OptionsLoader
		settings         config.CrossPlatformSettings
		detectedPlatform platform.DetectionResult
		tempDir          string
	)

	BeforeEach(func() {
		// Create basic settings
		settings = config.CrossPlatformSettings{}

		// Create platform detection result
		detectedPlatform = platform.DetectionResult{
			OS:           "linux",
			Distribution: "debian",
			DesktopEnv:   "gnome",
			Architecture: "amd64",
		}

		// Create temp directory for file-based tests
		var err error
		tempDir, err = os.MkdirTemp("", "options-loader-test-*")
		Expect(err).NotTo(HaveOccurred())

		loader = setup.NewOptionsLoader(settings, detectedPlatform)
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("Load", func() {
		Context("with static source", func() {
			It("should return empty options for static source", func() {
				source := &types.OptionsSource{
					Type: types.SourceTypeStatic,
				}

				options, err := loader.Load(source)
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(BeEmpty())
			})
		})

		Context("with system source", func() {
			It("should load available shells", func() {
				source := &types.OptionsSource{
					Type:       types.SourceTypeSystem,
					SystemType: "shells",
				}

				options, err := loader.Load(source)
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(HaveLen(3))

				labels := make([]string, len(options))
				for i, opt := range options {
					labels[i] = opt.Label
				}
				Expect(labels).To(ContainElement("zsh"))
				Expect(labels).To(ContainElement("bash"))
				Expect(labels).To(ContainElement("fish"))
			})

			It("should load desktop environments on Linux", func() {
				source := &types.OptionsSource{
					Type:       types.SourceTypeSystem,
					SystemType: "desktop_environments",
				}

				options, err := loader.Load(source)
				Expect(err).NotTo(HaveOccurred())
				Expect(options).NotTo(BeEmpty())

				labels := make([]string, len(options))
				for i, opt := range options {
					labels[i] = opt.Label
				}
				Expect(labels).To(ContainElement("gnome"))
				Expect(labels).To(ContainElement("kde"))
			})

			It("should return empty options for desktop environments on non-Linux", func() {
				macPlatform := platform.DetectionResult{
					OS: "darwin",
				}
				macLoader := setup.NewOptionsLoader(settings, macPlatform)

				source := &types.OptionsSource{
					Type:       types.SourceTypeSystem,
					SystemType: "desktop_environments",
				}

				options, err := macLoader.Load(source)
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(BeEmpty())
			})

			It("should return error for unknown system type", func() {
				source := &types.OptionsSource{
					Type:       types.SourceTypeSystem,
					SystemType: "unknown_type",
				}

				_, err := loader.Load(source)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown system type"))
			})
		})

		Context("with config source", func() {
			It("should return error for unknown transform", func() {
				source := &types.OptionsSource{
					Type:      types.SourceTypeConfig,
					Transform: "unknown_transform",
				}

				_, err := loader.Load(source)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown transform"))
			})
		})

		Context("with directory loading", func() {
			BeforeEach(func() {
				// Create test YAML files
				files := map[string]string{
					"option1.yaml": `name: "Option One"
description: "First option"`,
					"option2.yaml": `name: "Option Two"
description: "Second option"`,
					"invalid.txt": "not a yaml file",
					"option3.yml": `name: "Option Three"
description: "Third option"`,
				}

				for filename, content := range files {
					filePath := filepath.Join(tempDir, filename)
					err := os.WriteFile(filePath, []byte(content), 0644)
					Expect(err).NotTo(HaveOccurred())
				}

				// Create subdirectory (should be ignored)
				subdir := filepath.Join(tempDir, "subdir")
				err := os.Mkdir(subdir, 0755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should load options from YAML files in directory", func() {
				source := &types.OptionsSource{
					Type:      types.SourceTypeConfig,
					Transform: "load_directory",
					Path:      tempDir,
				}

				options, err := loader.Load(source)
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(HaveLen(3)) // 2 .yaml + 1 .yml files

				labels := make([]string, len(options))
				for i, opt := range options {
					labels[i] = opt.Label
				}
				Expect(labels).To(ContainElement("Option One"))
				Expect(labels).To(ContainElement("Option Two"))
				Expect(labels).To(ContainElement("Option Three"))
			})

			It("should use filename as value without extension", func() {
				source := &types.OptionsSource{
					Type:      types.SourceTypeConfig,
					Transform: "load_directory",
					Path:      tempDir,
				}

				options, err := loader.Load(source)
				Expect(err).NotTo(HaveOccurred())

				values := make([]string, len(options))
				for i, opt := range options {
					values[i] = opt.Value
				}
				Expect(values).To(ContainElement("option1"))
				Expect(values).To(ContainElement("option2"))
				Expect(values).To(ContainElement("option3"))
			})

			It("should return error for non-existent directory", func() {
				source := &types.OptionsSource{
					Type:      types.SourceTypeConfig,
					Transform: "load_directory",
					Path:      "/nonexistent/path",
				}

				_, err := loader.Load(source)
				Expect(err).To(HaveOccurred())
			})

			It("should return error for empty directory", func() {
				emptyDir := filepath.Join(tempDir, "empty")
				err := os.Mkdir(emptyDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				source := &types.OptionsSource{
					Type:      types.SourceTypeConfig,
					Transform: "load_directory",
					Path:      emptyDir,
				}

				_, err = loader.Load(source)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no valid YAML files"))
			})
		})

		Context("with plugin source", func() {
			It("should return empty options (not yet implemented)", func() {
				source := &types.OptionsSource{
					Type: types.SourceTypePlugin,
				}

				options, err := loader.Load(source)
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(BeEmpty())
			})
		})

		Context("with unknown source type", func() {
			It("should return error", func() {
				source := &types.OptionsSource{
					Type: "invalid_type",
				}

				_, err := loader.Load(source)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown options source type"))
			})
		})
	})

	Describe("Shell options", func() {
		It("should include descriptions for shells", func() {
			source := &types.OptionsSource{
				Type:       types.SourceTypeSystem,
				SystemType: "shells",
			}

			options, err := loader.Load(source)
			Expect(err).NotTo(HaveOccurred())

			for _, opt := range options {
				Expect(opt.Description).NotTo(BeEmpty())
				Expect(opt.Label).NotTo(BeEmpty())
				Expect(opt.Value).NotTo(BeEmpty())
			}
		})
	})
})
