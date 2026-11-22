package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Install with an organization plan approval", func() {
	var (
		home, policyPath, keyPath string
		policyYAML                string
		settings                  config.CrossPlatformSettings
		output                    bytes.Buffer
		installed                 []string
		installErrs               []error
	)
	git := types.CrossPlatformApp{Name: "git", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "git"}, Windows: types.OSConfig{InstallMethod: "winget", InstallCommand: "git"}}

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("CI", "true")
		GinkgoT().Setenv("DEVEX_CONFIG_DIR", filepath.Join(home, "config"))
		GinkgoT().Setenv("DEVEX_DATA_DIR", filepath.Join(home, "data"))
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", "")
		teamDir := filepath.Join(home, "team")
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		policyPath = filepath.Join(teamDir, "policy.yaml")
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))

		private, public, err := security.GenerateApprovalKey()
		Expect(err).NotTo(HaveOccurred())
		keyPath = filepath.Join(home, "approver.pem")
		Expect(os.WriteFile(keyPath, private, 0o600)).To(Succeed())
		policyYAML = "version: '2026.11.1'\nname: acme\napproval:\n  required: true\n  max_validity: 48h\n  approvers:\n    - id: it-security\n      public_key: " + public + "\n"
		Expect(os.WriteFile(policyPath, []byte(policyYAML), 0o600)).To(Succeed())

		oldPrepare, oldInstall := prepareApplicationInstallers, installApplication
		DeferCleanup(func() { prepareApplicationInstallers, installApplication = oldPrepare, oldInstall })
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		installed, installErrs = nil, nil
		installApplication = func(ctx context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			// The installers enforce the approval again from this context.
			policy, err := security.LoadOrgPolicy(home)
			Expect(err).NotTo(HaveOccurred())
			installErrs = append(installErrs, policy.CheckApproval(ctx))
			installed = append(installed, app.Name)
			return nil
		}
		settings = config.CrossPlatformSettings{HomeDir: home, ApplicationOverrides: []types.CrossPlatformApp{git}}
		output.Reset()
	})

	run := func(args ...string) error {
		cmd := NewRootCmd("test", mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}
	install := func(args ...string) error {
		cmd := NewInstallCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}
	preview := func() (digest string, approvalRequired bool) {
		viper.Set("dry-run", true)
		defer viper.Set("dry-run", false)
		output.Reset()
		Expect(install("git", "--format", "json")).To(Succeed())
		var plan struct {
			Digest             string `json:"digest"`
			OrganizationPolicy struct {
				ApprovalRequired bool `json:"approval_required"`
			} `json:"organization_policy"`
		}
		Expect(json.Unmarshal(output.Bytes(), &plan)).To(Succeed())
		output.Reset()
		return plan.Digest, plan.OrganizationPolicy.ApprovalRequired
	}
	// sign writes a signed decision as 'devex approval sign' does; running the
	// root command here would reset the configuration the digest binds.
	sign := func(digest, decision, reason string) string {
		data, err := os.ReadFile(keyPath)
		Expect(err).NotTo(HaveOccurred())
		key, err := security.ParseApprovalPrivateKey(data)
		Expect(err).NotTo(HaveOccurred())
		issued := time.Now().UTC().Truncate(time.Second)
		approval := security.PlanApproval{SchemaVersion: 1, PlanDigest: digest, Decision: decision, Approver: "it-security", Reason: reason, IssuedAt: issued, ExpiresAt: issued.Add(time.Hour)}
		Expect(approval.Sign(key)).To(Succeed())
		encoded, err := json.Marshal(approval)
		Expect(err).NotTo(HaveOccurred())
		path := filepath.Join(home, "approval.json")
		Expect(os.WriteFile(path, encoded, 0o600)).To(Succeed())
		return path
	}
	auditEvents := func() []state.AuditEvent {
		path, err := state.ConfiguredDBPath()
		Expect(err).NotTo(HaveOccurred())
		store, err := state.OpenBoltStore(path, 0o600)
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		events, err := store.ListAudit(state.AuditFilter{Event: "approval"})
		Expect(err).NotTo(HaveOccurred())
		return events
	}

	It("marks the previewed plan as needing approval", func() {
		digest, required := preview()
		Expect(digest).To(MatchRegexp(`^sha256:[0-9a-f]{64}$`))
		Expect(required).To(BeTrue())
	})

	It("refuses an unconfirmed install and records the refusal", func() {
		err := install("git", "--non-interactive")
		Expect(err).To(MatchError(ContainSubstring("requires an approved plan before installing")))
		Expect(installed).To(BeEmpty())
		events := auditEvents()
		Expect(events).To(HaveLen(1))
		Expect(events[0].Status).To(Equal(state.OperationStatusFailed))
		Expect(events[0].PolicyID).To(Equal("acme@2026.11.1"))
	})

	It("refuses an unconfirmed setup", func() {
		Expect(run("setup", "--non-interactive")).To(MatchError(ContainSubstring("requires an approved plan before installing")))
		Expect(installed).To(BeEmpty())
	})

	It("refuses a confirmed plan without an approval", func() {
		digest, _ := preview()
		Expect(install("git", "--non-interactive", "--expect-plan", digest)).To(MatchError(ContainSubstring("no approval was supplied")))
		Expect(installed).To(BeEmpty())
	})

	It("requires --expect-plan with --approval", func() {
		Expect(install("git", "--approval", "approval.json")).To(MatchError(ContainSubstring("use it with --expect-plan")))
	})

	It("installs an approved plan and records who approved it", func() {
		digest, _ := preview()
		approval := sign(digest, security.ApprovalApproved, "baseline tools")
		Expect(install("git", "--non-interactive", "--expect-plan", digest, "--approval", approval)).To(Succeed())
		Expect(installed).To(Equal([]string{"git"}))
		Expect(installErrs).To(Equal([]error{nil}))
		events := auditEvents()
		Expect(events).To(HaveLen(1))
		Expect(events[0].Status).To(Equal(state.OperationStatusSucceeded))
		Expect(events[0].Approver).To(Equal("it-security"))
		Expect(events[0].PlanID).To(Equal(digest))
		Expect(events[0].Target).To(Equal(digest))
	})

	It("refuses a rejected plan with the approver's reason", func() {
		digest, _ := preview()
		approval := sign(digest, security.ApprovalRejected, "use the LTS build")
		err := install("git", "--non-interactive", "--expect-plan", digest, "--approval", approval)
		Expect(err).To(MatchError(ContainSubstring("the plan was rejected (by it-security: use the LTS build)")))
		Expect(installed).To(BeEmpty())
		events := auditEvents()
		Expect(events).To(HaveLen(1))
		Expect(events[0].Status).To(Equal(state.OperationStatusFailed))
		Expect(events[0].Error).To(ContainSubstring("rejected"))
	})

	It("refuses an expired approval", func() {
		digest, _ := preview()
		data, err := os.ReadFile(keyPath)
		Expect(err).NotTo(HaveOccurred())
		key, err := security.ParseApprovalPrivateKey(data)
		Expect(err).NotTo(HaveOccurred())
		issued := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
		approval := security.PlanApproval{SchemaVersion: 1, PlanDigest: digest, Decision: security.ApprovalApproved, Approver: "it-security", IssuedAt: issued, ExpiresAt: issued.Add(time.Hour)}
		Expect(approval.Sign(key)).To(Succeed())
		encoded, err := json.Marshal(approval)
		Expect(err).NotTo(HaveOccurred())
		path := filepath.Join(home, "expired.json")
		Expect(os.WriteFile(path, encoded, 0o600)).To(Succeed())

		err = install("git", "--non-interactive", "--expect-plan", digest, "--approval", path)
		Expect(err).To(MatchError(ContainSubstring("the plan approval expired")))
		Expect(installed).To(BeEmpty())
	})

	It("refuses an approval of a different plan", func() {
		digest, _ := preview()
		approval := sign("sha256:"+"0000000000000000000000000000000000000000000000000000000000000000", security.ApprovalApproved, "")
		err := install("git", "--non-interactive", "--expect-plan", digest, "--approval", approval)
		Expect(err).To(MatchError(ContainSubstring("the approval is for a different plan")))
		Expect(installed).To(BeEmpty())
	})

	It("invalidates the approval when the policy changes", func() {
		digest, _ := preview()
		approval := sign(digest, security.ApprovalApproved, "")
		Expect(os.WriteFile(policyPath, []byte(policyYAML+"# revised\n"), 0o600)).To(Succeed())
		err := install("git", "--non-interactive", "--expect-plan", digest, "--approval", approval)
		Expect(err).To(MatchError(ContainSubstring("plan changed")))
		Expect(installed).To(BeEmpty())
		events := auditEvents()
		Expect(events).To(HaveLen(1))
		Expect(events[0].Status).To(Equal(state.OperationStatusFailed))
		Expect(events[0].PlanID).To(Equal(digest))
		Expect(events[0].Approver).To(Equal("it-security"))
		Expect(events[0].Error).To(ContainSubstring("plan changed"))
	})

	It("refuses an approval when the policy changed after the plan was resolved", func() {
		digest, _ := preview()
		approval := sign(digest, security.ApprovalApproved, "")
		snapshot := agentapi.ExecutionSnapshot{Digest: digest, Plan: agentapi.Plan{OrganizationPolicy: &agentapi.PolicyRef{SHA256: "resolved-under-an-older-policy"}}}
		_, err := approvePlan(context.Background(), home, snapshot, approval)
		Expect(err).To(MatchError(ContainSubstring("the organization policy changed")))
		Expect(auditEvents()).To(HaveLen(1))
	})

	It("generates keys without overwriting and verifies signed approvals", func() {
		newKey := filepath.Join(home, "new.pem")
		Expect(run("approval", "keygen", "--out", newKey)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("public_key: ed25519:"))
		info, err := os.Stat(newKey)
		Expect(err).NotTo(HaveOccurred())
		if runtime.GOOS != "windows" {
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
		}
		Expect(run("approval", "keygen", "--out", newKey)).To(MatchError(ContainSubstring("never overwrites")))

		digest := "sha256:" + "2222222222222222222222222222222222222222222222222222222222222222"
		approval := filepath.Join(home, "signed.json")
		Expect(run("approval", "sign", digest, "--key", keyPath, "--approver", "it-security", "--expires", "2h", "--out", approval)).To(Succeed())
		output.Reset()
		Expect(run("approval", "verify", approval, "--plan", digest)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Valid approval of " + digest + " by it-security"))
		Expect(run("approval", "sign", digest, "--key", keyPath, "--approver", "it-security", "--decision", "maybe", "--out", approval)).To(MatchError(ContainSubstring("--decision must be")))
		Expect(run("approval", "sign", digest, "--key", keyPath, "--approver", "it-security", "--out", keyPath)).To(MatchError(ContainSubstring("is the signing key")))
		Expect(os.ReadFile(keyPath)).To(ContainSubstring("PRIVATE KEY"))
		Expect(run("approval", "sign", "abc", "--key", keyPath, "--approver", "it-security", "--out", approval)).To(MatchError(ContainSubstring("must be the sha256")))
	})
})
