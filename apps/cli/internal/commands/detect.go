package commands

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
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
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("%s Technology Detection\n\n", cyan("🔍"))
	fmt.Printf("%s This feature requires the stack-detector plugin.\n", yellow("⚠️"))
	fmt.Printf("To install and use the stack detector:\n\n")
	fmt.Printf("1. The plugin will be automatically downloaded on first use\n")
	fmt.Printf("2. Run: devex plugin run tool-stackdetector detect\n")
	fmt.Printf("3. Or configure auto-detection in your devex config\n\n")
	fmt.Printf("For more information, see: https://docs.devex.sh/plugins/stack-detector\n")

	return nil
}

// runDetectAnalyze runs the analyze subcommand
func runDetectAnalyze(dir string, settings config.CrossPlatformSettings) error {
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("%s Detailed Technology Analysis\n\n", cyan("🔬"))
	fmt.Printf("%s This feature requires the stack-detector plugin.\n", yellow("⚠️"))
	fmt.Printf("Run: devex plugin run tool-stackdetector analyze %s\n", dir)

	return nil
}

// runDetectSuggest runs the suggest subcommand
func runDetectSuggest(settings config.CrossPlatformSettings) error {
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("%s Configuration Suggestions\n\n", cyan("💡"))
	fmt.Printf("%s This feature requires the stack-detector plugin.\n", yellow("⚠️"))
	fmt.Printf("Run: devex plugin run tool-stackdetector suggest\n")

	return nil
}

// runDetectApply runs the apply subcommand
func runDetectApply(settings config.CrossPlatformSettings) error {
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("%s Applying Detection Suggestions\n\n", cyan("🔧"))
	fmt.Printf("%s This feature requires the stack-detector plugin.\n", yellow("⚠️"))
	fmt.Printf("Run: devex plugin run tool-stackdetector apply\n")

	return nil
}
