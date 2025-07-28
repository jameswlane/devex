package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/spf13/cobra"
)

func init() {
	Register(NewListCmd)
}

// NewListCmd creates the list command
func NewListCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [installed|available]",
		Short: "List installed or available applications",
		Long: `List applications in your DevEx configuration.

Available subcommands:
  installed  - Show applications that are currently installed
  available  - Show all applications available for installation

Examples:
  devex list installed    # Show installed apps
  devex list available    # Show all available apps
  devex list             # Show both installed and available`,
		ValidArgs: []string{"installed", "available"},
		Run: func(cmd *cobra.Command, args []string) {
			runListCommand(cmd, args, repo, settings)
		},
	}

	return cmd
}

func runListCommand(cmd *cobra.Command, args []string, repo types.Repository, settings config.CrossPlatformSettings) {
	if len(args) == 0 {
		// Show both installed and available
		fmt.Println("📦 DevEx Application Status")
		fmt.Println("=" + strings.Repeat("=", 25))
		fmt.Println()

		showInstalled(repo, settings)
		fmt.Println()
		showAvailable(settings)
		return
	}

	switch args[0] {
	case "installed":
		showInstalled(repo, settings)
	case "available":
		showAvailable(settings)
	default:
		fmt.Printf("Error: Unknown subcommand '%s'\n", args[0])
		fmt.Println("Available subcommands: installed, available")
		os.Exit(1)
	}
}

func showInstalled(repo types.Repository, settings config.CrossPlatformSettings) {
	log.Info("Listing installed applications")

	fmt.Println("✅ Installed Applications")
	fmt.Println("-" + strings.Repeat("-", 23))

	if repo == nil {
		fmt.Println("❌ Database not available - cannot check installation status")
		return
	}

	// Get all available apps
	allApps := settings.GetAllApps()
	if len(allApps) == 0 {
		fmt.Println("No applications configured")
		return
	}

	installedCount := 0
	var installedApps []string

	// Check each app's installation status
	for _, app := range allApps {
		// Try to get installation record from database
		if installedApp, err := repo.GetApp(app.Name); err == nil && installedApp != nil {
			installedApps = append(installedApps, fmt.Sprintf("  • %s - %s", app.Name, app.Description))
			installedCount++
		}
	}

	if installedCount == 0 {
		fmt.Println("No applications currently installed via DevEx")
		fmt.Println()
		fmt.Println("💡 Tip: Run 'devex install' or 'devex setup' to install applications")
	} else {
		sort.Strings(installedApps)
		for _, app := range installedApps {
			fmt.Println(app)
		}
		fmt.Printf("\nTotal installed: %d applications\n", installedCount)
	}
}

func showAvailable(settings config.CrossPlatformSettings) {
	log.Info("Listing available applications")

	fmt.Println("📋 Available Applications")
	fmt.Println("-" + strings.Repeat("-", 23))

	allApps := settings.GetAllApps()
	if len(allApps) == 0 {
		fmt.Println("No applications configured")
		return
	}

	// Group apps by category
	categories := make(map[string][]types.CrossPlatformApp)
	for _, app := range allApps {
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

	totalCount := 0
	defaultCount := 0

	for _, category := range sortedCategories {
		apps := categories[category]
		if len(apps) == 0 {
			continue
		}

		fmt.Printf("\n🏷️  %s:\n", category)

		// Sort apps within category
		sort.Slice(apps, func(i, j int) bool {
			return apps[i].Name < apps[j].Name
		})

		for _, app := range apps {
			defaultMarker := ""
			if app.Default {
				defaultMarker = " (default)"
				defaultCount++
			}
			fmt.Printf("  • %s - %s%s\n", app.Name, app.Description, defaultMarker)
			totalCount++
		}
	}

	fmt.Printf("\nTotal available: %d applications (%d marked as default)\n", totalCount, defaultCount)
	fmt.Println()
	fmt.Println("💡 Tip: Use 'devex install' to install default apps or 'devex setup' for guided installation")
}
