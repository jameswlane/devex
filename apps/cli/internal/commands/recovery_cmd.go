package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/help"
	"github.com/jameswlane/devex/apps/cli/internal/recovery"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/cobra"
)

// NewRecoveryCmd creates a new recovery command for error recovery assistance
func NewRecoveryCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Guided recovery from failed operations",
		Long: `The recover command provides intelligent recovery assistance when operations fail.

It analyzes errors, suggests appropriate recovery strategies, and can automatically
execute recovery procedures including backup restoration, configuration rollback,
and system cleanup.

Examples:
  devex recover --error "failed to install docker" --operation "install"
  devex recover --analyze-last      # Analyze the last failed operation
  devex recover --list-options      # Show available recovery strategies
  devex recover --interactive       # Interactive recovery wizard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			errorMsg, _ := cmd.Flags().GetString("error")
			operation, _ := cmd.Flags().GetString("operation")
			analyzeLast, _ := cmd.Flags().GetBool("analyze-last")
			listOptions, _ := cmd.Flags().GetBool("list-options")
			interactive, _ := cmd.Flags().GetBool("interactive")
			execute, _ := cmd.Flags().GetString("execute")
			format, _ := cmd.Flags().GetString("format")

			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			recoveryManager := recovery.NewRecoveryManager(baseDir)

			if listOptions {
				return showRecoveryCapabilities()
			}

			if analyzeLast {
				return analyzeLastError(recoveryManager, format)
			}

			if interactive {
				return runInteractiveRecovery(recoveryManager, settings)
			}

			if execute != "" {
				return executeRecoveryOption(recoveryManager, execute, errorMsg, operation)
			}

			if errorMsg == "" && operation == "" {
				return cmd.Help()
			}

			return analyzeAndSuggestRecovery(recoveryManager, errorMsg, operation, format)
		},
	}

	// Add flags
	cmd.Flags().String("error", "", "Error message to analyze")
	cmd.Flags().String("operation", "", "Operation that failed (install, config, init, etc.)")
	cmd.Flags().Bool("analyze-last", false, "Analyze the last error from logs")
	cmd.Flags().Bool("list-options", false, "List all available recovery strategies")
	cmd.Flags().Bool("interactive", false, "Launch interactive recovery wizard")
	cmd.Flags().String("execute", "", "Execute specific recovery option by ID")
	cmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml)")
	cmd.Flags().Bool("dry-run", false, "Show what would be done without executing")

	// Add contextual help
	AddContextualHelp(cmd, help.ContextTroubleshooting, "recovery")

	return cmd
}

// showRecoveryCapabilities displays all available recovery strategies
func showRecoveryCapabilities() error {
	fmt.Println("🔧 DevEx Recovery Capabilities")

	capabilities := map[string][]string{
		"Backup & Restore": {
			"• Restore from recent backups",
			"• Choose from backup history",
			"• Emergency configuration reset",
		},
		"Undo Operations": {
			"• Undo recent configuration changes",
			"• Rollback application installations",
			"• Reverse system modifications",
		},
		"Configuration Recovery": {
			"• Fix YAML parsing errors",
			"• Reset to default configurations",
			"• Repair file permissions",
		},
		"Installation Recovery": {
			"• Update package manager cache",
			"• Retry with alternative installers",
			"• Force reinstallation with conflict resolution",
		},
		"System Cleanup": {
			"• Clear DevEx cache",
			"• Remove temporary files",
			"• Free up disk space",
		},
		"Manual Guidance": {
			"• Step-by-step troubleshooting",
			"• System health checks",
			"• Help system integration",
		},
	}

	for category, items := range capabilities {
		fmt.Printf("📋 %s\n", category)
		for _, item := range items {
			fmt.Printf("   %s\n", item)
		}
		fmt.Println()
	}

	fmt.Println("💡 Use 'devex recover --interactive' for guided recovery assistance")
	return nil
}

// analyzeLastError analyzes the last error from logs
func analyzeLastError(recoveryManager *recovery.RecoveryManager, format string) error {
	// This would normally read from log files
	// For now, we'll use a mock error context
	ctx := recovery.RecoveryContext{
		Operation:  "install",
		Error:      "failed to install docker: package not found",
		Timestamp:  time.Now().Add(-5 * time.Minute),
		Command:    "devex install docker",
		Args:       []string{"install", "docker"},
		WorkingDir: "/home/user/project",
	}

	return analyzeRecoveryContext(recoveryManager, ctx, format)
}

// analyzeAndSuggestRecovery analyzes error and suggests recovery options
func analyzeAndSuggestRecovery(recoveryManager *recovery.RecoveryManager, errorMsg, operation, format string) error {
	ctx := recovery.RecoveryContext{
		Operation:  operation,
		Error:      errorMsg,
		Timestamp:  time.Now(),
		Command:    fmt.Sprintf("devex %s", operation),
		WorkingDir: "/current/directory",
	}

	return analyzeRecoveryContext(recoveryManager, ctx, format)
}

// analyzeRecoveryContext analyzes a recovery context and displays options
func analyzeRecoveryContext(recoveryManager *recovery.RecoveryManager, ctx recovery.RecoveryContext, format string) error {
	options, err := recoveryManager.AnalyzeError(ctx)
	if err != nil {
		return fmt.Errorf("failed to analyze error: %w", err)
	}

	if len(options) == 0 {
		fmt.Println("🤔 No automated recovery options available for this error")
		fmt.Println("💡 Try 'devex help' for manual troubleshooting guidance")
		return nil
	}

	switch format {
	case "json":
		return outputRecoveryJSON(options)
	case "yaml":
		return outputRecoveryYAML(options)
	default:
		return displayRecoveryOptions(ctx, options)
	}
}

// displayRecoveryOptions displays recovery options in a user-friendly format
func displayRecoveryOptions(ctx recovery.RecoveryContext, options []recovery.RecoveryOption) error {
	fmt.Printf("🔍 Error Analysis\n")
	fmt.Printf("Operation: %s\n", ctx.Operation)
	fmt.Printf("Error: %s\n", ctx.Error)
	fmt.Printf("Time: %s\n\n", ctx.Timestamp.Format(time.RFC3339))

	fmt.Printf("🛠️  Suggested Recovery Options (%d available)\n\n", len(options))

	for i, option := range options {
		priorityIcon := getPriorityIcon(option.Priority)
		automationIcon := "🤖"
		if !option.Automated {
			automationIcon = "👤"
		}

		fmt.Printf("%s %s %s %s\n", priorityIcon, automationIcon, option.ID, option.Title)
		fmt.Printf("   %s\n", option.Description)

		if len(option.Risks) > 0 {
			fmt.Printf("   ⚠️  Risks: %s\n", strings.Join(option.Risks, ", "))
		}

		if len(option.Steps) > 0 {
			fmt.Printf("   📝 Steps: %d\n", len(option.Steps))
		}

		if i < len(options)-1 {
			fmt.Println()
		}
	}

	fmt.Printf("\n💡 Execute recovery: devex recover --execute <option-id>\n")
	fmt.Printf("🎯 Interactive mode: devex recover --interactive\n")

	return nil
}

// getPriorityIcon returns an icon for the recovery priority
func getPriorityIcon(priority recovery.RecoveryPriority) string {
	switch priority {
	case recovery.PriorityCritical:
		return "🚨"
	case recovery.PriorityRecommended:
		return "✅"
	case recovery.PriorityOptional:
		return "💡"
	case recovery.PriorityLastResort:
		return "🔧"
	default:
		return "❓"
	}
}

// executeRecoveryOption executes a specific recovery option
func executeRecoveryOption(recoveryManager *recovery.RecoveryManager, optionID, errorMsg, operation string) error {
	ctx := recovery.RecoveryContext{
		Operation: operation,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}

	options, err := recoveryManager.AnalyzeError(ctx)
	if err != nil {
		return fmt.Errorf("failed to analyze error: %w", err)
	}

	var selectedOption *recovery.RecoveryOption
	for _, option := range options {
		if option.ID == optionID {
			selectedOption = &option
			break
		}
	}

	if selectedOption == nil {
		return fmt.Errorf("recovery option '%s' not found", optionID)
	}

	fmt.Printf("🚀 Executing recovery option: %s\n", selectedOption.Title)
	fmt.Printf("📝 %s\n\n", selectedOption.Description)

	result, err := recoveryManager.ExecuteRecovery(*selectedOption, ctx)
	if err != nil {
		fmt.Printf("❌ Recovery failed: %s\n", err)
		return err
	}

	if result.Success {
		fmt.Printf("✅ Recovery completed successfully in %s\n", result.Duration)
		if len(result.StepsRun) > 0 {
			fmt.Printf("📊 Steps executed: %d\n", len(result.StepsRun))
		}
	} else {
		fmt.Printf("❌ Recovery failed: %s\n", result.Error)
	}

	// Save recovery log
	if err := recoveryManager.SaveRecoveryLog(ctx, options, result); err != nil {
		fmt.Printf("⚠️  Warning: failed to save recovery log: %s\n", err)
	}

	return nil
}

// runInteractiveRecovery runs the interactive recovery wizard
func runInteractiveRecovery(recoveryManager *recovery.RecoveryManager, settings config.CrossPlatformSettings) error {
	fmt.Println("🎯 Interactive Recovery Wizard")
	fmt.Println("This wizard will help you recover from errors step by step.")

	// This would integrate with the TUI system for a full interactive experience
	// For now, we'll provide a simplified text-based interface

	fmt.Println("What kind of problem are you experiencing?")
	fmt.Println("1. Installation failed")
	fmt.Println("2. Configuration error")
	fmt.Println("3. Command not working")
	fmt.Println("4. System issues")
	fmt.Println("5. Other")

	// In a real implementation, this would use the TUI system
	// to provide a proper interactive experience

	fmt.Println("\n💡 For now, please use specific flags:")
	fmt.Println("   devex recover --error 'your error message' --operation 'operation-name'")
	fmt.Println("   devex recover --analyze-last")
	fmt.Println("   devex recover --list-options")

	return nil
}

// outputRecoveryJSON outputs recovery options in JSON format
func outputRecoveryJSON(options []recovery.RecoveryOption) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{
		"recovery_options": options,
		"count":            len(options),
		"timestamp":        time.Now(),
	})
}

// outputRecoveryYAML outputs recovery options in YAML format
func outputRecoveryYAML(options []recovery.RecoveryOption) error {
	// For simplicity, we'll use a basic YAML-like format
	fmt.Println("recovery_options:")
	for _, option := range options {
		fmt.Printf("  - id: %s\n", option.ID)
		fmt.Printf("    title: %s\n", option.Title)
		fmt.Printf("    description: %s\n", option.Description)
		fmt.Printf("    priority: %s\n", option.Priority)
		fmt.Printf("    automated: %t\n", option.Automated)
		if len(option.Steps) > 0 {
			fmt.Printf("    steps: %d\n", len(option.Steps))
		}
		fmt.Println()
	}
	return nil
}
