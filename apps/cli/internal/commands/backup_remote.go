package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/backup/remote"
	"github.com/jameswlane/devex/apps/cli/internal/config"
)

// newRemoteBackupStore opens the configured remote store. Tests replace it.
var newRemoteBackupStore = func() (remote.Store, error) {
	env := func(primary, fallback string) string {
		if value := os.Getenv(primary); value != "" {
			return value
		}
		return os.Getenv(fallback)
	}
	return remote.NewS3(remote.S3Config{
		Endpoint:  viper.GetString("backup.remote.endpoint"),
		Bucket:    viper.GetString("backup.remote.bucket"),
		Region:    viper.GetString("backup.remote.region"),
		Insecure:  viper.GetBool("backup.remote.insecure"),
		AccessKey: env("DEVEX_BACKUP_ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID"),
		SecretKey: env("DEVEX_BACKUP_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY"),
	})
}

func remoteBackups() (remote.Remote, error) {
	store, err := newRemoteBackupStore()
	if err != nil {
		return remote.Remote{}, err
	}
	prefix := strings.Trim(viper.GetString("backup.remote.prefix"), "/")
	if prefix == "" {
		prefix = "devex-backups"
	}
	return remote.Remote{Store: store, Prefix: prefix + "/"}, nil
}

func remoteBackupPolicy() remote.Policy {
	return remote.Policy{
		Passphrase: os.Getenv("DEVEX_BACKUP_PASSPHRASE"),
		// Unencrypted uploads need an explicit backup.remote.encrypt: false.
		AllowUnencrypted: viper.IsSet("backup.remote.encrypt") && !viper.GetBool("backup.remote.encrypt"),
	}
}

func newConfigBackupPushCmd(settings config.CrossPlatformSettings) *cobra.Command {
	return &cobra.Command{
		Use:   "push <backup-id>",
		Short: "Upload a local backup to remote storage",
		Long: `Upload a local backup to the S3-compatible bucket in backup.remote.

The backup is encrypted with DEVEX_BACKUP_PASSPHRASE (AES-256-GCM, scrypt key
derivation). Without a passphrase the upload is refused unless
backup.remote.encrypt is explicitly false. The upload is read back and its
SHA-256 and size verified before the backup is listed remotely.`,
		Example:      `  DEVEX_BACKUP_PASSPHRASE=... devex config backup push 20261111-120000`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			location, err := remoteBackups()
			if err != nil {
				return err
			}
			manager := backup.NewBackupManager(devexpaths.ForHome(settings.HomeDir).Config)
			manifest, err := location.Push(cmd.Context(), manager, args[0], remoteBackupPolicy())
			if err != nil {
				return err
			}
			state := "encrypted"
			if !manifest.Encrypted {
				state = "unencrypted"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Uploaded backup %s (%d bytes, %s, sha256 %s)\n", manifest.ID, manifest.Size, state, manifest.SHA256[:12])
			return nil
		},
	}
}

func newConfigBackupPullCmd(settings config.CrossPlatformSettings) *cobra.Command {
	return &cobra.Command{
		Use:   "pull <backup-id>",
		Short: "Download a remote backup as a local backup",
		Long: `Download a backup from remote storage, verify its SHA-256 and size, decrypt
it with DEVEX_BACKUP_PASSPHRASE, and store it as a local backup. Nothing is
restored: review it with 'devex config backup list', then apply it with
'devex config backup restore <backup-id>'.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			location, err := remoteBackups()
			if err != nil {
				return err
			}
			manager := backup.NewBackupManager(devexpaths.ForHome(settings.HomeDir).Config)
			manifest, err := location.Pull(cmd.Context(), manager, args[0], remoteBackupPolicy())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Downloaded backup %s; restore it with 'devex config backup restore %s'\n", manifest.ID, manifest.ID)
			return nil
		},
	}
}

func newConfigBackupRemoteCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:          "remote",
		Short:        "List backups in remote storage",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unsupported format %q; use table or json", format)
			}
			location, err := remoteBackups()
			if err != nil {
				return err
			}
			manifests, err := location.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("list remote backups: %w", err)
			}
			if format == "json" {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(manifests)
			}
			if len(manifests) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No remote backups found")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPUSHED\tSIZE\tENCRYPTED")
			for _, manifest := range manifests {
				fmt.Fprintf(w, "%s\t%s\t%d\t%t\n", manifest.ID, manifest.PushedAt.Format(time.RFC3339), manifest.Size, manifest.Encrypted)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	return cmd
}
