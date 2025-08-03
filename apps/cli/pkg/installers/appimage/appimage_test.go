// Package appimage provides comprehensive tests for the AppImage installer.
//
// The AppImage installer is complex as it involves:
// 1. Parsing commands with download URL and binary name (format: "URL binaryName")
// 2. Checking if the AppImage binary is already installed
// 3. Downloading files from URLs
// 4. Extracting tar.gz files with security protections
// 5. Moving binaries to /usr/local/bin/ and setting permissions
// 6. Adding to repository
//
// Testing Challenges and Limitations:
// - utils.DownloadFile is a function (not variable), making it difficult to mock in unit tests
// - utilities.IsAppInstalled uses os.Stat directly, not the fs package abstraction
// - Full integration testing requires either dependency injection or integration test setup
//
// Current Test Coverage:
// ✓ New() function creating installer instance
// ✓ parseAppImageCommand() with valid and invalid formats
// ✓ Install() error handling for malformed commands
// ✓ extractTarball() with comprehensive security validations:
//   - Directory traversal prevention
//   - File size limit enforcement (100MB)
//   - Corrupted tarball handling
//   - Valid tarball extraction with directories
//
// ✓ Complete Repository interface mock implementation
//
// Tests Requiring Integration Approach:
// - Full Install() workflow (requires utils.DownloadFile mocking)
// - Download failure scenarios
// - installAppImage() function testing
// - Repository failure integration
// - utilities.IsAppInstalled behavior with real filesystem
package appimage

import (
	"archive/tar"
	"compress/gzip"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/types"
)

// MockRepository implements the Repository interface for testing
type MockRepository struct {
	Apps            []string
	ShouldFail      bool
	FailOnAddApp    bool
	FailOnDeleteApp bool
}

func (m *MockRepository) AddApp(name string) error {
	if m.FailOnAddApp {
		return errors.New("mock repository add app failure")
	}
	m.Apps = append(m.Apps, name)
	return nil
}

func (m *MockRepository) DeleteApp(name string) error {
	if m.FailOnDeleteApp {
		return errors.New("mock repository delete app failure")
	}
	for i, app := range m.Apps {
		if app == name {
			m.Apps = append(m.Apps[:i], m.Apps[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockRepository) GetApp(name string) (*types.AppConfig, error) {
	if m.ShouldFail {
		return nil, errors.New("mock repository get app failure")
	}
	// Simple mock implementation
	return &types.AppConfig{
		BaseConfig: types.BaseConfig{Name: name},
	}, nil
}

func (m *MockRepository) ListApps() ([]types.AppConfig, error) {
	if m.ShouldFail {
		return nil, errors.New("mock repository list apps failure")
	}
	apps := make([]types.AppConfig, 0, len(m.Apps))
	for _, name := range m.Apps {
		apps = append(apps, types.AppConfig{
			BaseConfig: types.BaseConfig{Name: name},
		})
	}
	return apps, nil
}

func (m *MockRepository) SaveApp(app types.AppConfig) error {
	if m.ShouldFail {
		return errors.New("mock repository save app failure")
	}
	// Add to apps if not already there
	for _, existing := range m.Apps {
		if existing == app.Name {
			return nil
		}
	}
	m.Apps = append(m.Apps, app.Name)
	return nil
}

func (m *MockRepository) Set(key string, value string) error {
	if m.ShouldFail {
		return errors.New("mock repository set failure")
	}
	return nil
}

func (m *MockRepository) Get(key string) (string, error) {
	if m.ShouldFail {
		return "", errors.New("mock repository get failure")
	}
	return "mock-value", nil
}

// Note: utilities.IsAppInstalled uses os.Stat directly for appimage checking,
// so we need to create actual files on the filesystem for installation checks
// Also, since utils.DownloadFile is a function and not a variable, we'll mock
// downloads by creating the expected files directly on the filesystem

var _ = Describe("AppImage Installer", func() {
	var (
		installer *AppImageInstaller
		mockRepo  *MockRepository
		memFs     afero.Fs
	)

	BeforeEach(func() {
		installer = New()
		mockRepo = &MockRepository{
			Apps:         []string{},
			ShouldFail:   false,
			FailOnAddApp: false,
		}

		// Set up in-memory filesystem
		memFs = afero.NewMemMapFs()
		fs.SetFs(memFs)

		// Note: utilities.IsAppInstalled for appimage checks os.Stat directly,
		// so we use the in-memory filesystem to control file existence
		// Since we can't easily mock utils.DownloadFile, tests will need to work
		// around this limitation or skip download-specific functionality
	})

	AfterEach(func() {
		// Reset filesystem to OS filesystem
		fs.UseOsFs()
	})

	Describe("New", func() {
		It("creates a new AppImage installer instance", func() {
			installer := New()
			Expect(installer).ToNot(BeNil())
			Expect(installer).To(BeAssignableToTypeOf(&AppImageInstaller{}))
		})
	})

	Describe("parseAppImageCommand", func() {
		Context("with valid command format", func() {
			It("parses URL and binary name correctly", func() {
				command := "https://example.com/app.tar.gz myapp"
				url, binaryName := parseAppImageCommand(command)

				Expect(url).To(Equal("https://example.com/app.tar.gz"))
				Expect(binaryName).To(Equal("myapp"))
			})
		})

		Context("with invalid command formats", func() {
			It("returns empty strings for too few arguments", func() {
				command := "https://example.com/app.tar.gz"
				url, binaryName := parseAppImageCommand(command)

				Expect(url).To(BeEmpty())
				Expect(binaryName).To(BeEmpty())
			})

			It("returns empty strings for too many arguments", func() {
				command := "https://example.com/app.tar.gz myapp extraarg"
				url, binaryName := parseAppImageCommand(command)

				Expect(url).To(BeEmpty())
				Expect(binaryName).To(BeEmpty())
			})

			It("returns empty strings for empty command", func() {
				command := ""
				url, binaryName := parseAppImageCommand(command)

				Expect(url).To(BeEmpty())
				Expect(binaryName).To(BeEmpty())
			})

			It("returns empty strings for whitespace-only command", func() {
				command := "   "
				url, binaryName := parseAppImageCommand(command)

				Expect(url).To(BeEmpty())
				Expect(binaryName).To(BeEmpty())
			})
		})
	})

	Describe("Install", func() {
		Context("with valid AppImage command", func() {
			It("notes limitation with full installation testing", func() {
				// Note: Full installation testing requires mocking utils.DownloadFile,
				// which isn't easily possible with the current architecture.
				// The individual components can be tested separately.
				Skip("Full installation testing requires integration test approach or utils.DownloadFile mocking")
			})

			It("demonstrates issue with utilities.IsAppInstalled integration", func() {
				// Note: utilities.IsAppInstalled uses os.Stat directly which doesn't work
				// with our in-memory filesystem. The function will proceed with installation
				// because the URL path doesn't exist as a real file.
				// This test documents the limitation rather than testing the skip behavior.
				Skip("utilities.IsAppInstalled integration limitation - cannot test skip behavior with in-memory filesystem")
			})

			It("continues with installation when utilities check has typical results", func() {
				// Note: This test would require mocking the download process
				Skip("Full installation flow testing requires download mocking")
			})
		})

		Context("with invalid command format", func() {
			It("returns error for malformed command", func() {
				command := "invalid-command"

				err := installer.Install(command, mockRepo)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid command format"))
			})
		})

		Context("with utilities.IsAppInstalled behavior", func() {
			It("notes the current limitation with utilities.IsAppInstalled integration", func() {
				// Note: utilities.IsAppInstalled uses os.Stat directly and cannot be easily mocked
				// in unit tests with in-memory filesystem. This would be better tested in integration tests.
				Skip("utilities.IsAppInstalled integration testing requires real filesystem or mocking")
			})
		})

		Context("when download fails", func() {
			It("notes limitation with mocking download failures", func() {
				// Note: Since utils.DownloadFile is a function and not a variable,
				// we cannot easily mock download failures in unit tests.
				// This would be better tested with integration tests or by modifying
				// the utils package to support dependency injection.

				// For demonstration, we can test that the function would fail
				// if the tarball file doesn't exist after "download"
				Skip("Download failure testing requires integration test approach")
			})
		})

		Context("when repository add fails", func() {
			It("returns error when adding to repository fails", func() {
				// Note: This test would also require mocking the download process
				Skip("Repository failure testing requires full installation flow mocking")
			})
		})
	})

	Describe("installAppImage", func() {
		Context("with valid inputs", func() {
			It("extracts and installs AppImage successfully when tarball exists", func() {
				// Note: This will still fail because utils.DownloadFile will try to download
				// For proper testing, we'd need to mock or modify installAppImage to accept a pre-downloaded file
				Skip("installAppImage testing requires utils.DownloadFile mocking or refactoring")
			})
		})

		Context("when extraction fails", func() {
			It("would return error when tarball extraction fails", func() {
				// Note: Testing extraction failures requires mocking the download step
				Skip("Extraction failure testing requires utils.DownloadFile mocking")
			})
		})

		Context("when file operations fail", func() {
			It("would return error when moving binary fails", func() {
				// Note: Testing file operation failures requires mocking the download step
				Skip("File operation failure testing requires utils.DownloadFile mocking")
			})
		})
	})

	Describe("extractTarball", func() {
		Context("with valid tarball", func() {
			It("extracts tarball successfully", func() {
				tarballPath := "/tmp/test.tar.gz"
				destDir := "/tmp/extract"

				// Create mock tarball with a file
				err := createMockTarball(tarballPath, "testfile")
				Expect(err).ToNot(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).ToNot(HaveOccurred())

				// Verify extracted file exists
				exists, err := fs.Exists("/tmp/extract/testfile")
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
			})

			It("extracts tarball with directories successfully", func() {
				tarballPath := "/tmp/test-with-dir.tar.gz"
				destDir := "/tmp/extract"

				// Create mock tarball with directory and file
				err := createMockTarballWithDirectory(tarballPath, "subdir", "testfile")
				Expect(err).ToNot(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).ToNot(HaveOccurred())

				// Verify directory and file exist
				dirExists, err := fs.DirExists("/tmp/extract/subdir")
				Expect(err).ToNot(HaveOccurred())
				Expect(dirExists).To(BeTrue())

				fileExists, err := fs.Exists("/tmp/extract/subdir/testfile")
				Expect(err).ToNot(HaveOccurred())
				Expect(fileExists).To(BeTrue())
			})
		})

		Context("with security threats", func() {
			It("prevents directory traversal attacks", func() {
				tarballPath := "/tmp/malicious.tar.gz"
				destDir := "/tmp/extract"

				// Create malicious tarball with directory traversal
				err := createMaliciousTarball(tarballPath, "../../../etc/passwd")
				Expect(err).ToNot(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tarball entry is outside the target directory"))
			})

			It("prevents files that exceed size limit", func() {
				tarballPath := "/tmp/large.tar.gz"
				destDir := "/tmp/extract"

				// Create tarball with file that would exceed size limit
				err := createLargeFileTarball(tarballPath, "largefile")
				Expect(err).ToNot(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("file size exceeds maximum allowed size"))
			})
		})

		Context("with invalid tarball", func() {
			It("returns error when tarball cannot be opened", func() {
				tarballPath := "/tmp/nonexistent.tar.gz"
				destDir := "/tmp/extract"

				err := extractTarball(tarballPath, destDir)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to open tarball"))
			})

			It("returns error when tarball is corrupted", func() {
				tarballPath := "/tmp/corrupted.tar.gz"
				destDir := "/tmp/extract"

				// Create corrupted tarball
				err := fs.WriteFile(tarballPath, []byte("not a valid gzip file"), 0o644)
				Expect(err).ToNot(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create gzip reader"))
			})
		})
	})
})

// Helper functions for creating mock tarballs

func createMockTarball(tarballPath, fileName string) error {
	file, err := fs.Create(tarballPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add a file to the tarball
	header := &tar.Header{
		Name:     fileName,
		Mode:     0o755,
		Size:     int64(len("mock content")),
		Typeflag: tar.TypeReg,
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tarWriter.Write([]byte("mock content")); err != nil {
		return err
	}

	return nil
}

func createMockTarballWithDirectory(tarballPath, dirName, fileName string) error {
	file, err := fs.Create(tarballPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add directory
	dirHeader := &tar.Header{
		Name:     dirName + "/",
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}

	if err := tarWriter.WriteHeader(dirHeader); err != nil {
		return err
	}

	// Add file in directory
	fileHeader := &tar.Header{
		Name:     dirName + "/" + fileName,
		Mode:     0o644,
		Size:     int64(len("mock file content")),
		Typeflag: tar.TypeReg,
	}

	if err := tarWriter.WriteHeader(fileHeader); err != nil {
		return err
	}

	if _, err := tarWriter.Write([]byte("mock file content")); err != nil {
		return err
	}

	return nil
}

func createMaliciousTarball(tarballPath, maliciousPath string) error {
	file, err := fs.Create(tarballPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add malicious file with directory traversal
	header := &tar.Header{
		Name:     maliciousPath,
		Mode:     0o644,
		Size:     int64(len("malicious content")),
		Typeflag: tar.TypeReg,
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tarWriter.Write([]byte("malicious content")); err != nil {
		return err
	}

	return nil
}

func createLargeFileTarball(tarballPath, fileName string) error {
	file, err := fs.Create(tarballPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Create a file header that claims to be larger than the limit
	const maxFileSize = 100 * 1024 * 1024       // 100MB limit
	const writeSize = maxFileSize + (10 * 1024) // Write slightly more than max

	header := &tar.Header{
		Name:     fileName,
		Mode:     0o644,
		Size:     writeSize, // Match what we'll actually write
		Typeflag: tar.TypeReg,
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Write data in chunks to exceed the limit
	chunkSize := 1024
	largeData := make([]byte, chunkSize)
	for i := 0; i < len(largeData); i++ {
		largeData[i] = 'A'
	}

	// Write enough chunks to exceed the maxFileSize limit
	chunksToWrite := int(writeSize / int64(chunkSize))
	for i := 0; i < chunksToWrite; i++ {
		if _, err := tarWriter.Write(largeData); err != nil {
			return err
		}
	}

	return nil
}
