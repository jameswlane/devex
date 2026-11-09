package sdk

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Artifact integrity", func() {
	var path string
	BeforeEach(func() {
		path = filepath.Join(GinkgoT().TempDir(), "artifact")
		Expect(os.WriteFile(path, []byte("payload"), 0600)).To(Succeed())
	})
	It("verifies a pinned digest and detects tampering", func() {
		metadata := ArtifactVerification{Policy: "required", SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("payload")))}
		result, err := VerifyArtifact(context.Background(), path, metadata)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.ChecksumVerified).To(BeTrue())
		Expect(result.SignatureVerified).To(BeFalse())
		Expect(os.WriteFile(path, []byte("tampered"), 0600)).To(Succeed())
		_, err = VerifyArtifact(context.Background(), path, metadata)
		Expect(err).To(MatchError(ContainSubstring("mismatch")))
	})
	It("reports optional absent metadata as skipped", func() {
		result, err := VerifyArtifact(context.Background(), path, ArtifactVerification{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(VerificationResult{}))
	})
	It("rejects missing required metadata and invalid policies or digests", func() {
		for _, metadata := range []ArtifactVerification{{Policy: "required"}, {Policy: "ignore"}, {SHA256: "bad"}, {SignatureURL: "https://example.invalid/sig"}} {
			_, err := VerifyArtifact(context.Background(), path, metadata)
			Expect(err).To(HaveOccurred())
		}
	})
	It("honors cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := VerifyArtifact(ctx, path, ArtifactVerification{})
		Expect(err).To(MatchError(context.Canceled))
	})
	It("does not silently accept unavailable signatures", func() {
		server := httptest.NewServer(http.NotFoundHandler())
		defer server.Close()
		_, err := VerifyArtifact(context.Background(), path, ArtifactVerification{SignatureURL: server.URL, PublicKeyPath: "missing-key"})
		Expect(err).To(HaveOccurred())
	})
	It("round trips plugin metadata flags and rejects missing values", func() {
		expected := ArtifactVerification{Policy: "required", SHA256: "digest", SignatureURL: "url", PublicKeyPath: "key", Fingerprint: "fingerprint"}
		rest, actual, err := ParseVerificationArgs(append(expected.Arguments(), "artifact"))
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).To(Equal(expected))
		Expect(rest).To(Equal([]string{"artifact"}))
		_, _, err = ParseVerificationArgs([]string{"--sha256"})
		Expect(err).To(HaveOccurred())
	})
})
