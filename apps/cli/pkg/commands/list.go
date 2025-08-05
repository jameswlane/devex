package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	Register(NewListCmd)
}

// InstalledApp represents an installed application with additional metadata
type InstalledApp struct {
	Name          string   `json:"name" yaml:"name"`
	Description   string   `json:"description" yaml:"description"`
	Category      string   `json:"category" yaml:"category"`
	InstallMethod string   `json:"install_method" yaml:"install_method"`
	Version       string   `json:"version,omitempty" yaml:"version,omitempty"`
	Dependencies  []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

// AvailableApp represents an available application with platform information
type AvailableApp struct {
	Name           string   `json:"name" yaml:"name"`
	Description    string   `json:"description" yaml:"description"`
	Category       string   `json:"category" yaml:"category"`
	InstallMethods []string `json:"install_methods" yaml:"install_methods"`
	Platforms      []string `json:"supported_platforms" yaml:"supported_platforms"`
	Recommended    bool     `json:"recommended" yaml:"recommended"`
	Dependencies   []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Installed      bool     `json:"installed" yaml:"installed"`
}

// CategoryInfo represents category information
type CategoryInfo struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	AppCount    int      `json:"app_count" yaml:"app_count"`
	Platforms   []string `json:"platforms" yaml:"platforms"`
}

// ListCommandOptions defines the options for the list command
type ListCommandOptions struct {
	Category    string
	Format      string
	Verbose     bool
	Search      string
	Installed   bool
	Available   bool
	Method      string
	Recommended bool
	Interactive bool
}

// NewListCmd creates the list command
func NewListCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [installed|available|categories]",
		Short: "List installed or available applications",
		Long: `List applications in your DevEx configuration.

Available subcommands:
  installed   - Show applications that are currently installed
  available   - Show all applications available for installation
  categories  - Show all available categories

Examples:
  devex list installed                    # Show installed apps
  devex list available                    # Show all available apps
  devex list available --category development  # Show development tools
  devex list installed --format json     # JSON output
  devex list available --search docker   # Search for Docker-related apps
  devex list categories                   # Show all categories`,
		ValidArgs: []string{"installed", "available", "categories"},
		Run: func(cmd *cobra.Command, args []string) {
			runListCommand(cmd, args, repo, settings)
		},
	}

	// Add flags
	cmd.Flags().StringP("category", "c", "", "Filter by category")
	cmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml)")
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed information")
	cmd.Flags().StringP("search", "s", "", "Search applications by name or description")
	cmd.Flags().String("method", "", "Filter by installation method")
	cmd.Flags().Bool("recommended", false, "Show only recommended applications")
	cmd.Flags().Bool("interactive", false, "Interactive selection mode")

	return cmd
}

func runListCommand(cmd *cobra.Command, args []string, repo types.Repository, settings config.CrossPlatformSettings) {
	// Parse flags
	options := parseListFlags(cmd)

	if len(args) == 0 {
		// Show both installed and available
		fmt.Println("📦 DevEx Application Status")
		fmt.Println("=" + strings.Repeat("=", 25))
		fmt.Println()

		showInstalledApps(repo, settings, options)
		fmt.Println()
		showAvailableApps(settings, options)
		return
	}

	switch args[0] {
	case "installed":
		showInstalledApps(repo, settings, options)
	case "available":
		showAvailableApps(settings, options)
	case "categories":
		showCategories(settings, options)
	default:
		fmt.Printf("Error: Unknown subcommand '%s'\n", args[0])
		fmt.Println("Available subcommands: installed, available, categories")
		os.Exit(1)
	}
}

// parseListFlags extracts flags from the command
func parseListFlags(cmd *cobra.Command) ListCommandOptions {
	category, _ := cmd.Flags().GetString("category")
	format, _ := cmd.Flags().GetString("format")
	verbose, _ := cmd.Flags().GetBool("verbose")
	search, _ := cmd.Flags().GetString("search")
	method, _ := cmd.Flags().GetString("method")
	recommended, _ := cmd.Flags().GetBool("recommended")
	interactive, _ := cmd.Flags().GetBool("interactive")

	return ListCommandOptions{
		Category:    category,
		Format:      format,
		Verbose:     verbose,
		Search:      search,
		Method:      method,
		Recommended: recommended,
		Interactive: interactive,
	}
}

func showInstalledApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) {
	log.Info("Listing installed applications")

	if repo == nil {
		fmt.Println("❌ Database not available - cannot check installation status")
		return
	}

	// Get installed applications from database
	installedApps, err := getInstalledApps(repo, settings, options)
	if err != nil {
		fmt.Printf("❌ Error retrieving installed applications: %v\n", err)
		return
	}

	// Apply filters
	installedApps = filterInstalledApps(installedApps, options)

	// Output based on format
	switch options.Format {
	case "json":
		outputInstalledJSON(installedApps)
	case "yaml":
		outputInstalledYAML(installedApps)
	default:
		outputInstalledTable(installedApps, options)
	}
}

func showAvailableApps(settings config.CrossPlatformSettings, options ListCommandOptions) {
	log.Info("Listing available applications")

	// Get available applications
	availableApps := getAvailableApps(settings, options)

	// Apply filters
	availableApps = filterAvailableApps(availableApps, options)

	// Output based on format
	switch options.Format {
	case "json":
		outputAvailableJSON(availableApps)
	case "yaml":
		outputAvailableYAML(availableApps)
	default:
		outputAvailableTable(availableApps, options)
	}
}

func showCategories(settings config.CrossPlatformSettings, options ListCommandOptions) {
	log.Info("Listing categories")

	// Get category information
	categories := getCategoryInfo(settings)

	// Output based on format
	switch options.Format {
	case "json":
		outputCategoriesJSON(categories)
	case "yaml":
		outputCategoriesYAML(categories)
	default:
		outputCategoriesTable(categories, options)
	}
}

// getInstalledApps retrieves installed applications from the database
func getInstalledApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) ([]InstalledApp, error) {
	var installedApps []InstalledApp

	// Get all available apps to cross-reference
	allApps := settings.GetAllApps()
	appMap := make(map[string]types.CrossPlatformApp)
	for _, app := range allApps {
		appMap[app.Name] = app
	}

	// Get installed apps from database
	dbApps, err := repo.ListApps()
	if err != nil {
		return nil, fmt.Errorf("failed to get installed apps: %w", err)
	}

	for _, dbApp := range dbApps {
		if configApp, exists := appMap[dbApp.Name]; exists {
			installedApp := InstalledApp{
				Name:          dbApp.Name,
				Description:   configApp.Description,
				Category:      configApp.Category,
				InstallMethod: dbApp.InstallMethod,
				Version:       "", // TODO: Add version tracking
				Dependencies:  configApp.GetOSConfig().Dependencies,
			}
			installedApps = append(installedApps, installedApp)
		}
	}

	return installedApps, nil
}

// getAvailableApps retrieves all available applications
func getAvailableApps(settings config.CrossPlatformSettings, options ListCommandOptions) []AvailableApp {
	allApps := settings.GetAllApps()
	availableApps := make([]AvailableApp, 0, len(allApps))
	for _, app := range allApps {
		if !app.IsSupported() {
			continue
		}

		osConfig := app.GetOSConfig()
		installMethods := []string{}
		if osConfig.InstallMethod != "" {
			installMethods = append(installMethods, osConfig.InstallMethod)
		}

		// Add alternative methods
		for _, alt := range osConfig.Alternatives {
			if alt.InstallMethod != "" {
				installMethods = append(installMethods, alt.InstallMethod)
			}
		}

		availableApp := AvailableApp{
			Name:           app.Name,
			Description:    app.Description,
			Category:       app.Category,
			InstallMethods: installMethods,
			Platforms:      getSupportedPlatforms(app),
			Recommended:    app.Default,
			Dependencies:   osConfig.Dependencies,
			Installed:      false, // TODO: Check if installed
		}
		availableApps = append(availableApps, availableApp)
	}

	return availableApps
}

// getCategoryInfo retrieves category information
func getCategoryInfo(settings config.CrossPlatformSettings) []CategoryInfo {
	categories := make(map[string]*CategoryInfo)

	allApps := settings.GetAllApps()
	for _, app := range allApps {
		category := app.Category
		if category == "" {
			category = "Other"
		}

		if _, exists := categories[category]; !exists {
			categories[category] = &CategoryInfo{
				Name:        category,
				Description: getCategoryDescription(category),
				AppCount:    0,
				Platforms:   []string{},
			}
		}

		categories[category].AppCount++
		// Add supported platforms
		platforms := getSupportedPlatforms(app)
		for _, platform := range platforms {
			if !contains(categories[category].Platforms, platform) {
				categories[category].Platforms = append(categories[category].Platforms, platform)
			}
		}
	}

	// Convert map to slice
	result := make([]CategoryInfo, 0, len(categories))
	for _, cat := range categories {
		result = append(result, *cat)
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// Helper functions
func getSupportedPlatforms(app types.CrossPlatformApp) []string {
	var platforms []string
	if app.Linux.InstallMethod != "" {
		platforms = append(platforms, "linux")
	}
	if app.MacOS.InstallMethod != "" {
		platforms = append(platforms, "macos")
	}
	if app.Windows.InstallMethod != "" {
		platforms = append(platforms, "windows")
	}
	if app.AllPlatforms.InstallMethod != "" {
		platforms = []string{"linux", "macos", "windows"}
	}
	return platforms
}

func getCategoryDescription(category string) string {
	descriptions := map[string]string{
		"development":           "Core development tools and IDEs",
		"databases":             "Database systems and management tools",
		"system_tools":          "System utilities and command-line tools",
		"optional":              "Optional productivity and entertainment applications",
		"programming_languages": "Programming language runtimes and tools",
		"shell":                 "Shell configurations and enhancements",
		"Other":                 "Miscellaneous applications",
	}
	if desc, exists := descriptions[category]; exists {
		return desc
	}
	return "Various applications"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Filter functions
func filterInstalledApps(apps []InstalledApp, options ListCommandOptions) []InstalledApp {
	filtered := make([]InstalledApp, 0, len(apps))

	for _, app := range apps {
		// Category filter
		if options.Category != "" && !strings.EqualFold(app.Category, options.Category) {
			continue
		}

		// Search filter
		if options.Search != "" {
			search := strings.ToLower(options.Search)
			if !strings.Contains(strings.ToLower(app.Name), search) &&
				!strings.Contains(strings.ToLower(app.Description), search) {
				continue
			}
		}

		// Method filter
		if options.Method != "" && !strings.EqualFold(app.InstallMethod, options.Method) {
			continue
		}

		filtered = append(filtered, app)
	}

	// Sort by name
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	return filtered
}

func filterAvailableApps(apps []AvailableApp, options ListCommandOptions) []AvailableApp {
	filtered := make([]AvailableApp, 0, len(apps))

	for _, app := range apps {
		// Category filter
		if options.Category != "" && !strings.EqualFold(app.Category, options.Category) {
			continue
		}

		// Search filter
		if options.Search != "" {
			search := strings.ToLower(options.Search)
			if !strings.Contains(strings.ToLower(app.Name), search) &&
				!strings.Contains(strings.ToLower(app.Description), search) {
				continue
			}
		}

		// Method filter
		if options.Method != "" {
			hasMethod := false
			for _, method := range app.InstallMethods {
				if strings.EqualFold(method, options.Method) {
					hasMethod = true
					break
				}
			}
			if !hasMethod {
				continue
			}
		}

		// Recommended filter
		if options.Recommended && !app.Recommended {
			continue
		}

		filtered = append(filtered, app)
	}

	// Sort by name
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	return filtered
}

// Output functions for installed apps
func outputInstalledJSON(apps []InstalledApp) {
	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputInstalledYAML(apps []InstalledApp) {
	data, err := yaml.Marshal(apps)
	if err != nil {
		fmt.Printf("Error formatting YAML: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputInstalledTable(apps []InstalledApp, options ListCommandOptions) {
	fmt.Println("✅ Installed Applications")
	fmt.Println("-" + strings.Repeat("-", 23))

	if len(apps) == 0 {
		fmt.Println("No applications currently installed via DevEx")
		fmt.Println()
		fmt.Println("💡 Tip: Run 'devex install' or 'devex setup' to install applications")
		return
	}

	if options.Verbose {
		// Detailed table output
		fmt.Printf("┌─────────────────┬─────────────────────────────────┬─────────────┬─────────────┐\n")
		fmt.Printf("│ %-15s │ %-31s │ %-11s │ %-11s │\n", "Application", "Description", "Category", "Method")
		fmt.Printf("├─────────────────┼─────────────────────────────────┼─────────────┼─────────────┤\n")

		for _, app := range apps {
			fmt.Printf("│ %-15s │ %-31s │ %-11s │ %-11s │\n",
				truncateString(app.Name, 15),
				truncateString(app.Description, 31),
				truncateString(app.Category, 11),
				truncateString(app.InstallMethod, 11))
		}
		fmt.Printf("└─────────────────┴─────────────────────────────────┴─────────────┴─────────────┘\n")
	} else {
		// Simple list output
		for _, app := range apps {
			fmt.Printf("  • %s - %s\n", app.Name, app.Description)
		}
	}

	fmt.Printf("\nTotal installed: %d applications\n", len(apps))
}

// Output functions for available apps
func outputAvailableJSON(apps []AvailableApp) {
	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputAvailableYAML(apps []AvailableApp) {
	data, err := yaml.Marshal(apps)
	if err != nil {
		fmt.Printf("Error formatting YAML: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputAvailableTable(apps []AvailableApp, options ListCommandOptions) {
	fmt.Println("📋 Available Applications")
	fmt.Println("-" + strings.Repeat("-", 23))

	if len(apps) == 0 {
		fmt.Println("No applications available for your platform")
		return
	}

	if options.Verbose {
		// Detailed table output
		fmt.Printf("┌─────────────────┬───────────────────────────────────┬─────────────────┬─────────────┐\n")
		fmt.Printf("│ %-15s │ %-35s │ %-15s │ %-11s │\n", "Application", "Description", "Install Methods", "Category")
		fmt.Printf("├─────────────────┼───────────────────────────────────┼─────────────────┼─────────────┤\n")

		for _, app := range apps {
			methods := strings.Join(app.InstallMethods, ", ")
			recommendedMarker := ""
			if app.Recommended {
				recommendedMarker = " ⭐"
			}
			fmt.Printf("│ %-15s │ %-35s │ %-15s │ %-11s │\n",
				truncateString(app.Name+recommendedMarker, 15),
				truncateString(app.Description, 35),
				truncateString(methods, 15),
				truncateString(app.Category, 11))
		}
		fmt.Printf("└─────────────────┴───────────────────────────────────┴─────────────────┴─────────────┘\n")
	} else {
		// Group by category
		categories := make(map[string][]AvailableApp)
		for _, app := range apps {
			category := app.Category
			if category == "" {
				category = "Other"
			}
			categories[category] = append(categories[category], app)
		}

		// Sort categories
		sortedCategories := make([]string, 0, len(categories))
		for category := range categories {
			sortedCategories = append(sortedCategories, category)
		}
		sort.Strings(sortedCategories)

		recommendedCount := 0
		for _, category := range sortedCategories {
			categoryApps := categories[category]
			if len(categoryApps) == 0 {
				continue
			}

			fmt.Printf("\n🏷️  %s:\n", category)

			for _, app := range categoryApps {
				recommendedMarker := ""
				if app.Recommended {
					recommendedMarker = " (recommended)"
					recommendedCount++
				}
				fmt.Printf("  • %s - %s%s\n", app.Name, app.Description, recommendedMarker)
			}
		}

		fmt.Printf("\nTotal available: %d applications (%d recommended)\n", len(apps), recommendedCount)
	}

	fmt.Println()
	fmt.Println("💡 Tip: Use 'devex install --app <name>' to install applications")
}

// Output functions for categories
func outputCategoriesJSON(categories []CategoryInfo) {
	data, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputCategoriesYAML(categories []CategoryInfo) {
	data, err := yaml.Marshal(categories)
	if err != nil {
		fmt.Printf("Error formatting YAML: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputCategoriesTable(categories []CategoryInfo, options ListCommandOptions) {
	fmt.Println("📂 Available Categories")
	fmt.Println("-" + strings.Repeat("-", 21))

	if len(categories) == 0 {
		fmt.Println("No categories available")
		return
	}

	if options.Verbose {
		// Detailed table output
		fmt.Printf("┌───────────────────────┬───────────────────────────────────┬───────┬─────────────────┐\n")
		fmt.Printf("│ %-19s │ %-35s │ %-5s │ %-15s │\n", "Category", "Description", "Apps", "Platforms")
		fmt.Printf("├───────────────────────┼───────────────────────────────────┼───────┼─────────────────┤\n")

		for _, cat := range categories {
			platforms := strings.Join(cat.Platforms, ", ")
			fmt.Printf("│ %-19s │ %-35s │ %-5d │ %-15s │\n",
				truncateString(cat.Name, 19),
				truncateString(cat.Description, 35),
				cat.AppCount,
				truncateString(platforms, 15))
		}
		fmt.Printf("└───────────────────────┴───────────────────────────────────┴───────┴─────────────────┘\n")
	} else {
		// Simple list output
		for _, cat := range categories {
			fmt.Printf("  • %s - %s (%d apps)\n", cat.Name, cat.Description, cat.AppCount)
		}
	}

	fmt.Printf("\nTotal categories: %d\n", len(categories))
	fmt.Println()
	fmt.Println("💡 Tip: Use 'devex list available --category <name>' to view apps in a specific category")
}

// Utility function
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
