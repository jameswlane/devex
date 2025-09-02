package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// TemplateSelectionModel represents the TUI state for template selection
type TemplateSelectionModel struct {
	templates   []templates.Template
	cursor      int
	selected    map[int]bool
	choice      *templates.Template
	quitting    bool
	width       int
	height      int
	showDetails bool
}

var (
	templateTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#7D56F4")).
				Padding(0, 1)

	templateItemStyle = lipgloss.NewStyle().
				PaddingLeft(4)

	selectedTemplateStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true)

	templateHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262"))
)

// NewTemplateSelectionModel creates a new template selection model
func NewTemplateSelectionModel(templatesManager *templates.TemplateManager) (*TemplateSelectionModel, error) {
	availableTemplates, err := templatesManager.GetAvailableTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	// Sort templates by category and then by name
	sort.Slice(availableTemplates, func(i, j int) bool {
		if availableTemplates[i].Metadata.Category == availableTemplates[j].Metadata.Category {
			return availableTemplates[i].Metadata.Name < availableTemplates[j].Metadata.Name
		}
		return availableTemplates[i].Metadata.Category < availableTemplates[j].Metadata.Category
	})

	return &TemplateSelectionModel{
		templates: availableTemplates,
		selected:  make(map[int]bool),
	}, nil
}

func (m TemplateSelectionModel) Init() tea.Cmd {
	return nil
}

func (m TemplateSelectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.templates)-1 {
				m.cursor++
			}

		case "enter", " ":
			// Select template and exit
			if m.cursor >= 0 && m.cursor < len(m.templates) {
				m.choice = &m.templates[m.cursor]
				return m, tea.Quit
			}

		case "d":
			m.showDetails = !m.showDetails

		case "?", "h":
			// Show help (for future enhancement)
		}
	}

	return m, nil
}

func (m TemplateSelectionModel) View() string {
	if m.quitting {
		return "Template selection cancelled.\n"
	}

	if m.choice != nil {
		return fmt.Sprintf("Selected template: %s\n", m.choice.Metadata.Name)
	}

	s := templateTitleStyle.Render("📋 DevEx Template Selection") + "\n\n"
	s += "Choose a development environment template:\n\n"

	// Group templates by category
	categories := make(map[string][]templates.Template)
	for _, template := range m.templates {
		categories[template.Metadata.Category] = append(categories[template.Metadata.Category], template)
	}

	currentIndex := 0
	titleCase := cases.Title(language.English)
	for category, categoryTemplates := range categories {
		s += fmt.Sprintf("  %s:\n", titleCase.String(category))

		for _, template := range categoryTemplates {
			cursor := " "
			if currentIndex == m.cursor {
				cursor = ">"
				s += selectedTemplateStyle.Render(fmt.Sprintf("  %s %s %s - %s",
					cursor, template.Metadata.Icon, template.Metadata.Name, template.Metadata.Description))
			} else {
				s += templateItemStyle.Render(fmt.Sprintf("  %s %s %s - %s",
					cursor, template.Metadata.Icon, template.Metadata.Name, template.Metadata.Description))
			}

			// Show additional details for selected template
			if currentIndex == m.cursor && m.showDetails {
				details := fmt.Sprintf("\n    Difficulty: %s | Time: %s | Platforms: %s",
					template.Metadata.Difficulty,
					template.Metadata.EstimatedTime,
					strings.Join(template.Metadata.Platforms, ", "))
				s += templateHelpStyle.Render(details)
			}

			s += "\n"
			currentIndex++
		}
		s += "\n"
	}

	// Help text
	help := "\n" + templateHelpStyle.Render("↑/↓: navigate • enter: select • d: toggle details • q: quit")
	s += help

	return s
}

// runInteractiveInitWithTemplates runs the enhanced template-based init
func runInteractiveInitWithTemplates(settings config.CrossPlatformSettings, repo types.Repository, force bool) error {
	// Initialize template manager
	templateManager := templates.NewTemplateManager(settings.HomeDir)

	reader := bufio.NewReader(os.Stdin)
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Println(cyan("\n🚀 DevEx Configuration Wizard v2.0"))
	fmt.Println("Create a personalized development environment with professional templates.")

	// Check for existing configuration
	configDir := settings.GetConfigDir()
	if _, err := os.Stat(configDir); !os.IsNotExist(err) && !force {
		fmt.Printf("%s Existing configuration found at %s\n", yellow("⚠️"), configDir)
		fmt.Print("Do you want to overwrite it? (y/N): ")
		response, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Configuration initialization cancelled.")
			return nil
		}
	}

	// Platform detection
	fmt.Println(cyan("\n📍 Platform Detection"))
	p := platform.DetectPlatform()
	fmt.Printf("  OS: %s\n", green(p.OS))
	if p.Distribution != "" {
		fmt.Printf("  Distribution: %s %s\n", green(p.Distribution), p.Version)
	}
	fmt.Printf("  Architecture: %s\n", green(p.Architecture))
	if p.DesktopEnv != "" {
		fmt.Printf("  Desktop: %s\n", green(p.DesktopEnv))
	}

	// Template selection mode choice
	fmt.Println(cyan("\n🎯 Template Selection Mode"))
	fmt.Println("How would you like to select your template?")
	fmt.Println("  1. Interactive TUI (recommended)")
	fmt.Println("  2. Simple list selection")
	fmt.Print("\nChoose mode (1-2): ")

	var modeChoice int
	if _, err := fmt.Scanln(&modeChoice); err != nil {
		// Default to interactive TUI on input error
		modeChoice = 1
	}

	var selectedTemplate *templates.Template
	var err error

	if modeChoice == 1 {
		// Use TUI for template selection
		selectedTemplate, err = selectTemplateWithTUI(templateManager)
		if err != nil {
			return fmt.Errorf("template selection failed: %w", err)
		}
	} else {
		// Use simple list selection
		selectedTemplate, err = selectTemplateFromList(templateManager, reader, yellow, cyan)
		if err != nil {
			return fmt.Errorf("template selection failed: %w", err)
		}
	}

	if selectedTemplate == nil {
		fmt.Println("No template selected. Exiting.")
		return nil
	}

	fmt.Printf("\n%s Selected template: %s %s\n",
		green("✅"), selectedTemplate.Metadata.Icon, selectedTemplate.Metadata.Name)
	fmt.Printf("Description: %s\n", selectedTemplate.Metadata.Description)
	fmt.Printf("Estimated setup time: %s\n", selectedTemplate.Metadata.EstimatedTime)

	// Ask for customizations
	fmt.Println(cyan("\n⚙️ Template Customization"))
	fmt.Print("Would you like to customize the template? (y/N): ")
	response, _ := reader.ReadString('\n')
	customize := strings.ToLower(strings.TrimSpace(response)) == "y"

	// Apply template with optional customizations
	if err := applyTemplate(*selectedTemplate, settings, p, customize, reader, green, yellow, cyan); err != nil {
		return fmt.Errorf("failed to apply template: %w", err)
	}

	fmt.Println(green("\n🎉 Configuration created successfully!"))
	fmt.Printf("Configuration saved to: %s\n", settings.GetConfigDir())
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Review your configuration files:")
	fmt.Println("     devex config show")
	fmt.Println("  2. Install selected applications:")
	fmt.Println("     devex install")
	fmt.Println("  3. Apply system settings:")
	fmt.Println("     devex system")

	return nil
}

// selectTemplateWithTUI uses the TUI for template selection
func selectTemplateWithTUI(templateManager *templates.TemplateManager) (*templates.Template, error) {
	model, err := NewTemplateSelectionModel(templateManager)
	if err != nil {
		return nil, err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI failed: %w", err)
	}

	if m, ok := finalModel.(TemplateSelectionModel); ok {
		return m.choice, nil
	}

	return nil, fmt.Errorf("unexpected model type")
}

// selectTemplateFromList uses simple list selection
func selectTemplateFromList(templateManager *templates.TemplateManager, reader *bufio.Reader, yellow, cyan func(...interface{}) string) (*templates.Template, error) {
	availableTemplates, err := templateManager.GetAvailableTemplates()
	if err != nil {
		return nil, err
	}

	fmt.Println(cyan("\n📋 Available Templates"))
	fmt.Println("Choose a template that best matches your needs:")

	// Group and display templates
	categories := make(map[string][]templates.Template)
	for _, template := range availableTemplates {
		categories[template.Metadata.Category] = append(categories[template.Metadata.Category], template)
	}

	templateIndex := 1
	indexToTemplate := make(map[int]templates.Template)
	titleCase := cases.Title(language.English)

	for category, categoryTemplates := range categories {
		fmt.Printf("\n  %s:\n", titleCase.String(category))
		for _, template := range categoryTemplates {
			fmt.Printf("    %d. %s %s - %s\n",
				templateIndex, template.Metadata.Icon, yellow(template.Metadata.Name), template.Metadata.Description)
			fmt.Printf("       Difficulty: %s | Time: %s\n",
				template.Metadata.Difficulty, template.Metadata.EstimatedTime)
			indexToTemplate[templateIndex] = template
			templateIndex++
		}
	}

	fmt.Printf("\nSelect template (1-%d): ", len(availableTemplates))
	var choice int
	if _, err := fmt.Scanln(&choice); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if choice < 1 || choice > len(availableTemplates) {
		return nil, fmt.Errorf("invalid choice: %d", choice)
	}

	selectedTemplate := indexToTemplate[choice]
	return &selectedTemplate, nil
}

// applyTemplate applies the selected template to create configuration
func applyTemplate(template templates.Template, settings config.CrossPlatformSettings, platformInfo platform.DetectionResult, customize bool, reader *bufio.Reader, green, yellow, cyan func(...interface{}) string) error {
	configDir := settings.GetConfigDir()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Customize template if requested
	if customize {
		if err := customizeTemplate(&template, reader, yellow, cyan); err != nil {
			return fmt.Errorf("template customization failed: %w", err)
		}
	}

	// Generate configuration files from template
	if err := generateConfigFromTemplate(template, configDir, platformInfo); err != nil {
		return fmt.Errorf("failed to generate configuration: %w", err)
	}

	return nil
}

// customizeTemplate allows user to customize the template
func customizeTemplate(template *templates.Template, reader *bufio.Reader, yellow, cyan func(...interface{}) string) error {
	fmt.Println(cyan("\n🔧 Template Customization"))

	// Shell customization
	fmt.Printf("Current shell: %s\n", template.Environment.Shell)
	fmt.Print("Change shell? (y/N): ")
	response, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		fmt.Print("Enter shell (bash/zsh/fish): ")
		shell, _ := reader.ReadString('\n')
		template.Environment.Shell = strings.TrimSpace(shell)
	}

	// Editor customization
	fmt.Printf("Current editor: %s\n", template.Environment.Editor)
	fmt.Print("Change editor? (y/N): ")
	response, _ = reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		fmt.Print("Enter editor (vim/vscode/nano/emacs): ")
		editor, _ := reader.ReadString('\n')
		template.Environment.Editor = strings.TrimSpace(editor)
	}

	// Applications customization
	fmt.Println("\nWould you like to modify the application selection?")
	fmt.Printf("Current applications (%d): ", len(template.Applications))
	for i, app := range template.Applications {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(app.Name)
	}
	fmt.Println()

	fmt.Print("Modify applications? (y/N): ")
	response, _ = reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		// Simple app customization - for now just ask about defaults
		fmt.Print("Include optional applications? (Y/n): ")
		response, _ = reader.ReadString('\n')
		includeOptional := strings.ToLower(strings.TrimSpace(response)) != "n"

		for i := range template.Applications {
			if !template.Applications[i].Default {
				template.Applications[i].Default = includeOptional
			}
		}
	}

	return nil
}

// generateConfigFromTemplate creates configuration files from template
func generateConfigFromTemplate(template templates.Template, configDir string, platformInfo platform.DetectionResult) error {
	// Create applications.yaml
	if err := createApplicationsConfig(template, configDir); err != nil {
		return fmt.Errorf("failed to create applications config: %w", err)
	}

	// Create environment.yaml
	if err := createEnvironmentConfig(template, configDir); err != nil {
		return fmt.Errorf("failed to create environment config: %w", err)
	}

	// Create system.yaml
	if err := createSystemConfig(template, configDir); err != nil {
		return fmt.Errorf("failed to create system config: %w", err)
	}

	// Create desktop.yaml if needed
	if template.Desktop != nil && platformInfo.DesktopEnv != "" {
		if err := createDesktopConfig(template, configDir, platformInfo); err != nil {
			return fmt.Errorf("failed to create desktop config: %w", err)
		}
	}

	// Create metadata file
	if err := createMetadataConfig(template, configDir, platformInfo); err != nil {
		return fmt.Errorf("failed to create metadata config: %w", err)
	}

	return nil
}

// createApplicationsConfig creates the applications.yaml file
func createApplicationsConfig(template templates.Template, configDir string) error {
	var finalApplications []types.AppConfig

	if template.Metadata.Additive {
		// For additive templates, start with default git application and add template apps
		gitApp := types.AppConfig{
			BaseConfig: types.BaseConfig{
				Name:        "git",
				Description: "Version control system",
				Category:    "development",
			},
			InstallMethod: "apt",
			Default:       true,
		}
		finalApplications = append(finalApplications, gitApp)

		// Add template applications
		finalApplications = append(finalApplications, template.Applications...)

		// Add comment indicating this is additive
		header := fmt.Sprintf(`# DevEx Applications Configuration
# Generated from template: %s (ADDITIVE)
# %s
# 
# This configuration adds applications to the default DevEx setup.
# Default applications (like git) are automatically included.

`, template.Metadata.Name, template.Metadata.Description)

		config := struct {
			Applications []types.AppConfig `yaml:"applications"`
		}{
			Applications: finalApplications,
		}

		return writeYAMLConfig(config, filepath.Join(configDir, "applications.yaml"), header)
	} else {
		// For non-additive templates, use only template applications
		config := struct {
			Applications []types.AppConfig `yaml:"applications"`
		}{
			Applications: template.Applications,
		}

		return writeYAMLConfig(config, filepath.Join(configDir, "applications.yaml"),
			fmt.Sprintf("# DevEx Applications Configuration\n# Generated from template: %s\n# %s\n\n",
				template.Metadata.Name, template.Metadata.Description))
	}
}

// createEnvironmentConfig creates the environment.yaml file
func createEnvironmentConfig(template templates.Template, configDir string) error {
	config := template.Environment

	return writeYAMLConfig(config, filepath.Join(configDir, "environment.yaml"),
		fmt.Sprintf("# DevEx Environment Configuration\n# Generated from template: %s\n\n",
			template.Metadata.Name))
}

// createSystemConfig creates the system.yaml file
func createSystemConfig(template templates.Template, configDir string) error {
	config := template.System

	return writeYAMLConfig(config, filepath.Join(configDir, "system.yaml"),
		fmt.Sprintf("# DevEx System Configuration\n# Generated from template: %s\n\n",
			template.Metadata.Name))
}

// createDesktopConfig creates the desktop.yaml file
func createDesktopConfig(template templates.Template, configDir string, platformInfo platform.DetectionResult) error {
	if template.Desktop == nil {
		return nil
	}

	// Override environment with detected platform
	desktopConfig := *template.Desktop
	desktopConfig.Environment = platformInfo.DesktopEnv

	return writeYAMLConfig(desktopConfig, filepath.Join(configDir, "desktop.yaml"),
		fmt.Sprintf("# DevEx Desktop Configuration\n# Generated from template: %s\n# Desktop Environment: %s\n\n",
			template.Metadata.Name, platformInfo.DesktopEnv))
}

// createMetadataConfig creates the devex.yaml metadata file
func createMetadataConfig(template templates.Template, configDir string, platformInfo platform.DetectionResult) error {
	metadata := struct {
		Template templates.TemplateMetadata `yaml:"template"`
		Platform platform.DetectionResult   `yaml:"platform"`
		Created  string                     `yaml:"created"`
	}{
		Template: template.Metadata,
		Platform: platformInfo,
		Created:  os.Getenv("USER"), // Simple user info
	}

	return writeYAMLConfig(metadata, filepath.Join(configDir, "devex.yaml"),
		fmt.Sprintf("# DevEx Configuration Metadata\n# Generated from template: %s\n\n",
			template.Metadata.Name))
}

// writeYAMLConfig writes a YAML configuration file with header
func writeYAMLConfig(config interface{}, path, header string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	content := header + string(data)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
