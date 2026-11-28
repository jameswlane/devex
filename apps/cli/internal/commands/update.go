package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/channels"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type packageMaintenanceExecutor interface {
	ExecutePluginWithIO(context.Context, string, []string, sdk.OperationIO) error
}

type packageMaintenanceOperation struct {
	manager string
	args    []string
}

// NewUpdateCmd refreshes repository metadata without upgrading applications.
func NewUpdateCmd(settings config.CrossPlatformSettings) *cobra.Command {
	return newPackageMaintenanceCmdWithRepo("update", nil, settings, nil)
}

// NewUpgradeCmd upgrades only packages belonging to the selected applications.
func NewUpgradeCmd(settings config.CrossPlatformSettings) *cobra.Command {
	return newPackageMaintenanceCmdWithRepo("upgrade", nil, settings, nil)
}

var maintenanceExecutor = func(ctx context.Context, provider pluginBootstrapProvider) (packageMaintenanceExecutor, error) {
	if provider != nil {
		if pb := provider(); pb != nil {
			return pb, nil
		}
	}
	pb, err := bootstrap.NewPluginBootstrap(viper.GetBool("offline") || viper.GetBool("skip-plugin-download"))
	if err != nil {
		return nil, err
	}
	if err := pb.Initialize(ctx); err != nil {
		return nil, err
	}
	return pb, nil
}

func newPackageMaintenanceCmd(action string, settings config.CrossPlatformSettings, provider pluginBootstrapProvider) *cobra.Command {
	return newPackageMaintenanceCmdWithRepo(action, nil, settings, provider)
}

func newPackageMaintenanceCmdWithRepo(action string, repo types.Repository, settings config.CrossPlatformSettings, provider pluginBootstrapProvider) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{catalogAnnotation: catalogRequired},
		Use:         action + " [apps...]", Short: "Refresh package metadata for configured applications",
		Long:    "Refresh repository metadata once per selected package manager. No packages are upgraded. With no selection, use configured applications (or catalog defaults).",
		Example: "  devex " + action + " git\n  devex " + action + " --categories development --dry-run\n  devex " + action + " --non-interactive",
		Args:    cobra.ArbitraryArgs, SilenceUsage: true,
	}
	if action == "upgrade" {
		cmd.Short = "Upgrade packages for configured applications"
		cmd.Long = "Upgrade only packages belonging to selected applications. With no selection, use configured applications (or catalog defaults). Managers without safe selected-package upgrades fail explicitly; no system-wide upgrade is substituted."
	}
	cmd.Flags().Bool("non-interactive", false, "Run without interactive prompts")
	cmd.Flags().StringSlice("categories", nil, "Select applications by category")
	cmd.Flags().Bool("dry-run", false, "Show exact plugin arguments without initializing or executing plugins")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Bind the executing command, avoiding sibling flags sharing a Viper key.
		if err := viper.BindPFlags(cmd.Flags()); err != nil {
			return err
		}
		// Select first, then resolve each app once: an installed app follows
		// its installed definition, any other app the channel preferences.
		// A per-app failure does not stop maintenance for the others.
		apps, _, err := agentapi.SelectRequestedApplications(cmd.Context(), settings, agentapi.PlanRequest{
			Names:        args,
			Categories:   viper.GetStringSlice("categories"),
			SkipChannels: true,
		})
		if err != nil {
			return err
		}
		prefs := channels.PreferencesFromViper()
		var unresolved []error
		resolvedApps := make([]types.CrossPlatformApp, 0, len(apps))
		for _, app := range apps {
			var resolved types.CrossPlatformApp
			var resolveErr error
			if repo != nil && channels.InstalledChannelForApp(repo, app) != "" {
				resolved, resolveErr = channels.InstalledDefinition(repo, app)
			} else {
				resolved, _, resolveErr = channels.Resolve(app, prefs, "")
			}
			if resolveErr != nil {
				unresolved = append(unresolved, resolveErr)
				continue
			}
			resolvedApps = append(resolvedApps, resolved)
		}
		apps = resolvedApps
		plan, skipped, failures := planPackageMaintenance(action, apps)
		failures = append(unresolved, failures...)
		for _, name := range skipped {
			cmd.Printf("%s manages its own updates; skipped\n", name)
		}
		for _, failure := range failures {
			cmd.PrintErrln(failure)
		}
		for _, op := range plan {
			cmd.Println("Plugin invocation: " + maintenanceInvocation(op))
			for _, native := range maintenanceNativeCommands(op) {
				cmd.Println("Native invocation: " + native)
			}
		}
		if viper.GetBool("dry-run") {
			return maintenanceFailures(action, failures)
		}
		if len(plan) == 0 {
			return maintenanceFailures(action, failures)
		}
		executor, err := maintenanceExecutor(cmd.Context(), provider)
		if err != nil {
			return errors.Join(maintenanceFailures(action, failures), err)
		}
		ctx := cmd.Context()
		if !interaction.IsInteractiveMode() {
			ctx = sdk.WithUnattended(ctx)
		}
		for _, op := range plan {
			if err := ctx.Err(); err != nil {
				failures = append(failures, err)
				break
			}
			var diagnostic maintenanceDiagnostic
			stderr := io.MultiWriter(cmd.ErrOrStderr(), &diagnostic)
			if err := executor.ExecutePluginWithIO(ctx, "package-manager-"+op.manager, op.args, sdk.OperationIO{Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: stderr}); err != nil {
				if op.manager == "apt" && op.args[0] == "upgrade-packages" && strings.Contains(strings.ToLower(err.Error()+string(diagnostic)), "unknown command") {
					err = fmt.Errorf("package-manager-apt is too old for targeted upgrades; update plugins: %w", err)
				}
				failure := fmt.Errorf("%s %s: %w", op.manager, action, err)
				cmd.PrintErrln(failure)
				failures = append(failures, failure)
			} else {
				cmd.Printf("%s %s succeeded\n", op.manager, action)
			}
		}
		return maintenanceFailures(action, failures)
	}
	return cmd
}

func maintenanceInvocation(op packageMaintenanceOperation) string {
	parts := make([]string, 0, 1+len(op.args))
	parts = append(parts, "devex-plugin-package-manager-"+op.manager)
	for _, arg := range op.args {
		parts = append(parts, strconv.Quote(arg))
	}
	return strings.Join(parts, " ")
}

func maintenanceFailures(action string, failures []error) error {
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%s failed (%d failures): %w", action, len(failures), errors.Join(failures...))
}

type maintenanceGroupKey struct {
	manager string
	scope   string
}

func planPackageMaintenance(action string, apps []types.CrossPlatformApp) ([]packageMaintenanceOperation, []string, []error) {
	grouped := map[maintenanceGroupKey][]string{}
	var skipped []string
	var failures []error
	for _, app := range apps {
		c := app.GetOSConfig()
		if c.SelfUpdating {
			skipped = append(skipped, app.Name)
			continue
		}
		supported := false
		switch c.InstallMethod {
		case "apt", "dnf", "zypper", "xbps", "eopkg", "winget":
			supported = true
		case "apk", "emerge":
			supported = action == "update"
		case "snap", "nixpkgs", "mas", "mise", "curlpipe":
			supported = action == "upgrade"
		}
		if !supported {
			failures = append(failures, fmt.Errorf("%s: manager %q does not support safe %s through this CLI", app.Name, c.InstallMethod, action))
			continue
		}
		if action == "update" {
			grouped[maintenanceGroupKey{manager: c.InstallMethod}] = nil
			continue
		}
		packages, scope, err := maintenancePackages(c)
		if c.InstallMethod == "mise" || c.InstallMethod == "curlpipe" {
			packages, scope, err = reinstallTargets(c)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", app.Name, err))
			continue
		}
		if len(packages) == 0 {
			failures = append(failures, fmt.Errorf("%s: no package identifiers configured", app.Name))
			continue
		}
		valid := true
		for _, pkg := range packages {
			if strings.HasPrefix(pkg, "-") || strings.ContainsAny(pkg, "\"'\\") {
				valid = false
			}
		}
		if !valid {
			failures = append(failures, fmt.Errorf("%s: unsafe package identifier", app.Name))
			continue
		}
		key := maintenanceGroupKey{manager: c.InstallMethod, scope: scope}
		grouped[key] = append(grouped[key], packages...)
	}
	keys := make([]maintenanceGroupKey, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].manager != keys[j].manager {
			return keys[i].manager < keys[j].manager
		}
		return keys[i].scope < keys[j].scope
	})
	plan := make([]packageMaintenanceOperation, 0, len(keys))
	for _, key := range keys {
		args := []string{action}
		if key.manager == "apt" && action == "upgrade" {
			args[0] = "upgrade-packages"
		}
		if key.manager == "mise" && action == "upgrade" {
			// The mise plugin's update command runs `mise upgrade <tools>`.
			args[0] = "update"
		}
		if key.manager == "curlpipe" && action == "upgrade" {
			// Install scripts fetch the newest release, so running the
			// script again is the vendor's documented upgrade.
			args[0] = "install"
		}
		if key.scope != "" {
			args = append(args, key.scope)
		}
		seen := map[string]bool{}
		for _, pkg := range grouped[key] {
			if !seen[pkg] {
				args = append(args, pkg)
				seen[pkg] = true
			}
		}
		plan = append(plan, packageMaintenanceOperation{manager: key.manager, args: args})
	}
	return plan, skipped, failures
}

// maintenancePackages resolves identifiers the way installation does, from
// install_command or from packages and options, so an upgrade targets the
// packages that were installed. Zypper keeps its --type selection in packages,
// which the identifier check refuses rather than upgrading a different kind of
// package. Winget returns its --scope separately so user-scope, machine-scope,
// and unscoped upgrades stay grouped by scope and unattended runs keep the
// explicit scope required by the winget plugin.
func maintenancePackages(c types.OSConfig) ([]string, string, error) {
	request, err := types.ResolvePackageInstall(c.InstallMethod, c.InstallCommand, c.Packages, c.Options)
	if err != nil {
		return nil, "", err
	}
	switch c.InstallMethod {
	case "zypper":
		return append(request.Options, request.Packages...), "", nil
	case "winget":
		scope := ""
		var extra []string
		for _, opt := range request.Options {
			if strings.HasPrefix(opt, "--scope=") {
				if scope != "" && scope != opt {
					return nil, "", fmt.Errorf("conflicting --scope values %q and %q", strings.TrimPrefix(scope, "--scope="), strings.TrimPrefix(opt, "--scope="))
				}
				scope = opt
			} else {
				extra = append(extra, opt)
			}
		}
		return append(extra, request.Packages...), scope, nil
	default:
		return request.Packages, "", nil
	}
}

// reinstallTargets names what a mise or curlpipe upgrade acts on: the mise
// tools that uninstall removes, or the install script URL. A script pinned by
// checksum would install the same release again, and a hook-driven entry has
// no script to run, so both are refused.
func reinstallTargets(c types.OSConfig) ([]string, string, error) {
	if c.InstallMethod == "curlpipe" {
		if c.IsHookDrivenCurlpipe() {
			return nil, "", fmt.Errorf("install hooks have no upgrade step; reinstall it with devex install")
		}
		if viper.GetBool("offline") {
			return nil, "", fmt.Errorf("rerunning the install script needs the network; retry without --offline")
		}
		if len(c.Verification.Arguments()) > 0 {
			return nil, "", fmt.Errorf("the install script is pinned by checksum; upgrade it by updating the catalog entry")
		}
		target := c.DownloadURL
		if target == "" {
			target = strings.TrimSpace(c.InstallCommand)
		}
		if target == "" || strings.ContainsAny(target, " \t") {
			return nil, "", fmt.Errorf("no single install script URL configured")
		}
		scope := ""
		if c.InstallScope == "user" {
			scope = "--user"
		}
		return []string{target}, scope, nil
	}
	if strings.TrimSpace(c.UninstallCommand) == "" {
		return nil, "", fmt.Errorf("no mise tools configured in uninstall_command")
	}
	return strings.Fields(c.UninstallCommand), "", nil
}

// maintenanceNativeCommands documents the audited plugin contracts.
func maintenanceNativeCommands(op packageMaintenanceOperation) []string {
	commands := map[string][]string{
		"apt:update": {"apt-get update"}, "dnf:update": {"dnf makecache --assumeyes"},
		"zypper:update": {"zypper --non-interactive refresh"}, "xbps:update": {"xbps-install --sync"},
		"eopkg:update": {"eopkg update-repo"}, "emerge:update": {"emerge --ask=n --sync"}, "apk:update": {"apk update"},
		"apt:upgrade-packages": {"apt-get update", "apt-get install --only-upgrade -y"},
		"dnf:upgrade":          {"dnf upgrade --assumeyes"}, "zypper:upgrade": {"zypper --non-interactive update"},
		"xbps:upgrade": {"xbps-install --yes --update"}, "eopkg:upgrade": {"eopkg upgrade --yes-all"},
		"snap:upgrade": {"snap refresh"}, "nixpkgs:upgrade": {"nix-env --upgrade"}, "mas:upgrade": {"mas upgrade"},
		"winget:update": {"winget source update"}, "winget:upgrade": {"winget upgrade"},
		"mise:update": {"mise upgrade"},
	}
	result := append([]string(nil), commands[op.manager+":"+op.args[0]]...)
	if op.manager == "apt" && interaction.IsInteractiveMode() {
		for i := range result {
			result[i] = strings.Replace(result[i], "apt-get update", "apt update", 1)
		}
	}
	if len(op.args) > 1 && len(result) > 0 {
		for _, pkg := range op.args[1:] {
			result[len(result)-1] += " " + strconv.Quote(pkg)
		}
	}
	return result
}

// maintenanceDiagnostic retains bounded stderr for legacy plugin failures whose
// returned error contains only an exit status. Diagnostics still stream normally.
type maintenanceDiagnostic []byte

func (d *maintenanceDiagnostic) Write(p []byte) (int, error) {
	const limit = 8192
	n := len(p)
	if len(p) >= limit {
		*d = append((*d)[:0], p[len(p)-limit:]...)
	} else {
		if len(*d)+len(p) > limit {
			*d = (*d)[len(*d)+len(p)-limit:]
		}
		*d = append(*d, p...)
	}
	return n, nil
}
