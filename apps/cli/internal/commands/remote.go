package commands

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/remoterun"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// NewRemoteCmd creates the remote command group.
func NewRemoteCmd(_ types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "remote",
		Short:        "Run DevEx on another host over SSH",
		SilenceUsage: true,
	}
	cmd.AddCommand(newRemoteInstallCmd(settings))
	return cmd
}

// remoteSettings maps remote install flags to their configuration keys. Each
// is also read from DEVEX_REMOTE_* environment variables and the remote:
// section of the config file. Host-key trust and consent (--accept-new-host-key,
// --yes) are deliberately flags only.
var remoteSettings = map[string]string{
	"host":         "remote.host",
	"port":         "remote.port",
	"identity":     "remote.identity",
	"known-hosts":  "remote.known-hosts",
	"remote-devex": "remote.devex",
	"ssh":          "remote.ssh",
}

func newRemoteInstallCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		acceptNewHostKey bool
		yes              bool
	)
	cmd := &cobra.Command{
		Use:   "install <apps...> --host [user@]host",
		Short: "Install applications on another host over SSH",
		Long: `Install the named applications on another host with the DevEx installed
there, over SSH.

The remote host must have DevEx installed and its own configuration. The
application names are checked against the catalog, then the remote DevEx
plans them (its platform decides the install methods) and the plan is shown
for confirmation. The remote install receives the confirmed plan's digest,
plans again and installs only if nothing changed (applications, methods,
order, dependencies, hooks or install commands); otherwise it refuses and
nothing is installed. The install runs unattended (CI=true) and its output is
streamed here; a remote failure exits nonzero.

SSH uses your OpenSSH client, configuration and agent, in batch mode: it
never prompts for passwords. Host keys must already be in known_hosts;
--accept-new-host-key trusts an unknown host on first connection (a changed
key is always refused). Installs on the remote host use sudo without a
password, so its user needs passwordless sudo for package managers, or must
be root.

--host, --port, --identity, --known-hosts, --remote-devex and --ssh can also
be set with DEVEX_REMOTE_HOST, DEVEX_REMOTE_PORT, DEVEX_REMOTE_IDENTITY,
DEVEX_REMOTE_KNOWN_HOSTS, DEVEX_REMOTE_DEVEX and DEVEX_REMOTE_SSH, or under
remote: in the config file; flags win. --dry-run also follows DEVEX_DRY_RUN
and dry-run in the config file.`,
		Example: `  devex remote install git curl --host me@devbox --dry-run
  devex remote install git --host me@devbox --identity ~/.ssh/devbox --yes`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		Annotations:  map[string]string{catalogAnnotation: catalogRequired},
		RunE: func(cmd *cobra.Command, apps []string) error {
			target, dryRun, err := remoteTarget(cmd, acceptNewHostKey)
			if err != nil {
				return err
			}
			if err := target.Validate(); err != nil {
				return err
			}
			if err := remoterun.ValidateApps(apps); err != nil {
				return err
			}
			ctx := cmd.Context()
			stderr := cmd.ErrOrStderr()
			// The remote platform decides support and methods, so names are
			// checked against the whole catalog, not the local platform's part.
			if err := agentapi.CheckCatalogNames(settings, apps); err != nil {
				return fmt.Errorf("check the applications locally: %w", err)
			}
			version, err := target.Version(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(stderr, "Remote %s: %s\n", target.Host, version)
			plan, err := target.RemotePlan(ctx, apps)
			if err != nil {
				return err
			}
			printRemotePlan(cmd, target.Host, plan)
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry run: nothing was installed.")
				return nil
			}
			if err := confirmRemote(cmd, yes, target.Host); err != nil {
				return err
			}
			fmt.Fprintf(stderr, "Installing on %s...\n", target.Host)
			if err := target.Install(ctx, apps, plan.Digest, cmd.OutOrStdout(), stderr); err != nil {
				if errors.Is(err, remoterun.ErrPlanChanged) {
					return fmt.Errorf("the plan on %s changed after it was confirmed; nothing was installed; review it again with --dry-run", target.Host)
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed %s on %s.\n", strings.Join(apps, ", "), target.Host)
			return nil
		},
	}
	cmd.Flags().String("host", "", "SSH destination, [user@]host (required)")
	cmd.Flags().Int("port", 0, "SSH port (default: from your ssh config, or 22)")
	cmd.Flags().String("identity", "", "Private key file (default: your SSH agent and config)")
	cmd.Flags().String("known-hosts", "", "known_hosts file to verify the host key with")
	cmd.Flags().BoolVar(&acceptNewHostKey, "accept-new-host-key", false, "Trust and record an unknown host key on first connection")
	cmd.Flags().String("remote-devex", "devex", "devex command on the remote host")
	cmd.Flags().String("ssh", "ssh", "Local ssh executable")
	cmd.Flags().Bool("dry-run", false, "Show the remote plan without installing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Install without prompting")
	return cmd
}

// remoteTarget resolves the effective target and dry-run setting: flags, then
// environment, then config file, then defaults.
func remoteTarget(cmd *cobra.Command, acceptNewHostKey bool) (remoterun.Target, bool, error) {
	for flag, key := range remoteSettings {
		if err := viper.BindPFlag(key, cmd.Flags().Lookup(flag)); err != nil {
			return remoterun.Target{}, false, err
		}
	}
	if err := viper.BindPFlag("dry-run", cmd.Flags().Lookup("dry-run")); err != nil {
		return remoterun.Target{}, false, err
	}
	target := remoterun.Target{
		Host:             strings.TrimSpace(viper.GetString("remote.host")),
		Identity:         viper.GetString("remote.identity"),
		KnownHosts:       viper.GetString("remote.known-hosts"),
		AcceptNewHostKey: acceptNewHostKey,
		RemoteDevex:      viper.GetString("remote.devex"),
		SSH:              viper.GetString("remote.ssh"),
	}
	if target.Host == "" {
		return target, false, errors.New("name the SSH host with --host [user@]host, DEVEX_REMOTE_HOST, or remote.host in the config file")
	}
	if port := strings.TrimSpace(viper.GetString("remote.port")); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil {
			return target, false, fmt.Errorf("invalid SSH port %q: use a number from 1 to 65535", port)
		}
		target.Port = value
	}
	return target, viper.GetBool("dry-run"), nil
}

func printRemotePlan(cmd *cobra.Command, host string, plan remoterun.Plan) {
	stderr := cmd.ErrOrStderr()
	fmt.Fprintf(stderr, "Plan on %s:\n", host)
	for _, app := range plan.Apps {
		fmt.Fprintf(stderr, "  install %s (%s)", app.Name, app.Method)
		if len(app.Effects) > 0 {
			fmt.Fprintf(stderr, "; effects: %s", strings.Join(app.Effects, ", "))
		}
		if len(app.Dependencies) > 0 {
			fmt.Fprintf(stderr, "; depends on: %s", strings.Join(app.Dependencies, ", "))
		}
		if app.Privileges != "" {
			fmt.Fprintf(stderr, "; privileges: %s", app.Privileges)
		}
		fmt.Fprintln(stderr)
	}
	if len(plan.DependencyOrder) > 0 {
		fmt.Fprintf(stderr, "  order: %s\n", strings.Join(plan.DependencyOrder, ", "))
	}
	if len(plan.Roots) > 0 {
		fmt.Fprintf(stderr, "  selected (their install steps and hooks run): %s\n", strings.Join(plan.Roots, ", "))
	}
	if plan.Policy.RollbackOnFailure {
		fmt.Fprintln(stderr, "  rollback on failure: enabled; a failed batch removes newly installed APT packages and restores backed-up files")
	}
	for _, warning := range plan.Warnings {
		fmt.Fprintf(stderr, "  warning: %s\n", warning)
	}
}

func confirmRemote(cmd *cobra.Command, yes bool, host string) error {
	if yes {
		return nil
	}
	if !interaction.IsInteractiveMode() {
		return fmt.Errorf("confirmation required; review with --dry-run, then use --yes to install on %s", host)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Install these applications on %s? [y/N]: ", host)
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
		return fmt.Errorf("cancelled; nothing was installed on %s", host)
	}
	return nil
}
