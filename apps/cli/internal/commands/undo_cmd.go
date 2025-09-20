package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
)

// NewUndoCmd creates a new undo command
func NewUndoCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		force  bool
		format string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "undo [operation-id]",
		Short: "Undo recent configuration changes",
		Long: `Undo recent configuration changes using backup and version history.

The undo system tracks all configuration operations and allows you to safely
rollback changes. You can undo the last operation or specify a particular
operation by its ID.

Examples:
  # Undo the most recent operation
  devex undo
  
  # Undo a specific operation
  devex undo add-20240817-143022
  
  # List available operations to undo
  devex undo --list
  
  # Force undo without confirmation
  devex undo --force
  
  # Show undo system status
  devex undo --status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			undoManager := undo.NewUndoManager(baseDir)

			// Handle list flag
			if cmd.Flags().Changed("list") {
				return listUndoOperations(undoManager, format, limit)
			}

			// Handle status flag
			if cmd.Flags().Changed("status") {
				return showUndoStatus(undoManager, format)
			}

			// Handle specific operation ID
			if len(args) > 0 {
				return undoSpecificOperation(undoManager, args[0], force)
			}

			// Default: undo last operation
			return undoLastOperation(undoManager, force)
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation and ignore risks")
	cmd.Flags().StringVar(&format, "format", "table", "Output format for list/status (table, json, yaml)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of operations to show in list")
	cmd.Flags().Bool("list", false, "List available operations to undo")
	cmd.Flags().Bool("status", false, "Show undo system status")

	return cmd
}

func listUndoOperations(undoManager *undo.UndoManager, format string, limit int) error {
	operations, err := undoManager.GetUndoableOperations(limit)
	if err != nil {
		return fmt.Errorf("failed to get undoable operations: %w", err)
	}

	if len(operations) == 0 {
		fmt.Println("No operations available to undo")
		return nil
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(operations, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(operations)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))
	default:
		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()
		red := color.New(color.FgRed).SprintFunc()
		gray := color.New(color.FgHiBlack).SprintFunc()

		fmt.Printf("Recent undoable operations (showing %d):\n\n", len(operations))
		for i, op := range operations {
			fmt.Printf("%s %s", green("●"), op.Operation)
			if !op.CanUndo {
				fmt.Printf(" %s", red("(cannot undo)"))
			}
			fmt.Println()

			fmt.Printf("    ID: %s\n", gray(op.ID))
			fmt.Printf("    Time: %s\n", op.Timestamp.Format("2006-01-02 15:04:05"))
			fmt.Printf("    Description: %s\n", op.Description)
			if op.Target != "" {
				fmt.Printf("    Target: %s\n", op.Target)
			}

			if len(op.UndoRisks) > 0 {
				fmt.Printf("    %s Risks: %s\n", yellow("⚠"), strings.Join(op.UndoRisks, ", "))
			}

			if i < len(operations)-1 {
				fmt.Println()
			}
		}

		fmt.Printf("\nUse 'devex undo <operation-id>' to undo a specific operation\n")
	}

	return nil
}

func showUndoStatus(undoManager *undo.UndoManager, format string) error {
	summary, err := undoManager.GetUndoSummary()
	if err != nil {
		return fmt.Errorf("failed to get undo summary: %w", err)
	}

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
		green := color.New(color.FgGreen).SprintFunc()
		blue := color.New(color.FgBlue).SprintFunc()
		gray := color.New(color.FgHiBlack).SprintFunc()

		fmt.Printf("%s Undo System Status\n\n", blue("●"))

		if summary.CanUndo {
			fmt.Printf("Status: %s\n", green("Ready"))
		} else {
			fmt.Printf("Status: %s\n", gray("No operations to undo"))
		}

		fmt.Printf("Total operations: %d\n", summary.TotalOperations)
		fmt.Printf("Undoable operations: %d\n", summary.UndoableOperations)

		if summary.LastOperation != nil {
			fmt.Printf("Last operation: %s\n", *summary.LastOperation)
		}

		if summary.LastUndo != nil {
			fmt.Printf("Last undo: %s\n", *summary.LastUndo)
		}

		if len(summary.RecentOperations) > 0 {
			fmt.Println("\nRecent operations:")
			for _, op := range summary.RecentOperations {
				fmt.Printf("  • %s\n", op)
			}
		}

		if summary.CanUndo {
			fmt.Printf("\nUse 'devex undo' to undo the last operation\n")
			fmt.Printf("Use 'devex undo --list' to see all undoable operations\n")
		}
	}

	return nil
}

func undoSpecificOperation(undoManager *undo.UndoManager, operationID string, force bool) error {
	// Get operation details
	operation, err := undoManager.GetOperationDetails(operationID)
	if err != nil {
		return fmt.Errorf("failed to get operation details: %w", err)
	}

	// Confirm unless forced
	if !force {
		fmt.Printf("Undo operation: %s\n", operation.Description)
		fmt.Printf("Target: %s\n", operation.Target)
		fmt.Printf("Time: %s\n", operation.Timestamp.Format("2006-01-02 15:04:05"))

		if len(operation.UndoRisks) > 0 {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("\n%s Risks:\n", yellow("⚠"))
			for _, risk := range operation.UndoRisks {
				fmt.Printf("  • %s\n", risk)
			}
		}

		fmt.Printf("\nContinue? [y/N]: ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			response = "n" // Default to no on input error
		}
		if strings.ToLower(response) != "y" {
			fmt.Println("Undo cancelled")
			return nil
		}
	}

	// Perform the undo
	result, err := undoManager.UndoOperation(operationID, force)
	if err != nil {
		return fmt.Errorf("failed to undo operation: %w", err)
	}

	return displayUndoResult(result)
}

func undoLastOperation(undoManager *undo.UndoManager, force bool) error {
	// Check if there are operations to undo
	canUndo, err := undoManager.CanUndo()
	if err != nil {
		return fmt.Errorf("failed to check undo availability: %w", err)
	}

	if !canUndo {
		fmt.Println("No operations available to undo")
		fmt.Println("Use 'devex undo --status' to see undo system status")
		return nil
	}

	// Get the last operation for confirmation
	operations, err := undoManager.GetUndoableOperations(1)
	if err != nil {
		return fmt.Errorf("failed to get operations: %w", err)
	}

	if len(operations) == 0 {
		fmt.Println("No operations available to undo")
		return nil
	}

	lastOp := operations[0]

	// Confirm unless forced
	if !force {
		fmt.Printf("Undo operation: %s\n", lastOp.Description)
		fmt.Printf("Target: %s\n", lastOp.Target)
		fmt.Printf("Time: %s\n", lastOp.Timestamp.Format("2006-01-02 15:04:05"))

		if len(lastOp.UndoRisks) > 0 {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("\n%s Risks:\n", yellow("⚠"))
			for _, risk := range lastOp.UndoRisks {
				fmt.Printf("  • %s\n", risk)
			}
		}

		fmt.Printf("\nContinue? [y/N]: ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			response = "n" // Default to no on input error
		}
		if strings.ToLower(response) != "y" {
			fmt.Println("Undo cancelled")
			return nil
		}
	}

	// Perform the undo
	result, err := undoManager.UndoLast(force)
	if err != nil {
		return fmt.Errorf("failed to undo operation: %w", err)
	}

	return displayUndoResult(result)
}

func displayUndoResult(result *undo.UndoResult) error {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("%s %s\n", green("✓"), result.Message)
	fmt.Printf("Restored from: %s\n", result.RestoredFrom)

	if result.NewBackupID != "" {
		fmt.Printf("Pre-undo backup: %s\n", result.NewBackupID)
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n%s Warnings:\n", yellow("⚠"))
		for _, warning := range result.Warnings {
			fmt.Printf("  • %s\n", warning)
		}
	}

	fmt.Printf("\nYou can now run 'devex config show' to verify the changes\n")
	return nil
}
