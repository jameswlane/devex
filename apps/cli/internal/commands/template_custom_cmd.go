package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

// NewTemplateCustomCmd creates the template custom command group
func NewTemplateCustomCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom",
		Short: "Manage custom team and user templates",
		Long: `Manage custom team and user templates for advanced configuration sharing.

Custom templates allow teams and individuals to create, share, and distribute
their own configuration templates beyond the built-in options. This enables
organizations to standardize their development environments and share best
practices across teams.`,
		Example: `  # Create a new custom template from current configuration
  devex template custom create my-team-stack --organization myorg

  # Install a custom template from a Git repository
  devex template custom install --git https://github.com/myorg/devex-templates

  # Install a local template
  devex template custom install ./my-template

  # List all available custom templates
  devex template custom list

  # Export a template for sharing
  devex template custom export my-template ./my-template.zip

  # Remove a custom template
  devex template custom remove my-template`,
	}

	// Add subcommands
	cmd.AddCommand(newTemplateCustomCreateCmd(repo, settings))
	cmd.AddCommand(newTemplateCustomInstallCmd(repo, settings))
	cmd.AddCommand(newTemplateCustomListCmd(repo, settings))
	cmd.AddCommand(newTemplateCustomInfoCmd(repo, settings))
	cmd.AddCommand(newTemplateCustomUpdateCmd(repo, settings))
	cmd.AddCommand(newTemplateCustomRemoveCmd(repo, settings))
	cmd.AddCommand(newTemplateCustomExportCmd(repo, settings))

	return cmd
}

// newTemplateCustomCreateCmd creates the template custom create command
func newTemplateCustomCreateCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		name            string
		description     string
		author          string
		organization    string
		license         string
		homepage        string
		repository      string
		categories      []string
		tags            []string
		sourceDir       string
		templateVersion string
	)

	cmd := &cobra.Command{
		Use:   "create <template-id>",
		Short: "Create a new custom template from current configuration",
		Long: `Create a new custom template from your current configuration.

This command will package your current devex configuration files into a
reusable template that can be shared with team members or the community.
The template will include all configuration files (applications, environment,
system, and desktop settings) from your current setup.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Create a basic user template
  devex template custom create my-stack --name "My Development Stack"

  # Create a team template with organization
  devex template custom create backend-stack \
    --name "Backend Development Stack" \
    --organization "myorg" \
    --description "Standard backend setup with Go, PostgreSQL, and Redis"

  # Create from a specific configuration directory
  devex template custom create test-stack --source-dir ./test-config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]

			// Set defaults
			if templateVersion == "" {
				templateVersion = "1.0.0"
			}
			if sourceDir == "" {
				sourceDir = settings.GetConfigDir()
			}
			if name == "" {
				titleID := strings.ReplaceAll(templateID, "-", " ")
				// Simple title case implementation for display purposes
				words := strings.Split(titleID, " ")
				var titleWords []string
				for _, word := range words {
					if len(word) > 0 {
						titleWords = append(titleWords, strings.ToUpper(word[:1])+strings.ToLower(word[1:]))
					}
				}
				name = strings.Join(titleWords, " ")
			}

			// Initialize managers
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			configDir := settings.GetConfigDir()

			versionManager := version.NewVersionManager(baseDir)
			backupManager := backup.NewBackupManager(baseDir)
			undoManager := undo.NewUndoManager(baseDir)

			customTemplateManager, err := templates.NewCustomTemplateManager(baseDir, configDir, versionManager, backupManager, undoManager)
			if err != nil {
				return fmt.Errorf("failed to initialize custom template manager: %w", err)
			}

			// Create manifest
			manifest := &templates.CustomTemplateManifest{
				ID:              templateID,
				Name:            name,
				Version:         templateVersion,
				Description:     description,
				Author:          author,
				Organization:    organization,
				License:         license,
				Homepage:        homepage,
				Repository:      repository,
				Categories:      categories,
				Tags:            tags,
				MinDevexVersion: "1.0.0", // TODO: Get from build info
			}

			// Create template
			if err := customTemplateManager.CreateTemplate(manifest, sourceDir); err != nil {
				return fmt.Errorf("failed to create custom template: %w", err)
			}

			templateType := "user"
			if organization != "" {
				templateType = "team"
			}

			fmt.Printf("✅ Successfully created %s template '%s' v%s\n", templateType, templateID, templateVersion)
			if organization != "" {
				fmt.Printf("   Organization: %s\n", organization)
			}
			if description != "" {
				fmt.Printf("   Description: %s\n", description)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Human-readable template name")
	cmd.Flags().StringVar(&description, "description", "", "Template description")
	cmd.Flags().StringVar(&author, "author", "", "Template author")
	cmd.Flags().StringVar(&organization, "organization", "", "Organization name (creates team template)")
	cmd.Flags().StringVar(&license, "license", "MIT", "Template license")
	cmd.Flags().StringVar(&homepage, "homepage", "", "Template homepage URL")
	cmd.Flags().StringVar(&repository, "repository", "", "Template repository URL")
	cmd.Flags().StringSliceVar(&categories, "categories", []string{"development"}, "Template categories")
	cmd.Flags().StringSliceVar(&tags, "tags", []string{}, "Template tags")
	cmd.Flags().StringVar(&sourceDir, "source-dir", "", "Source configuration directory (default: current config)")
	cmd.Flags().StringVar(&templateVersion, "version", "1.0.0", "Template version")

	return cmd
}

// newTemplateCustomInstallCmd creates the template custom install command
func newTemplateCustomInstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		gitURL    string
		httpURL   string
		localPath string
		branch    string
		tag       string
		token     string
		private   bool
	)

	cmd := &cobra.Command{
		Use:   "install <template-ref>",
		Short: "Install a custom template from various sources",
		Long: `Install a custom template from Git repositories, HTTP URLs, or local directories.

This command supports multiple installation sources:
- Git repositories (public and private)
- HTTP/HTTPS URLs pointing to template archives
- Local file system paths
- Template registries (coming soon)`,
		Args: cobra.ExactArgs(1),
		Example: `  # Install from Git repository (public)
  devex template custom install --git https://github.com/myorg/devex-templates

  # Install from private Git repository
  devex template custom install --git https://github.com/myorg/private-templates --private --token $GITHUB_TOKEN

  # Install from specific branch or tag
  devex template custom install --git https://github.com/myorg/templates --branch develop
  devex template custom install --git https://github.com/myorg/templates --tag v2.1.0

  # Install from HTTP URL
  devex template custom install --http https://example.com/templates/my-template.zip

  # Install from local directory
  devex template custom install --local ./my-template-dir`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateRef := args[0]

			// Initialize managers
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			configDir := settings.GetConfigDir()

			versionManager := version.NewVersionManager(baseDir)
			backupManager := backup.NewBackupManager(baseDir)
			undoManager := undo.NewUndoManager(baseDir)

			customTemplateManager, err := templates.NewCustomTemplateManager(baseDir, configDir, versionManager, backupManager, undoManager)
			if err != nil {
				return fmt.Errorf("failed to initialize custom template manager: %w", err)
			}

			// Determine source type and create source configuration
			var source *templates.TemplateSource

			switch {
			case gitURL != "":
				source = &templates.TemplateSource{
					Type:    "git",
					URL:     gitURL,
					Branch:  branch,
					Tag:     tag,
					Token:   token,
					Private: private,
				}
			case httpURL != "":
				source = &templates.TemplateSource{
					Type: "http",
					URL:  httpURL,
				}
			case localPath != "":
				source = &templates.TemplateSource{
					Type: "local",
					Path: localPath,
				}
			default:
				// Default to local path if no explicit source
				source = &templates.TemplateSource{
					Type: "local",
					Path: templateRef,
				}
			}

			// Install template
			if err := customTemplateManager.InstallTemplate(templateRef, source); err != nil {
				return fmt.Errorf("failed to install custom template: %w", err)
			}

			fmt.Printf("✅ Successfully installed custom template from %s source\n", source.Type)
			return nil
		},
	}

	cmd.Flags().StringVar(&gitURL, "git", "", "Install from Git repository URL")
	cmd.Flags().StringVar(&httpURL, "http", "", "Install from HTTP/HTTPS URL")
	cmd.Flags().StringVar(&localPath, "local", "", "Install from local directory path")
	cmd.Flags().StringVar(&branch, "branch", "", "Git branch to use (default: main/master)")
	cmd.Flags().StringVar(&tag, "tag", "", "Git tag to use")
	cmd.Flags().StringVar(&token, "token", "", "Authentication token for private repositories")
	cmd.Flags().BoolVar(&private, "private", false, "Repository is private")

	return cmd
}

// newTemplateCustomListCmd creates the template custom list command
func newTemplateCustomListCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		outputFormat string
		organization string
		category     string
		tag          string
		showSystem   bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available custom templates",
		Long: `List all available custom templates with filtering options.

This command displays all installed custom templates, including both user
and team templates. You can filter by organization, category, or tags to
find specific templates.`,
		Example: `  # List all custom templates
  devex template custom list

  # List templates in table format
  devex template custom list --output table

  # List templates for specific organization
  devex template custom list --organization myorg

  # List templates by category
  devex template custom list --category backend

  # List templates with specific tag
  devex template custom list --tag golang`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize managers
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			configDir := settings.GetConfigDir()

			versionManager := version.NewVersionManager(baseDir)
			backupManager := backup.NewBackupManager(baseDir)
			undoManager := undo.NewUndoManager(baseDir)

			customTemplateManager, err := templates.NewCustomTemplateManager(baseDir, configDir, versionManager, backupManager, undoManager)
			if err != nil {
				return fmt.Errorf("failed to initialize custom template manager: %w", err)
			}

			// Get all custom templates
			allTemplates, err := customTemplateManager.ListCustomTemplates()
			if err != nil {
				return fmt.Errorf("failed to list custom templates: %w", err)
			}

			// Apply filters
			var filteredTemplates []*templates.CustomTemplateManifest
			for _, template := range allTemplates {
				// Filter by organization
				if organization != "" && template.Organization != organization {
					continue
				}

				// Filter by category
				if category != "" {
					found := false
					for _, cat := range template.Categories {
						if cat == category {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				// Filter by tag
				if tag != "" {
					found := false
					for _, t := range template.Tags {
						if t == tag {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				filteredTemplates = append(filteredTemplates, template)
			}

			// Sort templates by organization, then by name
			sort.Slice(filteredTemplates, func(i, j int) bool {
				if filteredTemplates[i].Organization != filteredTemplates[j].Organization {
					return filteredTemplates[i].Organization < filteredTemplates[j].Organization
				}
				return filteredTemplates[i].Name < filteredTemplates[j].Name
			})

			// Output results
			switch outputFormat {
			case "json":
				return outputCustomTemplatesJSON(filteredTemplates)
			case "yaml":
				return outputCustomTemplatesYAML(filteredTemplates)
			case "table":
				return outputCustomTemplatesTable(filteredTemplates)
			default:
				return outputCustomTemplatesList(filteredTemplates)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "list", "Output format (list|table|json|yaml)")
	cmd.Flags().StringVar(&organization, "organization", "", "Filter by organization")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().BoolVar(&showSystem, "show-system", false, "Include system templates")

	return cmd
}

// newTemplateCustomInfoCmd creates the template custom info command
func newTemplateCustomInfoCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "info <template-id>",
		Short: "Show detailed information about a custom template",
		Long: `Show detailed information about a specific custom template.

This command displays comprehensive information about a custom template
including its manifest data, file contents, version history, and usage
statistics.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Show template information
  devex template custom info my-backend-stack

  # Show information in JSON format
  devex template custom info my-backend-stack --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]

			// Initialize managers
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			configDir := settings.GetConfigDir()

			versionManager := version.NewVersionManager(baseDir)
			backupManager := backup.NewBackupManager(baseDir)
			undoManager := undo.NewUndoManager(baseDir)

			customTemplateManager, err := templates.NewCustomTemplateManager(baseDir, configDir, versionManager, backupManager, undoManager)
			if err != nil {
				return fmt.Errorf("failed to initialize custom template manager: %w", err)
			}

			// Get template info
			template, err := customTemplateManager.GetCustomTemplate(templateID)
			if err != nil {
				return fmt.Errorf("failed to get template info: %w", err)
			}

			// Output results
			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(template, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal template info: %w", err)
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(template)
				if err != nil {
					return fmt.Errorf("failed to marshal template info: %w", err)
				}
				fmt.Print(string(data))
			default:
				return outputCustomTemplateInfo(template)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "info", "Output format (info|json|yaml)")

	return cmd
}

// newTemplateCustomUpdateCmd creates the template custom update command
func newTemplateCustomUpdateCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var newVersion string

	cmd := &cobra.Command{
		Use:   "update <template-id>",
		Short: "Update a custom template version",
		Long: `Update a custom template to a new version.

This command updates the version metadata of a custom template and
recalculates checksums. It's typically used when the template content
has been modified and you want to publish a new version.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Update template to a new version
  devex template custom update my-backend-stack --version 1.2.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]

			if newVersion == "" {
				return fmt.Errorf("--version flag is required")
			}

			// Initialize managers
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			configDir := settings.GetConfigDir()

			versionManager := version.NewVersionManager(baseDir)
			backupManager := backup.NewBackupManager(baseDir)
			undoManager := undo.NewUndoManager(baseDir)

			customTemplateManager, err := templates.NewCustomTemplateManager(baseDir, configDir, versionManager, backupManager, undoManager)
			if err != nil {
				return fmt.Errorf("failed to initialize custom template manager: %w", err)
			}

			// Update template
			if err := customTemplateManager.UpdateTemplate(templateID, newVersion); err != nil {
				return fmt.Errorf("failed to update template: %w", err)
			}

			fmt.Printf("✅ Successfully updated template '%s' to version %s\n", templateID, newVersion)
			return nil
		},
	}

	cmd.Flags().StringVar(&newVersion, "version", "", "New version number (required)")
	_ = cmd.MarkFlagRequired("version")

	return cmd
}

// newTemplateCustomRemoveCmd creates the template custom remove command
func newTemplateCustomRemoveCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <template-id>",
		Short: "Remove a custom template",
		Long: `Remove a custom template from the local system.

This command removes a custom template and all its associated files.
The operation can be undone using the 'devex undo' command if needed.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Remove a template (with confirmation)
  devex template custom remove my-old-template

  # Force remove without confirmation
  devex template custom remove my-old-template --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]

			// Initialize managers
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			configDir := settings.GetConfigDir()

			versionManager := version.NewVersionManager(baseDir)
			backupManager := backup.NewBackupManager(baseDir)
			undoManager := undo.NewUndoManager(baseDir)

			customTemplateManager, err := templates.NewCustomTemplateManager(baseDir, configDir, versionManager, backupManager, undoManager)
			if err != nil {
				return fmt.Errorf("failed to initialize custom template manager: %w", err)
			}

			// Get template info for confirmation
			template, err := customTemplateManager.GetCustomTemplate(templateID)
			if err != nil {
				return fmt.Errorf("template not found: %w", err)
			}

			// Confirm removal unless force flag is set
			if !force {
				fmt.Printf("Are you sure you want to remove template '%s' v%s? [y/N]: ", template.Name, template.Version)
				var response string
				_, _ = fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Operation cancelled")
					return nil
				}
			}

			// Remove template
			if err := customTemplateManager.RemoveTemplate(templateID); err != nil {
				return fmt.Errorf("failed to remove template: %w", err)
			}

			fmt.Printf("✅ Successfully removed template '%s'\n", templateID)
			fmt.Println("💡 You can undo this operation with: devex undo")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force removal without confirmation")

	return cmd
}

// newTemplateCustomExportCmd creates the template custom export command
func newTemplateCustomExportCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export <template-id>",
		Short: "Export a custom template as a distributable package",
		Long: `Export a custom template as a distributable ZIP package.

This command packages a custom template into a ZIP file that can be
shared with others or distributed through various channels. The exported
package contains all template files and manifest data.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Export template to current directory
  devex template custom export my-backend-stack

  # Export to specific path
  devex template custom export my-backend-stack --output ./exports/backend-v1.2.0.zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]

			// Initialize managers
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			configDir := settings.GetConfigDir()

			versionManager := version.NewVersionManager(baseDir)
			backupManager := backup.NewBackupManager(baseDir)
			undoManager := undo.NewUndoManager(baseDir)

			customTemplateManager, err := templates.NewCustomTemplateManager(baseDir, configDir, versionManager, backupManager, undoManager)
			if err != nil {
				return fmt.Errorf("failed to initialize custom template manager: %w", err)
			}

			// Get template info for filename
			template, err := customTemplateManager.GetCustomTemplate(templateID)
			if err != nil {
				return fmt.Errorf("failed to get template info: %w", err)
			}

			// Set default output path if not specified
			if outputPath == "" {
				outputPath = fmt.Sprintf("%s-v%s.zip", templateID, template.Version)
			}

			// Export template
			if err := customTemplateManager.ExportTemplate(templateID, outputPath); err != nil {
				return fmt.Errorf("failed to export template: %w", err)
			}

			fmt.Printf("✅ Successfully exported template '%s' v%s to %s\n", template.Name, template.Version, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: <template-id>-v<version>.zip)")

	return cmd
}

// Output helper functions

func outputCustomTemplatesList(templates []*templates.CustomTemplateManifest) error {
	if len(templates) == 0 {
		fmt.Println("No custom templates found")
		return nil
	}

	var currentOrg string
	for _, template := range templates {
		// Group by organization
		org := template.Organization
		if org == "" {
			org = "Personal"
		}

		if org != currentOrg {
			if currentOrg != "" {
				fmt.Println()
			}
			fmt.Printf("📁 %s Templates:\n", org)
			currentOrg = org
		}

		// Template info
		fmt.Printf("  • %s (v%s)\n", template.Name, template.Version)
		if template.Description != "" {
			fmt.Printf("    %s\n", template.Description)
		}
		if len(template.Categories) > 0 {
			fmt.Printf("    Categories: %s\n", strings.Join(template.Categories, ", "))
		}
		if len(template.Tags) > 0 {
			fmt.Printf("    Tags: %s\n", strings.Join(template.Tags, ", "))
		}
	}

	return nil
}

func outputCustomTemplatesTable(templates []*templates.CustomTemplateManifest) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tVERSION\tORGANIZATION\tCATEGORIES\tDESCRIPTION")
	fmt.Fprintln(w, "──\t────\t───────\t────────────\t──────────\t───────────")

	for _, template := range templates {
		org := template.Organization
		if org == "" {
			org = "-"
		}

		categories := strings.Join(template.Categories, ",")
		if categories == "" {
			categories = "-"
		}

		description := template.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			template.ID, template.Name, template.Version, org, categories, description)
	}

	return w.Flush()
}

func outputCustomTemplatesJSON(templates []*templates.CustomTemplateManifest) error {
	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputCustomTemplatesYAML(templates []*templates.CustomTemplateManifest) error {
	data, err := yaml.Marshal(templates)
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

func outputCustomTemplateInfo(template *templates.CustomTemplateManifest) error {
	fmt.Printf("📋 Template Information\n\n")
	fmt.Printf("ID:              %s\n", template.ID)
	fmt.Printf("Name:            %s\n", template.Name)
	fmt.Printf("Version:         %s\n", template.Version)
	fmt.Printf("Description:     %s\n", template.Description)

	if template.Author != "" {
		fmt.Printf("Author:          %s\n", template.Author)
	}

	if template.Organization != "" {
		fmt.Printf("Organization:    %s\n", template.Organization)
	}

	if template.License != "" {
		fmt.Printf("License:         %s\n", template.License)
	}

	if template.Homepage != "" {
		fmt.Printf("Homepage:        %s\n", template.Homepage)
	}

	if template.Repository != "" {
		fmt.Printf("Repository:      %s\n", template.Repository)
	}

	fmt.Printf("Min DevEx:       %s\n", template.MinDevexVersion)
	fmt.Printf("Created:         %s\n", template.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:         %s\n", template.UpdatedAt.Format("2006-01-02 15:04:05"))

	if len(template.Categories) > 0 {
		fmt.Printf("Categories:      %s\n", strings.Join(template.Categories, ", "))
	}

	if len(template.Tags) > 0 {
		fmt.Printf("Tags:            %s\n", strings.Join(template.Tags, ", "))
	}

	if len(template.Files) > 0 {
		fmt.Printf("Files:           %s\n", strings.Join(template.Files, ", "))
	}

	if len(template.Dependencies) > 0 {
		fmt.Printf("Dependencies:    %s\n", strings.Join(template.Dependencies, ", "))
	}

	if template.Checksum != "" {
		fmt.Printf("Checksum:        %s\n", template.Checksum)
	}

	fmt.Printf("\n📦 Source Information\n")
	fmt.Printf("Type:            %s\n", template.Source.Type)
	if template.Source.URL != "" {
		fmt.Printf("URL:             %s\n", template.Source.URL)
	}
	if template.Source.Path != "" {
		fmt.Printf("Path:            %s\n", template.Source.Path)
	}
	if template.Source.Branch != "" {
		fmt.Printf("Branch:          %s\n", template.Source.Branch)
	}
	if template.Source.Tag != "" {
		fmt.Printf("Tag:             %s\n", template.Source.Tag)
	}

	if len(template.Metadata) > 0 {
		fmt.Printf("\n🏷️  Metadata\n")
		for key, value := range template.Metadata {
			fmt.Printf("%-15s  %v\n", key+":", value)
		}
	}

	return nil
}
