package setup

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// FailingMockRepositoryWithSystem implements both Repository and SystemRepository but always fails
type FailingMockRepositoryWithSystem struct{}

func (m *FailingMockRepositoryWithSystem) ListApps() ([]types.AppConfig, error) {
	return nil, fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) SaveApp(app types.AppConfig) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) Set(key, value string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) Get(key string) (string, error) {
	return "", fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) GetAll() (map[string]string, error) {
	return nil, fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) DeleteApp(name string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) AddApp(name string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) GetApp(name string) (*types.AppConfig, error) {
	return nil, fmt.Errorf("mock repository error")
}

var _ = Describe("SetupModel", func() {
	Describe("saveThemePreference", func() {
		It("should save theme preference successfully", func() {
			mockRepo := mocks.NewMockRepository()

			model := &SetupModel{
				repo: mockRepo,
				system: SystemInfo{
					themes: []string{"Tokyo Night", "Kanagawa", "Light Theme"},
				},
				selections: UISelections{
					selectedTheme: 1, // Select "Kanagawa"
				},
			}

			err := model.saveThemePreference()

			Expect(err).ToNot(HaveOccurred())

			// Verify the theme was stored in the repository
			storedTheme, err := mockRepo.Get("global_theme")
			Expect(err).ToNot(HaveOccurred())
			Expect(storedTheme).To(Equal("Kanagawa"))
		})

		It("should handle invalid selected theme index", func() {
			mockRepo := mocks.NewMockRepository()

			model := &SetupModel{
				repo: mockRepo,
				system: SystemInfo{
					themes: []string{"Tokyo Night", "Kanagawa"},
				},
				selections: UISelections{
					selectedTheme: 999, // Invalid index
				},
			}

			// Should panic or handle gracefully
			Expect(func() {
				model.saveThemePreference()
			}).To(Panic())
		})

		It("should handle empty themes list", func() {
			mockRepo := mocks.NewMockRepository()

			model := &SetupModel{
				repo: mockRepo,
				system: SystemInfo{
					themes: []string{},
				},
				selections: UISelections{
					selectedTheme: 0,
				},
			}

			// Should panic or handle gracefully
			Expect(func() {
				model.saveThemePreference()
			}).To(Panic())
		})
	})

	// TODO: Convert remaining test functions to Ginkgo format
	// - Additional saveThemePreference tests
	// - ThemeStep navigation tests
	// - Integration tests
	// - Edge case tests
})
