package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// NewTemplateCmd creates a new template command with versioning support
func NewTemplateCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage DevEx templates and their versions",
		Long: `Manage DevEx templates including built-in and user templates.

This command provides comprehensive template management including:
  • List available templates and their versions
  • Check for template updates
  • Update specific templates or all at once
  • View detailed template information
  • Manage template versioning and history

Templates are versioned using semantic versioning and support automatic updates
for built-in templates while preserving user customizations.

Examples:
  # List all templates with version information
  devex template list
  
  # Check for available template updates
  devex template check-updates
  
  # Update a specific template
  devex template update full-stack
  
  # Update all templates
  devex template update --all
  
  # Show detailed template information
  devex template info backend
  
  # Show template version summary
  devex template status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newTemplateListCmd(settings))
	cmd.AddCommand(newTemplateCheckUpdatesCmd(settings))
	cmd.AddCommand(newTemplateUpdateCmd(settings))
	cmd.AddCommand(newTemplateInfoCmd(settings))
	cmd.AddCommand(newTemplateStatusCmd(settings))
	cmd.AddCommand(newTemplateInitCmd(settings))
	cmd.AddCommand(NewTemplateCustomCmd(repo, settings))

	return cmd
}

func newTemplateListCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		format      string
		source      string
		showUpdates bool
		includeUser bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available templates with version information",
		Long: `List all available templates including built-in and user templates.

Shows version information, update status, and template metadata for easy overview
of your template ecosystem.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			tvm := templates.NewTemplateVersionManager(baseDir)

			// Initialize and scan templates
			if err := tvm.InitializeTemplateVersioning(); err != nil {
				return fmt.Errorf("failed to initialize template versioning: %w", err)
			}

			if err := tvm.ScanAndRegisterTemplates(); err != nil {
				return fmt.Errorf("failed to scan templates: %w", err)
			}

			templateVersions, err := tvm.GetTemplateVersions()
			if err != nil {
				return fmt.Errorf("failed to get template versions: %w", err)
			}

			// Filter templates
			filteredTemplates := make(map[string]*templates.TemplateVersion)
			for id, template := range templateVersions {
				if source != "" && template.Source != source {
					continue
				}
				if !includeUser && template.Source == "user" {
					continue
				}
				if showUpdates && !template.UpdateAvailable {
					continue
				}
				filteredTemplates[id] = template
			}

			return displayTemplateList(filteredTemplates, format)
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source (builtin, user)")
	cmd.Flags().BoolVar(&showUpdates, "updates-only", false, "Show only templates with available updates")
	cmd.Flags().BoolVar(&includeUser, "include-user", false, "Include user templates in output")

	return cmd
}

func newTemplateCheckUpdatesCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "check-updates",
		Short: "Check for available template updates",
		Long: `Check if any built-in templates have updates available.

This command scans all registered templates and compares their current versions
with the latest available versions to identify update opportunities.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			tvm := templates.NewTemplateVersionManager(baseDir)

			// Initialize template versioning
			if err := tvm.InitializeTemplateVersioning(); err != nil {
				return fmt.Errorf("failed to initialize template versioning: %w", err)
			}

			// Scan for templates first
			if err := tvm.ScanAndRegisterTemplates(); err != nil {
				return fmt.Errorf("failed to scan templates: %w", err)
			}

			updates, err := tvm.CheckForUpdates()
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			return displayUpdateCheck(updates, format, tvm)
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")

	return cmd
}

func newTemplateUpdateCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		all    bool
		force  bool
		format string
	)

	cmd := &cobra.Command{
		Use:   "update [template-name]",
		Short: "Update templates to their latest versions",
		Long: `Update one or more templates to their latest versions.

This command safely updates templates by creating backups and undo points
before making changes. Updates preserve user customizations and provide
rollback capabilities.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for --no-tui flag
			noTUI, _ := cmd.Flags().GetBool("no-tui")

			var templateID string
			if len(args) > 0 {
				templateID = args[0]
			}

			if !noTUI {
				return runTemplateUpdateWithProgress(settings, templateID, all, force, format)
			}

			// Fallback to original implementation
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			tvm := templates.NewTemplateVersionManager(baseDir)

			// Initialize template versioning
			if err := tvm.InitializeTemplateVersioning(); err != nil {
				return fmt.Errorf("failed to initialize template versioning: %w", err)
			}

			// Scan for templates
			if err := tvm.ScanAndRegisterTemplates(); err != nil {
				return fmt.Errorf("failed to scan templates: %w", err)
			}

			if all {
				// Update all templates
				results, err := tvm.UpdateAllTemplates(force)
				if err != nil {
					return fmt.Errorf("failed to update templates: %w", err)
				}
				return displayUpdateResults(results, format)
			}

			if len(args) == 0 {
				return fmt.Errorf("template name required (use --all to update all templates)")
			}

			// Update specific template
			result, err := tvm.UpdateTemplate(templateID, force)
			if err != nil {
				return fmt.Errorf("failed to update template %s: %w", templateID, err)
			}

			return displayUpdateResults([]*templates.TemplateUpdateResult{result}, format)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Update all templates")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force update even if no update is available")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	cmd.Flags().Bool("no-tui", false, "Disable TUI progress display")

	return cmd
}

func newTemplateInfoCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "info <template-name>",
		Short: "Show detailed information about a template",
		Long: `Display comprehensive information about a specific template including
version details, manifest data, and update status.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			tvm := templates.NewTemplateVersionManager(baseDir)

			// Initialize template versioning
			if err := tvm.InitializeTemplateVersioning(); err != nil {
				return fmt.Errorf("failed to initialize template versioning: %w", err)
			}

			if err := tvm.ScanAndRegisterTemplates(); err != nil {
				return fmt.Errorf("failed to scan templates: %w", err)
			}

			templateVersion, manifest, err := tvm.GetTemplateInfo(templateID)
			if err != nil {
				return fmt.Errorf("failed to get template info: %w", err)
			}

			return displayTemplateInfo(templateVersion, manifest, format)
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")

	return cmd
}

func newTemplateStatusCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show template system status and summary",
		Long: `Display a summary of the template system including counts of templates,
update availability, and system health.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			tvm := templates.NewTemplateVersionManager(baseDir)

			// Initialize template versioning
			if err := tvm.InitializeTemplateVersioning(); err != nil {
				return fmt.Errorf("failed to initialize template versioning: %w", err)
			}

			if err := tvm.ScanAndRegisterTemplates(); err != nil {
				return fmt.Errorf("failed to scan templates: %w", err)
			}

			summary, err := tvm.GetTemplateVersionSummary()
			if err != nil {
				return fmt.Errorf("failed to get template summary: %w", err)
			}

			return displayTemplateSummary(summary, format)
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")

	return cmd
}

func newTemplateInitCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize template versioning system",
		Long: `Initialize the template versioning system and scan for existing templates.

This command sets up the template versioning infrastructure and registers
all discovered templates in the version tracking system.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			tvm := templates.NewTemplateVersionManager(baseDir)

			fmt.Println("Initializing template versioning system...")

			if err := tvm.InitializeTemplateVersioning(); err != nil {
				return fmt.Errorf("failed to initialize template versioning: %w", err)
			}

			fmt.Println("Scanning for templates...")

			if err := tvm.ScanAndRegisterTemplates(); err != nil {
				return fmt.Errorf("failed to scan templates: %w", err)
			}

			summary, err := tvm.GetTemplateVersionSummary()
			if err != nil {
				return fmt.Errorf("failed to get summary: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			fmt.Printf("%s Template versioning initialized successfully!\n", green("✓"))
			fmt.Printf("Registered %d templates (%d builtin, %d user)\n",
				summary.TotalTemplates, summary.BuiltinTemplates, summary.UserTemplates)

			if summary.UpdatesAvailable > 0 {
				yellow := color.New(color.FgYellow).SprintFunc()
				fmt.Printf("%s %d template updates available\n", yellow("⚠"), summary.UpdatesAvailable)
				fmt.Println("Run 'devex template check-updates' to see available updates")
			}

			return nil
		},
	}

	return cmd
}

// Display functions

func displayTemplateList(templates map[string]*templates.TemplateVersion, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(templates, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(templates)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))
	default:
		// Table format
		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()
		blue := color.New(color.FgBlue).SprintFunc()
		gray := color.New(color.FgHiBlack).SprintFunc()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tVERSION\tSOURCE\tUPDATE\tDESCRIPTION")
		fmt.Fprintln(w, "---\t----\t-------\t------\t------\t-----------")

		for id, template := range templates {
			updateStatus := gray("-")
			if template.UpdateAvailable {
				updateStatus = yellow(fmt.Sprintf("v%s", template.LatestVersion))
			}

			sourceColor := blue
			if template.Source == "user" {
				sourceColor = green
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				id,
				template.Name,
				template.Version,
				sourceColor(template.Source),
				updateStatus,
				template.Description)
		}
		_ = w.Flush()
	}

	return nil
}

func displayUpdateCheck(updates []string, format string, tvm *templates.TemplateVersionManager) error {
	if len(updates) == 0 {
		green := color.New(color.FgGreen).SprintFunc()
		fmt.Printf("%s All templates are up to date\n", green("✓"))
		return nil
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(updates, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(updates)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))
	default:
		// Table format
		yellow := color.New(color.FgYellow).SprintFunc()
		blue := color.New(color.FgBlue).SprintFunc()

		fmt.Printf("%s %d template updates available:\n\n", yellow("⚠"), len(updates))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "TEMPLATE\tCURRENT\tLATEST\tDESCRIPTION")
		fmt.Fprintln(w, "--------\t-------\t------\t-----------")

		templateVersions, err := tvm.GetTemplateVersions()
		if err != nil {
			return fmt.Errorf("failed to get template versions: %w", err)
		}

		for _, templateID := range updates {
			if template, exists := templateVersions[templateID]; exists {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					blue(templateID),
					template.Version,
					yellow(template.LatestVersion),
					template.Description)
			}
		}
		_ = w.Flush()

		fmt.Printf("\nRun 'devex template update --all' to update all templates\n")
	}

	return nil
}

// runTemplateUpdateWithProgress runs template update with TUI progress tracking
func runTemplateUpdateWithProgress(settings config.CrossPlatformSettings, templateID string, all, force bool, format string) error {
	runner := tui.NewProgressRunner(context.Background(), settings)
	defer runner.Quit()

	return runner.RunTemplateOperation("update", templateID, all, force, format)
}

func displayUpdateResults(results []*templates.TemplateUpdateResult, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(results)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))
	default:
		// Table format
		green := color.New(color.FgGreen).SprintFunc()
		red := color.New(color.FgRed).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()

		for _, result := range results {
			if result.Success {
				fmt.Printf("%s %s\n", green("✓"), result.Message)
				if result.OldVersion != result.NewVersion {
					fmt.Printf("  Version: %s → %s\n", result.OldVersion, result.NewVersion)
				}
				if len(result.FilesUpdated) > 0 {
					fmt.Printf("  Files updated: %d\n", len(result.FilesUpdated))
				}
				if result.BackupCreated != "" {
					fmt.Printf("  Backup: %s\n", result.BackupCreated)
				}
			} else {
				fmt.Printf("%s %s: %s\n", red("✗"), result.TemplateID, result.Message)
			}

			if len(result.Warnings) > 0 {
				for _, warning := range result.Warnings {
					fmt.Printf("  %s %s\n", yellow("⚠"), warning)
				}
			}

			fmt.Println()
		}
	}

	return nil
}

func displayTemplateInfo(templateVersion *templates.TemplateVersion, manifest *templates.VersionableTemplateManifest, format string) error {
	info := struct {
		Version  *templates.TemplateVersion             `json:"version" yaml:"version"`
		Manifest *templates.VersionableTemplateManifest `json:"manifest" yaml:"manifest"`
	}{
		Version:  templateVersion,
		Manifest: manifest,
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(info)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))
	default:
		// Formatted display
		blue := color.New(color.FgBlue).SprintFunc()
		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()
		gray := color.New(color.FgHiBlack).SprintFunc()

		fmt.Printf("%s Template Information\n\n", blue("●"))

		fmt.Printf("ID: %s\n", templateVersion.ID)
		fmt.Printf("Name: %s\n", templateVersion.Name)
		fmt.Printf("Version: %s", templateVersion.Version)
		if templateVersion.UpdateAvailable {
			fmt.Printf(" %s (Latest: %s)", yellow("(Update Available)"), templateVersion.LatestVersion)
		}
		fmt.Println()

		fmt.Printf("Source: %s\n", templateVersion.Source)
		fmt.Printf("Description: %s\n", templateVersion.Description)
		fmt.Printf("Last Updated: %s\n", templateVersion.LastUpdated.Format("2006-01-02 15:04:05"))

		if manifest.Author != "" {
			fmt.Printf("Author: %s\n", manifest.Author)
		}
		if manifest.License != "" {
			fmt.Printf("License: %s\n", manifest.License)
		}
		if manifest.Homepage != "" {
			fmt.Printf("Homepage: %s\n", manifest.Homepage)
		}

		if len(manifest.Categories) > 0 {
			fmt.Printf("Categories: %s\n", strings.Join(manifest.Categories, ", "))
		}
		if len(manifest.Tags) > 0 {
			fmt.Printf("Tags: %s\n", strings.Join(manifest.Tags, ", "))
		}

		fmt.Printf("Files: %d\n", len(manifest.Files))
		if len(manifest.Files) > 0 {
			for _, file := range manifest.Files {
				fmt.Printf("  • %s\n", gray(file))
			}
		}

		fmt.Printf("Checksum: %s\n", gray(templateVersion.Checksum))

		if templateVersion.UpdateAvailable {
			fmt.Printf("\n%s Update available! Run 'devex template update %s' to update.\n",
				green("✓"), templateVersion.ID)
		}
	}

	return nil
}

func displayTemplateSummary(summary *templates.TemplateVersionSummary, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(summary)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))
	default:
		// Formatted display
		blue := color.New(color.FgBlue).SprintFunc()
		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()

		fmt.Printf("%s Template System Status\n\n", blue("●"))

		fmt.Printf("Total Templates: %d\n", summary.TotalTemplates)
		fmt.Printf("  Built-in: %d\n", summary.BuiltinTemplates)
		fmt.Printf("  User: %d\n", summary.UserTemplates)

		if summary.UpdatesAvailable > 0 {
			fmt.Printf("Updates Available: %s\n", yellow(fmt.Sprintf("%d", summary.UpdatesAvailable)))
			if len(summary.TemplatesWithUpdates) > 0 {
				fmt.Printf("Templates with updates:\n")
				for _, templateID := range summary.TemplatesWithUpdates {
					fmt.Printf("  • %s\n", templateID)
				}
			}
		} else {
			fmt.Printf("Updates Available: %s\n", green("0"))
		}

		fmt.Printf("Last Checked: %s\n", summary.LastChecked)

		if summary.UpdatesAvailable > 0 {
			fmt.Printf("\nRun 'devex template update --all' to update all templates\n")
		}
	}

	return nil
}
