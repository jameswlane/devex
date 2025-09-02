package theme_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installer/theme"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Mock repository for testing
type mockRepository struct {
	data map[string]string
	apps []types.AppConfig
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		data: make(map[string]string),
		apps: []types.AppConfig{},
	}
}

func (r *mockRepository) Set(key, value string) error {
	r.data[key] = value
	return nil
}

func (r *mockRepository) Get(key string) (string, error) {
	value, exists := r.data[key]
	if !exists {
		return "", fmt.Errorf("key not found")
	}
	return value, nil
}

func (r *mockRepository) AddApp(appName string) error {
	r.apps = append(r.apps, types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: appName,
		},
	})
	return nil
}

func (r *mockRepository) DeleteApp(name string) error {
	for i, app := range r.apps {
		if app.Name == name {
			r.apps = append(r.apps[:i], r.apps[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("app not found")
}

func (r *mockRepository) GetApp(name string) (*types.AppConfig, error) {
	for _, app := range r.apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, fmt.Errorf("app not found")
}

func (r *mockRepository) ListApps() ([]types.AppConfig, error) {
	return r.apps, nil
}

func (r *mockRepository) SaveApp(app types.AppConfig) error {
	for i, existing := range r.apps {
		if existing.Name == app.Name {
			r.apps[i] = app
			return nil
		}
	}
	r.apps = append(r.apps, app)
	return nil
}

// Mock command executor for testing
type mockCommandExecutor struct {
	executedCommands []string
	shouldFail       bool
}

func (e *mockCommandExecutor) Execute(ctx context.Context, command string) error {
	if e.shouldFail {
		return fmt.Errorf("command execution failed")
	}
	e.executedCommands = append(e.executedCommands, command)
	return nil
}

func TestTheme(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Theme Manager Suite")
}

var _ = Describe("Theme Manager", func() {
	var (
		manager  *theme.Manager
		repo     *mockRepository
		executor *mockCommandExecutor
		ctx      context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		repo = newMockRepository()
		executor = &mockCommandExecutor{}
		manager = theme.New(repo, executor)
	})

	Describe("Theme Selection", func() {
		It("should store theme selection for an app", func() {
			err := manager.SelectTheme("vscode", "dark")
			Expect(err).ToNot(HaveOccurred())

			value, exists := repo.data["app_theme_vscode"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("dark"))
		})

		It("should retrieve selected theme for an app", func() {
			repo.data["app_theme_vscode"] = "dark"

			theme, err := manager.GetSelectedTheme("vscode")
			Expect(err).ToNot(HaveOccurred())
			Expect(theme).To(Equal("dark"))
		})

		It("should return error when no theme is selected", func() {
			_, err := manager.GetSelectedTheme("vscode")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Global Theme", func() {
		It("should set global theme preference", func() {
			err := manager.SetGlobalTheme("dracula")
			Expect(err).ToNot(HaveOccurred())

			value, exists := repo.data["global_theme"]
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal("dracula"))
		})

		It("should get global theme preference", func() {
			repo.data["global_theme"] = "dracula"

			theme, err := manager.GetGlobalTheme()
			Expect(err).ToNot(HaveOccurred())
			Expect(theme).To(Equal("dracula"))
		})

		It("should apply global theme to an app", func() {
			repo.data["global_theme"] = "dark"

			themes := []types.Theme{
				{Name: "light"},
				{Name: "dark"},
				{Name: "dracula"},
			}

			err := manager.UseGlobalTheme("vscode", themes)
			Expect(err).ToNot(HaveOccurred())

			selectedTheme, _ := manager.GetSelectedTheme("vscode")
			Expect(selectedTheme).To(Equal("dark"))
		})

		It("should error when global theme is not available for app", func() {
			repo.data["global_theme"] = "nonexistent"

			themes := []types.Theme{
				{Name: "light"},
				{Name: "dark"},
			}

			err := manager.UseGlobalTheme("vscode", themes)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not available"))
		})
	})

	Describe("Theme Application", func() {
		It("should apply theme files", func() {
			repo.data["app_theme_vscode"] = "dark"

			themes := []types.Theme{
				{
					Name: "dark",
					Files: []types.ConfigFile{
						{
							Source:      "/themes/dark/config.json",
							Destination: "~/.config/vscode/config.json",
						},
					},
				},
			}

			err := manager.ApplyTheme(ctx, "vscode", themes)
			Expect(err).ToNot(HaveOccurred())
			Expect(executor.executedCommands).To(HaveLen(2)) // mkdir and cp commands
		})

		It("should handle missing selected theme", func() {
			themes := []types.Theme{
				{Name: "dark"},
			}

			err := manager.ApplyTheme(ctx, "vscode", themes)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no theme selected"))
		})

		It("should continue applying files even if one fails", func() {
			repo.data["app_theme_vscode"] = "dark"
			executor.shouldFail = true

			themes := []types.Theme{
				{
					Name: "dark",
					Files: []types.ConfigFile{
						{
							Source:      "/themes/dark/config1.json",
							Destination: "~/.config/vscode/config1.json",
						},
						{
							Source:      "/themes/dark/config2.json",
							Destination: "~/.config/vscode/config2.json",
						},
					},
				},
			}

			// Should not return error even though individual files fail
			err := manager.ApplyTheme(ctx, "vscode", themes)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Theme Validation", func() {
		It("should validate a correct theme", func() {
			theme := types.Theme{
				Name: "dark",
				Files: []types.ConfigFile{
					{
						Source:      "/source/file",
						Destination: "/dest/file",
					},
				},
			}

			err := manager.ValidateTheme(theme)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject theme with empty name", func() {
			theme := types.Theme{
				Name: "",
				Files: []types.ConfigFile{
					{Source: "/source", Destination: "/dest"},
				},
			}

			err := manager.ValidateTheme(theme)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("name cannot be empty"))
		})

		It("should reject theme with no files", func() {
			theme := types.Theme{
				Name:  "dark",
				Files: []types.ConfigFile{},
			}

			err := manager.ValidateTheme(theme)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("at least one configuration file"))
		})

		It("should reject theme with invalid file paths", func() {
			theme := types.Theme{
				Name: "dark",
				Files: []types.ConfigFile{
					{Source: "", Destination: "/dest"},
				},
			}

			err := manager.ValidateTheme(theme)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty source path"))
		})
	})

	Describe("Helper Functions", func() {
		It("should list available theme names", func() {
			themes := []types.Theme{
				{Name: "dark"},
				{Name: "light"},
				{Name: "dracula"},
			}

			names := manager.ListAvailableThemes(themes)
			Expect(names).To(Equal([]string{"dark", "light", "dracula"}))
		})
	})
})
