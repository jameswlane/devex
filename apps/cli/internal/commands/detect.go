package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/stackdetect"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// newStackDetectExecutor returns a plugin executor with tool-stackdetector
// installed. Only that plugin is downloaded, and only when it is missing.
var newStackDetectExecutor = func(ctx context.Context) (stackdetect.Executor, error) {
	plugins, err := bootstrap.NewPluginBootstrap(false)
	if err != nil {
		return nil, fmt.Errorf("initialize plugins: %w", err)
	}
	if _, err := plugins.EnsurePluginWithContext(ctx, stackdetect.PluginName); err != nil {
		return nil, err
	}
	return plugins, nil
}

// detectCommand reports whether cmd is detect or one of its subcommands; they
// initialize only the stack detector plugin themselves.
func detectCommand(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current.Name() == "detect" && current.Parent() != nil && current.Parent().Parent() == nil {
			return true
		}
	}
	return false
}

type detectOptions struct {
	format     string
	confidence float64
	category   string
}

func (o *detectOptions) register(cmd *cobra.Command, categoryHelp string) {
	cmd.Flags().StringVar(&o.format, "format", "table", "Output format (table, json)")
	cmd.Flags().Float64Var(&o.confidence, "confidence", 0.5, "Minimum confidence, from 0.0 to 1.0")
	cmd.Flags().StringVar(&o.category, "category", "", categoryHelp)
}

func (o *detectOptions) validate() error {
	if o.format != "table" && o.format != "json" {
		return fmt.Errorf("unsupported format %q; use table or json", o.format)
	}
	return stackdetect.ValidateConfidence(o.confidence)
}

func dirArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

func runStackReport(ctx context.Context, cmd *cobra.Command, args []string) (stackdetect.Report, error) {
	dir, err := stackdetect.ResolveDir(dirArg(args))
	if err != nil {
		return stackdetect.Report{}, err
	}
	executor, err := newStackDetectExecutor(ctx)
	if err != nil {
		return stackdetect.Report{}, err
	}
	return stackdetect.Run(ctx, executor, dir, cmd.ErrOrStderr())
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// NewDetectCmd creates the detect command group for technology stack detection.
func NewDetectCmd(_ types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		opts     detectOptions
		detailed bool
		output   string
	)
	cmd := &cobra.Command{
		Use:   "detect [directory]",
		Short: "Detect the technologies a project uses",
		Long: `Scan a project directory and report the languages, frameworks, build tools
and infrastructure it uses. Detection runs the tool-stackdetector plugin,
which DevEx downloads the first time it is needed (not with --offline).

Detection only reads the project. Use 'devex detect suggest' to see which
DevEx applications match the project, and 'devex detect apply' to add them to
your configuration.`,
		Example: `  # Detect technologies in the current directory
  devex detect

  # Only high-confidence languages, as JSON
  devex detect ./service --confidence 0.8 --category Language --format json

  # Save the full detection result
  devex detect --output detection.json`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			report, err := runStackReport(cmd.Context(), cmd, args)
			if err != nil {
				return err
			}
			result := struct {
				ProjectPath  string                   `json:"project_path"`
				Summary      stackdetect.Summary      `json:"summary"`
				Technologies []stackdetect.Technology `json:"technologies"`
			}{report.ProjectPath, report.Summary, stackdetect.Filter(report.Technologies, opts.confidence, opts.category)}
			if output != "" {
				file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
				if err != nil {
					return fmt.Errorf("save detection results: %w", err)
				}
				writeErr := writeJSON(file, result)
				if closeErr := file.Close(); writeErr == nil {
					writeErr = closeErr
				}
				if writeErr != nil {
					return fmt.Errorf("save detection results: %w", writeErr)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Saved detection results to %s\n", output)
			}
			if opts.format == "json" {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			printTechnologies(cmd.OutOrStdout(), result.ProjectPath, result.Summary, result.Technologies, detailed)
			return nil
		},
	}
	opts.register(cmd, "Only technologies in this category (for example Language, Framework)")
	cmd.Flags().BoolVar(&detailed, "detailed", false, "Show the files that identified each technology")
	cmd.Flags().StringVar(&output, "output", "", "Also save the detection result as JSON to this file")

	cmd.AddCommand(newDetectAnalyzeCmd())
	cmd.AddCommand(newDetectSuggestCmd(settings))
	cmd.AddCommand(newDetectApplyCmd(settings))
	return cmd
}

func printTechnologies(out io.Writer, project string, summary stackdetect.Summary, technologies []stackdetect.Technology, detailed bool) {
	fmt.Fprintf(out, "Project: %s\n", project)
	if summary.PrimaryLanguage != "" {
		fmt.Fprintf(out, "Primary language: %s\n", summary.PrimaryLanguage)
	}
	if summary.ProjectType != "" {
		fmt.Fprintf(out, "Project type: %s\n", summary.ProjectType)
	}
	fmt.Fprintln(out)
	if len(technologies) == 0 {
		fmt.Fprintln(out, "No technologies detected at this confidence.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TECHNOLOGY\tCATEGORY\tCONFIDENCE")
	for _, technology := range technologies {
		fmt.Fprintf(w, "%s\t%s\t%d/10\n", technology.Name, technology.Category, technology.Confidence)
		if detailed && len(technology.Files) > 0 {
			fmt.Fprintf(w, "  files: %s\t\t\n", strings.Join(technology.Files, ", "))
		}
	}
	_ = w.Flush()
}

func newDetectAnalyzeCmd() *cobra.Command {
	var opts detectOptions
	cmd := &cobra.Command{
		Use:   "analyze [directory]",
		Short: "Show a full project analysis",
		Long: `Show the full analysis of a project: detected technologies, declared
dependencies, project size, issues and the plugin's recommendations.`,
		Example: `  devex detect analyze
  devex detect analyze ./service --format json`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			report, err := runStackReport(cmd.Context(), cmd, args)
			if err != nil {
				return err
			}
			report.Technologies = stackdetect.Filter(report.Technologies, opts.confidence, opts.category)
			if opts.format == "json" {
				return writeJSON(cmd.OutOrStdout(), report)
			}
			out := cmd.OutOrStdout()
			printTechnologies(out, report.ProjectPath, report.Summary, report.Technologies, false)
			size := report.ProjectSize
			fmt.Fprintf(out, "\nFiles: %d (%d code, %d config, %d docs), %d lines of code\n", size.TotalFiles, size.CodeFiles, size.ConfigFiles, size.DocumentFiles, size.LinesOfCode)
			fmt.Fprintf(out, "Dependencies: %d\n", len(report.Dependencies))
			for _, section := range []struct {
				title string
				items []string
			}{{"Issues", report.Issues}, {"Recommendations", report.Recommendations}} {
				if len(section.items) == 0 {
					continue
				}
				fmt.Fprintf(out, "\n%s:\n", section.title)
				for _, item := range section.items {
					fmt.Fprintf(out, "  - %s\n", item)
				}
			}
			return nil
		},
	}
	opts.register(cmd, "Only technologies in this category")
	return cmd
}

// detectSuggestions runs detection and maps the result to catalog applications.
func detectSuggestions(cmd *cobra.Command, args []string, settings config.CrossPlatformSettings, opts detectOptions) ([]stackdetect.Suggestion, error) {
	report, err := runStackReport(cmd.Context(), cmd, args)
	if err != nil {
		return nil, err
	}
	configuredApps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
	if err != nil {
		return nil, fmt.Errorf("read your application configuration: %w", err)
	}
	configured := map[string]bool{}
	for _, app := range configuredApps {
		configured[strings.ToLower(app.Name)] = true
	}
	technologies := stackdetect.Filter(report.Technologies, opts.confidence, opts.category)
	return stackdetect.Suggest(technologies, settings.GetAllApps(), configured), nil
}

func newDetectSuggestCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var opts detectOptions
	cmd := &cobra.Command{
		Use: "suggest [directory]",
		// Suggestions resolve against the application catalog, like devex add.
		Annotations: map[string]string{catalogAnnotation: catalogRequired},
		Short:       "Suggest DevEx applications for a project",
		Long: `Suggest the DevEx applications that match the technologies detected in a
project. Applications already in your configuration are marked as configured.
Nothing is changed; use 'devex detect apply' to add the suggestions.`,
		Example: `  devex detect suggest
  devex detect suggest --category Language --format json`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			suggestions, err := detectSuggestions(cmd, args, settings, opts)
			if err != nil {
				return err
			}
			if opts.format == "json" {
				return writeJSON(cmd.OutOrStdout(), suggestions)
			}
			printSuggestions(cmd.OutOrStdout(), suggestions)
			return nil
		},
	}
	opts.register(cmd, "Only suggestions from technologies in this category")
	return cmd
}

func printSuggestions(out io.Writer, suggestions []stackdetect.Suggestion) {
	if len(suggestions) == 0 {
		fmt.Fprintln(out, "No DevEx applications match the detected technologies.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "APPLICATION\tCONFIDENCE\tDETECTED\tSTATUS")
	for _, suggestion := range suggestions {
		status := "suggested"
		if suggestion.Configured {
			status = "configured"
		}
		fmt.Fprintf(w, "%s\t%.0f%%\t%s\t%s\n", suggestion.App, suggestion.Confidence*100, strings.Join(suggestion.Technologies, ", "), status)
	}
	_ = w.Flush()
}

// DetectApplyResult reports what detect apply added or would add.
type DetectApplyResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"`
	DryRun  bool     `json:"dry_run"`
}

func newDetectApplyCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		opts   detectOptions
		dryRun bool
		yes    bool
	)
	cmd := &cobra.Command{
		Use: "apply [directory]",
		// Suggestions resolve against the application catalog, like devex add.
		Annotations: map[string]string{catalogAnnotation: catalogRequired},
		Short:       "Add suggested applications to your configuration",
		Long: `Add the DevEx applications suggested for a project to your configuration,
with their dependencies. This changes configuration only; run 'devex install'
afterwards to install them.

The applications are shown first. Each addition backs up your configuration
and is validated like 'devex add'. Applications already configured are
skipped. Without a terminal, --yes is required.`,
		Example: `  # Preview the additions
  devex detect apply --dry-run

  # Add high-confidence suggestions without prompting
  devex detect apply --confidence 0.8 --yes`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			suggestions, err := detectSuggestions(cmd, args, settings, opts)
			if err != nil {
				return err
			}
			result := DetectApplyResult{Added: []string{}, Skipped: []string{}, DryRun: dryRun}
			var pending []string
			for _, suggestion := range suggestions {
				if suggestion.Configured {
					result.Skipped = append(result.Skipped, suggestion.App)
					continue
				}
				pending = append(pending, suggestion.App)
			}
			stderr := cmd.ErrOrStderr()
			if len(pending) == 0 {
				fmt.Fprintln(stderr, "Nothing to add: every suggested application is already configured, or none matched.")
				return writeApplyResult(cmd, opts.format, result)
			}
			fmt.Fprintf(stderr, "Configuration preview: add %s\n", strings.Join(pending, ", "))
			if dryRun {
				result.Added = pending
				return writeApplyResult(cmd, opts.format, result)
			}
			if !yes {
				if !interaction.IsInteractiveMode() || opts.format == "json" {
					return fmt.Errorf("confirmation required; review with --dry-run, then use --yes to add these applications")
				}
				fmt.Fprint(stderr, "Add these applications and their dependencies to your configuration? [y/N]: ")
				answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if err != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
					return fmt.Errorf("apply cancelled; configuration unchanged")
				}
			}
			for _, name := range pending {
				if err := cmd.Context().Err(); err != nil {
					return err
				}
				plan, err := planAddition(cmd.Context(), settings, name)
				if err != nil {
					// A dependency added for an earlier application is already present.
					if strings.Contains(err.Error(), "already in your configuration") {
						result.Skipped = append(result.Skipped, name)
						continue
					}
					return applyFailure(cmd, opts.format, result, fmt.Errorf("add %s: %w", name, err))
				}
				if err := saveAddition(settings, plan); err != nil {
					return applyFailure(cmd, opts.format, result, fmt.Errorf("add %s: %w", name, err))
				}
				for _, app := range plan {
					result.Added = append(result.Added, app.Name)
				}
			}
			return writeApplyResult(cmd, opts.format, result)
		},
	}
	opts.register(cmd, "Only suggestions from technologies in this category")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be added without changing configuration")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Add without prompting")
	return cmd
}

// applyFailure reports what was added before err, then returns err.
func applyFailure(cmd *cobra.Command, format string, result DetectApplyResult, err error) error {
	if len(result.Added) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Added before the failure: %s\n", strings.Join(result.Added, ", "))
	}
	if format == "json" {
		_ = writeJSON(cmd.OutOrStdout(), result)
	}
	return err
}

func writeApplyResult(cmd *cobra.Command, format string, result DetectApplyResult) error {
	if format == "json" {
		return writeJSON(cmd.OutOrStdout(), result)
	}
	out := cmd.OutOrStdout()
	verb := "Added"
	if result.DryRun {
		verb = "Would add"
	}
	if len(result.Added) > 0 {
		fmt.Fprintf(out, "%s: %s\n", verb, strings.Join(result.Added, ", "))
	}
	if len(result.Skipped) > 0 {
		fmt.Fprintf(out, "Already configured: %s\n", strings.Join(result.Skipped, ", "))
	}
	if len(result.Added) > 0 && !result.DryRun {
		fmt.Fprintln(out, "Run 'devex install' to install them.")
	}
	return nil
}
