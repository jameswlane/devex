package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/channels"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/execinputs"
	"github.com/jameswlane/devex/apps/cli/internal/help"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/system"
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
		expectPlan     string
		approvalFile   string
	)

	cmd := &cobra.Command{
		Annotations: map[string]string{catalogAnnotation: catalogRequired},
		Use:         "install [apps...]",
		Short:       "Install development applications",
		Long: `Install applications with cross-platform package manager support.

The install command automates the installation of your development environment with:
  • Cross-platform package manager selection (APT, DNF, Pacman, Flatpak, Brew, etc.)
  • Dependency resolution and conflict handling
  • Post-installation configuration and shell setup
  • Theme application and desktop environment setup

Configuration hierarchy (highest to lowest priority):
  • Command-line flags
  • Environment variables (DEVEX_*)
  • Configuration files ($DEVEX_CONFIG_DIR/config.yaml)
  • Default values

Configuration files are located in $DEVEX_DATA_DIR/config/:
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

  # Install without prompts (also DEVEX_NONINTERACTIVE=1 or CI)
  devex install --non-interactive --config ./server.yaml

  # Install a lifecycle template
  devex install --template databases --answer databases=postgres,redis --non-interactive

  # Dry run to preview changes
  devex install --dry-run

  # Install with verbose output
  devex install --verbose`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			channel := strings.TrimSpace(viper.GetString("channel"))
			if cmd.Flags().Changed("channel") {
				flagVal, _ := cmd.Flags().GetString("channel")
				channel = strings.TrimSpace(flagVal)
			}
			format, _ := cmd.Flags().GetString("format")
			if format == "json" {
				if !viper.GetBool("dry-run") || templateID != "" {
					return writeAPIJSON(cmd, nil, &agentapi.Error{Category: "invalid_request", Message: "JSON install requires --dry-run without --template"})
				}
				planReq := agentapi.PlanRequest{
					Names:               args,
					Categories:          viper.GetStringSlice("categories"),
					Channel:             channel,
					Repo:                repo,
					RefuseChannelSwitch: true,
				}
				snapshot, err := agentapi.ResolveExecution(ctx, settings, planReq, currentExecutionPolicy(), currentExecutionTrust())
				if err == nil {
					for _, app := range snapshot.Roots {
						_ = system.CheckApplicationRequirements(ctx, app.Name, app.GetOSConfig().SystemRequirements, func(message string) { fmt.Fprintln(cmd.ErrOrStderr(), message) })
					}
				}
				// The digest lets a caller confirm this plan and install exactly it
				// with --expect-plan. Roots are the applications whose lifecycles
				// run; other planned dependencies are left to their package manager.
				return writeAPIJSON(cmd, struct {
					agentapi.Plan
					Roots  []string                 `json:"roots"`
					Policy agentapi.ExecutionPolicy `json:"policy"`
					Digest string                   `json:"digest"`
				}{snapshot.Plan, snapshot.RootNames(), snapshot.Policy, snapshot.Digest}, err)
			}
			if format != "table" {
				return fmt.Errorf("unsupported format %q", format)
			}

			// Get configuration from Viper (respects hierarchy)
			verbose := viper.GetBool("verbose")
			dryRun := viper.GetBool("dry-run")
			categories := viper.GetStringSlice("categories")

			if approvalFile != "" && expectPlan == "" {
				return fmt.Errorf("--approval approves a confirmed plan; use it with --expect-plan <digest>")
			}
			if expectPlan != "" {
				if dryRun || templateID != "" {
					return fmt.Errorf("--expect-plan installs a confirmed plan; it cannot be combined with --dry-run or --template")
				}
				if err := validateInstallInputs(args, categories); err != nil {
					return fmt.Errorf("invalid inputs: %w", err)
				}
				// Resolve once, verify, then execute exactly this snapshot: its
				// roots, settings and policy, never a fresh selection.
				snapshot, err := agentapi.ResolveExecution(ctx, settings, agentapi.PlanRequest{
					Names:               args,
					Categories:          categories,
					Channel:             channel,
					Repo:                repo,
					RefuseChannelSwitch: true,
				}, currentExecutionPolicy(), currentExecutionTrust())
				if err != nil {
					return fmt.Errorf("check the confirmed plan: %w", err)
				}
				if snapshot.Digest != expectPlan {
					recordPlanChangedApproval(settings.HomeDir, expectPlan, approvalFile)
					return fmt.Errorf("%s (confirmed %s, now %s); nothing was installed; review the plan again with --dry-run --format json", agentapi.PlanChangedMessage, expectPlan, snapshot.Digest)
				}
				ctx, err = approvePlan(ctx, settings.HomeDir, snapshot, approvalFile)
				if err != nil {
					return err
				}
				return executeSnapshot(ctx, cmd, snapshot, verbose, repo, getPluginBootstrap)
			}

			if !dryRun {
				if err := refuseUnapprovedInstall(ctx, settings.HomeDir); err != nil {
					return err
				}
			}
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

			if viper.GetBool("rollback-on-failure") && !dryRun {
				selection, err := agentapi.SelectApplicationsWithWarnings(ctx, settings, agentapi.PlanRequest{
					Names:               args,
					Categories:          categories,
					Channel:             channel,
					Repo:                repo,
					RefuseChannelSwitch: true,
				})
				if err != nil {
					return err
				}
				for _, warning := range selection.Warnings {
					fmt.Fprintln(cmd.OutOrStdout(), warning)
				}
				if len(selection.Apps) == 0 && len(selection.Warnings) > 0 {
					return nil
				}
				fmt.Fprintln(cmd.ErrOrStderr(), rollbackEnabledNotice)
				return runRollbackBatch(ctx, cmd.ErrOrStderr(), selection.Apps, repo, selection.Settings, getPluginBootstrap)
			}
			return executeInstallWithOutputAndChannel(ctx, args, categories, channel, verbose, dryRun, repo, settings, cmd.OutOrStdout(), getPluginBootstrap)
		},
		SilenceUsage: true, // Prevent usage spam on runtime errors
	}

	cmd.Flags().Bool("rollback-on-failure", false, "Roll back reversible steps if installation fails")
	_ = viper.BindPFlag("rollback-on-failure", cmd.Flags().Lookup("rollback-on-failure"))
	cmd.Flags().String("format", "table", "Output format (table, json; JSON requires --dry-run)")
	// Define command-specific flags
	cmd.Flags().String("channel", "", "Install channel to use (for example official, mise, brew); env: DEVEX_CHANNEL")
	cmd.Flags().StringSlice("categories", nil, "Install apps from specific categories")
	cmd.Flags().StringVar(&templateID, "template", "", "Install a DevEx lifecycle template by ID or path")
	cmd.Flags().StringArrayVar(&answers, "answer", nil, "Template input answer in key=value form")
	cmd.Flags().StringArrayVar(&answerFiles, "answers", nil, "YAML file containing template input answers")
	cmd.Flags().StringVar(&expectPlan, "expect-plan", "", "Install only if the plan digest from 'install --dry-run --format json' still matches")
	cmd.Flags().StringVar(&approvalFile, "approval", "", "Signed plan approval file for --expect-plan, when the organization policy requires one")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Install without the TUI; fail instead of prompting for template inputs")

	// Bind flags to Viper for hierarchical config
	_ = viper.BindPFlag("channel", cmd.Flags().Lookup("channel"))
	_ = viper.BindPFlag("categories", cmd.Flags().Lookup("categories"))
	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))

	// Add contextual help integration
	AddContextualHelp(cmd, help.ContextCommand, "install")

	return cmd
}

// executeInstall implements the core installation logic with proper context handling
func executeInstall(ctx context.Context, apps []string, categories []string, verbose, dryRun bool, repo types.Repository, settings config.CrossPlatformSettings) error {
	return executeInstallWithOutput(ctx, apps, categories, verbose, dryRun, repo, settings, os.Stdout, nil)
}

func executeInstallWithOutput(ctx context.Context, apps, categories []string, verbose, dryRun bool, repo types.Repository, settings config.CrossPlatformSettings, out io.Writer, provider pluginBootstrapProvider) error {
	return executeInstallWithOutputAndChannel(ctx, apps, categories, strings.TrimSpace(viper.GetString("channel")), verbose, dryRun, repo, settings, out, provider)
}

func executeInstallWithOutputAndChannel(ctx context.Context, apps, categories []string, channel string, verbose, dryRun bool, repo types.Repository, settings config.CrossPlatformSettings, out io.Writer, provider pluginBootstrapProvider) error {
	return executeInstallWithChannelPolicy(ctx, apps, categories, channel, false, verbose, dryRun, repo, settings, out, provider)
}

func executeInstallWithChannelPolicy(ctx context.Context, apps, categories []string, channel string, refuseChannelSwitch, verbose, dryRun bool, repo types.Repository, settings config.CrossPlatformSettings, out io.Writer, provider pluginBootstrapProvider) error {
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
		"channel", channel,
	)

	// Validate inputs
	if err := validateInstallInputs(apps, categories); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Input validation failed")
		return fmt.Errorf("invalid inputs: %w", err)
	}

	selection, err := agentapi.SelectApplicationsWithWarnings(ctx, settings, agentapi.PlanRequest{
		Names:               apps,
		Categories:          categories,
		Channel:             channel,
		Repo:                repo,
		RefuseChannelSwitch: refuseChannelSwitch,
	})
	if err != nil {
		return err
	}
	for _, warning := range selection.Warnings {
		fmt.Fprintln(out, warning)
	}
	appsToInstall := selection.Apps
	settings = selection.Settings

	// Handle dry run
	if dryRun {
		return previewInstallationWithRepo(ctx, appsToInstall, repo, provider, out)
	}
	if len(appsToInstall) == 0 && len(selection.Warnings) > 0 {
		return nil
	}
	return installSelected(ctx, span, appsToInstall, repo, settings, out, provider)
}

// approvePlan verifies a signed approval of the confirmed plan digest when the
// organization policy requires one (or when one is supplied), records the
// decision in the audit log, and marks ctx as running the approved plan.
func approvePlan(ctx context.Context, homeDir string, snapshot agentapi.ExecutionSnapshot, approvalFile string) (context.Context, error) {
	digest := snapshot.Digest
	policy, err := security.LoadOrgPolicy(homeDir)
	if err != nil {
		return ctx, err
	}
	// The approval binds the policy the plan was resolved with.
	planned := ""
	if ref := snapshot.Plan.OrganizationPolicy; ref != nil {
		planned = ref.SHA256
	}
	current := ""
	if policy != nil {
		current = policy.SHA256
	}
	if planned != current {
		recordPlanChangedApproval(homeDir, digest, approvalFile)
		return ctx, fmt.Errorf("%s (the organization policy changed); nothing was installed; review the plan again with --dry-run --format json", agentapi.PlanChangedMessage)
	}
	if approvalFile == "" {
		if !policy.ApprovalRequired() {
			return ctx, nil
		}
		err := policy.CheckApproval(ctx)
		state.RecordApprovalDecision(digest, "", approvalPolicyID(policy), err)
		return ctx, err
	}
	approval, err := security.LoadPlanApproval(approvalFile)
	if err != nil {
		state.RecordApprovalDecision(digest, "", approvalPolicyID(policy), err)
		return ctx, err
	}
	approved, err := policy.VerifyApproval(approval, digest, time.Now())
	state.RecordApprovalDecision(digest, approval.Approver, approvalPolicyID(policy), err)
	if err != nil {
		return ctx, err
	}
	log.Info("Plan approval verified", "digest", digest, "approver", approved.Approver, "expires_at", approved.ExpiresAt)
	return security.WithApprovedPlan(ctx, approved), nil
}

// refuseUnapprovedInstall refuses an unconfirmed install up front when the
// organization policy requires approved plans; the installers enforce it too.
func refuseUnapprovedInstall(ctx context.Context, homeDir string) error {
	policy, err := security.LoadOrgPolicy(homeDir)
	if err != nil || !policy.ApprovalRequired() {
		return err
	}
	err = policy.CheckApproval(ctx)
	state.RecordApprovalDecision("", "", approvalPolicyID(policy), err)
	return err
}

// recordPlanChangedApproval audits a refused approved plan whose plan or
// policy changed after it was confirmed.
func recordPlanChangedApproval(homeDir, digest, approvalFile string) {
	policy, err := security.LoadOrgPolicy(homeDir)
	if err != nil || (approvalFile == "" && !policy.ApprovalRequired()) {
		return
	}
	approver := ""
	if approvalFile != "" {
		if approval, loadErr := security.LoadPlanApproval(approvalFile); loadErr == nil {
			approver = approval.Approver
		}
	}
	state.RecordApprovalDecision(digest, approver, approvalPolicyID(policy), fmt.Errorf("%s; nothing was installed", agentapi.PlanChangedMessage))
}

func approvalPolicyID(policy *security.OrgPolicy) string {
	if policy == nil {
		return ""
	}
	return policy.DisplayName() + "@" + policy.Version
}

const rollbackEnabledNotice = "Rollback enabled: failed batches remove newly installed APT packages and restore backed-up files; hooks and dependencies may require manual recovery."

// currentExecutionPolicy reads the effective install policy that a confirmed
// plan digest binds.
func currentExecutionPolicy() agentapi.ExecutionPolicy {
	return agentapi.ExecutionPolicy{
		RollbackOnFailure:  viper.GetBool("rollback-on-failure"),
		InstallConcurrency: installConcurrency(),
		Offline:            viper.GetBool("offline"),
		NoCache:            viper.GetBool("no-cache"),
		SkipPluginDownload: viper.GetBool("skip-plugin-download"),
		AptListsMaxAge:     viper.GetDuration("apt-lists-max-age").String(),
	}
}

// executeSnapshot installs a verified snapshot's roots with its settings,
// choosing the execution path from its policy.
func executeSnapshot(ctx context.Context, cmd *cobra.Command, snapshot agentapi.ExecutionSnapshot, verbose bool, repo types.Repository, provider pluginBootstrapProvider) error {
	_, span := tracer.Start(ctx, "install_command",
		trace.WithAttributes(
			attribute.StringSlice("apps", snapshot.RootNames()),
			attribute.String("plan_digest", snapshot.Digest),
			attribute.String("platform", runtime.GOOS),
		),
	)
	defer span.End()
	snapshot.Settings.Verbose = verbose
	return executeSnapshotObserved(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), snapshot, repo, provider, nil)
}

func executeSnapshotObserved(ctx context.Context, out, errOut io.Writer, snapshot agentapi.ExecutionSnapshot, repo types.Repository, provider pluginBootstrapProvider, observe installObserver) error {
	if err := channels.CheckRefusedChannelSwitches(repo, snapshot.Roots); err != nil {
		return err
	}
	settings := snapshot.Settings
	if err := pinExecution(snapshot); err != nil {
		return err
	}
	for _, warning := range snapshot.Plan.Warnings {
		if strings.HasPrefix(warning, "Skipped ") {
			fmt.Fprintln(out, warning)
		}
	}
	if len(snapshot.Roots) == 0 && len(snapshot.Plan.Warnings) > 0 {
		return nil
	}
	// Pin the confirmed local files for every step that reads them.
	ctx = execinputs.With(ctx, snapshot.Inputs)
	if snapshot.Policy.RollbackOnFailure {
		fmt.Fprintln(errOut, rollbackEnabledNotice)
		err := runRollbackBatch(ctx, errOut, snapshot.Roots, repo, settings, provider)
		if observe != nil {
			status := "installed"
			if err != nil {
				status = "failed"
			}
			for _, app := range snapshot.Roots {
				observe(app.Name, status, err)
			}
		}
		return err
	}
	// A confirmed install always runs the plain lifecycle, never the TUI, and never switches channels.
	return runPlainInstallObserved(ctx, out, snapshot.Roots, repo, settings, provider, observe, false)
}

// currentExecutionTrust reads the effective plugin trust configuration that a
// confirmed plan digest binds.
func currentExecutionTrust() agentapi.ExecutionTrust {
	trust := agentapi.ExecutionTrust{PluginRegistryURL: bootstrap.GetRegistryURL(), PluginPublicKey: viper.GetString("plugin.public-key")}
	if trust.PluginPublicKey != "" {
		trust.PluginPublicKeyDigest = execinputs.Fingerprint(trust.PluginPublicKey)
	}
	return trust
}

// pinExecution makes the rest of the process read the confirmed policy and
// trust values, so later reads of configuration, the environment or
// config.yaml cannot change them. Viper values set here take precedence over
// flags, environment and config files.
func pinExecution(snapshot agentapi.ExecutionSnapshot) error {
	if key := snapshot.Trust.PluginPublicKey; key != "" && execinputs.Fingerprint(key) != snapshot.Trust.PluginPublicKeyDigest {
		return fmt.Errorf("plugin public key %s %w; nothing was installed", key, execinputs.ErrChanged)
	}
	policy := snapshot.Policy
	viper.Set("rollback-on-failure", policy.RollbackOnFailure)
	viper.Set("install-concurrency", policy.InstallConcurrency)
	viper.Set("offline", policy.Offline)
	viper.Set("no-cache", policy.NoCache)
	viper.Set("skip-plugin-download", policy.SkipPluginDownload)
	viper.Set("apt-lists-max-age", policy.AptListsMaxAge)
	viper.Set("plugin.public-key", snapshot.Trust.PluginPublicKey)
	// The environment variable takes precedence over config.yaml, which plugin
	// bootstrap would otherwise read again.
	if err := os.Setenv("DEVEX_PLUGIN_REGISTRY_URL", snapshot.Trust.PluginRegistryURL); err != nil {
		return fmt.Errorf("pin the plugin registry: %w", err)
	}
	return nil
}

// installSelected installs already selected applications.
func installSelected(ctx context.Context, span trace.Span, appsToInstall []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings, out io.Writer, provider pluginBootstrapProvider) error {
	if !interaction.IsInteractiveMode() {
		return runPlainInstall(ctx, out, appsToInstall, repo, settings, provider)
	}

	if err := prepareApplicationInstallers(ctx, provider); err != nil {
		return err
	}
	if err := prepareChannelSwitches(ctx, out, appsToInstall, repo); err != nil {
		return err
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

func (e pluginBootstrapExecutor) ExecutePlugin(ctx context.Context, pluginName string, args []string, opts templateexec.PluginExecOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.bootstrap == nil {
		return fmt.Errorf("plugin bootstrap is required")
	}
	if opts.MinVersion != "" {
		if _, err := e.bootstrap.EnsurePluginVersionWithContext(ctx, pluginName, opts.MinVersion); err != nil {
			return err
		}
	}
	return e.bootstrap.ExecutePluginWithOptions(ctx, pluginName, args, sdk.PluginRunOptions{
		IO:  sdk.OperationIO{Stdin: os.Stdin, Stdout: os.Stderr, Stderr: os.Stderr},
		Env: sortedEnvSlice(opts.Env),
		Dir: opts.WorkingDir,
	})
}

func sortedEnvSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

func executeTemplateInstall(ctx context.Context, idOrPath string, opts templateInstallOptions, settings config.CrossPlatformSettings, getPluginBootstrap pluginBootstrapProvider) error {
	homeDir, err := devexHomeDir()
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
	result, err := execRuntime.Execute(ctx, *plan, templateexec.ExecuteOptions{Concurrency: installConcurrency(), Source: idOrPath, DryRun: opts.DryRun, RollbackOnFailure: viper.GetBool("rollback-on-failure")})
	reportRollback(os.Stderr, result)
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

func resolveAppsByName(s config.CrossPlatformSettings, names []string) ([]types.CrossPlatformApp, error) {
	return agentapi.ResolveAppsByName(s, names)
}
func resolveAppsByCategory(s config.CrossPlatformSettings, categories []string) ([]types.CrossPlatformApp, error) {
	return agentapi.ResolveAppsByCategory(s, categories)
}
func normalizeInstallLookup(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

// validateInstallInputs validates the installation inputs
func validateInstallInputs(apps []string, categories []string) error {
	// Apps and categories are mutually exclusive for clarity
	if len(apps) > 0 && len(categories) > 0 {
		return fmt.Errorf("cannot specify both specific apps and categories - choose one approach")
	}
	return nil
}

// previewInstallation shows what would be installed without executing
func previewInstallation(ctx context.Context, apps []types.CrossPlatformApp, provider pluginBootstrapProvider, out io.Writer) error {
	return previewInstallationWithRepo(ctx, apps, nil, provider, out)
}

func previewInstallationWithRepo(ctx context.Context, apps []types.CrossPlatformApp, repo types.Repository, provider pluginBootstrapProvider, out io.Writer) error {
	var plugins map[string]sdk.PluginMetadata
	if provider != nil {
		if b := provider(); b != nil {
			plugins = b.GetManager().ListPluginsWithContext(ctx)
		}
	}

	// Normal install defers bootstrap until after preview; discover existing local
	// metadata directly so dry runs stay offline and can still display privileges.
	if plugins == nil {
		if home, err := utils.GetHomeDir(); err == nil {
			manager := sdk.NewExecutableManager(devexpaths.ForHome(home).ReadPath(devexpaths.ForHome(home).Plugins()))
			manager.SetPluginFilter(security.PluginFilter(home))
			plugins = manager.ListPluginsWithContext(ctx)
		}
	}
	log.Info("Dry run - showing what would be installed:")

	for _, app := range apps {
		if switchPlan, switching := detectChannelSwitch(repo, app); switching {
			if !switchPlan.CanRemove {
				fmt.Fprintf(out, "Would fail switching %s from channel %s to %s: %s; run 'devex uninstall --app %q' first\n",
					app.Name, switchPlan.OldChannel, switchPlan.NewChannel, switchPlan.Reason, app.Name)
			} else {
				fmt.Fprintf(out, "Would switch %s from channel %s to %s (would uninstall %s channel first, then install via %s)\n",
					app.Name, switchPlan.OldChannel, switchPlan.NewChannel, switchPlan.OldChannel, switchPlan.NewChannel)
			}
		}
		// Preview reports failures without executing hooks or installations.
		_ = system.CheckApplicationRequirements(ctx, app.Name, app.GetOSConfig().SystemRequirements, func(message string) { fmt.Fprintln(out, message) })
		if len(app.GetOSConfig().PreInstall)+len(app.GetOSConfig().PostInstall) > 0 {
			hooks, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: "install-hooks"})
			if err != nil {
				return fmt.Errorf("preview %s hooks: %w", app.Name, err)
			}
			fmt.Fprintf(out, "  Hooks and configuration copies: scope=user, intended user=%s\n", hooks.IntendedUser.Username)
			for _, hook := range append(app.GetOSConfig().PreInstall, app.GetOSConfig().PostInstall...) {
				privilege, err := utils.InstallHookPrivilege(hook)
				if err != nil {
					return fmt.Errorf("preview %s hook: %w", app.Name, err)
				}
				if privilege.Scope == sdk.ScopeSystem {
					plan, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{Privilege: privilege, Name: "install-hook"})
					if err != nil {
						return fmt.Errorf("preview %s system hook: %w", app.Name, err)
					}
					fmt.Fprintf(out, "  Install hook: scope=system, intended user=%s, elevation required=%t\n", hooks.IntendedUser.Username, plan.Elevate)
				}
			}
		}
		configfiles.Apply(ctx, app.Name, app.GetOSConfig().ConfigFiles, true, func(message string) { fmt.Fprintln(out, message) })

		osConfig := app.GetOSConfig()
		pluginName := "package-manager-" + osConfig.InstallMethod
		installArgs := []string{"install"}
		if osConfig.InstallMethod == "curlpipe" && osConfig.InstallScope == "user" {
			installArgs = append(installArgs, "--user")
		}
		if request, reqErr := types.ResolvePackageInstall(osConfig.InstallMethod, osConfig.InstallCommand, osConfig.Packages, osConfig.Options); reqErr == nil {
			installArgs = append(installArgs, request.Options...)
		}
		privilege := sdk.PluginOperationPrivilege(plugins[pluginName].PluginInfo, installArgs)
		channelDetail := formatDryRunChannelDetail(app)
		if privilege == nil {
			fmt.Fprintf(out, "Would install %s%s (scope: unspecified; legacy or unavailable plugin metadata)\n", app.Name, channelDetail)
			continue
		}
		plan, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{Privilege: *privilege, Name: pluginName})
		if err != nil {
			return fmt.Errorf("preview %s privilege: %w", app.Name, err)
		}
		fmt.Fprintf(out, "Would install %s%s (scope: %s, intended user: %s, elevation required: %t)\n", app.Name, channelDetail, privilege.Scope, plan.IntendedUser.Username, plan.Elevate)
	}

	log.Info("Total applications", "count", len(apps))
	log.Info("Use --verbose for detailed installation commands")

	return nil
}

func formatDryRunChannelDetail(app types.CrossPlatformApp) string {
	selected := strings.TrimSpace(app.SelectedChannel())
	if selected == "" {
		return ""
	}
	infos := app.Channels()
	ordered := make([]types.ChannelInfo, 0, len(infos))
	for _, ch := range infos {
		if ch.Default {
			ordered = append(ordered, ch)
		}
	}
	for _, ch := range infos {
		if !ch.Default {
			ordered = append(ordered, ch)
		}
	}
	offered := types.FormatAvailableChannels(ordered)
	if offered != "" && offered != "none" {
		return fmt.Sprintf(" via channel %s (channels: %s)", selected, offered)
	}
	return fmt.Sprintf(" via channel %s", selected)
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
