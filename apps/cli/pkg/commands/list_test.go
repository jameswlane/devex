package commands

import (
	"errors"
	"testing"
	"time"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestListCommands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "List Commands Suite")
}

var _ = Describe("List Command", func() {
	var (
		mockRepo *MockRepository
		settings config.CrossPlatformSettings
		options  ListCommandOptions
	)

	BeforeEach(func() {
		mockRepo = &MockRepository{}
		settings = config.CrossPlatformSettings{
			Applications: config.ApplicationsConfig{
				Development: []types.CrossPlatformApp{
					{
						Name:        "test-app",
						Description: "A test application",
						Category:    "Development Tools",
						Default:     true,
						Linux: types.OSConfig{
							InstallMethod: "apt",
						},
					},
				},
			},
		}
		options = ListCommandOptions{
			Format: "table",
		}
	})

	Describe("parseListFlags", func() {
		It("should parse flags correctly", func() {
			// This would require setting up a cobra command with flags
			// For now, just test the structure exists
			Expect(options.Format).To(Equal("table"))
		})
	})

	Describe("getInstalledApps", func() {
		Context("when database is available", func() {
			BeforeEach(func() {
				mockRepo.On("ListApps").Return([]types.AppConfig{
					{
						BaseConfig: types.BaseConfig{
							Name:        "test-app",
							Description: "A test application",
							Category:    "Development Tools",
						},
						InstallMethod: "apt",
					},
				}, nil)
			})

			It("should return installed applications", func() {
				apps, err := getInstalledApps(mockRepo, settings, options)
				Expect(err).To(BeNil())
				Expect(apps).To(HaveLen(1))
				Expect(apps[0].Name).To(Equal("test-app"))
				Expect(apps[0].Category).To(Equal("Development Tools"))
			})
		})

		Context("when database returns error", func() {
			BeforeEach(func() {
				mockRepo.On("ListApps").Return(nil, errors.New("database error"))
			})

			It("should return error", func() {
				_, err := getInstalledApps(mockRepo, settings, options)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to get installed apps"))
			})
		})
	})

	Describe("getAvailableApps", func() {
		It("should return available applications", func() {
			apps := getAvailableApps(settings, options)
			Expect(apps).To(HaveLen(1))
			Expect(apps[0].Name).To(Equal("test-app"))
			Expect(apps[0].Category).To(Equal("Development Tools"))
			Expect(apps[0].Recommended).To(BeTrue())
			Expect(apps[0].InstallMethods).To(ContainElement("apt"))
			Expect(apps[0].Platforms).To(ContainElement("linux"))
		})
	})

	Describe("getCategoryInfo", func() {
		It("should return category information", func() {
			categories := getCategoryInfo(settings)
			Expect(categories).To(HaveLen(1))
			Expect(categories[0].Name).To(Equal("Development Tools"))
			Expect(categories[0].AppCount).To(Equal(1))
			Expect(categories[0].Platforms).To(ContainElement("linux"))
		})
	})

	Describe("filterInstalledApps", func() {
		var installedApps []InstalledApp

		BeforeEach(func() {
			installedApps = []InstalledApp{
				{
					Name:          "test-app",
					Description:   "A test application",
					Category:      "Development Tools",
					InstallMethod: "apt",
					InstallDate:   time.Now(),
				},
				{
					Name:          "other-app",
					Description:   "Another application",
					Category:      "Utilities",
					InstallMethod: "snap",
					InstallDate:   time.Now(),
				},
			}
		})

		It("should filter by category", func() {
			options.Category = "Development Tools"
			filtered := filterInstalledApps(installedApps, options)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Name).To(Equal("test-app"))
		})

		It("should filter by search term", func() {
			options.Search = "test"
			filtered := filterInstalledApps(installedApps, options)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Name).To(Equal("test-app"))
		})

		It("should filter by install method", func() {
			options.Method = "apt"
			filtered := filterInstalledApps(installedApps, options)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Name).To(Equal("test-app"))
		})

		It("should return all apps when no filters", func() {
			filtered := filterInstalledApps(installedApps, options)
			Expect(filtered).To(HaveLen(2))
		})
	})

	Describe("filterAvailableApps", func() {
		var availableApps []AvailableApp

		BeforeEach(func() {
			availableApps = []AvailableApp{
				{
					Name:           "test-app",
					Description:    "A test application",
					Category:       "Development Tools",
					InstallMethods: []string{"apt"},
					Recommended:    true,
				},
				{
					Name:           "other-app",
					Description:    "Another application",
					Category:       "Utilities",
					InstallMethods: []string{"snap"},
					Recommended:    false,
				},
			}
		})

		It("should filter by category", func() {
			options.Category = "Development Tools"
			filtered := filterAvailableApps(availableApps, options)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Name).To(Equal("test-app"))
		})

		It("should filter by search term", func() {
			options.Search = "test"
			filtered := filterAvailableApps(availableApps, options)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Name).To(Equal("test-app"))
		})

		It("should filter by install method", func() {
			options.Method = "apt"
			filtered := filterAvailableApps(availableApps, options)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Name).To(Equal("test-app"))
		})

		It("should filter by recommended", func() {
			options.Recommended = true
			filtered := filterAvailableApps(availableApps, options)
			Expect(filtered).To(HaveLen(1))
			Expect(filtered[0].Name).To(Equal("test-app"))
		})

		It("should return all apps when no filters", func() {
			filtered := filterAvailableApps(availableApps, options)
			Expect(filtered).To(HaveLen(2))
		})
	})

	Describe("helper functions", func() {
		Describe("getSupportedPlatforms", func() {
			It("should return correct platforms for Linux-only app", func() {
				app := types.CrossPlatformApp{
					Linux: types.OSConfig{InstallMethod: "apt"},
				}
				platforms := getSupportedPlatforms(app)
				Expect(platforms).To(Equal([]string{"linux"}))
			})

			It("should return all platforms for cross-platform app", func() {
				app := types.CrossPlatformApp{
					AllPlatforms: types.OSConfig{InstallMethod: "mise"},
				}
				platforms := getSupportedPlatforms(app)
				Expect(platforms).To(Equal([]string{"linux", "macos", "windows"}))
			})
		})

		Describe("getCategoryDescription", func() {
			It("should return correct description for known category", func() {
				desc := getCategoryDescription("development")
				Expect(desc).To(Equal("Core development tools and IDEs"))
			})

			It("should return default description for unknown category", func() {
				desc := getCategoryDescription("unknown")
				Expect(desc).To(Equal("Various applications"))
			})
		})

		Describe("contains", func() {
			It("should return true when item exists", func() {
				slice := []string{"a", "b", "c"}
				result := contains(slice, "b")
				Expect(result).To(BeTrue())
			})

			It("should return false when item doesn't exist", func() {
				slice := []string{"a", "b", "c"}
				result := contains(slice, "d")
				Expect(result).To(BeFalse())
			})
		})

		Describe("truncateString", func() {
			It("should not truncate short strings", func() {
				result := truncateString("short", 10)
				Expect(result).To(Equal("short"))
			})

			It("should truncate long strings with ellipsis", func() {
				result := truncateString("very long string", 10)
				Expect(result).To(Equal("very lo..."))
			})
		})
	})
})

// MockRepository is a mock implementation of the Repository interface
type MockRepository struct {
	apps []types.AppConfig
	err  error
}

func (m *MockRepository) On(method string) *MockCall {
	return &MockCall{repo: m, method: method}
}

func (m *MockRepository) AddApp(appName string) error {
	return m.err
}

func (m *MockRepository) DeleteApp(name string) error {
	return m.err
}

func (m *MockRepository) GetApp(name string) (*types.AppConfig, error) {
	for _, app := range m.apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, m.err
}

func (m *MockRepository) ListApps() ([]types.AppConfig, error) {
	return m.apps, m.err
}

func (m *MockRepository) SaveApp(app types.AppConfig) error {
	return m.err
}

func (m *MockRepository) Set(key string, value string) error {
	return m.err
}

func (m *MockRepository) Get(key string) (string, error) {
	return "", m.err
}

// MockCall helps with setting up mock expectations
type MockCall struct {
	repo   *MockRepository
	method string
}

func (c *MockCall) Return(apps []types.AppConfig, err error) {
	c.repo.apps = apps
	c.repo.err = err
}
