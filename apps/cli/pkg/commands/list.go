package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Table formatting constants and configuration
const (
	// Status icons
	InstalledIcon = "✅"
	AvailableIcon = "📦"

	// Default category for uncategorized apps
	DefaultCategory = "Other"

	// Default column widths (fallback values)
	defaultMinNameWidth        = 12
	defaultMinDescriptionWidth = 30
	defaultMinCategoryWidth    = 10
	defaultMinMethodWidth      = 8
	defaultMinStatusWidth      = 6

	// Maximum column widths to prevent overly wide tables
	maxNameWidth        = 25
	maxDescriptionWidth = 50
	maxCategoryWidth    = 20
	maxMethodWidth      = 20
)

// TableConfig represents table formatting configuration with dynamic sizing
type TableConfig struct {
	NameWidth        int
	DescriptionWidth int
	CategoryWidth    int
	MethodWidth      int
	StatusWidth      int
}

// NewInstalledAppTableConfig creates a table configuration optimized for installed apps
func NewInstalledAppTableConfig(apps []InstalledApp) *TableConfig {
	tc := &TableConfig{
		NameWidth:        defaultMinNameWidth,
		DescriptionWidth: defaultMinDescriptionWidth,
		CategoryWidth:    defaultMinCategoryWidth,
		MethodWidth:      defaultMinMethodWidth,
		StatusWidth:      defaultMinStatusWidth,
	}

	tc.calculateInstalledAppWidths(apps)
	return tc
}

// NewAvailableAppTableConfig creates a table configuration optimized for available apps
func NewAvailableAppTableConfig(apps []AvailableApp) *TableConfig {
	tc := &TableConfig{
		NameWidth:        defaultMinNameWidth,
		DescriptionWidth: defaultMinDescriptionWidth,
		CategoryWidth:    defaultMinCategoryWidth,
		MethodWidth:      defaultMinMethodWidth,
		StatusWidth:      defaultMinStatusWidth,
	}

	tc.calculateAvailableAppWidths(apps)
	return tc
}

// calculateInstalledAppWidths dynamically calculates optimal column widths based on actual data
func (tc *TableConfig) calculateInstalledAppWidths(apps []InstalledApp) {
	if len(apps) == 0 {
		return
	}

	for _, app := range apps {
		// Calculate optimal width for each column based on content
		nameLen := len(app.Name)
		if nameLen > tc.NameWidth && nameLen <= maxNameWidth {
			tc.NameWidth = nameLen
		}

		descLen := len(app.Description)
		if descLen > tc.DescriptionWidth && descLen <= maxDescriptionWidth {
			tc.DescriptionWidth = descLen
		}

		catLen := len(app.Category)
		if catLen > tc.CategoryWidth && catLen <= maxCategoryWidth {
			tc.CategoryWidth = catLen
		}

		methodLen := len(app.InstallMethod)
		if methodLen > tc.MethodWidth && methodLen <= maxMethodWidth {
			tc.MethodWidth = methodLen
		}
	}
}

// calculateAvailableAppWidths dynamically calculates optimal column widths for available apps
func (tc *TableConfig) calculateAvailableAppWidths(apps []AvailableApp) {
	if len(apps) == 0 {
		return
	}

	for _, app := range apps {
		// Account for recommended marker in name width
		nameLen := len(app.Name)
		if app.Recommended {
			nameLen += 2 // " ⭐"
		}
		if nameLen > tc.NameWidth && nameLen <= maxNameWidth {
			tc.NameWidth = nameLen
		}

		descLen := len(app.Description)
		if descLen > tc.DescriptionWidth && descLen <= maxDescriptionWidth {
			tc.DescriptionWidth = descLen
		}

		catLen := len(app.Category)
		if catLen > tc.CategoryWidth && catLen <= maxCategoryWidth {
			tc.CategoryWidth = catLen
		}

		// Calculate method width based on joined methods
		methodsStr := strings.Join(app.InstallMethods, ", ")
		methodLen := len(methodsStr)
		if methodLen > tc.MethodWidth && methodLen <= maxMethodWidth {
			tc.MethodWidth = methodLen
		}
	}
}

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

	if err := executeListCommand(cmd, args, repo, settings, options); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		os.Exit(1)
	}
}

func executeListCommand(cmd *cobra.Command, args []string, repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) error {
	if len(args) == 0 {
		// Show both installed and available
		fmt.Fprintln(cmd.OutOrStdout(), "📦 DevEx Application Status")
		fmt.Fprintln(cmd.OutOrStdout(), "="+strings.Repeat("=", 25))
		fmt.Fprintln(cmd.OutOrStdout())

		if err := showInstalledApps(repo, settings, options, cmd.OutOrStdout()); err != nil {
			return fmt.Errorf("failed to show installed apps: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		if err := showAvailableApps(repo, settings, options, cmd.OutOrStdout()); err != nil {
			return fmt.Errorf("failed to show available apps: %w", err)
		}
		return nil
	}

	validArgs := []string{"installed", "available", "categories"}
	switch args[0] {
	case "installed":
		return showInstalledApps(repo, settings, options, cmd.OutOrStdout())
	case "available":
		return showAvailableApps(repo, settings, options, cmd.OutOrStdout())
	case "categories":
		return showCategories(settings, options, cmd.OutOrStdout())
	default:
		return fmt.Errorf("unknown subcommand '%s': available options are: %s",
			args[0], strings.Join(validArgs, ", "))
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

func showInstalledApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions, writer io.Writer) error {
	log.Info("Listing installed applications")

	if repo == nil {
		fmt.Fprintln(writer, "❌ Database not available - cannot check installation status")
		return nil
	}

	// Get installed applications from database
	installedApps, err := getInstalledApps(repo, settings, options)
	if err != nil {
		fmt.Fprintf(writer, "❌ Error retrieving installed applications: %v\n", err)
		return err
	}

	// Apply filters
	installedApps = filterInstalledApps(installedApps, options)

	// Output based on format
	switch options.Format {
	case "json":
		return outputInstalledJSON(installedApps, writer)
	case "yaml":
		return outputInstalledYAML(installedApps, writer)
	default:
		return outputInstalledTable(installedApps, options, writer)
	}
}

func showAvailableApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions, writer io.Writer) error {
	log.Info("Listing available applications")

	// Get available applications
	availableApps := getAvailableApps(repo, settings, options)

	// Apply filters
	availableApps = filterAvailableApps(availableApps, options)

	// Output based on format
	switch options.Format {
	case "json":
		return outputAvailableJSON(availableApps, writer)
	case "yaml":
		return outputAvailableYAML(availableApps, writer)
	default:
		return outputAvailableTable(availableApps, options, writer)
	}
}

func showCategories(settings config.CrossPlatformSettings, options ListCommandOptions, writer io.Writer) error {
	log.Info("Listing categories")

	// Get category information
	categories := getCategoryInfo(settings)

	// Output based on format
	switch options.Format {
	case "json":
		return outputCategoriesJSON(categories, writer)
	case "yaml":
		return outputCategoriesYAML(categories, writer)
	default:
		return outputCategoriesTable(categories, options, writer)
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
				// Note: Version tracking is not implemented yet as the database schema
				// doesn't store version information. This would require:
				// 1. Database migration to add version column
				// 2. Version detection logic for different package managers
				// 3. Integration with installation tracking
				Version:      "",
				Dependencies: configApp.GetOSConfig().Dependencies,
			}
			installedApps = append(installedApps, installedApp)
		}
	}

	return installedApps, nil
}

// getAvailableApps retrieves all available applications with installation status
// Optimized to pre-filter supported apps and cache database calls for better performance
func getAvailableApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) []AvailableApp {
	allApps := settings.GetAllApps()

	// Pre-filter supported apps to avoid processing unsupported ones
	supportedApps := make([]types.CrossPlatformApp, 0, len(allApps))
	for _, app := range allApps {
		if app.IsSupported() {
			supportedApps = append(supportedApps, app)
		}
	}

	// Pre-allocate with exact capacity for better memory efficiency
	availableApps := make([]AvailableApp, 0, len(supportedApps))

	// Cache installed apps lookup once to avoid repeated database calls
	installedApps := getInstalledAppsCache(repo)

	// Process only supported apps
	for _, app := range supportedApps {
		osConfig := app.GetOSConfig()

		// Pre-allocate install methods slice with estimated capacity
		installMethods := make([]string, 0, 1+len(osConfig.Alternatives))

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
			Installed:      installedApps[app.Name],
		}
		availableApps = append(availableApps, availableApp)
	}

	return availableApps
}

// getInstalledAppsCache creates a map of installed app names for quick lookup
// This avoids repeated database calls when checking installation status
func getInstalledAppsCache(repo types.Repository) map[string]bool {
	installedApps := make(map[string]bool)

	if repo == nil {
		return installedApps
	}

	dbApps, err := repo.ListApps()
	if err != nil {
		return installedApps
	}

	for _, app := range dbApps {
		installedApps[app.Name] = true
	}

	return installedApps
}

// getCategoryInfo retrieves category information
// Optimized to use a map for platform deduplication for better performance
func getCategoryInfo(settings config.CrossPlatformSettings) []CategoryInfo {
	categories := make(map[string]*CategoryInfo)

	allApps := settings.GetAllApps()
	for _, app := range allApps {
		category := app.Category
		if category == "" {
			category = DefaultCategory
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
		// Add supported platforms using map[string]struct{} for O(1) deduplication
		platforms := getSupportedPlatforms(app)
		platformSet := make(map[string]struct{})

		// Add existing platforms to set
		for _, platform := range categories[category].Platforms {
			platformSet[platform] = struct{}{}
		}

		// Add new platforms to set
		for _, platform := range platforms {
			platformSet[platform] = struct{}{}
		}

		// Rebuild platforms slice from set
		categories[category].Platforms = make([]string, 0, len(platformSet))
		for platform := range platformSet {
			categories[category].Platforms = append(categories[category].Platforms, platform)
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
func outputInstalledJSON(apps []InstalledApp, writer io.Writer) error {
	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		return fmt.Errorf("error formatting JSON: %w", err)
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func outputInstalledYAML(apps []InstalledApp, writer io.Writer) error {
	data, err := yaml.Marshal(apps)
	if err != nil {
		return fmt.Errorf("error formatting YAML: %w", err)
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func outputInstalledTable(apps []InstalledApp, options ListCommandOptions, writer io.Writer) error {
	fmt.Fprintln(writer, "✅ Installed Applications")
	fmt.Fprintln(writer, "-"+strings.Repeat("-", 23))

	if len(apps) == 0 {
		fmt.Fprintln(writer, "No applications currently installed via DevEx")
		fmt.Fprintln(writer)
		fmt.Fprintln(writer, "💡 Tip: Run 'devex install' or 'devex setup' to install applications")
		return nil
	}

	config := NewInstalledAppTableConfig(apps)
	if options.Verbose {
		if err := renderInstalledAppsTable(apps, config, writer); err != nil {
			return err
		}
	} else {
		if err := renderInstalledAppsList(apps, writer); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(writer, "\nTotal installed: %d applications\n", len(apps))
	return err
}

// Output functions for available apps
func outputAvailableJSON(apps []AvailableApp, writer io.Writer) error {
	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		return fmt.Errorf("error formatting JSON: %w", err)
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func outputAvailableYAML(apps []AvailableApp, writer io.Writer) error {
	data, err := yaml.Marshal(apps)
	if err != nil {
		return fmt.Errorf("error formatting YAML: %w", err)
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func outputAvailableTable(apps []AvailableApp, options ListCommandOptions, writer io.Writer) error {
	fmt.Fprintln(writer, "📋 Available Applications")
	fmt.Fprintln(writer, "-"+strings.Repeat("-", 23))

	if len(apps) == 0 {
		fmt.Fprintln(writer, "No applications available for your platform")
		return nil
	}

	config := NewAvailableAppTableConfig(apps)
	if options.Verbose {
		if err := renderAvailableAppsTable(apps, config, writer); err != nil {
			return err
		}
	} else {
		if err := renderAvailableAppsList(apps, writer); err != nil {
			return err
		}
	}

	fmt.Fprintln(writer)
	_, err := fmt.Fprintln(writer, "💡 Tip: Use 'devex install --app <name>' to install applications")
	return err
}

// Output functions for categories
func outputCategoriesJSON(categories []CategoryInfo, writer io.Writer) error {
	data, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		return fmt.Errorf("error formatting JSON: %w", err)
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func outputCategoriesYAML(categories []CategoryInfo, writer io.Writer) error {
	data, err := yaml.Marshal(categories)
	if err != nil {
		return fmt.Errorf("error formatting YAML: %w", err)
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func outputCategoriesTable(categories []CategoryInfo, options ListCommandOptions, writer io.Writer) error {
	fmt.Fprintln(writer, "📂 Available Categories")
	fmt.Fprintln(writer, "-"+strings.Repeat("-", 21))

	if len(categories) == 0 {
		fmt.Fprintln(writer, "No categories available")
		return nil
	}

	if options.Verbose {
		// Detailed table output with dynamic widths
		config := &TableConfig{
			NameWidth:        19,
			DescriptionWidth: 35,
			CategoryWidth:    5,  // for "Apps" column
			MethodWidth:      15, // for "Platforms" column
		}

		// Calculate optimal widths
		for _, cat := range categories {
			nameLen := len(cat.Name)
			if nameLen > config.NameWidth && nameLen <= 25 {
				config.NameWidth = nameLen
			}

			descLen := len(cat.Description)
			if descLen > config.DescriptionWidth && descLen <= 40 {
				config.DescriptionWidth = descLen
			}

			platformsStr := strings.Join(cat.Platforms, ", ")
			platformLen := len(platformsStr)
			if platformLen > config.MethodWidth && platformLen <= 20 {
				config.MethodWidth = platformLen
			}
		}

		// Print table header
		fmt.Fprintf(writer, "┌%s┬%s┬───────┬%s┐\n",
			strings.Repeat("─", config.NameWidth+2),
			strings.Repeat("─", config.DescriptionWidth+2),
			strings.Repeat("─", config.MethodWidth+2))

		fmt.Fprintf(writer, "│ %-*s │ %-*s │ %-5s │ %-*s │\n",
			config.NameWidth, "Category",
			config.DescriptionWidth, "Description",
			"Apps",
			config.MethodWidth, "Platforms")

		fmt.Fprintf(writer, "├%s┼%s┼───────┼%s┤\n",
			strings.Repeat("─", config.NameWidth+2),
			strings.Repeat("─", config.DescriptionWidth+2),
			strings.Repeat("─", config.MethodWidth+2))

		// Print table rows
		for _, cat := range categories {
			platforms := strings.Join(cat.Platforms, ", ")
			fmt.Fprintf(writer, "│ %-*s │ %-*s │ %-5d │ %-*s │\n",
				config.NameWidth, truncateString(cat.Name, config.NameWidth),
				config.DescriptionWidth, truncateString(cat.Description, config.DescriptionWidth),
				cat.AppCount,
				config.MethodWidth, truncateString(platforms, config.MethodWidth))
		}

		// Print table footer
		fmt.Fprintf(writer, "└%s┴%s┴───────┴%s┘\n",
			strings.Repeat("─", config.NameWidth+2),
			strings.Repeat("─", config.DescriptionWidth+2),
			strings.Repeat("─", config.MethodWidth+2))
	} else {
		// Simple list output
		for _, cat := range categories {
			fmt.Fprintf(writer, "  • %s - %s (%d apps)\n", cat.Name, cat.Description, cat.AppCount)
		}
	}

	fmt.Fprintf(writer, "\nTotal categories: %d\n", len(categories))
	fmt.Fprintln(writer)
	_, err := fmt.Fprintln(writer, "💡 Tip: Use 'devex list available --category <name>' to view apps in a specific category")
	return err
}

// Utility functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// renderInstalledAppsTable displays installed apps in a detailed table format
func renderInstalledAppsTable(apps []InstalledApp, config *TableConfig, writer io.Writer) error {
	// Create table header format
	headerFormat := fmt.Sprintf("│ %%-%ds │ %%-%ds │ %%-%ds │ %%-%ds │\n",
		config.NameWidth, config.DescriptionWidth, config.CategoryWidth, config.MethodWidth)

	// Print table borders and header
	fmt.Fprintf(writer, "┌%s┬%s┬%s┬%s┐\n",
		strings.Repeat("─", config.NameWidth+2),
		strings.Repeat("─", config.DescriptionWidth+2),
		strings.Repeat("─", config.CategoryWidth+2),
		strings.Repeat("─", config.MethodWidth+2))

	fmt.Fprintf(writer, headerFormat, "Application", "Description", "Category", "Method")

	fmt.Fprintf(writer, "├%s┼%s┼%s┼%s┤\n",
		strings.Repeat("─", config.NameWidth+2),
		strings.Repeat("─", config.DescriptionWidth+2),
		strings.Repeat("─", config.CategoryWidth+2),
		strings.Repeat("─", config.MethodWidth+2))

	// Print table rows
	for _, app := range apps {
		fmt.Fprintf(writer, headerFormat,
			truncateString(app.Name, config.NameWidth),
			truncateString(app.Description, config.DescriptionWidth),
			truncateString(app.Category, config.CategoryWidth),
			truncateString(app.InstallMethod, config.MethodWidth))
	}

	// Print table footer
	_, err := fmt.Fprintf(writer, "└%s┴%s┴%s┴%s┘\n",
		strings.Repeat("─", config.NameWidth+2),
		strings.Repeat("─", config.DescriptionWidth+2),
		strings.Repeat("─", config.CategoryWidth+2),
		strings.Repeat("─", config.MethodWidth+2))

	return err
}

// renderInstalledAppsList displays installed apps in a simple list format
func renderInstalledAppsList(apps []InstalledApp, writer io.Writer) error {
	for _, app := range apps {
		if _, err := fmt.Fprintf(writer, "• %s (%s) - %s\n", app.Name, app.InstallMethod, app.Category); err != nil {
			return err
		}
	}
	return nil
}

// renderAvailableAppsTable displays available apps in a detailed table format with dynamic sizing
func renderAvailableAppsTable(apps []AvailableApp, config *TableConfig, writer io.Writer) error {
	// Create table header format
	headerFormat := fmt.Sprintf("│ %%-%ds │ %%-%ds │ %%-%ds │ %%-%ds │ %%-%ds │\n",
		config.NameWidth, config.DescriptionWidth, config.MethodWidth,
		config.CategoryWidth, config.StatusWidth)

	// Print table borders and header
	fmt.Fprintf(writer, "┌%s┬%s┬%s┬%s┬%s┐\n",
		strings.Repeat("─", config.NameWidth+2),
		strings.Repeat("─", config.DescriptionWidth+2),
		strings.Repeat("─", config.MethodWidth+2),
		strings.Repeat("─", config.CategoryWidth+2),
		strings.Repeat("─", config.StatusWidth+2))

	fmt.Fprintf(writer, headerFormat, "Application", "Description", "Install Methods", "Category", "Status")

	fmt.Fprintf(writer, "├%s┼%s┼%s┼%s┼%s┤\n",
		strings.Repeat("─", config.NameWidth+2),
		strings.Repeat("─", config.DescriptionWidth+2),
		strings.Repeat("─", config.MethodWidth+2),
		strings.Repeat("─", config.CategoryWidth+2),
		strings.Repeat("─", config.StatusWidth+2))

	// Print table rows
	for _, app := range apps {
		methods := strings.Join(app.InstallMethods, ", ")
		recommendedMarker := ""
		if app.Recommended {
			recommendedMarker = " ⭐"
		}
		statusIcon := AvailableIcon
		if app.Installed {
			statusIcon = InstalledIcon
		}

		fmt.Fprintf(writer, headerFormat,
			truncateString(app.Name+recommendedMarker, config.NameWidth),
			truncateString(app.Description, config.DescriptionWidth),
			truncateString(methods, config.MethodWidth),
			truncateString(app.Category, config.CategoryWidth),
			statusIcon)
	}

	// Print table footer
	_, err := fmt.Fprintf(writer, "└%s┴%s┴%s┴%s┴%s┘\n",
		strings.Repeat("─", config.NameWidth+2),
		strings.Repeat("─", config.DescriptionWidth+2),
		strings.Repeat("─", config.MethodWidth+2),
		strings.Repeat("─", config.CategoryWidth+2),
		strings.Repeat("─", config.StatusWidth+2))

	return err
}

// renderAvailableAppsList displays available apps grouped by category
func renderAvailableAppsList(apps []AvailableApp, writer io.Writer) error {
	categories := groupAppsByCategory(apps)
	sortedCategories := getSortedCategories(categories)
	recommendedCount, installedCount, err := renderCategorizedApps(categories, sortedCategories, writer)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(writer, "\nTotal available: %d applications (%d recommended, %d installed)\n",
		len(apps), recommendedCount, installedCount)
	return err
}

// groupAppsByCategory groups apps by their category
func groupAppsByCategory(apps []AvailableApp) map[string][]AvailableApp {
	categories := make(map[string][]AvailableApp)
	for _, app := range apps {
		category := app.Category
		if category == "" {
			category = DefaultCategory
		}
		categories[category] = append(categories[category], app)
	}
	return categories
}

// getSortedCategories returns sorted category names
func getSortedCategories(categories map[string][]AvailableApp) []string {
	sortedCategories := make([]string, 0, len(categories))
	for category := range categories {
		sortedCategories = append(sortedCategories, category)
	}
	sort.Strings(sortedCategories)
	return sortedCategories
}

// renderCategorizedApps renders apps grouped by category and returns counts
// Optimized to calculate counts in a single pass for better performance
func renderCategorizedApps(categories map[string][]AvailableApp, sortedCategories []string, writer io.Writer) (int, int, error) {
	recommendedCount := 0
	installedCount := 0

	for _, category := range sortedCategories {
		categoryApps := categories[category]
		if len(categoryApps) == 0 {
			continue
		}

		_, err := fmt.Fprintf(writer, "\n🏷️  %s:\n", category)
		if err != nil {
			return 0, 0, err
		}

		for _, app := range categoryApps {
			recommendedMarker := ""
			if app.Recommended {
				recommendedMarker = " (recommended)"
				recommendedCount++
			}
			if app.Installed {
				installedCount++
			}

			statusIcon := AvailableIcon
			if app.Installed {
				statusIcon = InstalledIcon
			}

			_, err := fmt.Fprintf(writer, "  %s %s - %s%s\n", statusIcon, app.Name, app.Description, recommendedMarker)
			if err != nil {
				return 0, 0, err
			}
		}
	}

	return recommendedCount, installedCount, nil
}
