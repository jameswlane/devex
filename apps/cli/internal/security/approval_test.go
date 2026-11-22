package security

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Organization plan approval", func() {
	const digest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	var (
		teamDir, publicKey string
		privateKey         []byte
		now                time.Time
	)

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		teamDir = filepath.Join(home, "team")
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		original := SystemOrgPolicyPath
		SystemOrgPolicyPath = filepath.Join(home, "missing-system-policy.yaml")
		DeferCleanup(func() { SystemOrgPolicyPath = original })
		var err error
		privateKey, publicKey, err = GenerateApprovalKey()
		Expect(err).NotTo(HaveOccurred())
		now = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	})

	load := func(approval string) (*OrgPolicy, error) {
		Expect(os.WriteFile(filepath.Join(teamDir, "policy.yaml"), []byte("version: '1'\nname: acme\n"+approval), 0o600)).To(Succeed())
		return LoadOrgPolicy("")
	}
	requiredPolicy := func() *OrgPolicy {
		policy, err := load("approval:\n  required: true\n  max_validity: 24h\n  approvers:\n    - id: it\n      public_key: " + publicKey + "\n")
		Expect(err).NotTo(HaveOccurred())
		return policy
	}
	signed := func(change func(*PlanApproval)) PlanApproval {
		approval := PlanApproval{SchemaVersion: 1, PlanDigest: digest, Decision: ApprovalApproved, Approver: "it", IssuedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
		if change != nil {
			change(&approval)
		}
		key, err := ParseApprovalPrivateKey(privateKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(approval.Sign(key)).To(Succeed())
		return approval
	}
	kindOf := func(err error) ApprovalErrorKind {
		var approvalErr *ApprovalError
		Expect(errors.As(err, &approvalErr)).To(BeTrue())
		return approvalErr.Kind
	}

	Describe("policy loading", func() {
		It("fails closed on an approval section it cannot trust", func() {
			_, err := load("approval:\n  required: true\n")
			Expect(err).To(MatchError(ContainSubstring("at least one approval.approvers entry")))
			_, err = load("approval:\n  approvers:\n    - id: it\n      public_key: rsa:abc\n")
			Expect(err).To(MatchError(ContainSubstring("must use the ed25519:<base64> form")))
			_, err = load("approval:\n  approvers:\n    - id: it\n      public_key: " + publicKey + "\n    - id: it\n      public_key: " + publicKey + "\n")
			Expect(err).To(MatchError(ContainSubstring("listed twice")))
			_, err = load("approval:\n  max_validity: forever\n")
			Expect(err).To(MatchError(ContainSubstring("positive duration")))
			_, err = load("approval:\n  approvers:\n    - id: it\n      key: x\n")
			Expect(err).To(HaveOccurred())
		})

		It("defaults to no approval and the default validity cap", func() {
			policy, err := load("")
			Expect(err).NotTo(HaveOccurred())
			Expect(policy.ApprovalRequired()).To(BeFalse())
			Expect(policy.MaxApprovalValidity()).To(Equal(DefaultApprovalMaxValidity))
			Expect(policy.CheckApproval(context.Background())).To(Succeed())
			var none *OrgPolicy
			Expect(none.CheckApproval(context.Background())).To(Succeed())
		})
	})

	Describe("verifying an approval", func() {
		It("accepts a valid approval from a listed approver", func() {
			policy := requiredPolicy()
			approved, err := policy.VerifyApproval(signed(nil), digest, now)
			Expect(err).NotTo(HaveOccurred())
			Expect(approved).To(Equal(ApprovedPlan{Digest: digest, Approver: "it", ExpiresAt: now.Add(time.Hour), PolicySHA256: policy.SHA256}))
		})

		It("reports an authentic rejection with its reason", func() {
			_, err := requiredPolicy().VerifyApproval(signed(func(a *PlanApproval) { a.Decision = ApprovalRejected; a.Reason = "not now" }), digest, now)
			Expect(kindOf(err)).To(Equal(ApprovalWasRejected))
			Expect(err).To(MatchError(ContainSubstring("by it: not now")))
		})

		It("refuses expired, future, overlong and changed approvals", func() {
			policy := requiredPolicy()
			_, err := policy.VerifyApproval(signed(nil), digest, now.Add(2*time.Hour))
			Expect(kindOf(err)).To(Equal(ApprovalExpired))
			_, err = policy.VerifyApproval(signed(func(a *PlanApproval) { a.IssuedAt = now.Add(time.Hour); a.ExpiresAt = now.Add(2 * time.Hour) }), digest, now)
			Expect(err).To(MatchError(ContainSubstring("in the future")))
			_, err = policy.VerifyApproval(signed(func(a *PlanApproval) { a.ExpiresAt = a.IssuedAt.Add(25 * time.Hour) }), digest, now)
			Expect(err).To(MatchError(ContainSubstring("longer than the policy's 24h0m0s")))
			_, err = policy.VerifyApproval(signed(func(a *PlanApproval) { a.ExpiresAt = a.IssuedAt }), digest, now)
			Expect(err).To(MatchError(ContainSubstring("expires_at must be after issued_at")))
			_, err = policy.VerifyApproval(signed(nil), "sha256:other", now)
			Expect(kindOf(err)).To(Equal(ApprovalPlanChanged))
		})

		It("refuses unsigned, tampered and unknown-approver records", func() {
			policy := requiredPolicy()
			tampered := signed(nil)
			tampered.ExpiresAt = tampered.ExpiresAt.Add(time.Minute)
			_, err := policy.VerifyApproval(tampered, digest, now)
			Expect(kindOf(err)).To(Equal(ApprovalInvalid))
			tampered = signed(func(a *PlanApproval) { a.Decision = ApprovalRejected })
			tampered.Decision = ApprovalApproved
			_, err = policy.VerifyApproval(tampered, digest, now)
			Expect(err).To(MatchError(ContainSubstring("signature does not verify")))
			unsigned := signed(nil)
			unsigned.Signature = ""
			_, err = policy.VerifyApproval(unsigned, digest, now)
			Expect(kindOf(err)).To(Equal(ApprovalInvalid))
			_, err = policy.VerifyApproval(signed(func(a *PlanApproval) { a.Approver = "someone" }), digest, now)
			Expect(err).To(MatchError(ContainSubstring(`approver "someone" is not listed`)))
			_, err = policy.VerifyApproval(signed(func(a *PlanApproval) { a.Decision = "maybe" }), digest, now)
			Expect(err).To(MatchError(ContainSubstring(`unknown decision "maybe"`)))
			_, err = policy.VerifyApproval(signed(func(a *PlanApproval) { a.SchemaVersion = 2 }), digest, now)
			Expect(err).To(MatchError(ContainSubstring("unsupported schema_version 2")))
		})

		It("needs configured approvers", func() {
			policy, err := load("")
			Expect(err).NotTo(HaveOccurred())
			_, err = policy.VerifyApproval(signed(nil), digest, now)
			Expect(err).To(MatchError(ContainSubstring("no organization policy approvers are configured")))
		})

		It("refuses to sign with a malformed key instead of panicking", func() {
			approval := PlanApproval{SchemaVersion: 1}
			Expect(approval.Sign(nil)).To(MatchError(ContainSubstring("expected an Ed25519 private key")))
		})

		It("loads records strictly", func() {
			path := filepath.Join(teamDir, "approval.json")
			Expect(os.WriteFile(path, []byte(`{"schema_version":1,"extra":true}`), 0o600)).To(Succeed())
			_, err := LoadPlanApproval(path)
			Expect(err).To(MatchError(ContainSubstring("unknown field")))
		})
	})

	Describe("enforcing during installation", func() {
		It("requires a verified, unexpired approval for this policy file", func() {
			policy := requiredPolicy()
			Expect(kindOf(policy.CheckApproval(context.Background()))).To(Equal(ApprovalMissing))

			valid := ApprovedPlan{Digest: digest, Approver: "it", ExpiresAt: time.Now().Add(time.Hour), PolicySHA256: policy.SHA256}
			Expect(policy.CheckApproval(WithApprovedPlan(context.Background(), valid))).To(Succeed())

			expired := valid
			expired.ExpiresAt = time.Now().Add(-time.Second)
			Expect(kindOf(policy.CheckApproval(WithApprovedPlan(context.Background(), expired)))).To(Equal(ApprovalExpired))

			otherPolicy := valid
			otherPolicy.PolicySHA256 = "different"
			Expect(kindOf(policy.CheckApproval(WithApprovedPlan(context.Background(), otherPolicy)))).To(Equal(ApprovalPlanChanged))
		})
	})
})
