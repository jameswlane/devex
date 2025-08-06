package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// outputInstalledJSON outputs installed apps in JSON format
func outputInstalledJSON(apps []InstalledApp, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(apps); err != nil {
		return fmt.Errorf("failed to serialize %d installed apps to JSON: %w", len(apps), err)
	}
	return nil
}

// outputInstalledYAML outputs installed apps in YAML format
func outputInstalledYAML(apps []InstalledApp, writer io.Writer) error {
	encoder := yaml.NewEncoder(writer)
	encoder.SetIndent(2)
	defer encoder.Close()
	if err := encoder.Encode(apps); err != nil {
		return fmt.Errorf("failed to serialize %d installed apps to YAML: %w", len(apps), err)
	}
	return nil
}

// outputInstalledTable outputs installed apps in table format
func outputInstalledTable(apps []InstalledApp, options ListCommandOptions, writer io.Writer) error {
	if len(apps) == 0 {
		fmt.Fprintln(writer, "No installed applications found.")
		return nil
	}

	config := NewInstalledAppTableConfig(apps)

	if options.Verbose {
		return renderInstalledAppsTable(apps, config, writer)
	} else {
		return renderInstalledAppsList(apps, writer)
	}
}

// outputAvailableJSON outputs available apps in JSON format
func outputAvailableJSON(apps []AvailableApp, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(apps); err != nil {
		return fmt.Errorf("failed to serialize %d available apps to JSON: %w", len(apps), err)
	}
	return nil
}

// outputAvailableYAML outputs available apps in YAML format
func outputAvailableYAML(apps []AvailableApp, writer io.Writer) error {
	encoder := yaml.NewEncoder(writer)
	encoder.SetIndent(2)
	defer encoder.Close()
	if err := encoder.Encode(apps); err != nil {
		return fmt.Errorf("failed to serialize %d available apps to YAML: %w", len(apps), err)
	}
	return nil
}

// outputAvailableTable outputs available apps in table format
func outputAvailableTable(apps []AvailableApp, options ListCommandOptions, writer io.Writer) error {
	if len(apps) == 0 {
		fmt.Fprintln(writer, "No applications found matching the specified criteria.")
		return nil
	}

	config := NewAvailableAppTableConfig(apps)

	if options.Verbose {
		return renderAvailableAppsTable(apps, config, writer)
	} else {
		return renderAvailableAppsList(apps, writer)
	}
}

// outputCategoriesJSON outputs categories in JSON format
func outputCategoriesJSON(categories []CategoryInfo, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(categories); err != nil {
		return fmt.Errorf("failed to serialize %d categories to JSON: %w", len(categories), err)
	}
	return nil
}

// outputCategoriesYAML outputs categories in YAML format
func outputCategoriesYAML(categories []CategoryInfo, writer io.Writer) error {
	encoder := yaml.NewEncoder(writer)
	encoder.SetIndent(2)
	defer encoder.Close()
	if err := encoder.Encode(categories); err != nil {
		return fmt.Errorf("failed to serialize %d categories to YAML: %w", len(categories), err)
	}
	return nil
}

// outputCategoriesTable outputs categories in table format
func outputCategoriesTable(categories []CategoryInfo, options ListCommandOptions, writer io.Writer) error {
	if len(categories) == 0 {
		fmt.Fprintln(writer, "No categories found.")
		return nil
	}

	// Pre-compute repeated strings for performance
	nameSep := strings.Repeat(TableHorizontal, 20)
	descSep := strings.Repeat(TableHorizontal, 50)
	countSep := strings.Repeat(TableHorizontal, 8)
	platformSep := strings.Repeat(TableHorizontal, 25)

	// Header
	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s\n",
		TableTopLeft, nameSep, TableTeeDown, descSep, TableTeeDown, countSep, TableTeeDown, platformSep, TableTopRight)

	fmt.Fprintf(writer, "%s %-18s %s %-48s %s %-6s %s %-23s %s\n",
		TableVertical, "Category", TableVertical, "Description", TableVertical, "Count", TableVertical, "Platforms", TableVertical)

	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s\n",
		TableTeeRight, nameSep, TableCross, descSep, TableCross, countSep, TableCross, platformSep, TableTeeLeft)

	// Data rows
	for i, category := range categories {
		// Truncate fields to fit column widths
		name := truncateString(category.Category, 18)
		desc := truncateString(category.Description, 48)
		platforms := truncateString(strings.Join(category.Platforms, ", "), 23)

		fmt.Fprintf(writer, "%s %-18s %s %-48s %s %-6d %s %-23s %s\n",
			TableVertical, name, TableVertical, desc, TableVertical, category.Count, TableVertical, platforms, TableVertical)

		// Add separator between rows (except for last row)
		if i < len(categories)-1 {
			fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s\n",
				TableTeeRight, nameSep, TableCross, descSep, TableCross, countSep, TableCross, platformSep, TableTeeLeft)
		}
	}

	// Footer
	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s\n",
		TableBottomLeft, nameSep, TableTeeUp, descSep, TableTeeUp, countSep, TableTeeUp, platformSep, TableBottomRight)

	return nil
}

// truncateString truncates a string to a maximum length, adding "..." if needed
func truncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// renderInstalledAppsTable renders installed apps in a detailed table format
func renderInstalledAppsTable(apps []InstalledApp, config *TableConfig, writer io.Writer) error {
	// Pre-compute repeated strings for performance
	nameSep := strings.Repeat(TableHorizontal, config.NameWidth+2)
	descSep := strings.Repeat(TableHorizontal, config.DescriptionWidth+2)
	catSep := strings.Repeat(TableHorizontal, config.CategoryWidth+2)
	methodSep := strings.Repeat(TableHorizontal, config.MethodWidth+2)
	statusSep := strings.Repeat(TableHorizontal, config.StatusWidth+2)

	// Header
	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s%s%s\n",
		TableTopLeft, nameSep, TableTeeDown, descSep, TableTeeDown, catSep, TableTeeDown, methodSep, TableTeeDown, statusSep, TableTopRight)

	fmt.Fprintf(writer, "%s %-*s %s %-*s %s %-*s %s %-*s %s %-*s %s\n",
		TableVertical, config.NameWidth, "Name",
		TableVertical, config.DescriptionWidth, "Description",
		TableVertical, config.CategoryWidth, "Category",
		TableVertical, config.MethodWidth, "Method",
		TableVertical, config.StatusWidth, "Status", TableVertical)

	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s%s%s\n",
		TableTeeRight, nameSep, TableCross, descSep, TableCross, catSep, TableCross, methodSep, TableCross, statusSep, TableTeeLeft)

	// Data rows
	for i, app := range apps {
		// Truncate fields to fit column widths
		name := truncateString(app.Name, config.NameWidth)
		desc := truncateString(app.Description, config.DescriptionWidth)
		category := truncateString(app.Category, config.CategoryWidth)
		method := truncateString(app.Method, config.MethodWidth)
		status := truncateString(app.Status, config.StatusWidth)

		fmt.Fprintf(writer, "%s %-*s %s %-*s %s %-*s %s %-*s %s %-*s %s\n",
			TableVertical, config.NameWidth, name,
			TableVertical, config.DescriptionWidth, desc,
			TableVertical, config.CategoryWidth, category,
			TableVertical, config.MethodWidth, method,
			TableVertical, config.StatusWidth, status, TableVertical)

		// Add separator between rows (except for last row)
		if i < len(apps)-1 {
			fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s%s%s\n",
				TableTeeRight, nameSep, TableCross, descSep, TableCross, catSep, TableCross, methodSep, TableCross, statusSep, TableTeeLeft)
		}
	}

	// Footer
	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s%s%s\n",
		TableBottomLeft, nameSep, TableTeeUp, descSep, TableTeeUp, catSep, TableTeeUp, methodSep, TableTeeUp, statusSep, TableBottomRight)

	return nil
}

// renderInstalledAppsList renders installed apps in a simple list format
func renderInstalledAppsList(apps []InstalledApp, writer io.Writer) error {
	fmt.Fprintf(writer, "Installed Applications (%d):\n\n", len(apps))

	for _, app := range apps {
		fmt.Fprintf(writer, "• %s (%s) - %s\n", app.Name, app.Method, app.Description)
	}

	return nil
}

// renderAvailableAppsTable renders available apps in a detailed table format
func renderAvailableAppsTable(apps []AvailableApp, config *TableConfig, writer io.Writer) error {
	// Pre-compute repeated strings for performance
	nameSep := strings.Repeat(TableHorizontal, config.NameWidth+2)
	descSep := strings.Repeat(TableHorizontal, config.DescriptionWidth+2)
	catSep := strings.Repeat(TableHorizontal, config.CategoryWidth+2)
	methodSep := strings.Repeat(TableHorizontal, config.MethodWidth+2)
	platformSep := strings.Repeat(TableHorizontal, config.StatusWidth+2) // Reusing StatusWidth for Platform

	// Header
	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s%s%s\n",
		TableTopLeft, nameSep, TableTeeDown, descSep, TableTeeDown, catSep, TableTeeDown, methodSep, TableTeeDown, platformSep, TableTopRight)

	fmt.Fprintf(writer, "%s %-*s %s %-*s %s %-*s %s %-*s %s %-*s %s\n",
		TableVertical, config.NameWidth, "Name",
		TableVertical, config.DescriptionWidth, "Description",
		TableVertical, config.CategoryWidth, "Category",
		TableVertical, config.MethodWidth, "Method",
		TableVertical, config.StatusWidth, "Platform", TableVertical)

	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s%s%s\n",
		TableTeeRight, nameSep, TableCross, descSep, TableCross, catSep, TableCross, methodSep, TableCross, platformSep, TableTeeLeft)

	// Data rows
	for i, app := range apps {
		// Truncate fields to fit column widths
		name := truncateString(app.Name, config.NameWidth)
		desc := truncateString(app.Description, config.DescriptionWidth)
		category := truncateString(app.Category, config.CategoryWidth)
		method := truncateString(app.Method, config.MethodWidth)
		platform := truncateString(app.Platform, config.StatusWidth)

		// Add recommended indicator
		if app.Recommended {
			name = "⭐ " + name
		}

		fmt.Fprintf(writer, "%s %-*s %s %-*s %s %-*s %s %-*s %s %-*s %s\n",
			TableVertical, config.NameWidth, name,
			TableVertical, config.DescriptionWidth, desc,
			TableVertical, config.CategoryWidth, category,
			TableVertical, config.MethodWidth, method,
			TableVertical, config.StatusWidth, platform, TableVertical)

		// Add separator between rows (except for last row)
		if i < len(apps)-1 {
			fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s%s%s\n",
				TableTeeRight, nameSep, TableCross, descSep, TableCross, catSep, TableCross, methodSep, TableCross, platformSep, TableTeeLeft)
		}
	}

	// Footer
	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s%s%s%s\n",
		TableBottomLeft, nameSep, TableTeeUp, descSep, TableTeeUp, catSep, TableTeeUp, methodSep, TableTeeUp, platformSep, TableBottomRight)

	return nil
}

// renderAvailableAppsList renders available apps in a simple list format
func renderAvailableAppsList(apps []AvailableApp, writer io.Writer) error {
	// Group by category for better organization
	categories := groupAppsByCategory(apps)
	sortedCategories := getSortedCategories(categories)

	totalApps, totalRecommended, err := renderCategorizedApps(categories, sortedCategories, writer)
	if err != nil {
		return err
	}

	fmt.Fprintf(writer, "\nTotal: %d applications (%d recommended)\n", totalApps, totalRecommended)
	return nil
}

// renderCategorizedApps renders apps grouped by category
func renderCategorizedApps(categories map[string][]AvailableApp, sortedCategories []string, writer io.Writer) (int, int, error) {
	totalApps := 0
	totalRecommended := 0

	for _, category := range sortedCategories {
		apps := categories[category]
		recommendedCount := 0

		for _, app := range apps {
			if app.Recommended {
				recommendedCount++
			}
		}

		fmt.Fprintf(writer, "\n%s (%d apps", category, len(apps))
		if recommendedCount > 0 {
			fmt.Fprintf(writer, ", %d recommended", recommendedCount)
		}
		fmt.Fprintf(writer, "):\n")

		for _, app := range apps {
			prefix := "  • "
			if app.Recommended {
				prefix = "  ⭐ "
			}
			fmt.Fprintf(writer, "%s%s - %s\n", prefix, app.Name, app.Description)
		}

		totalApps += len(apps)
		totalRecommended += recommendedCount
	}

	return totalApps, totalRecommended, nil
}
