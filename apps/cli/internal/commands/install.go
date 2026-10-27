package commands

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/help"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var tracer = otel.Tracer("devex/commands/install")

func init() {
	Register(NewInstallCmd)
}

func NewInstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	return NewInstallCmdWithPluginProvider(repo, settings, nil)
}

func NewInstallCmdWithPluginProvider(repo types.Repository, settings config.CrossPlatformSettings, getPluginBootstrap pluginBootstrapProvider) *cobra.Command {
	var (
		templateID     string
		answers        []string
		answerFiles    []string
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "install [apps...]",
		Short: "Install development applications",
		Long: `Install applications with cross-platform package manager support.

The install command automates the installation of your development environment with:
  • Cross-platform package manager selection (APT, DNF, Pacman, Flatpak, Brew, etc.)
  • Dependency resolution and conflict handling
  • Post-installation configuration and shell setup
  • Theme application and desktop environment setup

Configuration hierarchy (highest to lowest priority):
  • Command-line flags
  • Environment variables (DEVEX_*)
  • Configuration files (~/.devex/config.yaml)
  • Default values

Configuration files are located in ~/.local/share/devex/config/:
  • applications.yaml - Development tools and applications
  • environment.yaml - Programming languages and fonts
  • desktop.yaml - Themes, extensions, and desktop settings
  • system.yaml - Git configuration and system settings`,
		Example: `  # Install default applications
  devex install

  # Install specific applications
  devex install docker git vscode

  # Install with categories
  devex install --categories development,databases

  # Install a lifecycle template
  devex install --template databases --answer databases=postgres,redis --non-interactive

  # Dry run to preview changes
  devex install --dry-run

  # Install with verbose output
  devex install --verbose`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Get configuration from Viper (respects hierarchy)
			verbose := viper.GetBool("verbose")
			dryRun := viper.GetBool("dry-run")
			categories := viper.GetStringSlice("categories")

			if templateID != "" {
				if len(args) > 0 || len(categories) > 0 {
					return fmt.Errorf("--template cannot be combined with app arguments or --categories")
				}
				return executeTemplateInstall(ctx, templateID, templateInstallOptions{
					Answers:        answers,
					AnswerFiles:    answerFiles,
					NonInteractive: nonInteractive,
					DryRun:         dryRun,
				}, settings, getPluginBootstrap)
			}

			return executeInstall(ctx, args, categories, verbose, dryRun, repo, settings)
		},
		SilenceUsage: true, // Prevent usage spam on runtime errors
	}

	// Define command-specific flags
	cmd.Flags().StringSlice("categories", nil, "Install apps from specific categories")
	cmd.Flags().StringVar(&templateID, "template", "", "Install a DevEx lifecycle template by ID or path")
	cmd.Flags().StringArrayVar(&answers, "answer", nil, "Template input answer in key=value form")
	cmd.Flags().StringArrayVar(&answerFiles, "answers", nil, "YAML file containing template input answers")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Fail instead of prompting for missing template inputs")

	// Bind flags to Viper for hierarchical config
	_ = viper.BindPFlag("categories", cmd.Flags().Lookup("categories"))
	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))

	// Add contextual help integration
	AddContextualHelp(cmd, help.ContextCommand, "install")

	return cmd
}

// executeInstall implements the core installation logic with proper context handling
func executeInstall(ctx context.Context, apps []string, categories []string, verbose, dryRun bool, repo types.Repository, settings config.CrossPlatformSettings) error {
	_, span := tracer.Start(ctx, "install_command",
		trace.WithAttributes(
			attribute.StringSlice("apps", apps),
			attribute.StringSlice("categories", categories),
			attribute.Bool("verbose", verbose),
			attribute.Bool("dry_run", dryRun),
			attribute.String("platform", runtime.GOOS),
		),
	)
	defer span.End()

	// Note: Context will be properly propagated when TUI and other functions support it

	// Update settings with runtime configuration
	settings.Verbose = verbose

	log.Debug("Starting installation process",
		"verbose", verbose,
		"dry-run", dryRun,
		"apps", apps,
		"categories", categories,
	)

	// Validate inputs
	if err := validateInstallInputs(apps, categories); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Input validation failed")
		return fmt.Errorf("invalid inputs: %w", err)
	}

	var appsToInstall []types.CrossPlatformApp
	var err error
	switch {
	case len(apps) > 0:
		log.Info("Specific app installation requested", "apps", apps)
		appsToInstall, err = resolveAppsByName(settings, apps)
	case len(categories) > 0:
		log.Info("Category-based installation requested", "categories", categories)
		appsToInstall, err = resolveAppsByCategory(settings, categories)
	default:
		appsToInstall = settings.GetDefaultApps()
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "App resolution failed")
		return err
	}
	if len(appsToInstall) == 0 {
		return fmt.Errorf("no applications matched the install request")
	}

	// Handle dry run
	if dryRun {
		return previewInstallation(appsToInstall)
	}

	// Start TUI installation with cross-platform apps
	span.AddEvent("starting_installation", trace.WithAttributes(
		attribute.Int("app_count", len(appsToInstall)),
	))

	// Pass context to tui.StartInstallation for proper cancellation support
	if err := tui.StartInstallation(ctx, appsToInstall, repo, settings); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Installation failed")
		return fmt.Errorf("installation failed: %w", err)
	}

	// Switch to zsh as the final step (after all installations are complete)
	log.Info("Switching to zsh shell as final step")
	// TODO: Update switchToZsh to accept context parameter
	if err := switchToZsh(); err != nil { //nolint:contextcheck
		log.Warn("Failed to switch to zsh shell", "error", err)
		log.Info("You can manually switch to zsh later with: chsh -s $(which zsh)")
	} else {
		log.Info("Successfully switched to zsh shell. Please restart your terminal or run 'exec zsh' to use the new shell.")
	}

	span.SetStatus(codes.Ok, "Installation completed successfully")
	return nil
}

type templateInstallOptions struct {
	Answers        []string
	AnswerFiles    []string
	NonInteractive bool
	DryRun         bool
}

type pluginBootstrapExecutor struct {
	bootstrap *bootstrap.PluginBootstrap
}

var templateInstallPluginExecutorResolver = resolveTemplateInstallPluginExecutor

func (e pluginBootstrapExecutor) ExecutePlugin(ctx context.Context, pluginName string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.bootstrap == nil {
		return fmt.Errorf("plugin bootstrap is required")
	}
	// The SDK has no context-aware ExecutePlugin yet; ctx is checked above.
	return e.bootstrap.ExecutePlugin(pluginName, args) //nolint:contextcheck
}

func executeTemplateInstall(ctx context.Context, idOrPath string, opts templateInstallOptions, settings config.CrossPlatformSettings, getPluginBootstrap pluginBootstrapProvider) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine user home directory: %w", err)
	}

	planner := templates.TemplatePlanner{
		Manager:     templates.NewTemplateManager(homeDir),
		AppResolver: templates.NewAppResolver(settings.GetApplicationByName),
	}
	plan, err := planner.Plan(ctx, idOrPath, templates.PlanOptions{
		Action: templates.LifecycleInstall,
		InputOptions: templates.InputResolverOptions{
			Answers:        opts.Answers,
			AnswerFiles:    opts.AnswerFiles,
			NonInteractive: opts.NonInteractive,
		},
	})
	if err != nil {
		return err
	}

	var store state.StateStore
	if !opts.DryRun {
		dbPath, err := state.ConfiguredDBPath()
		if err != nil {
			return err
		}
		boltStore, err := state.OpenBoltStore(dbPath, 0o600)
		if err != nil {
			return err
		}
		defer boltStore.Close()
		store = boltStore
	}

	pluginExecutor, err := templateInstallPluginExecutorResolver(getPluginBootstrap, opts.DryRun)
	if err != nil {
		return err
	}
	execRuntime := templateexec.NewExecutionRuntime(store, pluginExecutor)
	result, err := execRuntime.Execute(ctx, *plan, templateexec.ExecuteOptions{DryRun: opts.DryRun})
	if err != nil {
		return err
	}
	if result.DryRun {
		fmt.Printf("Template install plan %s contains %d step(s)\n", result.RunID, result.StepsPlanned)
		return nil
	}
	fmt.Printf("Template install run %s completed: %d/%d step(s) executed\n", result.RunID, result.StepsExecuted, result.StepsPlanned)
	return nil
}

func resolveTemplateInstallPluginExecutor(getPluginBootstrap pluginBootstrapProvider, dryRun bool) (templateexec.PluginExecutor, error) {
	if getPluginBootstrap != nil {
		if pluginBootstrap := getPluginBootstrap(); pluginBootstrap != nil {
			return pluginBootstrapExecutor{bootstrap: pluginBootstrap}, nil
		}
	}
	if dryRun {
		return nil, nil
	}
	return nil, fmt.Errorf("plugin runtime is unavailable for template install")
}

func resolveAppsByName(settings config.CrossPlatformSettings, requested []string) ([]types.CrossPlatformApp, error) {
	apps := settings.GetAllApps()
	byName := make(map[string]types.CrossPlatformApp, len(apps))
	for _, app := range apps {
		if !app.IsSupported() {
			continue
		}
		byName[normalizeInstallLookup(app.Name)] = app
		osConfig := app.GetOSConfig()
		if osConfig.InstallCommand != "" {
			byName[normalizeInstallLookup(osConfig.InstallCommand)] = app
		}
	}

	resolved := make([]types.CrossPlatformApp, 0, len(requested))
	for _, name := range requested {
		app, ok := byName[normalizeInstallLookup(name)]
		if !ok {
			return nil, fmt.Errorf("application '%s' not found", name)
		}
		resolved = append(resolved, app)
	}
	return dedupeApps(resolved), nil
}

func resolveAppsByCategory(settings config.CrossPlatformSettings, categories []string) ([]types.CrossPlatformApp, error) {
	requested := make(map[string]bool, len(categories))
	requestedLabels := make(map[string]string, len(categories))
	for _, category := range categories {
		normalized := normalizeInstallLookup(category)
		requested[normalized] = true
		requestedLabels[normalized] = category
	}

	var resolved []types.CrossPlatformApp
	available := make(map[string]bool)
	matched := make(map[string]bool, len(requested))
	for _, app := range settings.GetAllApps() {
		if app.Category != "" {
			available[app.Category] = true
		}
		normalizedCategory := normalizeInstallLookup(app.Category)
		if requested[normalizedCategory] {
			matched[normalizedCategory] = true
		}
		if requested[normalizedCategory] && app.IsSupported() {
			resolved = append(resolved, app)
		}
	}
	if len(matched) != len(requested) {
		unknown := make([]string, 0, len(requested)-len(matched))
		for category := range requested {
			if !matched[category] {
				unknown = append(unknown, requestedLabels[category])
			}
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown categories: %s", strings.Join(unknown, ", "))
	}
	if len(resolved) == 0 {
		categoryNames := make([]string, 0, len(available))
		for category := range available {
			categoryNames = append(categoryNames, category)
		}
		sort.Strings(categoryNames)
		return nil, fmt.Errorf("no supported applications found for categories %s; available categories: %s", strings.Join(categories, ","), strings.Join(categoryNames, ", "))
	}
	return dedupeApps(resolved), nil
}

func dedupeApps(apps []types.CrossPlatformApp) []types.CrossPlatformApp {
	seen := make(map[string]bool, len(apps))
	deduped := make([]types.CrossPlatformApp, 0, len(apps))
	for _, app := range apps {
		key := normalizeInstallLookup(app.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, app)
	}
	return deduped
}

func normalizeInstallLookup(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// validateInstallInputs validates the installation inputs
func validateInstallInputs(apps []string, categories []string) error {
	// Apps and categories are mutually exclusive for clarity
	if len(apps) > 0 && len(categories) > 0 {
		return fmt.Errorf("cannot specify both specific apps and categories - choose one approach")
	}
	return nil
}

// previewInstallation shows what would be installed without executing
func previewInstallation(apps []types.CrossPlatformApp) error {
	log.Info("Dry run - showing what would be installed:")

	for _, app := range apps {
		log.Info("Would install", "app", app.Name, "category", app.Category)
	}

	log.Info("Total applications", "count", len(apps))
	log.Info("Use --verbose for detailed installation commands")

	return nil
}

// switchToZsh attempts to switch the user's shell to zsh
func switchToZsh() error {
	// Get the full path to zsh
	zshPath, err := utils.GetShellPath("zsh")
	if err != nil {
		return err
	}

	// Change the user's shell to zsh
	return utils.ChangeUserShell(zshPath)
}
