package apt_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/apt"
)

var _ = Describe("Repository Validator", func() {
	Describe("ValidateAptRepo", func() {
		Context("with valid repository strings", func() {
			It("validates standard APT repository", func() {
				repo := "deb https://download.docker.com/linux/ubuntu focal stable"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates repository with options", func() {
				repo := "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu focal stable"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates repository with HTTP", func() {
				repo := "deb http://archive.ubuntu.com/ubuntu focal main"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates repository with multiple components", func() {
				repo := "deb https://packages.microsoft.com/ubuntu/20.04/prod focal main restricted universe multiverse"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates deb-src repository", func() {
				repo := "deb-src https://download.docker.com/linux/ubuntu focal stable"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with invalid repository strings", func() {
			It("rejects empty repository string", func() {
				err := apt.ValidateAptRepo("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("repository string cannot be empty"))
			})

			It("rejects too short repository string", func() {
				err := apt.ValidateAptRepo("deb http")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("repository string too short"))
			})

			It("rejects repository without 'deb' keyword", func() {
				err := apt.ValidateAptRepo("https://example.com/ubuntu focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing required keywords"))
			})

			It("rejects repository without URL", func() {
				err := apt.ValidateAptRepo("deb focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing required keywords"))
			})

			It("rejects repository with invalid URL", func() {
				err := apt.ValidateAptRepo("deb ht tp://invalid-url focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid URL format"))
			})

			It("rejects repository with FTP URL", func() {
				err := apt.ValidateAptRepo("deb ftp://example.com/ubuntu focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid URL format"))
			})

			It("rejects repository with unsupported scheme", func() {
				err := apt.ValidateAptRepo("deb file:///local/repo focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid URL format"))
			})
		})

		Context("with suspicious content", func() {
			It("rejects repository with command injection attempt", func() {
				err := apt.ValidateAptRepo("deb https://example.com/ubuntu; rm -rf / #")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious content"))
			})

			It("rejects repository with pipe character", func() {
				err := apt.ValidateAptRepo("deb https://example.com/ubuntu | cat /etc/passwd")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious content"))
			})

			It("rejects repository with shell commands", func() {
				err := apt.ValidateAptRepo("deb https://example.com/ubuntu $(whoami)")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious content"))
			})

			It("rejects repository with path traversal", func() {
				err := apt.ValidateAptRepo("deb https://example.com/../../../etc/passwd focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious content"))
			})

			It("rejects repository with wildcards", func() {
				err := apt.ValidateAptRepo("deb https://example.com/ubuntu* focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious content"))
			})

			It("rejects repository with home directory reference", func() {
				err := apt.ValidateAptRepo("deb https://example.com/~/malicious focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious content"))
			})

			It("rejects repository with bash execution", func() {
				err := apt.ValidateAptRepo("deb https://example.com/ubuntu bash focal main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious content"))
			})

			It("rejects repository with ampersand", func() {
				err := apt.ValidateAptRepo("deb https://example.com/ubuntu focal main & malicious-command")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious content"))
			})
		})

		Context("edge cases", func() {
			It("validates repository with port number", func() {
				repo := "deb https://example.com:8080/ubuntu focal main"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates repository with path parameters", func() {
				repo := "deb https://packages.cloud.google.com/apt cloud-sdk main"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates repository with subdomain", func() {
				repo := "deb https://apt.releases.hashicorp.com focal main"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates repository with IP address", func() {
				repo := "deb https://192.168.1.100/ubuntu focal main"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("validates repository with complex options", func() {
				repo := "deb [arch=amd64,arm64 signed-by=/etc/apt/keyrings/example.gpg] https://example.com/ubuntu focal main restricted"
				err := apt.ValidateAptRepo(repo)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
