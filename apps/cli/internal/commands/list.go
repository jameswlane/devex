package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func init() {
	Register(NewListCmd)
}

// NewListCmd creates the list command with comprehensive subcommand support
func NewListCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "list [installed|available|categories]",
		Short:     "List applications in your DevEx configuration",
		ValidArgs: []string{"installed", "available", "categories"},
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListCommandWithError(cmd, args, repo, settings)
		},
	}

	// Add flags
	cmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml)")
	cmd.Flags().StringP("category", "c", "", "Filter by category")
	cmd.Flags().StringP("search", "s", "", "Search applications by name or description")
	cmd.Flags().String("method", "", "Filter by installation method")
	cmd.Flags().Bool("recommended", false, "Show only recommended applications")
	cmd.Flags().Bool("interactive", false, "Interactive selection mode")
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed information")

	return cmd
}

// runListCommandWithError handles the main execution logic for list commands and returns errors
func runListCommandWithError(cmd *cobra.Command, args []string, repo types.Repository, settings config.CrossPlatformSettings) error {
	options := parseListFlags(cmd)
	return executeListCommand(cmd, args, repo, settings, options)
}

// executeListCommand executes the appropriate list subcommand with error handling
func executeListCommand(cmd *cobra.Command, args []string, repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) error {
	writer := cmd.OutOrStdout()

	if len(args) == 0 {
		return cmd.Help()
	}

	subcommand := args[0]
	validArgs := []string{"installed", "available", "categories"}

	switch subcommand {
	case "installed":
		return showInstalledApps(repo, settings, options, writer)
	case "available":
		return showAvailableApps(repo, settings, options, writer)
	case "categories":
		return showCategories(settings, options, writer)
	default:
		// Check if it's a similar command and suggest corrections
		var suggestions []string
		for _, valid := range validArgs {
			if strings.Contains(valid, subcommand) || strings.Contains(subcommand, valid) {
				suggestions = append(suggestions, valid)
			}
		}

		if len(suggestions) > 0 {
			return fmt.Errorf("unknown subcommand '%s': did you mean '%s'? Available options: %s",
				subcommand, suggestions[0], strings.Join(validArgs, ", "))
		}

		return fmt.Errorf("unknown subcommand '%s': available options are: %s",
			subcommand, strings.Join(validArgs, ", "))
	}
}

// parseListFlags extracts and validates command flags into options struct
func parseListFlags(cmd *cobra.Command) ListCommandOptions {
	format, _ := cmd.Flags().GetString("format")
	category, _ := cmd.Flags().GetString("category")
	search, _ := cmd.Flags().GetString("search")
	method, _ := cmd.Flags().GetString("method")
	recommended, _ := cmd.Flags().GetBool("recommended")
	interactive, _ := cmd.Flags().GetBool("interactive")
	verbose, _ := cmd.Flags().GetBool("verbose")

	return ListCommandOptions{
		Format:      format,
		Category:    category,
		Search:      search,
		Method:      method,
		Recommended: recommended,
		Interactive: interactive,
		Verbose:     verbose,
	}
}

// showInstalledApps displays installed applications based on options
func showInstalledApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions, writer io.Writer) error {
	apps, err := getInstalledApps(repo, settings, options)
	if err != nil {
		return fmt.Errorf("failed to retrieve installed applications: %w", err)
	}

	switch strings.ToLower(options.Format) {
	case "json":
		return outputInstalledJSON(apps, writer)
	case "yaml":
		return outputInstalledYAML(apps, writer)
	case "table":
		return outputInstalledTable(apps, options, writer)
	default:
		return fmt.Errorf("unsupported format '%s': supported formats are table, json, yaml", options.Format)
	}
}

// showAvailableApps displays available applications based on options
func showAvailableApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions, writer io.Writer) error {
	apps := getAvailableApps(repo, settings, options)

	switch strings.ToLower(options.Format) {
	case "json":
		return outputAvailableJSON(apps, writer)
	case "yaml":
		return outputAvailableYAML(apps, writer)
	case "table":
		return outputAvailableTable(apps, options, writer)
	default:
		return fmt.Errorf("unsupported format '%s': supported formats are table, json, yaml", options.Format)
	}
}

// showCategories displays category information based on options
func showCategories(settings config.CrossPlatformSettings, options ListCommandOptions, writer io.Writer) error {
	categories := getCategoryInfo(settings)

	switch strings.ToLower(options.Format) {
	case "json":
		return outputCategoriesJSON(categories, writer)
	case "yaml":
		return outputCategoriesYAML(categories, writer)
	case "table":
		return outputCategoriesTable(categories, options, writer)
	default:
		return fmt.Errorf("unsupported format '%s': supported formats are table, json, yaml", options.Format)
	}
}
