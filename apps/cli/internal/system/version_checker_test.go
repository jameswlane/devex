package system_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/system"
)

var _ = Describe("VersionChecker", func() {
	var versionChecker *system.VersionChecker

	BeforeEach(func() {
		versionChecker = system.NewVersionChecker()
	})

	Describe("CompareVersions", func() {
		Context("when handling different version requirement formats", func() {
			It("should handle empty and 'latest' requirements", func() {
				meets, err := versionChecker.CompareVersions("1.0.0", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.0.0", "latest")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())
			})

			It("should handle plus notation (>=)", func() {
				meets, err := versionChecker.CompareVersions("1.13.5", "1.13+")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.12.9", "1.13+")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})

			It("should handle explicit >= operator", func() {
				meets, err := versionChecker.CompareVersions("2.0.0", ">=1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.18.0", ">=1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})

			It("should handle > operator", func() {
				meets, err := versionChecker.CompareVersions("2.0.0", ">1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.19.0", ">1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})

			It("should handle <= operator", func() {
				meets, err := versionChecker.CompareVersions("1.19.0", "<=1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.20.0", "<=1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})

			It("should handle < operator", func() {
				meets, err := versionChecker.CompareVersions("1.18.0", "<1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.19.0", "<1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})

			It("should handle caret range (^)", func() {
				meets, err := versionChecker.CompareVersions("1.2.5", "^1.2.3")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("2.0.0", "^1.2.3")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())

				meets, err = versionChecker.CompareVersions("1.1.0", "^1.2.3")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})

			It("should handle tilde range (~)", func() {
				meets, err := versionChecker.CompareVersions("1.2.5", "~1.2.3")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.3.0", "~1.2.3")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())

				meets, err = versionChecker.CompareVersions("1.2.2", "~1.2.3")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})

			It("should handle exact version match", func() {
				meets, err := versionChecker.CompareVersions("1.2.3", "1.2.3")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.2.4", "1.2.3")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})
		})

		Context("when handling edge cases", func() {
			It("should handle versions with missing patch numbers", func() {
				meets, err := versionChecker.CompareVersions("1.19", "1.19+")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("1.19.0", "1.19")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())
			})

			It("should return errors for invalid version formats", func() {
				_, err := versionChecker.CompareVersions("invalid", "1.0.0")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid version format"))

				_, err = versionChecker.CompareVersions("1.0.0", "invalid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid version format"))
			})

			It("should handle major version only comparisons", func() {
				meets, err := versionChecker.CompareVersions("18.17.0", ">=18.0")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeTrue())

				meets, err = versionChecker.CompareVersions("17.9.0", ">=18.0")
				Expect(err).ToNot(HaveOccurred())
				Expect(meets).To(BeFalse())
			})
		})
	})

	Describe("CheckDockerVersion", func() {
		Context("when Docker version checking is mocked", func() {
			It("should parse Docker version output correctly", func() {
				// This test would require mocking the command execution
				// For now, we'll test the logic with a simple unit test
				Skip("Requires mocked command execution - tested in integration")
			})
		})
	})

	Describe("CheckGitVersion", func() {
		Context("when Git version checking is mocked", func() {
			It("should parse Git version output correctly", func() {
				// This test would require mocking the command execution
				// For now, we'll test the logic with a simple unit test
				Skip("Requires mocked command execution - tested in integration")
			})
		})
	})

	Describe("CheckGoVersion", func() {
		Context("when Go version checking is mocked", func() {
			It("should parse Go version output correctly", func() {
				// This test would require mocking the command execution
				// For now, we'll test the logic with a simple unit test
				Skip("Requires mocked command execution - tested in integration")
			})
		})
	})

	Describe("CheckNodeVersion", func() {
		Context("when Node.js version checking is mocked", func() {
			It("should parse Node.js version output correctly", func() {
				// This test would require mocking the command execution
				// For now, we'll test the logic with a simple unit test
				Skip("Requires mocked command execution - tested in integration")
			})
		})
	})

	Describe("CheckPythonVersion", func() {
		Context("when Python version checking is mocked", func() {
			It("should parse Python version output correctly", func() {
				// This test would require mocking the command execution
				// For now, we'll test the logic with a simple unit test
				Skip("Requires mocked command execution - tested in integration")
			})
		})
	})
})
