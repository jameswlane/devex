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
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
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
	return newPackageMaintenanceCmd("update", settings, nil)
}

// NewUpgradeCmd upgrades only packages belonging to the selected applications.
func NewUpgradeCmd(settings config.CrossPlatformSettings) *cobra.Command {
	return newPackageMaintenanceCmd("upgrade", settings, nil)
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
		apps, _, err := agentapi.SelectApplications(cmd.Context(), settings, agentapi.PlanRequest{Names: args, Categories: viper.GetStringSlice("categories")})
		if err != nil {
			return err
		}
		plan, failures := planPackageMaintenance(action, apps)
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

func planPackageMaintenance(action string, apps []types.CrossPlatformApp) ([]packageMaintenanceOperation, []error) {
	grouped := map[string][]string{}
	var failures []error
	for _, app := range apps {
		c := app.GetOSConfig()
		supported := false
		switch c.InstallMethod {
		case "apt", "dnf", "zypper", "xbps", "eopkg":
			supported = true
		case "apk", "emerge":
			supported = action == "update"
		case "snap", "nixpkgs":
			supported = action == "upgrade"
		}
		if !supported {
			failures = append(failures, fmt.Errorf("%s: manager %q does not support safe %s through this CLI", app.Name, c.InstallMethod, action))
			continue
		}
		if action == "update" {
			grouped[c.InstallMethod] = nil
			continue
		}
		packages, err := installers.CatalogPackageArguments(c.InstallMethod, c.InstallCommand)
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
		grouped[c.InstallMethod] = append(grouped[c.InstallMethod], packages...)
	}
	managers := make([]string, 0, len(grouped))
	for manager := range grouped {
		managers = append(managers, manager)
	}
	sort.Strings(managers)
	plan := make([]packageMaintenanceOperation, 0, len(managers))
	for _, manager := range managers {
		args := []string{action}
		if manager == "apt" && action == "upgrade" {
			args[0] = "upgrade-packages"
		}
		seen := map[string]bool{}
		for _, pkg := range grouped[manager] {
			if !seen[pkg] {
				args = append(args, pkg)
				seen[pkg] = true
			}
		}
		plan = append(plan, packageMaintenanceOperation{manager: manager, args: args})
	}
	return plan, failures
}

// maintenanceNativeCommands documents the audited plugin contracts.
func maintenanceNativeCommands(op packageMaintenanceOperation) []string {
	commands := map[string][]string{
		"apt:update": {"apt-get update"}, "dnf:update": {"dnf makecache"},
		"zypper:update": {"zypper --non-interactive refresh"}, "xbps:update": {"xbps-install --sync"},
		"eopkg:update": {"eopkg update-repo"}, "emerge:update": {"emerge --ask=n --sync"}, "apk:update": {"apk update"},
		"apt:upgrade-packages": {"apt-get update", "apt-get install --only-upgrade -y"},
		"dnf:upgrade":          {"dnf upgrade --assumeyes"}, "zypper:upgrade": {"zypper --non-interactive update"},
		"xbps:upgrade": {"xbps-install --yes --update"}, "eopkg:upgrade": {"eopkg upgrade --yes-all"},
		"snap:upgrade": {"snap refresh"}, "nixpkgs:upgrade": {"nix-env --upgrade"},
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
