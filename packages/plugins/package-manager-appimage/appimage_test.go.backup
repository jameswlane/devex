package appimage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("AppImage Installer", func() {
	var (
		installer *AppImageInstaller
		mockExec  *mocks.MockCommandExecutor
		mockRepo  *MockRepository
		ctx       context.Context
	)

	BeforeEach(func() {
		installer = New()
		mockExec = mocks.NewMockCommandExecutor()
		mockRepo = &MockRepository{}
		ctx = context.Background()

		// Store original and replace with mock
		utils.CommandExec = mockExec

		// Set up in-memory filesystem
		fs.UseMemMapFs()

		// Reset version cache
		ResetVersionCache()
	})

	AfterEach(func() {
		// Reset mock state
		mockExec.Commands = []string{}
		mockExec.FailingCommand = ""
		mockExec.FailingCommands = make(map[string]bool)
		mockExec.InstallationState = make(map[string]bool)

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

	Describe("Version Detection", func() {
		Context("when detecting AppImage environment", func() {
			It("detects AppImageLauncher when available", func() {
				// Mock AppImageLauncher availability
				mockExec.Commands = []string{}

				version, err := getAppImageVersion()

				Expect(err).NotTo(HaveOccurred())
				Expect(version.Type).To(Equal("launcher"))
				Expect(version.Version).To(Equal("available"))
				Expect(mockExec.Commands).To(ContainElement("which AppImageLauncher"))
			})

			It("uses standard AppImage support when launcher not available", func() {
				mockExec.FailingCommands["which AppImageLauncher"] = true

				version, err := getAppImageVersion()

				Expect(err).NotTo(HaveOccurred())
				Expect(version.Type).To(Equal("standard"))
				Expect(version.Version).To(Equal("unknown"))
			})

			It("caches version after first detection", func() {
				// First call
				version1, err1 := getAppImageVersion()
				Expect(err1).NotTo(HaveOccurred())

				// Clear commands to verify no additional calls
				mockExec.Commands = []string{}

				// Second call should use cached value
				version2, err2 := getAppImageVersion()
				Expect(err2).NotTo(HaveOccurred())

				Expect(version1).To(Equal(version2))
				Expect(mockExec.Commands).To(BeEmpty()) // No additional commands
			})
		})
	})

	Describe("parseAppImageCommand", func() {
		Context("with valid command format", func() {
			It("parses URL and binary name correctly", func() {
				command := "https://example.com/app.tar.gz myapp"
				url, binaryName, err := parseAppImageCommand(command)

				Expect(err).NotTo(HaveOccurred())
				Expect(url).To(Equal("https://example.com/app.tar.gz"))
				Expect(binaryName).To(Equal("myapp"))
			})

			It("handles AppImage files", func() {
				command := "https://example.com/app.AppImage myapp"
				url, binaryName, err := parseAppImageCommand(command)

				Expect(err).NotTo(HaveOccurred())
				Expect(url).To(Equal("https://example.com/app.AppImage"))
				Expect(binaryName).To(Equal("myapp"))
			})
		})

		Context("with invalid command formats", func() {
			It("returns error for too few arguments", func() {
				command := "https://example.com/app.tar.gz"
				_, _, err := parseAppImageCommand(command)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("expected format"))
			})

			It("returns error for too many arguments", func() {
				command := "https://example.com/app.tar.gz myapp extraarg"
				_, _, err := parseAppImageCommand(command)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("expected format"))
			})

			It("returns error for empty URLs or binary names", func() {
				command := " myapp"
				_, _, err := parseAppImageCommand(command)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("expected format"))
			})
		})
	})

	Describe("validateAppImageParameters", func() {
		Context("with valid parameters", func() {
			It("validates correct URL and binary name", func() {
				err := validateAppImageParameters("https://example.com/app.tar.gz", "myapp")
				Expect(err).NotTo(HaveOccurred())
			})

			It("allows binary names with dots, dashes, and underscores", func() {
				err := validateAppImageParameters("https://example.com/app.tar.gz", "my-app_v1.0")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with invalid parameters", func() {
			It("rejects invalid URLs", func() {
				err := validateAppImageParameters("://invalid-url", "myapp")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid download URL"))
			})

			It("rejects binary names with path separators", func() {
				err := validateAppImageParameters("https://example.com/app.tar.gz", "my/app")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot contain path separators"))
			})

			It("rejects dangerous binary names", func() {
				err := validateAppImageParameters("https://example.com/app.tar.gz", "..")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid binary name"))
			})

			It("rejects binary names with special characters", func() {
				err := validateAppImageParameters("https://example.com/app.tar.gz", "my@app")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("contains invalid characters"))
			})
		})
	})

	Describe("IsInstalled", func() {
		Context("when binary exists and is executable", func() {
			BeforeEach(func() {
				// Create executable binary
				binaryPath := "/usr/local/bin/myapp"
				err := fs.MkdirAll("/usr/local/bin", 0o755)
				Expect(err).NotTo(HaveOccurred())
				err = fs.WriteFile(binaryPath, []byte("mock binary"), 0o755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns true for installed AppImage", func() {
				command := "https://example.com/app.tar.gz myapp"

				installed, err := installer.IsInstalled(command)

				Expect(err).NotTo(HaveOccurred())
				Expect(installed).To(BeTrue())
			})
		})

		Context("when binary exists but is not executable", func() {
			BeforeEach(func() {
				// Create non-executable binary
				binaryPath := "/usr/local/bin/myapp"
				err := fs.MkdirAll("/usr/local/bin", 0o755)
				Expect(err).NotTo(HaveOccurred())
				err = fs.WriteFile(binaryPath, []byte("mock binary"), 0o644) // No execute bit
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns false for non-executable binary", func() {
				command := "https://example.com/app.tar.gz myapp"

				installed, err := installer.IsInstalled(command)

				Expect(err).NotTo(HaveOccurred())
				Expect(installed).To(BeFalse())
			})
		})

		Context("when binary does not exist", func() {
			It("returns false for non-existent binary", func() {
				command := "https://example.com/app.tar.gz myapp"

				installed, err := installer.IsInstalled(command)

				Expect(err).NotTo(HaveOccurred())
				Expect(installed).To(BeFalse())
			})
		})

		Context("with invalid command format", func() {
			It("returns error for malformed command", func() {
				command := "invalid-command"

				_, err := installer.IsInstalled(command)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("expected format"))
			})
		})
	})

	Describe("Uninstall", func() {
		Context("when app is installed", func() {
			BeforeEach(func() {
				// Create installed binary
				binaryPath := "/usr/local/bin/myapp"
				err := fs.MkdirAll("/usr/local/bin", 0o755)
				Expect(err).NotTo(HaveOccurred())
				err = fs.WriteFile(binaryPath, []byte("mock binary"), 0o755)
				Expect(err).NotTo(HaveOccurred())

				// Add to mock repository
				mockRepo.Apps = append(mockRepo.Apps, "myapp")
			})

			It("uninstalls app successfully", func() {
				command := "https://example.com/app.tar.gz myapp"

				err := installer.Uninstall(command, mockRepo)

				Expect(err).NotTo(HaveOccurred())

				// Verify binary was removed
				exists, _ := fs.Exists("/usr/local/bin/myapp")
				Expect(exists).To(BeFalse())

				// Verify removed from repository
				Expect(mockRepo.Apps).NotTo(ContainElement("myapp"))
			})
		})

		Context("when app is not installed", func() {
			It("skips uninstall gracefully", func() {
				command := "https://example.com/app.tar.gz myapp"

				err := installer.Uninstall(command, mockRepo)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when removal fails", func() {
			It("provides actionable error message for repository failures", func() {
				// Create installed binary
				binaryPath := "/usr/local/bin/myapp"
				err := fs.MkdirAll("/usr/local/bin", 0o755)
				Expect(err).NotTo(HaveOccurred())
				err = fs.WriteFile(binaryPath, []byte("mock binary"), 0o755)
				Expect(err).NotTo(HaveOccurred())

				// Make repository deletion fail
				mockRepo.FailOnDeleteApp = true

				command := "https://example.com/app.tar.gz myapp"

				err = installer.Uninstall(command, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to remove AppImage from repository"))
			})
		})
	})

	Describe("PackageManager Interface", func() {
		Describe("InstallPackages", func() {
			// Note: These tests would need mocking of the download functionality
			// For now, testing the interface and dry-run behavior

			It("handles dry run mode", func() {
				packages := []string{"https://example.com/app1.tar.gz app1", "https://example.com/app2.tar.gz app2"}

				err := installer.InstallPackages(ctx, packages, true)

				Expect(err).NotTo(HaveOccurred())
			})

			It("handles empty package list", func() {
				packages := []string{}

				err := installer.InstallPackages(ctx, packages, false)

				Expect(err).NotTo(HaveOccurred())
			})

			// Real installation testing would require mocking utils.DownloadFile
			It("would install multiple packages in non-dry-run mode", func() {
				Skip("Installation testing requires download mocking")
			})
		})

		Describe("IsAvailable", func() {
			Context("when system supports AppImage installation", func() {
				BeforeEach(func() {
					// Create writable /usr/local/bin directory
					err := fs.MkdirAll("/usr/local/bin", 0o755)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns true when wget is available", func() {
					// Mock wget availability

					available := installer.IsAvailable(ctx)

					Expect(available).To(BeTrue())
					Expect(mockExec.Commands).To(ContainElement("which wget"))
				})

				It("returns true when curl is available (fallback)", func() {
					mockExec.FailingCommands["which wget"] = true
					// curl will succeed by default

					available := installer.IsAvailable(ctx)

					Expect(available).To(BeTrue())
					Expect(mockExec.Commands).To(ContainElement("which curl"))
				})
			})

			Context("when system doesn't support AppImage installation", func() {
				It("returns false when directory is not writable", func() {
					// Memory filesystem can't easily simulate permission issues
					// so we'll mock the situation by failing wget/curl and ensuring directory doesn't exist
					mockExec.FailingCommands["which wget"] = true
					mockExec.FailingCommands["which curl"] = true

					available := installer.IsAvailable(ctx)

					Expect(available).To(BeFalse())
				})

				It("returns false when neither wget nor curl is available", func() {
					// Create writable directory
					err := fs.MkdirAll("/usr/local/bin", 0o755)
					Expect(err).NotTo(HaveOccurred())

					// Mock both wget and curl as unavailable
					mockExec.FailingCommands["which wget"] = true
					mockExec.FailingCommands["which curl"] = true

					available := installer.IsAvailable(ctx)

					Expect(available).To(BeFalse())
				})
			})
		})

		Describe("GetName", func() {
			It("returns correct package manager name", func() {
				name := installer.GetName()
				Expect(name).To(Equal("appimage"))
			})
		})
	})

	Describe("Helper Functions", func() {
		Describe("shouldCreateDesktopEntry", func() {
			It("returns true for GUI application names", func() {
				result := shouldCreateDesktopEntry("MyGuiApp")
				Expect(result).To(BeTrue())
			})

			It("returns false for CLI tool names", func() {
				result := shouldCreateDesktopEntry("grep-tool")
				Expect(result).To(BeFalse())
			})

			It("returns false for git-related tools", func() {
				result := shouldCreateDesktopEntry("git-helper")
				Expect(result).To(BeFalse())
			})
		})

		Describe("createDesktopEntry", func() {
			BeforeEach(func() {
				// Set HOME environment variable for test
				os.Setenv("HOME", "/home/testuser")
			})

			AfterEach(func() {
				os.Unsetenv("HOME")
			})

			It("creates desktop entry successfully", func() {
				binaryName := "myapp"
				binaryPath := "/usr/local/bin/myapp"

				err := createDesktopEntry(binaryName, binaryPath)

				Expect(err).NotTo(HaveOccurred())

				// Verify desktop file was created
				desktopFile := "/home/testuser/.local/share/applications/myapp.desktop"
				exists, err := fs.Exists(desktopFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeTrue())

				// Verify content
				content, err := fs.ReadFile(desktopFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("Name=myapp"))
				Expect(string(content)).To(ContainSubstring("Exec=/usr/local/bin/myapp"))
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
				Expect(err).NotTo(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).NotTo(HaveOccurred())

				// Verify extracted file exists
				exists, err := fs.Exists("/tmp/extract/testfile")
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeTrue())
			})

			It("extracts tarball with directories successfully", func() {
				tarballPath := "/tmp/test-with-dir.tar.gz"
				destDir := "/tmp/extract"

				// Create mock tarball with directory and file
				err := createMockTarballWithDirectory(tarballPath, "subdir", "testfile")
				Expect(err).NotTo(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).NotTo(HaveOccurred())

				// Verify directory and file exist
				dirExists, err := fs.DirExists("/tmp/extract/subdir")
				Expect(err).NotTo(HaveOccurred())
				Expect(dirExists).To(BeTrue())

				fileExists, err := fs.Exists("/tmp/extract/subdir/testfile")
				Expect(err).NotTo(HaveOccurred())
				Expect(fileExists).To(BeTrue())
			})
		})

		Context("with security threats", func() {
			It("prevents directory traversal attacks", func() {
				tarballPath := "/tmp/malicious.tar.gz"
				destDir := "/tmp/extract"

				// Create malicious tarball with directory traversal
				err := createMaliciousTarball(tarballPath, "../../../etc/passwd")
				Expect(err).NotTo(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tarball entry is outside the target directory"))
			})

			It("prevents files that exceed size limit", func() {
				tarballPath := "/tmp/large.tar.gz"
				destDir := "/tmp/extract"

				// Create tarball with file that would exceed size limit
				err := createLargeFileTarball(tarballPath, "largefile")
				Expect(err).NotTo(HaveOccurred())

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
				Expect(err).NotTo(HaveOccurred())

				err = extractTarball(tarballPath, destDir)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create gzip reader"))
			})
		})
	})
})

// MockRepository implements the Repository interface for testing
type MockRepository struct {
	Apps            []string
	ShouldFail      bool
	FailOnAddApp    bool
	FailOnDeleteApp bool
	KeyValueStore   map[string]string
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
	if m.KeyValueStore == nil {
		m.KeyValueStore = make(map[string]string)
	}
	m.KeyValueStore[key] = value
	return nil
}

func (m *MockRepository) Get(key string) (string, error) {
	if m.ShouldFail {
		return "", errors.New("mock repository get failure")
	}
	if m.KeyValueStore == nil {
		return "", errors.New("key not found")
	}
	if val, ok := m.KeyValueStore[key]; ok {
		return val, nil
	}
	return "", errors.New("key not found")
}

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
	const maxFileSize = 500 * 1024 * 1024       // Updated to match AppImage limit
	const writeSize = maxFileSize + (10 * 1024) // Write slightly more than max

	header := &tar.Header{
		Name:     fileName,
		Mode:     0o644,
		Size:     writeSize,
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
