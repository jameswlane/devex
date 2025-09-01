package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/help"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

// InitConfig represents the configuration created by init command
type InitConfig struct {
	Profile      string             `yaml:"profile"`
	Platform     PlatformConfig     `yaml:"platform"`
	Applications ApplicationsConfig `yaml:"applications"`
	Environment  EnvironmentConfig  `yaml:"environment"`
	System       SystemConfig       `yaml:"system"`
	Desktop      DesktopConfig      `yaml:"desktop,omitempty"`
}

// PlatformConfig represents platform detection results
type PlatformConfig struct {
	OS           string `yaml:"os"`
	Distribution string `yaml:"distribution,omitempty"`
	Version      string `yaml:"version,omitempty"`
	Architecture string `yaml:"architecture"`
	Desktop      string `yaml:"desktop_environment,omitempty"`
}

// ApplicationsConfig represents application preferences
type ApplicationsConfig struct {
	Categories []string `yaml:"categories"`
	Defaults   bool     `yaml:"install_defaults"`
	Custom     []string `yaml:"custom_apps,omitempty"`
}

// EnvironmentConfig represents environment preferences
type EnvironmentConfig struct {
	Languages []string `yaml:"languages"`
	Shell     string   `yaml:"shell"`
	Editor    string   `yaml:"editor"`
}

// SystemConfig represents system preferences
type SystemConfig struct {
	GitConfig   bool   `yaml:"configure_git"`
	SSHConfig   bool   `yaml:"configure_ssh"`
	GlobalTheme string `yaml:"global_theme,omitempty"`
}

// DesktopConfig represents desktop environment preferences
type DesktopConfig struct {
	Environment string   `yaml:"environment"`
	Themes      []string `yaml:"themes,omitempty"`
	Extensions  []string `yaml:"extensions,omitempty"`
}

// ProfileTemplate represents a pre-configured profile
type ProfileTemplate struct {
	Name        string
	Description string
	Categories  []string
	Languages   []string
	Tools       []string
}

var profileTemplates = []ProfileTemplate{
	{
		Name:        "full-stack",
		Description: "Full-stack web development with Node.js, Python, and databases",
		Categories:  []string{"development", "database", "container"},
		Languages:   []string{"node", "python", "go"},
		Tools:       []string{"docker", "git", "vscode", "postgresql", "redis"},
	},
	{
		Name:        "backend",
		Description: "Backend development with Go, databases, and containers",
		Categories:  []string{"development", "database", "container"},
		Languages:   []string{"go", "python", "rust"},
		Tools:       []string{"docker", "git", "postgresql", "redis", "rabbitmq"},
	},
	{
		Name:        "frontend",
		Description: "Frontend development with Node.js and modern web tools",
		Categories:  []string{"development", "browser"},
		Languages:   []string{"node", "typescript"},
		Tools:       []string{"git", "vscode", "chrome", "firefox-developer"},
	},
	{
		Name:        "devops",
		Description: "DevOps and infrastructure with containers and automation",
		Categories:  []string{"development", "container", "infrastructure"},
		Languages:   []string{"go", "python", "bash"},
		Tools:       []string{"docker", "kubernetes", "terraform", "ansible", "git"},
	},
	{
		Name:        "data-science",
		Description: "Data science with Python, R, and Jupyter",
		Categories:  []string{"development", "database", "science"},
		Languages:   []string{"python", "r", "julia"},
		Tools:       []string{"jupyter", "git", "postgresql", "mongodb"},
	},
	{
		Name:        "minimal",
		Description: "Minimal setup with just essential tools",
		Categories:  []string{"development"},
		Languages:   []string{},
		Tools:       []string{"git", "vim"},
	},
	{
		Name:        "custom",
		Description: "Create your own custom configuration",
		Categories:  []string{},
		Languages:   []string{},
		Tools:       []string{},
	},
}

// NewInitCmd creates a new init command
func NewInitCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		interactive  bool
		profile      string
		force        bool
		validate     bool
		export       bool
		importFile   string
		useTemplates bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize or validate DevEx configuration",
		Long: `Initialize a new DevEx configuration or validate an existing one.

This command helps you:
  • Create a personalized configuration based on your needs
  • Validate existing configuration files
  • Import/export configurations for sharing
  • Set up profile-based configurations (full-stack, backend, frontend, etc.)

Examples:
  # Interactive configuration wizard
  devex init
  
  # Use a predefined profile
  devex init --profile full-stack
  
  # Validate existing configuration
  devex init --validate
  
  # Export current configuration
  devex init --export > my-config.yaml
  
  # Import configuration from file
  devex init --import team-config.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if validate {
				return validateConfiguration(settings)
			}

			if export {
				return exportInitConfiguration(settings)
			}

			if importFile != "" {
				return importInitConfiguration(importFile, settings, force)
			}

			if profile != "" && profile != "custom" {
				return initWithProfile(profile, settings, repo, force)
			}

			// Use new template system if requested
			if useTemplates {
				return runInteractiveInitWithTemplates(settings, repo, force)
			}

			// Default to interactive mode
			return runInteractiveInit(settings, repo, force)
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", true, "Run in interactive mode")
	cmd.Flags().StringVarP(&profile, "profile", "p", "", "Use a predefined profile (full-stack, backend, frontend, devops, data-science, minimal)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing configuration")
	cmd.Flags().BoolVar(&validate, "validate", false, "Validate existing configuration")
	cmd.Flags().BoolVar(&export, "export", false, "Export current configuration")
	cmd.Flags().StringVar(&importFile, "import", "", "Import configuration from file")
	cmd.Flags().BoolVarP(&useTemplates, "templates", "t", false, "Use the new template system (recommended)")

	// Add contextual help integration
	AddContextualHelp(cmd, help.ContextGettingStarted, "init")

	return cmd
}

func runInteractiveInit(settings config.CrossPlatformSettings, repo types.Repository, force bool) error {
	reader := bufio.NewReader(os.Stdin)
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Println(cyan("\n🚀 DevEx Configuration Wizard"))
	fmt.Println("This wizard will help you create a personalized development environment configuration.")

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

	config := InitConfig{}

	// Platform detection
	fmt.Println(cyan("\n📍 Platform Detection"))
	p := platform.DetectPlatform()
	config.Platform = PlatformConfig{
		OS:           p.OS,
		Distribution: p.Distribution,
		Version:      p.Version,
		Architecture: p.Architecture,
		Desktop:      p.DesktopEnv,
	}
	fmt.Printf("  OS: %s\n", green(p.OS))
	if p.Distribution != "" {
		fmt.Printf("  Distribution: %s %s\n", green(p.Distribution), p.Version)
	}
	fmt.Printf("  Architecture: %s\n", green(p.Architecture))
	if p.DesktopEnv != "" {
		fmt.Printf("  Desktop: %s\n", green(p.DesktopEnv))
	}

	// Profile selection
	fmt.Println(cyan("\n👤 Profile Selection"))
	fmt.Println("Choose a profile that best matches your needs:")
	for i, prof := range profileTemplates {
		fmt.Printf("  %d. %s - %s\n", i+1, yellow(prof.Name), prof.Description)
	}
	fmt.Print("\nSelect profile (1-7): ")

	var profileChoice int
	_, _ = fmt.Scanln(&profileChoice)

	if profileChoice < 1 || profileChoice > len(profileTemplates) {
		profileChoice = len(profileTemplates) // Default to custom
	}

	selectedProfile := profileTemplates[profileChoice-1]
	config.Profile = selectedProfile.Name

	// If not custom, apply profile defaults
	if selectedProfile.Name != "custom" {
		config.Applications.Categories = selectedProfile.Categories
		config.Environment.Languages = selectedProfile.Languages
		config.Applications.Custom = selectedProfile.Tools
		config.Applications.Defaults = true
	} else {
		// Custom configuration
		fmt.Println(cyan("\n📦 Application Categories"))
		fmt.Println("Which categories of applications do you want to install?")
		categories := []string{"development", "database", "container", "browser", "communication", "productivity"}
		selectedCategories := []string{}

		for _, cat := range categories {
			fmt.Printf("Include %s? (y/N): ", yellow(cat))
			response, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(response)) == "y" {
				selectedCategories = append(selectedCategories, cat)
			}
		}
		config.Applications.Categories = selectedCategories

		// Programming languages
		fmt.Println(cyan("\n🔧 Programming Languages"))
		fmt.Println("Which programming languages do you use?")
		languages := []string{"node", "python", "go", "rust", "java", "ruby", "php", "dotnet"}
		selectedLanguages := []string{}

		for _, lang := range languages {
			fmt.Printf("Install %s? (y/N): ", yellow(lang))
			response, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(response)) == "y" {
				selectedLanguages = append(selectedLanguages, lang)
			}
		}
		config.Environment.Languages = selectedLanguages

		// Install defaults
		fmt.Print("\nInstall default applications for selected categories? (Y/n): ")
		response, _ := reader.ReadString('\n')
		config.Applications.Defaults = strings.ToLower(strings.TrimSpace(response)) != "n"
	}

	// Shell configuration
	fmt.Println(cyan("\n🐚 Shell Configuration"))
	currentShell := os.Getenv("SHELL")
	if currentShell == "" {
		currentShell = "/bin/bash"
	}
	fmt.Printf("Current shell: %s\n", green(filepath.Base(currentShell)))
	fmt.Print("Change shell? (y/N): ")
	response, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		fmt.Print("Enter shell (bash/zsh/fish): ")
		shell, _ := reader.ReadString('\n')
		config.Environment.Shell = strings.TrimSpace(shell)
	} else {
		config.Environment.Shell = filepath.Base(currentShell)
	}

	// Editor preference
	fmt.Println(cyan("\n📝 Editor Preference"))
	fmt.Print("Preferred editor (vim/neovim/vscode/sublime/atom): ")
	editor, _ := reader.ReadString('\n')
	config.Environment.Editor = strings.TrimSpace(editor)
	if config.Environment.Editor == "" {
		config.Environment.Editor = "vim"
	}

	// System configuration
	fmt.Println(cyan("\n⚙️  System Configuration"))
	fmt.Print("Configure Git settings? (Y/n): ")
	response, _ = reader.ReadString('\n')
	config.System.GitConfig = strings.ToLower(strings.TrimSpace(response)) != "n"

	fmt.Print("Configure SSH? (Y/n): ")
	response, _ = reader.ReadString('\n')
	config.System.SSHConfig = strings.ToLower(strings.TrimSpace(response)) != "n"

	fmt.Print("Set a global theme? (y/N): ")
	response, _ = reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		fmt.Print("Theme name (e.g., 'Tokyo Night', 'Dracula', 'Nord'): ")
		theme, _ := reader.ReadString('\n')
		config.System.GlobalTheme = strings.TrimSpace(theme)
	}

	// Desktop environment configuration
	if p.DesktopEnv != "" {
		fmt.Println(cyan("\n🖥️  Desktop Environment"))
		fmt.Printf("Configure %s desktop? (Y/n): ", p.DesktopEnv)
		response, _ = reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(response)) != "n" {
			config.Desktop.Environment = p.DesktopEnv

			if strings.ToLower(p.DesktopEnv) == "gnome" {
				fmt.Print("Install GNOME extensions? (Y/n): ")
				response, _ = reader.ReadString('\n')
				if strings.ToLower(strings.TrimSpace(response)) != "n" {
					config.Desktop.Extensions = []string{
						"dash-to-dock",
						"blur-my-shell",
						"user-themes",
					}
				}
			}
		}
	}

	// Save configuration
	if err := saveInitConfig(config, settings); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println(green("\n✅ Configuration created successfully!"))
	fmt.Printf("Configuration saved to: %s\n", settings.GetConfigDir())
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Review your configuration files")
	fmt.Println("  2. Run 'devex install' to install selected applications")
	fmt.Println("  3. Run 'devex system' to apply system settings")

	return nil
}

func initWithProfile(profileName string, settings config.CrossPlatformSettings, repo types.Repository, force bool) error {
	// Find the profile
	var selectedProfile *ProfileTemplate
	for _, prof := range profileTemplates {
		if prof.Name == profileName {
			selectedProfile = &prof
			break
		}
	}

	if selectedProfile == nil {
		return fmt.Errorf("unknown profile: %s", profileName)
	}

	// Check for existing configuration
	configDir := settings.GetConfigDir()
	if _, err := os.Stat(configDir); !os.IsNotExist(err) && !force {
		return fmt.Errorf("configuration already exists. Use --force to overwrite")
	}

	// Create configuration from profile
	p := platform.DetectPlatform()
	config := InitConfig{
		Profile: selectedProfile.Name,
		Platform: PlatformConfig{
			OS:           p.OS,
			Distribution: p.Distribution,
			Version:      p.Version,
			Architecture: p.Architecture,
			Desktop:      p.DesktopEnv,
		},
		Applications: ApplicationsConfig{
			Categories: selectedProfile.Categories,
			Defaults:   true,
			Custom:     selectedProfile.Tools,
		},
		Environment: EnvironmentConfig{
			Languages: selectedProfile.Languages,
			Shell:     filepath.Base(os.Getenv("SHELL")),
			Editor:    "vim",
		},
		System: SystemConfig{
			GitConfig: true,
			SSHConfig: true,
		},
	}

	// Add desktop configuration if applicable
	if p.DesktopEnv != "" {
		config.Desktop = DesktopConfig{
			Environment: p.DesktopEnv,
		}
	}

	// Save configuration
	if err := saveInitConfig(config, settings); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("%s Configuration created with '%s' profile!\n", green("✅"), profileName)
	fmt.Printf("Configuration saved to: %s\n", settings.GetConfigDir())

	return nil
}

func validateConfiguration(settings config.CrossPlatformSettings) error {
	fmt.Println("🔍 Validating DevEx configuration...")

	errors := []string{}
	warnings := []string{}

	// Check configuration directory
	configDir := settings.GetConfigDir()
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		errors = append(errors, fmt.Sprintf("Configuration directory not found: %s", configDir))
	}

	// Validate each configuration file
	configFiles := []string{
		"applications.yaml",
		"environment.yaml",
		"system.yaml",
		"desktop.yaml",
	}

	for _, file := range configFiles {
		path := filepath.Join(configDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if file == "desktop.yaml" {
				// Desktop config is optional
				warnings = append(warnings, fmt.Sprintf("Optional file missing: %s", file))
			} else {
				errors = append(errors, fmt.Sprintf("Required file missing: %s", file))
			}
			continue
		}

		// Try to parse the YAML
		data, err := os.ReadFile(path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to read %s: %v", file, err))
			continue
		}

		var content map[string]interface{}
		if err := yaml.Unmarshal(data, &content); err != nil {
			errors = append(errors, fmt.Sprintf("Invalid YAML in %s: %v", file, err))
		}
	}

	// Validate application configurations
	apps := settings.GetApplications()
	for _, app := range apps {
		if err := app.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("Invalid app config for %s: %v", app.Name, err))
		}
	}

	// Print results
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	if len(errors) == 0 && len(warnings) == 0 {
		fmt.Println(green("✅ Configuration is valid!"))
		return nil
	}

	if len(warnings) > 0 {
		fmt.Println(yellow("\n⚠️  Warnings:"))
		for _, warning := range warnings {
			fmt.Printf("  • %s\n", warning)
		}
	}

	if len(errors) > 0 {
		fmt.Println(red("\n❌ Errors:"))
		for _, err := range errors {
			fmt.Printf("  • %s\n", err)
		}
		return fmt.Errorf("configuration validation failed with %d errors", len(errors))
	}

	return nil
}

func exportInitConfiguration(settings config.CrossPlatformSettings) error {
	// Create a comprehensive export of all configurations
	export := map[string]interface{}{
		"version": "1.0",
		"platform": map[string]string{
			"os":           runtime.GOOS,
			"architecture": runtime.GOARCH,
		},
		"applications": settings.GetApplications(),
		"environment":  settings.GetEnvironmentSettings(),
		"system":       settings.GetSystemSettings(),
	}

	// Add desktop settings if available
	if desktop := settings.GetDesktopSettings(); desktop != nil {
		export["desktop"] = desktop
	}

	// Marshal to YAML
	data, err := yaml.Marshal(export)
	if err != nil {
		return fmt.Errorf("failed to export configuration: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

func importInitConfiguration(file string, settings config.CrossPlatformSettings, force bool) error {
	// Check for existing configuration
	configDir := settings.GetConfigDir()
	if _, err := os.Stat(configDir); !os.IsNotExist(err) && !force {
		return fmt.Errorf("configuration already exists. Use --force to overwrite")
	}

	// Read import file
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read import file: %w", err)
	}

	// Parse the configuration
	var imported map[string]interface{}
	if err := yaml.Unmarshal(data, &imported); err != nil {
		return fmt.Errorf("invalid configuration file: %w", err)
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save each configuration section
	sections := map[string]string{
		"applications": "applications.yaml",
		"environment":  "environment.yaml",
		"system":       "system.yaml",
		"desktop":      "desktop.yaml",
	}

	for section, filename := range sections {
		if content, ok := imported[section]; ok {
			data, err := yaml.Marshal(content)
			if err != nil {
				return fmt.Errorf("failed to process %s section: %w", section, err)
			}

			path := filepath.Join(configDir, filename)
			if err := os.WriteFile(path, data, 0600); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}
	}

	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("%s Configuration imported successfully!\n", green("✅"))
	fmt.Printf("Configuration saved to: %s\n", configDir)

	return nil
}

func saveInitConfig(config InitConfig, settings config.CrossPlatformSettings) error {
	configDir := settings.GetConfigDir()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create undo operation before initialization
	baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
	undoManager := undo.NewUndoManager(baseDir)

	metadata := map[string]interface{}{
		"profile":    config.Profile,
		"platform":   config.Platform.OS,
		"desktop":    config.Platform.Desktop,
		"shell":      config.Environment.Shell,
		"languages":  config.Environment.Languages,
		"categories": config.Applications.Categories,
	}

	undoOp, err := undoManager.RecordOperation("init",
		fmt.Sprintf("Initialized configuration with profile: %s", config.Profile),
		config.Profile,
		metadata)
	if err != nil {
		// Log warning but don't block the operation
		fmt.Fprintf(os.Stderr, "Warning: Failed to record undo operation: %v\n", err)
	}

	// Create a metadata file with the init configuration
	metadataPath := filepath.Join(configDir, "devex.yaml")
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	// Create basic configuration files if they don't exist
	if err := createDefaultConfigs(configDir, config); err != nil {
		return fmt.Errorf("failed to create default configs: %w", err)
	}

	// Create new version after successful initialization
	vm := version.NewVersionManager(baseDir)
	_, versionErr := vm.UpdateVersion(
		fmt.Sprintf("Initialized configuration with profile: %s", config.Profile),
		[]string{
			fmt.Sprintf("Initialized DevEx with %s profile", config.Profile),
			fmt.Sprintf("Platform: %s", config.Platform.OS),
			fmt.Sprintf("Desktop: %s", config.Platform.Desktop),
			fmt.Sprintf("Shell: %s", config.Environment.Shell),
			fmt.Sprintf("Languages: %v", config.Environment.Languages),
		},
	)
	if versionErr != nil {
		// Log warning but don't fail the operation
		fmt.Fprintf(os.Stderr, "Warning: Failed to create version: %v\n", versionErr)
	}

	// Update undo operation with completion info
	if undoOp != nil {
		if updateErr := undoManager.UpdateOperation(undoOp.ID); updateErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to update undo operation: %v\n", updateErr)
		}
	}

	return nil
}

func createDefaultConfigs(configDir string, config InitConfig) error {
	// Only create files if they don't exist
	files := map[string]string{
		"applications.yaml": getDefaultApplicationsConfig(config),
		"environment.yaml":  getDefaultEnvironmentConfig(config),
		"system.yaml":       getDefaultSystemConfig(config),
	}

	if config.Desktop.Environment != "" {
		files["desktop.yaml"] = getDefaultDesktopConfig(config)
	}

	for filename, content := range files {
		path := filepath.Join(configDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}
	}

	return nil
}

func getDefaultApplicationsConfig(config InitConfig) string {
	// Return a minimal applications config based on selections
	return fmt.Sprintf(`# DevEx Applications Configuration
# Generated by: devex init --profile %s

applications:
  - name: git
    category: development
    default: true
    install_method: apt
    install_command: git

# Add more applications as needed
`, config.Profile)
}

func getDefaultEnvironmentConfig(config InitConfig) string {
	return fmt.Sprintf(`# DevEx Environment Configuration
# Generated by: devex init --profile %s

shell: %s
editor: %s

languages: %v

# Add more environment settings as needed
`, config.Profile, config.Environment.Shell, config.Environment.Editor, config.Environment.Languages)
}

func getDefaultSystemConfig(config InitConfig) string {
	gitConfig := "false"
	if config.System.GitConfig {
		gitConfig = "true"
	}
	sshConfig := "false"
	if config.System.SSHConfig {
		sshConfig = "true"
	}

	result := fmt.Sprintf(`# DevEx System Configuration
# Generated by: devex init --profile %s

configure_git: %s
configure_ssh: %s
`, config.Profile, gitConfig, sshConfig)

	if config.System.GlobalTheme != "" {
		result += fmt.Sprintf("global_theme: '%s'\n", config.System.GlobalTheme)
	}

	return result
}

func getDefaultDesktopConfig(config InitConfig) string {
	return fmt.Sprintf(`# DevEx Desktop Configuration
# Generated by: devex init --profile %s

desktop_environment: %s

# Add desktop-specific settings here
`, config.Profile, config.Desktop.Environment)
}
