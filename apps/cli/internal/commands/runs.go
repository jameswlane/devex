package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/state"
)

func NewRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Inspect recorded template lifecycle runs",
		Long:  "Inspect recorded template lifecycle runs, operations, and managed resources from the local DevEx state database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newRunsListCmd())
	cmd.AddCommand(newRunsShowCmd())
	cmd.AddCommand(newRunsResourcesCmd())
	cmd.AddCommand(newRunsOperationsCmd())
	return cmd
}

func newRunsListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recorded template lifecycle runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openTemplateStateStore()
			if err != nil {
				return err
			}
			defer store.Close()

			runs, err := store.ListRuns(cmd.Context(), state.RunFilter{})
			if err != nil {
				return err
			}
			sortRuns(runs)
			return displayRunsList(runs, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	return cmd
}

func newRunsShowCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show a recorded template lifecycle run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openTemplateStateStore()
			if err != nil {
				return err
			}
			defer store.Close()

			run, err := store.GetRun(cmd.Context(), args[0])
			if err != nil {
				return formatStateNotFound("run", args[0], err)
			}
			return displayRun(*run, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	return cmd
}

func newRunsResourcesCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "resources <run-id>",
		Short: "List resources recorded for a template lifecycle run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openTemplateStateStore()
			if err != nil {
				return err
			}
			defer store.Close()

			if _, err := store.GetRun(cmd.Context(), args[0]); err != nil {
				return formatStateNotFound("run", args[0], err)
			}
			resources, err := store.ListResourcesByRun(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			resources = sortTemplateResources(resources)
			return displayRunResources(resources, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	return cmd
}

func newRunsOperationsCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "operations <run-id>",
		Short: "List operations recorded for a template lifecycle run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openTemplateStateStore()
			if err != nil {
				return err
			}
			defer store.Close()

			if _, err := store.GetRun(cmd.Context(), args[0]); err != nil {
				return formatStateNotFound("run", args[0], err)
			}
			operations, err := store.ListOperationsByRun(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			sortOperations(operations)
			return displayRunOperations(operations, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	return cmd
}

func sortRuns(runs []state.TemplateRun) {
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].StartedAt.Equal(runs[j].StartedAt) {
			return runs[i].ID < runs[j].ID
		}
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
}

func sortOperations(operations []state.OperationRecord) {
	sort.SliceStable(operations, func(i, j int) bool {
		if operations[i].StartedAt.Equal(operations[j].StartedAt) {
			return operations[i].ID < operations[j].ID
		}
		return operations[i].StartedAt.Before(operations[j].StartedAt)
	})
}

func displayRunsList(runs []state.TemplateRun, format string) error {
	format = normalizeFormat(format)
	switch format {
	case "json":
		return printJSON(runs)
	case "yaml":
		return printYAML(runs)
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "RUN ID\tTEMPLATE\tACTION\tSTATUS\tSTARTED\tCOMPLETED")
		fmt.Fprintln(w, "------\t--------\t------\t------\t-------\t---------")
		for _, run := range runs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				run.ID,
				run.TemplateID,
				run.Action,
				run.Status,
				run.StartedAt.Format("2006-01-02 15:04:05"),
				formatOptionalTime(run.CompletedAt),
			)
		}
		return w.Flush()
	default:
		return unsupportedFormat(format)
	}
}

func displayRun(run state.TemplateRun, format string) error {
	format = normalizeFormat(format)
	switch format {
	case "json":
		return printJSON(run)
	case "yaml":
		return printYAML(run)
	case "table":
		fmt.Printf("Run ID: %s\n", run.ID)
		fmt.Printf("Template: %s@%s\n", run.TemplateID, run.TemplateVersion)
		fmt.Printf("Action: %s\n", run.Action)
		fmt.Printf("Status: %s\n", run.Status)
		fmt.Printf("Started: %s\n", run.StartedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Completed: %s\n", formatOptionalTime(run.CompletedAt))
		if run.Error != "" {
			fmt.Printf("Error: %s\n", run.Error)
		}
		fmt.Printf("Steps: %d\n", len(run.Plan.Steps))
		return nil
	default:
		return unsupportedFormat(format)
	}
}

func displayRunResources(resources []state.ManagedResource, format string) error {
	format = normalizeFormat(format)
	switch format {
	case "json":
		return printJSON(resources)
	case "yaml":
		return printYAML(resources)
	case "table":
		return printTemplateResourceTable(resources)
	default:
		return unsupportedFormat(format)
	}
}

func displayRunOperations(operations []state.OperationRecord, format string) error {
	format = normalizeFormat(format)
	switch format {
	case "json":
		return printJSON(operations)
	case "yaml":
		return printYAML(operations)
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "OPERATION ID\tSTEP\tACTION\tREQUEST\tSTATUS\tERROR")
		fmt.Fprintln(w, "------------\t----\t------\t-------\t------\t-----")
		for _, operation := range operations {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				operation.ID,
				operation.StepID,
				operation.Action,
				formatExecutionRequest(operation.Request.Plugin, operation.Request.Command, operation.Request.Args),
				operation.Status,
				operation.Error,
			)
		}
		return w.Flush()
	default:
		return unsupportedFormat(format)
	}
}

func normalizeFormat(format string) string {
	return strings.ToLower(strings.TrimSpace(format))
}

func formatExecutionRequest(plugin string, command string, args []string) string {
	parts := make([]string, 0, 2+len(args))
	if plugin != "" {
		parts = append(parts, plugin)
	}
	if command != "" {
		parts = append(parts, command)
	}
	parts = append(parts, args...)
	return strings.Join(parts, " ")
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05")
}

func printJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printYAML(value any) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func unsupportedFormat(format string) error {
	return fmt.Errorf("unsupported format %q (expected table, json, or yaml)", format)
}

func formatStateNotFound(kind string, id string, err error) error {
	if errors.Is(err, state.ErrNotFound) {
		return fmt.Errorf("%s '%s' not found", kind, id)
	}
	return err
}
