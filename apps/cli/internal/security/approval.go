package security

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// DefaultApprovalMaxValidity caps how long an approval stays valid when the
// policy sets no approval.max_validity.
const DefaultApprovalMaxValidity = 72 * time.Hour

// approvalClockSkew tolerates small clock differences between the approver's
// machine and the installing machine.
const approvalClockSkew = 5 * time.Minute

const approvalSignaturePrefix = "devex-plan-approval-v1\n"

// Approval decisions.
const (
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
)

// OrgApprover is an identity whose signed decisions the policy trusts.
type OrgApprover struct {
	ID        string `yaml:"id" json:"id"`
	PublicKey string `yaml:"public_key" json:"public_key"`
}

// OrgApprovalPolicy requires a signed approval of the exact plan before an
// installation runs.
type OrgApprovalPolicy struct {
	Required    bool          `yaml:"required" json:"required"`
	MaxValidity string        `yaml:"max_validity,omitempty" json:"max_validity,omitempty"`
	Approvers   []OrgApprover `yaml:"approvers,omitempty" json:"approvers"`
}

// PlanApproval is an approver's signed decision about one plan digest.
type PlanApproval struct {
	SchemaVersion int       `json:"schema_version"`
	PlanDigest    string    `json:"plan_digest"`
	Decision      string    `json:"decision"`
	Approver      string    `json:"approver"`
	Reason        string    `json:"reason,omitempty"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Signature     string    `json:"signature,omitempty"`
}

// ApprovalErrorKind classifies why an installation has no valid approval.
type ApprovalErrorKind string

const (
	ApprovalMissing     ApprovalErrorKind = "missing"
	ApprovalInvalid     ApprovalErrorKind = "invalid"
	ApprovalPlanChanged ApprovalErrorKind = "plan_changed"
	ApprovalWasRejected ApprovalErrorKind = "rejected"
	ApprovalExpired     ApprovalErrorKind = "expired"
)

// ApprovalError explains why the organization policy refused an installation.
type ApprovalError struct {
	Kind          ApprovalErrorKind
	PolicyName    string
	PolicyVersion string
	Detail        string
}

func (e *ApprovalError) Error() string {
	name := e.PolicyName
	if name == "" {
		name = "organization"
	}
	prefix := fmt.Sprintf("organization policy %s %s", name, e.PolicyVersion)
	switch e.Kind {
	case ApprovalMissing:
		return fmt.Sprintf("%s requires an approved plan before installing (%s); preview it with 'devex install <apps> --dry-run --format json', have an approver sign its digest with 'devex approval sign', then run 'devex install <apps> --expect-plan <digest> --approval <file>'", prefix, e.Detail)
	case ApprovalWasRejected:
		return fmt.Sprintf("%s: the plan was rejected (%s); nothing was installed", prefix, e.Detail)
	case ApprovalExpired:
		return fmt.Sprintf("%s: the plan approval expired (%s); nothing more will be installed; request a new approval", prefix, e.Detail)
	case ApprovalPlanChanged:
		return fmt.Sprintf("%s: the approval is for a different plan (%s); nothing was installed; request approval for the current plan", prefix, e.Detail)
	default:
		return fmt.Sprintf("%s: invalid plan approval (%s); nothing was installed", prefix, e.Detail)
	}
}

func (p *OrgPolicy) approvalError(kind ApprovalErrorKind, format string, args ...any) error {
	return &ApprovalError{Kind: kind, PolicyName: p.Name, PolicyVersion: p.Version, Detail: fmt.Sprintf(format, args...)}
}

// ApprovalRequired reports whether installations need a signed plan approval.
func (p *OrgPolicy) ApprovalRequired() bool {
	return p != nil && p.Approval.Required
}

// MaxApprovalValidity returns the longest validity window the policy accepts.
func (p *OrgPolicy) MaxApprovalValidity() time.Duration {
	if p == nil || p.Approval.MaxValidity == "" {
		return DefaultApprovalMaxValidity
	}
	validity, err := time.ParseDuration(p.Approval.MaxValidity)
	if err != nil {
		return DefaultApprovalMaxValidity
	}
	return validity
}

func (a OrgApprovalPolicy) validate() error {
	if a.Required && len(a.Approvers) == 0 {
		return fmt.Errorf("approval.required needs at least one approval.approvers entry")
	}
	if a.MaxValidity != "" {
		validity, err := time.ParseDuration(a.MaxValidity)
		if err != nil || validity <= 0 {
			return fmt.Errorf("approval.max_validity must be a positive duration such as 72h")
		}
	}
	seen := map[string]bool{}
	for _, approver := range a.Approvers {
		if strings.TrimSpace(approver.ID) == "" {
			return fmt.Errorf("approval.approvers entries need an id")
		}
		if seen[approver.ID] {
			return fmt.Errorf("approval.approvers id %q is listed twice", approver.ID)
		}
		seen[approver.ID] = true
		if _, err := ParseApprovalPublicKey(approver.PublicKey); err != nil {
			return fmt.Errorf("approval.approvers %q public_key: %w", approver.ID, err)
		}
	}
	return nil
}

// ParseApprovalPublicKey parses an "ed25519:<base64>" approver key.
func ParseApprovalPublicKey(value string) (ed25519.PublicKey, error) {
	encoded, ok := strings.CutPrefix(strings.TrimSpace(value), "ed25519:")
	if !ok {
		return nil, fmt.Errorf("must use the ed25519:<base64> form")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("must be a base64 Ed25519 public key")
	}
	return ed25519.PublicKey(key), nil
}

// FormatApprovalPublicKey formats a key for approval.approvers[].public_key.
func FormatApprovalPublicKey(key ed25519.PublicKey) string {
	return "ed25519:" + base64.StdEncoding.EncodeToString(key)
}

// GenerateApprovalKey returns a PKCS#8 PEM private key and its policy public key.
func GenerateApprovalKey() (privatePEM []byte, publicKey string, err error) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, "", err
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, "", err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), FormatApprovalPublicKey(public), nil
}

// ParseApprovalPrivateKey reads a PKCS#8 PEM Ed25519 key, such as one from
// 'devex approval keygen' or 'openssl genpkey -algorithm ed25519'.
func ParseApprovalPrivateKey(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("expected a PKCS#8 PEM private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	private, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected an Ed25519 private key")
	}
	return private, nil
}

func (a PlanApproval) signedBytes() ([]byte, error) {
	a.Signature = ""
	data, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return append([]byte(approvalSignaturePrefix), data...), nil
}

// Sign sets the approval's signature.
func (a *PlanApproval) Sign(key ed25519.PrivateKey) error {
	if len(key) != ed25519.PrivateKeySize {
		return fmt.Errorf("expected an Ed25519 private key")
	}
	data, err := a.signedBytes()
	if err != nil {
		return err
	}
	a.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, data))
	return nil
}

// LoadPlanApproval reads an approval record, rejecting unknown fields.
func LoadPlanApproval(path string) (PlanApproval, error) {
	var approval PlanApproval
	data, err := os.ReadFile(path)
	if err != nil {
		return approval, fmt.Errorf("read plan approval %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&approval); err != nil {
		return approval, fmt.Errorf("invalid plan approval %s: %w", path, err)
	}
	return approval, nil
}

// VerifyApproval checks that an approval was signed by a policy approver for
// this plan digest, approves it, and is valid at now. A rejection is only
// reported once its signature is verified.
func (p *OrgPolicy) VerifyApproval(approval PlanApproval, planDigest string, now time.Time) (ApprovedPlan, error) {
	if p == nil || len(p.Approval.Approvers) == 0 {
		return ApprovedPlan{}, errors.New("no organization policy approvers are configured; --approval needs an approval.approvers entry in the organization policy")
	}
	if approval.SchemaVersion != 1 {
		return ApprovedPlan{}, p.approvalError(ApprovalInvalid, "unsupported schema_version %d", approval.SchemaVersion)
	}
	var public ed25519.PublicKey
	for _, approver := range p.Approval.Approvers {
		if approver.ID == approval.Approver {
			public, _ = ParseApprovalPublicKey(approver.PublicKey)
		}
	}
	if public == nil {
		return ApprovedPlan{}, p.approvalError(ApprovalInvalid, "approver %q is not listed in the policy", approval.Approver)
	}
	signature, err := base64.StdEncoding.DecodeString(approval.Signature)
	data, marshalErr := approval.signedBytes()
	if err != nil || marshalErr != nil || !ed25519.Verify(public, data, signature) {
		return ApprovedPlan{}, p.approvalError(ApprovalInvalid, "signature does not verify for approver %q", approval.Approver)
	}
	if approval.PlanDigest != planDigest {
		return ApprovedPlan{}, p.approvalError(ApprovalPlanChanged, "approved %s, current plan %s", approval.PlanDigest, planDigest)
	}
	switch approval.Decision {
	case ApprovalApproved:
	case ApprovalRejected:
		reason := approval.Reason
		if reason == "" {
			reason = "no reason given"
		}
		return ApprovedPlan{}, p.approvalError(ApprovalWasRejected, "by %s: %s", approval.Approver, reason)
	default:
		return ApprovedPlan{}, p.approvalError(ApprovalInvalid, "unknown decision %q", approval.Decision)
	}
	if !approval.ExpiresAt.After(approval.IssuedAt) {
		return ApprovedPlan{}, p.approvalError(ApprovalInvalid, "expires_at must be after issued_at")
	}
	if validity := approval.ExpiresAt.Sub(approval.IssuedAt); validity > p.MaxApprovalValidity() {
		return ApprovedPlan{}, p.approvalError(ApprovalInvalid, "valid for %s, longer than the policy's %s", validity, p.MaxApprovalValidity())
	}
	if approval.IssuedAt.After(now.Add(approvalClockSkew)) {
		return ApprovedPlan{}, p.approvalError(ApprovalInvalid, "issued_at %s is in the future", approval.IssuedAt.Format(time.RFC3339))
	}
	if !now.Before(approval.ExpiresAt) {
		return ApprovedPlan{}, p.approvalError(ApprovalExpired, "expired at %s", approval.ExpiresAt.Format(time.RFC3339))
	}
	return ApprovedPlan{Digest: planDigest, Approver: approval.Approver, ExpiresAt: approval.ExpiresAt, PolicySHA256: p.SHA256}, nil
}

// ApprovedPlan is a verified approval carried through an installation.
type ApprovedPlan struct {
	Digest       string
	Approver     string
	ExpiresAt    time.Time
	PolicySHA256 string
}

type approvedPlanKey struct{}

// WithApprovedPlan marks ctx as running a verified, approved plan.
func WithApprovedPlan(ctx context.Context, approved ApprovedPlan) context.Context {
	return context.WithValue(ctx, approvedPlanKey{}, approved)
}

// CheckApproval blocks an installation step when the policy requires an
// approval and ctx carries no approval that is verified against this policy
// file and still valid. Expiry is checked again at every step, so a long run
// stops once its approval expires.
func (p *OrgPolicy) CheckApproval(ctx context.Context) error {
	if !p.ApprovalRequired() {
		return nil
	}
	approved, ok := ctx.Value(approvedPlanKey{}).(ApprovedPlan)
	if !ok {
		return p.approvalError(ApprovalMissing, "no approval was supplied")
	}
	if approved.PolicySHA256 != p.SHA256 {
		return p.approvalError(ApprovalPlanChanged, "the organization policy changed after the approval was verified")
	}
	if !time.Now().Before(approved.ExpiresAt) {
		return p.approvalError(ApprovalExpired, "expired at %s", approved.ExpiresAt.Format(time.RFC3339))
	}
	return nil
}
