package apt_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/installers/apt"
)

var _ = Describe("GPG Functions", func() {
	var server *httptest.Server

	BeforeEach(func() {
		// Use memory filesystem for testing
		fs.UseMemMapFs()

		// Create test HTTP server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/valid-key":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\ntest key content\n-----END PGP PUBLIC KEY BLOCK-----"))
			case "/not-found":
				w.WriteHeader(http.StatusNotFound)
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
	})

	AfterEach(func() {
		server.Close()
		// Restore OS filesystem
		fs.UseOsFs()
	})

	Describe("DownloadGPGKey", func() {
		Context("when GPG key file does not exist", func() {
			It("downloads GPG key successfully without dearmor", func() {
				destination := "/tmp/test-key.gpg"
				err := apt.DownloadGPGKey(server.URL+"/valid-key", destination, false)
				Expect(err).NotTo(HaveOccurred())

				// Verify file was created
				exists, err := fs.FileExistsAndIsFile(destination)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeTrue())
			})
		})

		Context("when GPG key file already exists", func() {
			It("skips download when file exists", func() {
				destination := "/tmp/existing-key.gpg"

				// Create the file first
				err := fs.WriteFile(destination, []byte("existing key"), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = apt.DownloadGPGKey(server.URL+"/valid-key", destination, false)
				Expect(err).NotTo(HaveOccurred())

				// Verify original content wasn't changed
				content, err := fs.ReadFile(destination)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("existing key"))
			})
		})

		Context("when download fails", func() {
			It("returns error for 404 response", func() {
				destination := "/tmp/test-key.gpg"
				err := apt.DownloadGPGKey(server.URL+"/not-found", destination, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unexpected HTTP status code: 404"))
			})

			It("returns error for invalid URL", func() {
				destination := "/tmp/test-key.gpg"
				err := apt.DownloadGPGKey("invalid-url", destination, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to download GPG key"))
			})
		})

		Context("with dearmor option", func() {
			It("attempts to call ProcessGPGKey when dearmor is true", func() {
				destination := "/tmp/test-key.gpg"

				// This will fail because ProcessGPGKey tries to run gpg command
				// But it validates that the dearmor path is taken
				err := apt.DownloadGPGKey(server.URL+"/valid-key", destination, true)
				Expect(err).To(HaveOccurred()) // Expected because gpg command will fail in test environment
			})
		})
	})
})
