package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/jameswlane/devex/apps/cli/internal/selfupdate"
)

// NewSelfUpdateCmd allows injected clients and executable paths for safe tests.
func NewSelfUpdateCmd(current string, updater *selfupdate.Updater) *cobra.Command {
	var version string
	var check, pre, rollback bool
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Check for or install a verified DevEx CLI release",
		Long: `Install the latest stable DevEx CLI, or select --version vX.Y.Z.
Archives are checked against the release's SHA-256 checksums. When cosign is
installed, its Sigstore bundle must also verify. Package-managed installations
must be updated with their package manager. No privilege escalation is used.`,
		Example:      "  devex self-update --check\n  devex self-update --version v0.3.3\n  devex self-update --pre\n  devex self-update --rollback",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if updater == nil {
				client := selfupdate.NewClient()
				client.Token = os.Getenv("GITHUB_TOKEN")
				updater = selfupdate.NewUpdater(client)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			dryRun := viper.GetBool("dry-run")
			if rollback {
				if dryRun {
					cmd.Println("Would restore devex.previous.")
					return nil
				}
				if err := updater.Rollback(ctx); err != nil {
					return err
				}
				cmd.Println("Restored the previous DevEx binary.")
				return nil
			}
			if viper.GetBool("offline") {
				return fmt.Errorf("self-update requires a network connection; omit --offline or use --rollback")
			}
			release, err := updater.Client.Find(ctx, version, pre)
			if err != nil {
				return err
			}
			if check || dryRun {
				if selfupdate.Newer(release.Tag, current) {
					cmd.Printf("DevEx %s is available (you have %s).\n", strings.TrimPrefix(release.Tag, "v"), current)
				} else {
					cmd.Printf("DevEx %s is installed; selected release is %s.\n", current, release.Tag)
				}
				cmd.Printf("Release notes: %s\n", release.URL)
				return nil
			}
			if version == "" && !selfupdate.Newer(release.Tag, current) {
				if _, err := selfupdate.ParseVersion(current); err == nil {
					cmd.Printf("DevEx %s is up to date.\n", current)
					return nil
				}
			}
			if err := updater.Install(ctx, release); err != nil {
				return err
			}
			cmd.Printf("Updated DevEx to %s.\nRelease notes: %s\n", release.Tag, release.URL)
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Install a specific release (vX.Y.Z)")
	cmd.Flags().BoolVar(&check, "check", false, "Report the selected release without changing files")
	cmd.Flags().BoolVar(&pre, "pre", false, "Include prereleases")
	cmd.Flags().BoolVar(&rollback, "rollback", false, "Restore devex.previous without downloading")
	cmd.MarkFlagsMutuallyExclusive("rollback", "version")
	cmd.MarkFlagsMutuallyExclusive("rollback", "check")
	cmd.MarkFlagsMutuallyExclusive("rollback", "pre")
	return cmd
}

func terminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// startUpdateNotification must stay cheap when suppressed, especially for the
// bare-command startup path. All path and network work happens in the callback.
func startUpdateNotification(cmd *cobra.Command, version string) *selfupdate.Notification {
	policy := updateNotificationPolicy(cmd, version, true)
	if cmd.Name() == "self-update" || cmd.Name() == "completion" || !policy.Enabled() {
		return nil
	}
	if !terminalWriter(cmd.OutOrStdout()) || !terminalWriter(cmd.ErrOrStderr()) {
		return nil
	}
	token := os.Getenv("GITHUB_TOKEN")
	return selfupdate.StartNotification(cmd.Context(), func(ctx context.Context) string {
		dir, err := selfupdate.CacheDir()
		if err != nil {
			return ""
		}
		client := selfupdate.NewClient()
		client.Token = token
		return selfupdate.CheckNotification(ctx, client, version, dir, time.Now())
	})
}

func updateNotificationPolicy(cmd *cobra.Command, version string, tty bool) selfupdate.NotificationPolicy {
	format, _ := cmd.Flags().GetString("format")
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
	return selfupdate.NotificationPolicy{
		Version: version, Format: format,
		TTY:            tty,
		CI:             os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "",
		NonInteractive: nonInteractive || viper.GetBool("non-interactive"),
		Disabled:       os.Getenv("DEVEX_NO_UPDATE_CHECK") == "1" || !viper.GetBool("update_check"),
		Offline:        viper.GetBool("offline"),
	}
}
