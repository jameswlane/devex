package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/security"
)

// NewApprovalCmd creates the plan approval commands used when an organization
// policy requires a signed approval before installing.
func NewApprovalCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approval",
		Short: "Sign and verify installation plan approvals",
		Long: `Sign and verify approvals of installation plans.

When the organization policy sets approval.required, installs run only as a
confirmed plan with a signed approval of its digest:

  1. The requester previews the plan and sends its digest to an approver:
       devex install <apps> --dry-run --format json
  2. An approver listed in approval.approvers signs a decision:
       devex approval sign <digest> --key approver.pem --approver <id> --out approval.json
  3. The requester installs exactly that plan:
       devex install <apps> --expect-plan <digest> --approval approval.json

A rejected, expired or invalid approval, or any change to the plan or policy,
refuses the install. Every check is recorded in 'devex runs audit --event approval'.`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newApprovalKeygenCmd(), newApprovalSignCmd(), newApprovalVerifyCmd(settings))
	return cmd
}

func newApprovalKeygenCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:          "keygen",
		Short:        "Create an approver signing key",
		Example:      `  devex approval keygen --out ~/.config/devex/approver.pem`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if out == "" {
				return fmt.Errorf("--out is required")
			}
			private, public, err := security.GenerateApprovalKey()
			if err != nil {
				return err
			}
			file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return fmt.Errorf("write approver key: %w (keygen never overwrites a key)", err)
			}
			if _, err := file.Write(private); err != nil {
				_ = file.Close()
				return fmt.Errorf("write approver key: %w", err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("write approver key: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote private key %s; keep it secret.\nAdd this approver to the organization policy:\n\napproval:\n  approvers:\n    - id: <approver id>\n      public_key: %s\n", out, public)
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Path for the new PEM private key (created with mode 0600)")
	return cmd
}

func newApprovalSignCmd() *cobra.Command {
	var keyPath, approver, decision, reason, out string
	var expires time.Duration
	cmd := &cobra.Command{
		Use:   "sign <plan-digest>",
		Short: "Sign an approval or rejection of a plan digest",
		Example: `  # Approve a plan for 24 hours
  devex approval sign sha256:... --key approver.pem --approver it-security --out approval.json

  # Reject it
  devex approval sign sha256:... --key approver.pem --approver it-security --decision rejected --reason "use the LTS release" --out approval.json`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyPath == "" || approver == "" || out == "" {
				return fmt.Errorf("--key, --approver and --out are required")
			}
			if !strings.HasPrefix(args[0], "sha256:") {
				return fmt.Errorf("plan digest %q must be the sha256:... digest from 'devex install --dry-run --format json'", args[0])
			}
			if decision != security.ApprovalApproved && decision != security.ApprovalRejected {
				return fmt.Errorf("--decision must be %s or %s", security.ApprovalApproved, security.ApprovalRejected)
			}
			if expires <= 0 {
				return fmt.Errorf("--expires must be positive")
			}
			data, err := os.ReadFile(keyPath)
			if err != nil {
				return fmt.Errorf("read approver key: %w", err)
			}
			key, err := security.ParseApprovalPrivateKey(data)
			if err != nil {
				return fmt.Errorf("read approver key %s: %w", keyPath, err)
			}
			now := time.Now().UTC().Truncate(time.Second)
			approval := security.PlanApproval{SchemaVersion: 1, PlanDigest: args[0], Decision: decision, Approver: approver, Reason: reason, IssuedAt: now, ExpiresAt: now.Add(expires)}
			if err := approval.Sign(key); err != nil {
				return err
			}
			encoded, err := json.MarshalIndent(approval, "", "  ")
			if err != nil {
				return err
			}
			if keyInfo, statErr := os.Stat(keyPath); statErr == nil {
				if outInfo, outErr := os.Stat(out); outErr == nil && os.SameFile(keyInfo, outInfo) {
					return fmt.Errorf("--out %s is the signing key; choose another path", out)
				}
			}
			if err := os.WriteFile(out, append(encoded, '\n'), 0o600); err != nil {
				return fmt.Errorf("write approval: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s decision for %s to %s (expires %s)\n", decision, args[0], out, approval.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&keyPath, "key", "", "Approver PEM private key")
	cmd.Flags().StringVar(&approver, "approver", "", "Approver id from the policy's approval.approvers")
	cmd.Flags().StringVar(&decision, "decision", security.ApprovalApproved, "Decision: approved or rejected")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason recorded with the decision")
	cmd.Flags().DurationVar(&expires, "expires", 24*time.Hour, "How long the decision stays valid (at most the policy's approval.max_validity)")
	cmd.Flags().StringVar(&out, "out", "", "Path for the approval JSON")
	return cmd
}

func newApprovalVerifyCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var plan string
	cmd := &cobra.Command{
		Use:          "verify <approval-file>",
		Short:        "Check an approval against the active organization policy",
		Example:      `  devex approval verify approval.json --plan sha256:...`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			policy, err := security.LoadOrgPolicy(settings.HomeDir)
			if err != nil {
				return err
			}
			approval, err := security.LoadPlanApproval(args[0])
			if err != nil {
				return err
			}
			if plan == "" {
				plan = approval.PlanDigest
			}
			approved, err := policy.VerifyApproval(approval, plan, time.Now())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Valid approval of %s by %s until %s\n", approved.Digest, approved.Approver, approved.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&plan, "plan", "", "Plan digest the approval must cover (default: the digest in the file)")
	return cmd
}
