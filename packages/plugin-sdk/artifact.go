package sdk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ArtifactVerification describes independently pinned download integrity metadata.
// Optional metadata is checked whenever supplied; required policy needs a digest
// or a signature and a local trusted public key. Consent is a separate policy.
type ArtifactVerification struct {
	Policy        string `json:"policy" yaml:"policy" mapstructure:"policy"`
	SHA256        string `json:"sha256" yaml:"sha256" mapstructure:"sha256"`
	SignatureURL  string `json:"signature_url" yaml:"signature_url" mapstructure:"signature_url"`
	PublicKeyPath string `json:"public_key" yaml:"public_key" mapstructure:"public_key"`
	Fingerprint   string `json:"fingerprint" yaml:"fingerprint" mapstructure:"fingerprint"`
}

// VerificationResult distinguishes checks actually performed from skipped checks.
type VerificationResult struct {
	ChecksumVerified  bool
	SignatureVerified bool
}

// VerifyArtifact checks the exact file that will be installed or executed.
func VerifyArtifact(ctx context.Context, path string, metadata ArtifactVerification) (VerificationResult, error) {
	result := VerificationResult{}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if metadata.Policy != "" && metadata.Policy != "optional" && metadata.Policy != "required" {
		return result, fmt.Errorf("unknown verification policy %q", metadata.Policy)
	}
	if metadata.Policy == "required" && metadata.SHA256 == "" && metadata.SignatureURL == "" {
		return result, fmt.Errorf("required verification metadata is missing")
	}
	if metadata.SHA256 != "" {
		expected := strings.ToLower(strings.TrimPrefix(metadata.SHA256, "sha256:"))
		decoded, err := hex.DecodeString(expected)
		if err != nil || len(decoded) != sha256.Size {
			return result, fmt.Errorf("invalid SHA256 digest")
		}
		file, err := os.Open(path)
		if err != nil {
			return result, err
		}
		defer func() { _ = file.Close() }()
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return result, err
		}
		if hex.EncodeToString(hash.Sum(nil)) != expected {
			return result, fmt.Errorf("artifact checksum mismatch")
		}
		result.ChecksumVerified = true
	}
	if metadata.SignatureURL != "" {
		if metadata.PublicKeyPath == "" || strings.Contains(metadata.PublicKeyPath, "://") {
			return result, fmt.Errorf("signature verification requires a local trusted public key")
		}
		verifier := NewGPGVerifier()
		verifier.SetExpectedFingerprint(metadata.Fingerprint)
		if err := verifier.LoadPublicKey(metadata.PublicKeyPath); err != nil {
			return result, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadata.SignatureURL, nil)
		if err != nil {
			return result, err
		}
		response, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			return result, err
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return result, fmt.Errorf("signature unavailable: HTTP %d", response.StatusCode)
		}
		signature, err := os.CreateTemp("", "devex-signature-*")
		if err != nil {
			return result, err
		}
		defer func() { _ = os.Remove(signature.Name()) }()
		_, copyErr := io.Copy(signature, io.LimitReader(response.Body, 1024*1024))
		closeErr := signature.Close()
		if copyErr != nil {
			return result, copyErr
		}
		if closeErr != nil {
			return result, closeErr
		}
		if err := verifier.VerifySignature(path, signature.Name()); err != nil {
			return result, err
		}
		result.SignatureVerified = true
	}
	return result, ctx.Err()
}

// ParseVerificationArgs removes integrity options from a direct download request.
// One metadata set applies to one artifact, preventing accidental pin reuse.
func ParseVerificationArgs(args []string) ([]string, ArtifactVerification, error) {
	metadata := ArtifactVerification{}
	var rest []string
	for i := 0; i < len(args); i++ {
		var target *string
		switch args[i] {
		case "--verification-policy":
			target = &metadata.Policy
		case "--sha256":
			target = &metadata.SHA256
		case "--signature-url":
			target = &metadata.SignatureURL
		case "--public-key":
			target = &metadata.PublicKeyPath
		case "--signer-fingerprint":
			target = &metadata.Fingerprint
		}
		if target == nil {
			rest = append(rest, args[i])
			continue
		}
		if i+1 == len(args) {
			return nil, metadata, fmt.Errorf("missing value for %s", args[i])
		}
		i++
		*target = args[i]
	}
	if metadata.Policy != "" && metadata.Policy != "required" && metadata.Policy != "optional" {
		return nil, metadata, fmt.Errorf("unknown verification policy %q", metadata.Policy)
	}
	return rest, metadata, nil
}

// Arguments encodes metadata for plugin argv dispatch without shell interpolation.
func (m ArtifactVerification) Arguments() []string {
	var args []string
	for _, entry := range [][2]string{{"--verification-policy", m.Policy}, {"--sha256", m.SHA256}, {"--signature-url", m.SignatureURL}, {"--public-key", m.PublicKeyPath}, {"--signer-fingerprint", m.Fingerprint}} {
		if entry[1] != "" {
			args = append(args, entry[0], entry[1])
		}
	}
	return args
}
