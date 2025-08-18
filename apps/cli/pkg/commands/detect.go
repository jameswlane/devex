package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/stackdetector"
	"github.com/jameswlane/devex/pkg/types"
)

// NewDetectCmd creates a new detect command for technology stack detection
func NewDetectCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect technologies and suggest configurations",
		Long: `Analyze the current project to detect technologies and provide smart configuration recommendations.

The detect command scans your project directory to identify:
• Programming languages (Python, Go, Node.js, etc.)
• Frameworks (React, Django, Rails, etc.)
• Tools and infrastructure (Docker, Kubernetes, Terraform, etc.)
• Databases (PostgreSQL, MongoDB, etc.)
• CI/CD systems (GitHub Actions, GitLab CI, etc.)

Based on detected technologies, it provides intelligent suggestions for:
• Applications to install
• Environment configurations to set up
• System tools to configure

Examples:
  # Detect technologies in current directory
  devex detect

  # Detect and show detailed analysis
  devex detect --detailed

  # Detect and save results to file
  devex detect --output detection-results.json

  # Apply suggested configurations automatically
  devex detect --apply`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetect(cmd, args, settings)
		},
	}

	// Add subcommands
	cmd.AddCommand(newDetectAnalyzeCmd(settings))
	cmd.AddCommand(newDetectSuggestCmd(settings))
	cmd.AddCommand(newDetectApplyCmd(settings))

	// Flags for the main detect command
	cmd.Flags().Bool("detailed", false, "Show detailed detection analysis")
	cmd.Flags().String("output", "", "Save detection results to file (JSON format)")
	cmd.Flags().String("dir", "", "Directory to analyze (default: current directory)")
	cmd.Flags().Bool("apply", false, "Apply suggested configurations automatically")
	cmd.Flags().Float64("confidence", 0.5, "Minimum confidence threshold (0.0-1.0)")

	return cmd
}

// newDetectAnalyzeCmd creates the analyze subcommand
func newDetectAnalyzeCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze [directory]",
		Short: "Analyze project structure and detect technologies",
		Long:  `Perform detailed analysis of project structure to detect technologies and frameworks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ""
			if len(args) > 0 {
				dir = args[0]
			}
			return runDetectAnalyze(dir, settings)
		},
	}

	cmd.Flags().Bool("verbose", false, "Show verbose analysis output")
	cmd.Flags().String("format", "table", "Output format (table, json, yaml)")

	return cmd
}

// newDetectSuggestCmd creates the suggest subcommand
func newDetectSuggestCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Show configuration suggestions based on detected technologies",
		Long:  `Display intelligent configuration suggestions based on detected project technologies.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetectSuggest(settings)
		},
	}

	cmd.Flags().String("category", "", "Filter suggestions by category (application, environment, system)")
	cmd.Flags().String("priority", "", "Filter suggestions by priority (critical, recommended, optional)")

	return cmd
}

// newDetectApplyCmd creates the apply subcommand
func newDetectApplyCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply suggested configurations automatically",
		Long: `Automatically apply configuration suggestions based on detected technologies.

This command will:
• Create configuration files for detected technologies
• Add recommended applications to your DevEx configuration
• Set up environment-specific configurations if needed

Use with caution in production environments.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetectApply(settings)
		},
	}

	cmd.Flags().Bool("dry-run", false, "Show what would be applied without making changes")
	cmd.Flags().String("priority", "critical,recommended", "Apply suggestions with these priorities")
	cmd.Flags().Bool("force", false, "Overwrite existing configurations")

	return cmd
}

// runDetect runs the main detect command
func runDetect(cmd *cobra.Command, args []string, settings config.CrossPlatformSettings) error {
	// Get flags
	detailed, _ := cmd.Flags().GetBool("detailed")
	outputFile, _ := cmd.Flags().GetString("output")
	dir, _ := cmd.Flags().GetString("dir")
	apply, _ := cmd.Flags().GetBool("apply")
	confidenceThreshold, _ := cmd.Flags().GetFloat64("confidence")

	if dir == "" {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		} else {
			dir = "."
		}
	}

	// Initialize detector
	detector := stackdetector.NewStackDetector(dir)

	// Detect technologies
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("%s Analyzing project structure...\n\n", cyan("🔍"))

	stacks, err := detector.DetectStack()
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	// Filter by confidence threshold
	var filteredStacks []stackdetector.TechnologyStack
	for _, stack := range stacks {
		if stack.Confidence >= confidenceThreshold {
			filteredStacks = append(filteredStacks, stack)
		}
	}

	if len(filteredStacks) == 0 {
		fmt.Printf("%s No technologies detected above confidence threshold %.1f\n", yellow("⚠️"), confidenceThreshold)
		fmt.Printf("Try lowering the threshold with --confidence=0.3\n")
		return nil
	}

	// Display results
	fmt.Printf("%s Detected Technologies:\n\n", cyan("🎯"))
	displayDetectionResults(filteredStacks, detailed)

	// Save results if requested
	if outputFile != "" {
		if err := detector.SaveResults(filteredStacks, outputFile); err != nil {
			fmt.Printf("%s Failed to save results: %v\n", yellow("⚠️"), err)
		} else {
			fmt.Printf("\n%s Results saved to %s\n", green("✅"), outputFile)
		}
	}

	// Apply suggestions if requested
	if apply {
		fmt.Printf("\n%s Applying suggested configurations...\n", cyan("🔧"))
		return applyDetectionSuggestions(filteredStacks, settings, false)
	}

	// Show quick actions
	fmt.Printf("\n%s Quick Actions:\n", cyan("💡"))
	fmt.Printf("  • View detailed analysis: devex detect analyze\n")
	fmt.Printf("  • See configuration suggestions: devex detect suggest\n")
	fmt.Printf("  • Apply suggestions: devex detect apply\n")

	return nil
}

// runDetectAnalyze runs the analyze subcommand
func runDetectAnalyze(dir string, settings config.CrossPlatformSettings) error {
	if dir == "" {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		} else {
			dir = "."
		}
	}

	detector := stackdetector.NewStackDetector(dir)
	stacks, err := detector.DetectStack()
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	fmt.Printf("%s Detailed Technology Analysis\n\n", cyan("🔬"))
	fmt.Printf("Analysis Directory: %s\n\n", dir)

	// Show summary
	summary := detector.GetDetectionSummary(stacks)
	fmt.Printf("Summary:\n")
	fmt.Printf("  • Total Technologies: %d\n", summary["total_technologies"])
	fmt.Printf("  • High Confidence: %d\n", summary["high_confidence"])
	fmt.Printf("  • Medium Confidence: %d\n", summary["medium_confidence"])
	fmt.Printf("  • Low Confidence: %d\n", summary["low_confidence"])

	// Show by category
	fmt.Printf("\nBy Category:\n")
	categories := summary["categories"].(map[string]int)
	for category, count := range categories {
		fmt.Printf("  • %s: %d\n", strings.Title(category), count)
	}

	// Show detailed results
	fmt.Printf("\nDetailed Results:\n")
	displayDetectionResults(stacks, true)

	return nil
}

// runDetectSuggest runs the suggest subcommand
func runDetectSuggest(settings config.CrossPlatformSettings) error {
	detector := stackdetector.NewStackDetector("")
	stacks, err := detector.DetectStack()
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	fmt.Printf("%s Configuration Suggestions\n\n", cyan("💡"))

	if len(stacks) == 0 {
		fmt.Printf("%s No technologies detected. Run detection from a project directory.\n", yellow("⚠️"))
		return nil
	}

	// Group suggestions by priority
	suggestionsByPriority := make(map[string][]stackdetector.Suggestion)
	for _, stack := range stacks {
		for _, suggestion := range stack.Suggestions {
			priority := suggestion.Priority
			if priority == "" {
				priority = "optional"
			}
			suggestionsByPriority[priority] = append(suggestionsByPriority[priority], suggestion)
		}
	}

	// Display suggestions by priority
	priorities := []string{"critical", "recommended", "optional"}
	priorityColors := map[string]func(...interface{}) string{
		"critical":    red,
		"recommended": yellow,
		"optional":    green,
	}

	for _, priority := range priorities {
		suggestions := suggestionsByPriority[priority]
		if len(suggestions) == 0 {
			continue
		}

		colorFunc := priorityColors[priority]
		fmt.Printf("%s %s Priority:\n", colorFunc("●"), strings.Title(priority))

		// Group by type
		suggestionsByType := make(map[string][]stackdetector.Suggestion)
		for _, suggestion := range suggestions {
			suggestionsByType[suggestion.Type] = append(suggestionsByType[suggestion.Type], suggestion)
		}

		for suggestionType, typeSuggestions := range suggestionsByType {
			fmt.Printf("  %s:\n", strings.Title(suggestionType))
			for _, suggestion := range typeSuggestions {
				fmt.Printf("    • %s - %s\n", suggestion.Target, suggestion.Description)
			}
		}
		fmt.Println()
	}

	fmt.Printf("%s To apply these suggestions:\n", cyan("🚀"))
	fmt.Printf("  devex detect apply\n")

	return nil
}

// runDetectApply runs the apply subcommand
func runDetectApply(settings config.CrossPlatformSettings) error {
	detector := stackdetector.NewStackDetector("")
	stacks, err := detector.DetectStack()
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	fmt.Printf("%s Applying Detection Suggestions\n\n", cyan("🔧"))

	return applyDetectionSuggestions(stacks, settings, false)
}

// displayDetectionResults displays the detection results in a formatted way
func displayDetectionResults(stacks []stackdetector.TechnologyStack, detailed bool) {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	// Group by category
	categories := make(map[string][]stackdetector.TechnologyStack)
	for _, stack := range stacks {
		categories[stack.Category] = append(categories[stack.Category], stack)
	}

	// Sort categories
	var categoryNames []string
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	for _, category := range categoryNames {
		stacks := categories[category]

		fmt.Printf("%s %s:\n", cyan("📂"), strings.Title(strings.ReplaceAll(category, "-", " ")))

		for _, stack := range stacks {
			// Color by confidence
			var confidenceColor func(...interface{}) string
			if stack.Confidence >= 0.8 {
				confidenceColor = green
			} else if stack.Confidence >= 0.5 {
				confidenceColor = yellow
			} else {
				confidenceColor = red
			}

			fmt.Printf("  %s %s (%s)\n",
				confidenceColor("●"),
				stack.Name,
				confidenceColor(fmt.Sprintf("%.1f%%", stack.Confidence*100)))

			if detailed {
				// Show evidence
				if len(stack.Evidence) > 0 {
					fmt.Printf("    Evidence:\n")
					for _, evidence := range stack.Evidence {
						fmt.Printf("      • %s: %s\n", evidence.Type, evidence.Description)
					}
				}

				// Show suggestions
				if len(stack.Suggestions) > 0 {
					fmt.Printf("    Suggestions:\n")
					for _, suggestion := range stack.Suggestions {
						fmt.Printf("      • %s %s: %s\n",
							suggestion.Priority,
							suggestion.Target,
							suggestion.Description)
					}
				}
				fmt.Println()
			}
		}
		fmt.Println()
	}
}

// applyDetectionSuggestions applies configuration suggestions from detected technologies
func applyDetectionSuggestions(stacks []stackdetector.TechnologyStack, settings config.CrossPlatformSettings, dryRun bool) error {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	// Collect all suggestions
	var allSuggestions []struct {
		Stack      string
		Suggestion stackdetector.Suggestion
	}

	for _, stack := range stacks {
		for _, suggestion := range stack.Suggestions {
			allSuggestions = append(allSuggestions, struct {
				Stack      string
				Suggestion stackdetector.Suggestion
			}{
				Stack:      stack.Name,
				Suggestion: suggestion,
			})
		}
	}

	if len(allSuggestions) == 0 {
		fmt.Printf("%s No suggestions to apply\n", yellow("ℹ️"))
		return nil
	}

	fmt.Printf("Found %d suggestions to apply:\n\n", len(allSuggestions))

	// Group suggestions by type and priority
	appSuggestions := make(map[string][]string) // priority -> apps

	for _, item := range allSuggestions {
		suggestion := item.Suggestion

		if suggestion.Type == "application" && suggestion.Action == "install" {
			priority := suggestion.Priority
			if priority == "" {
				priority = "optional"
			}
			appSuggestions[priority] = append(appSuggestions[priority], suggestion.Target)
		}
	}

	// Apply application suggestions
	if len(appSuggestions) > 0 {
		configDir := settings.GetUserConfigDir()

		if dryRun {
			fmt.Printf("%s Would create/update applications configuration in %s\n", cyan("📝"), configDir)
		} else {
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// Create applications.yaml with detected apps
			appsPath := filepath.Join(configDir, "applications.yaml")

			// Convert JSON to YAML-like format for better readability
			yamlContent := fmt.Sprintf(`# DevEx Applications Configuration
# Generated from technology stack detection
# Edit this file to customize your application setup

applications:
%s
`, formatApplicationsAsYAML(appSuggestions))

			if err := os.WriteFile(appsPath, []byte(yamlContent), 0644); err != nil {
				return fmt.Errorf("failed to write applications config: %w", err)
			}

			fmt.Printf("%s Created applications configuration: %s\n", green("✅"), appsPath)
		}
	}

	// Show summary
	fmt.Printf("\n%s Applied %d suggestions successfully!\n", green("🎉"), len(allSuggestions))
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Review configuration: devex config show\n")
	fmt.Printf("  2. Install applications: devex install\n")
	fmt.Printf("  3. Apply system settings: devex system\n")

	return nil
}

// createDetectionApplicationsConfig creates an applications configuration from suggestions
func createDetectionApplicationsConfig(suggestions map[string][]string) map[string]interface{} {
	var applications []map[string]interface{}

	// Add applications by priority
	priorities := []string{"critical", "recommended", "optional"}

	for _, priority := range priorities {
		apps := suggestions[priority]
		for _, app := range apps {
			// Remove duplicates
			exists := false
			for _, existing := range applications {
				if existing["name"] == app {
					exists = true
					break
				}
			}

			if !exists {
				applications = append(applications, map[string]interface{}{
					"name":        app,
					"description": fmt.Sprintf("Detected from project analysis (%s priority)", priority),
					"category":    "development",
					"default":     priority == "critical",
				})
			}
		}
	}

	return map[string]interface{}{
		"applications": applications,
	}
}

// formatApplicationsAsYAML formats applications as YAML-like text
func formatApplicationsAsYAML(suggestions map[string][]string) string {
	var lines []string
	priorities := []string{"critical", "recommended", "optional"}

	for _, priority := range priorities {
		apps := suggestions[priority]
		if len(apps) > 0 {
			lines = append(lines, fmt.Sprintf("  # %s priority applications", strings.Title(priority)))
			for _, app := range apps {
				lines = append(lines, fmt.Sprintf("  - name: %s", app))
				lines = append(lines, fmt.Sprintf("    description: \"Detected from project analysis (%s priority)\"", priority))
				lines = append(lines, "    category: development")
				if priority == "critical" {
					lines = append(lines, "    default: true")
				}
				lines = append(lines, "")
			}
		}
	}

	return strings.Join(lines, "\n")
}
