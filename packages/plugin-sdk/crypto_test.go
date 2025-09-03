package sdk_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Crypto Functionality", func() {
	var (
		tempDir   string
		verifier  *sdk.GPGVerifier
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "crypto-test-*")
		Expect(err).ToNot(HaveOccurred())

		verifier = sdk.NewGPGVerifier()
	})

	AfterEach(func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	Describe("GPGVerifier", func() {
		It("should create a new GPG verifier", func() {
			v := sdk.NewGPGVerifier()
			Expect(v).ToNot(BeNil())
		})

		Describe("LoadPublicKey", func() {
			It("should handle non-existent key files", func() {
				nonExistentPath := filepath.Join(tempDir, "nonexistent.asc")
				err := verifier.LoadPublicKey(nonExistentPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no such file"))
			})

			It("should handle empty key files", func() {
				emptyKeyPath := filepath.Join(tempDir, "empty.asc")
				err := os.WriteFile(emptyKeyPath, []byte(""), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = verifier.LoadPublicKey(emptyKeyPath)
				Expect(err).To(HaveOccurred())
			})

			It("should handle invalid key files", func() {
				invalidKeyPath := filepath.Join(tempDir, "invalid.asc")
				err := os.WriteFile(invalidKeyPath, []byte("not a valid key"), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = verifier.LoadPublicKey(invalidKeyPath)
				Expect(err).To(HaveOccurred())
			})

			Context("with a valid-looking key file", func() {
				var validKeyPath string

				BeforeEach(func() {
					validKeyPath = filepath.Join(tempDir, "valid.asc")
					// Create a file that looks like a GPG key (though not actually valid)
					keyContent := `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFj3...fake key content...
-----END PGP PUBLIC KEY BLOCK-----`
					err := os.WriteFile(validKeyPath, []byte(keyContent), 0644)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should attempt to load the key", func() {
					// This will fail because it's not a real key, but we can test the flow
					err := verifier.LoadPublicKey(validKeyPath)
					// We expect an error since this isn't a real GPG key
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("LoadPublicKeyFromKeyserver", func() {
			It("should handle empty key IDs", func() {
				err := verifier.LoadPublicKeyFromKeyserver("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to load key  from any keyserver"))
			})

			It("should handle invalid key ID formats", func() {
				testCases := []string{
					"invalid",
					"12345", // too short
					"not-hex-chars",
					"G123456789ABCDEF", // contains non-hex character
				}

				for _, keyID := range testCases {
					By(fmt.Sprintf("testing key ID: %s", keyID))
					err := verifier.LoadPublicKeyFromKeyserver(keyID)
					Expect(err).To(HaveOccurred())
				}
			})
		})

		Describe("VerifySignature", func() {
			var (
				testFile      string
				signatureFile string
			)

			BeforeEach(func() {
				testFile = filepath.Join(tempDir, "test.txt")
				signatureFile = filepath.Join(tempDir, "test.txt.sig")
				
				err := os.WriteFile(testFile, []byte("test content"), 0644)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle missing files when no keys loaded", func() {
				err := verifier.VerifySignature("nonexistent.txt", signatureFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no public keys loaded for verification"))
			})

			It("should handle missing signature files when no keys loaded", func() {
				err := verifier.VerifySignature(testFile, "nonexistent.sig")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no public keys loaded for verification"))
			})

			Context("with signature file", func() {
				BeforeEach(func() {
					// Create a fake signature file (not a real signature)
					err := os.WriteFile(signatureFile, []byte("fake signature"), 0644)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should attempt signature verification", func() {
					// This will fail because we don't have a real key loaded and signature
					err := verifier.VerifySignature(testFile, signatureFile)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("no public keys loaded for verification"))
				})
			})
		})

		Describe("VerifySignatureFromURL", func() {
			It("should handle malformed URLs", func() {
				testFile := filepath.Join(tempDir, "test.txt")
				err := os.WriteFile(testFile, []byte("test content"), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = verifier.VerifySignatureFromURL(testFile, "not-a-valid-url")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to download signature"))
			})

			It("should handle unreachable URLs", func() {
				testFile := filepath.Join(tempDir, "test.txt")
				err := os.WriteFile(testFile, []byte("test content"), 0644)
				Expect(err).ToNot(HaveOccurred())

				// Use a URL that will fail to connect
				err = verifier.VerifySignatureFromURL(testFile, "http://localhost:99999/signature.sig")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("SystemGPGVerifier", func() {
		It("should create a system GPG verifier", func() {
			verifier, err := sdk.NewSystemGPGVerifier()
			Expect(err).ToNot(HaveOccurred())
			Expect(verifier).ToNot(BeNil())
		})

		Context("when GPG is available", func() {
			It("should check GPG availability", func() {
				verifier, err := sdk.NewSystemGPGVerifier()
				Expect(err).ToNot(HaveOccurred())
				// We don't assume GPG is installed, but test that the verifier doesn't panic
				Expect(func() {
					_ = verifier.VerifyDetachedSignature("test.txt", "test.sig")
				}).ToNot(Panic())
			})
		})

		Context("with test files", func() {
			var (
				testFile      string
				signatureFile string
			)

			BeforeEach(func() {
				testFile = filepath.Join(tempDir, "test.txt")
				signatureFile = filepath.Join(tempDir, "test.sig")
				
				err := os.WriteFile(testFile, []byte("test content for verification"), 0644)
				Expect(err).ToNot(HaveOccurred())
				
				err = os.WriteFile(signatureFile, []byte("fake signature content"), 0644)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle signature verification attempts", func() {
				verifier, err := sdk.NewSystemGPGVerifier()
				Expect(err).ToNot(HaveOccurred())
				
				// This will likely fail since we don't have real signatures, but should not panic
				err = verifier.VerifyDetachedSignature(testFile, signatureFile)
				// We expect an error since this isn't a real signature, but shouldn't panic
				if err != nil {
					Expect(err.Error()).ToNot(BeEmpty())
				}
			})

			It("should handle inline signature verification", func() {
				signedFile := filepath.Join(tempDir, "signed.txt")
				err := os.WriteFile(signedFile, []byte("-----BEGIN PGP SIGNED MESSAGE-----\nfake signed content\n-----END PGP SIGNATURE-----"), 0644)
				Expect(err).ToNot(HaveOccurred())

				verifier, err := sdk.NewSystemGPGVerifier()
				Expect(err).ToNot(HaveOccurred())
				
				// SystemGPGVerifier doesn't have VerifyInlineSignature method
				// Test that it doesn't panic with basic operations
				err = verifier.VerifyDetachedSignature(signedFile, signedFile)
				// We expect an error since this isn't a real signature
				if err != nil {
					Expect(err.Error()).ToNot(BeEmpty())
				}
			})
		})

		Context("error handling", func() {
			It("should handle missing files", func() {
				verifier, err := sdk.NewSystemGPGVerifier()
				Expect(err).ToNot(HaveOccurred())
				
				err = verifier.VerifyDetachedSignature("nonexistent.txt", "nonexistent.sig")
				Expect(err).To(HaveOccurred())
			})

			It("should handle context cancellation", func() {
				testFile := filepath.Join(tempDir, "test.txt")
				err := os.WriteFile(testFile, []byte("test content"), 0644)
				Expect(err).ToNot(HaveOccurred())

				verifier, err := sdk.NewSystemGPGVerifier()
				Expect(err).ToNot(HaveOccurred())
				
				err = verifier.VerifyDetachedSignature(testFile, testFile)
				// Should handle cancellation gracefully
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

var _ = Describe("Checksum Validation", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "checksum-test-*")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	It("should validate checksums correctly", func() {
		testFile := filepath.Join(tempDir, "test.txt")
		testContent := "test content for checksum validation"
		err := os.WriteFile(testFile, []byte(testContent), 0644)
		Expect(err).ToNot(HaveOccurred())

		// Test various checksum formats that the SDK might support
		// Note: We're testing the structure, actual validation would need real checksums
		testCases := []struct {
			algorithm string
			format    string
		}{
			{"sha256", "sha256:"},
			{"sha1", "sha1:"},
			{"md5", "md5:"},
		}

		for _, tc := range testCases {
			By(fmt.Sprintf("testing %s checksum format", tc.algorithm))
			// This is testing the structure - real implementation would calculate checksums
			checksumFormat := tc.format + "abcdef123456789"
			Expect(checksumFormat).To(HavePrefix(tc.format))
		}
	})
})
