package sdk_test

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Plugin Trust & Signature Verification", func() {
	var (
		tempDir       string
		keyPairA      *openpgp.Entity
		keyPairAFingerprint string
		pubKeyAPath   string
		testFile      string
		testSigPath   string
		testContent   []byte
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "plugin-trust-test-*")
		Expect(err).ToNot(HaveOccurred())

		// Generate test PGP entity A
		keyPairA, err = openpgp.NewEntity("Test Signer A", "Testing", "signerA@example.com", nil)
		Expect(err).ToNot(HaveOccurred())
		keyPairAFingerprint = fmt.Sprintf("%X", keyPairA.PrimaryKey.Fingerprint)

		// Export public key A to armored file
		pubKeyAPath = filepath.Join(tempDir, "keyA.asc")
		pubFile, err := os.Create(pubKeyAPath)
		Expect(err).ToNot(HaveOccurred())
		armorWriter, err := armor.Encode(pubFile, openpgp.PublicKeyType, nil)
		Expect(err).ToNot(HaveOccurred())
		err = keyPairA.Serialize(armorWriter)
		Expect(err).ToNot(HaveOccurred())
		err = armorWriter.Close()
		Expect(err).ToNot(HaveOccurred())
		err = pubFile.Close()
		Expect(err).ToNot(HaveOccurred())

		// Create test payload file
		testContent = []byte("payload content for plugin binary verification")
		testFile = filepath.Join(tempDir, "plugin.tar.gz")
		err = os.WriteFile(testFile, testContent, 0644)
		Expect(err).ToNot(HaveOccurred())

		// Create detached signature file with Key A
		testSigPath = filepath.Join(tempDir, "plugin.tar.gz.sig")
		sigFile, err := os.Create(testSigPath)
		Expect(err).ToNot(HaveOccurred())
		err = openpgp.ArmoredDetachSign(sigFile, keyPairA, bytes.NewReader(testContent), nil)
		Expect(err).ToNot(HaveOccurred())
		err = sigFile.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	Describe("Fail-Closed Signature Checks (P1-6)", func() {
		Context("when VerifySignatures is enabled without a public key", func() {
			It("should fail closed and return ErrNoPublicKey", func() {
				downloader := sdk.NewSecureDownloader(sdk.DownloaderConfig{
					VerifySignatures: true,
					PublicKeyPath:    "",
				})

				err := downloader.ExportVerifyPluginSignature(testFile, "https://example.com/plugin.tar.gz")
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, sdk.ErrNoPublicKey)).To(BeTrue())
			})
		})
	})

	Describe("Signer Fingerprint Validation (P1-7)", func() {
		Context("with GPGVerifier (Go crypto)", func() {
			It("should accept valid signature when fingerprint matches", func() {
				verifier := sdk.NewGPGVerifier()
				err := verifier.LoadPublicKey(pubKeyAPath)
				Expect(err).ToNot(HaveOccurred())

				verifier.SetExpectedFingerprint(keyPairAFingerprint)
				err = verifier.VerifySignature(testFile, testSigPath)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject valid signature when fingerprint does not match", func() {
				verifier := sdk.NewGPGVerifier()
				err := verifier.LoadPublicKey(pubKeyAPath)
				Expect(err).ToNot(HaveOccurred())

				// Set wrong expected fingerprint
				verifier.SetExpectedFingerprint("0123456789ABCDEF0123456789ABCDEF01234567")
				err = verifier.VerifySignature(testFile, testSigPath)
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, sdk.ErrSignerMismatch)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("expected 0123456789ABCDEF0123456789ABCDEF01234567"))
			})

			It("should reject a fingerprint that only matches the signer's suffix", func() {
				verifier := sdk.NewGPGVerifier()
				err := verifier.LoadPublicKey(pubKeyAPath)
				Expect(err).ToNot(HaveOccurred())

				verifier.SetExpectedFingerprint(keyPairAFingerprint[len(keyPairAFingerprint)-8:])
				err = verifier.VerifySignature(testFile, testSigPath)
				Expect(errors.Is(err, sdk.ErrSignerMismatch)).To(BeTrue())
			})
		})

		Context("with SystemGPGVerifier (system fallback)", func() {
			It("should verify with matching fingerprint and reject with mismatched fingerprint", func() {
				if _, err := exec.LookPath("gpg"); err != nil {
					Skip("gpg binary not available on system")
				}

				sysVerifier, err := sdk.NewSystemGPGVerifier()
				Expect(err).ToNot(HaveOccurred())
				defer func() { _ = sysVerifier.Close() }()

				err = sysVerifier.ImportPublicKey(pubKeyAPath)
				Expect(err).ToNot(HaveOccurred())

				// Matching fingerprint -> passes
				sysVerifier.SetExpectedFingerprint(keyPairAFingerprint)
				err = sysVerifier.VerifyDetachedSignature(testFile, testSigPath)
				Expect(err).ToNot(HaveOccurred())

				// Mismatched fingerprint -> fails with ErrSignerMismatch
				sysVerifier.SetExpectedFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
				err = sysVerifier.VerifyDetachedSignature(testFile, testSigPath)
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, sdk.ErrSignerMismatch)).To(BeTrue())
			})
		})
	})

	Describe("Keyring Isolation & User HOME Untouched (P1-7)", func() {
		It("should never modify user HOME or create ~/.gnupg under HOME", func() {
			if _, err := exec.LookPath("gpg"); err != nil {
				Skip("gpg binary not available on system")
			}

			fakeHome := filepath.Join(tempDir, "fake_home")
			err := os.MkdirAll(fakeHome, 0700)
			Expect(err).ToNot(HaveOccurred())

			// Save and change HOME env var to fakeHome
			origHome := os.Getenv("HOME")
			err = os.Setenv("HOME", fakeHome)
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = os.Setenv("HOME", origHome) }()

			sysVerifier, err := sdk.NewSystemGPGVerifier()
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = sysVerifier.Close() }()

			err = sysVerifier.ImportPublicKey(pubKeyAPath)
			Expect(err).ToNot(HaveOccurred())

			err = sysVerifier.VerifyDetachedSignature(testFile, testSigPath)
			Expect(err).ToNot(HaveOccurred())

			// Verify fakeHome/.gnupg does not exist
			fakeGnupg := filepath.Join(fakeHome, ".gnupg")
			Expect(fakeGnupg).ToNot(BeAnExistingFile())
			_, statErr := os.Stat(fakeGnupg)
			Expect(os.IsNotExist(statErr)).To(BeTrue())
		})
	})

	Describe("Downloader HTTP Signature Verification Flow", func() {
		It("should download signature and verify successfully", func() {
			sigBytes, err := os.ReadFile(testSigPath)
			Expect(err).ToNot(HaveOccurred())

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/plugin.tar.gz.sig" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(sigBytes)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			downloader := sdk.NewSecureDownloader(sdk.DownloaderConfig{
				VerifySignatures: true,
				PublicKeyPath:    pubKeyAPath,
			})

			err = downloader.ExportVerifyPluginSignature(testFile, server.URL+"/plugin.tar.gz")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
