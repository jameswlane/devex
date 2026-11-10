package sdk

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"os"
	"path/filepath"
)

var _ = Describe("Cached artifact handoff", func() {
	It("preserves the private path in argv and copies bytes without network", func() {
		source := filepath.Join(GinkgoT().TempDir(), "source")
		destination := filepath.Join(GinkgoT().TempDir(), "destination")
		Expect(os.WriteFile(source, []byte("fixture"), 0600)).To(Succeed())
		Expect(os.WriteFile(destination, []byte("previous longer bytes"), 0600)).To(Succeed())
		metadata := ArtifactVerification{CachedPath: source, SHA256: "pinned"}
		args, parsed, err := ParseVerificationArgs(append(metadata.Arguments(), "https://example.test/tool"))
		Expect(err).NotTo(HaveOccurred())
		Expect(args).To(Equal([]string{"https://example.test/tool"}))
		Expect(parsed).To(Equal(metadata))
		Expect(CopyCachedArtifact(context.Background(), source, destination)).To(Succeed())
		Expect(os.Remove(source)).To(Succeed())
		Expect(os.ReadFile(destination)).To(Equal([]byte("fixture")))
	})
	It("requires a pinned checksum for a local handoff", func() {
		_, _, err := ParseVerificationArgs([]string{"--artifact-path", "/tmp/artifact", "https://example.test/tool"})
		Expect(err).To(MatchError(ContainSubstring("requires a pinned --sha256")))
	})

	It("propagates missing files and cancellation", func() {
		Expect(CopyCachedArtifact(context.Background(), "/missing-artifact", "/unused")).To(HaveOccurred())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(CopyCachedArtifact(ctx, "/unused", "/unused")).To(MatchError(context.Canceled))
	})
})
