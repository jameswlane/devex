package commands

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

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

func newRemoteInstallCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		target remoterun.Target
		dryRun bool
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "install <apps...> --host [user@]host",
		Short: "Install applications on another host over SSH",
		Long: `Install the named applications on another host with the DevEx installed
there, over SSH.

The remote host must have DevEx installed and its own configuration. The
applications are checked locally, then the remote DevEx plans them (its
platform decides the install methods) and the plan is shown for
confirmation. Just before installing, the remote plan is made again and the
run is refused if it changed. The install runs unattended (CI=true) and its
output is streamed here; a remote failure exits nonzero.

SSH uses your OpenSSH client, configuration and agent, in batch mode: it
never prompts for passwords. Host keys must already be in known_hosts;
--accept-new-host-key trusts an unknown host on first connection (a changed
key is always refused). Installs on the remote host use sudo without a
password, so its user needs passwordless sudo for package managers, or must
be root.`,
		Example: `  devex remote install git curl --host me@devbox --dry-run
  devex remote install git --host me@devbox --identity ~/.ssh/devbox --yes`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		Annotations:  map[string]string{catalogAnnotation: catalogRequired},
		RunE: func(cmd *cobra.Command, apps []string) error {
			if err := target.Validate(); err != nil {
				return err
			}
			if err := remoterun.ValidateApps(apps); err != nil {
				return err
			}
			ctx := cmd.Context()
			stderr := cmd.ErrOrStderr()
			if _, err := agentapi.PlanInstall(ctx, settings, agentapi.PlanRequest{Names: apps}); err != nil {
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
			fmt.Fprintf(stderr, "Plan on %s:\n", target.Host)
			for _, app := range plan.Apps {
				fmt.Fprintf(stderr, "  install %s (%s)\n", app.Name, app.Method)
			}
			for _, warning := range plan.Warnings {
				fmt.Fprintf(stderr, "  warning: %s\n", warning)
			}
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry run: nothing was installed.")
				return nil
			}
			if err := confirmRemote(cmd, yes, target.Host); err != nil {
				return err
			}
			again, err := target.RemotePlan(ctx, apps)
			if err != nil {
				return err
			}
			if !remoterun.Same(plan, again) {
				return fmt.Errorf("the plan on %s changed after it was confirmed; review it again", target.Host)
			}
			fmt.Fprintf(stderr, "Installing on %s...\n", target.Host)
			if err := target.Install(ctx, apps, cmd.OutOrStdout(), stderr); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed %s on %s.\n", strings.Join(apps, ", "), target.Host)
			return nil
		},
	}
	cmd.Flags().StringVar(&target.Host, "host", "", "SSH destination, [user@]host (required)")
	cmd.Flags().IntVar(&target.Port, "port", 0, "SSH port (default: from your ssh config, or 22)")
	cmd.Flags().StringVar(&target.Identity, "identity", "", "Private key file (default: your SSH agent and config)")
	cmd.Flags().StringVar(&target.KnownHosts, "known-hosts", "", "known_hosts file to verify the host key with")
	cmd.Flags().BoolVar(&target.AcceptNewHostKey, "accept-new-host-key", false, "Trust and record an unknown host key on first connection")
	cmd.Flags().StringVar(&target.RemoteDevex, "remote-devex", "devex", "devex command on the remote host")
	cmd.Flags().StringVar(&target.SSH, "ssh", "ssh", "Local ssh executable")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the remote plan without installing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Install without prompting")
	_ = cmd.MarkFlagRequired("host")
	return cmd
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
