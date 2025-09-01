package appimage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type AppImageInstaller struct{}

// AppImageVersion information
type AppImageVersion struct {
	Type    string // Type 1 or Type 2
	Version string // Version string if available
}

// Cached AppImage version to avoid repeated detection
var cachedAppImageVersion *AppImageVersion

// ResetVersionCache resets the cached AppImage version (useful for testing)
func ResetVersionCache() {
	cachedAppImageVersion = nil
}

// getAppImageVersion detects AppImage runtime information
func getAppImageVersion() (*AppImageVersion, error) {
	if cachedAppImageVersion != nil {
		return cachedAppImageVersion, nil
	}

	// Check if AppImageLauncher is available
	_, err := utils.CommandExec.RunShellCommand("which AppImageLauncher")
	if err == nil {
		// AppImageLauncher is available
		cachedAppImageVersion = &AppImageVersion{
			Type:    "launcher",
			Version: "available",
		}
		log.Debug("Detected AppImageLauncher availability")
		return cachedAppImageVersion, nil
	}

	// Default AppImage support
	cachedAppImageVersion = &AppImageVersion{
		Type:    "standard",
		Version: "unknown",
	}

	log.Debug("Using standard AppImage support")
	return cachedAppImageVersion, nil
}

func New() *AppImageInstaller {
	return &AppImageInstaller{}
}

func (a *AppImageInstaller) Install(command string, repo types.Repository) error {
	log.Debug("AppImage Installer: Starting installation", "command", command)

	// Detect AppImage environment early
	if _, err := getAppImageVersion(); err != nil {
		log.Warn("Failed to detect AppImage environment, using defaults", "error", err)
	}

	// Run background validation for better performance
	validator := utilities.NewBackgroundValidator(30 * time.Second)
	validator.AddSuite(utilities.CreateSystemValidationSuite("appimage"))
	validator.AddSuite(utilities.CreateNetworkValidationSuite())

	ctx := context.Background()
	if err := validator.RunValidations(ctx); err != nil {
		return utilities.WrapError(err, utilities.ErrorTypeSystem, "install", command, "appimage")
	}

	// Parse command to extract download URL and binary name with validation
	downloadURL, binaryName, err := parseAppImageCommand(command)
	if err != nil {
		return fmt.Errorf("invalid command format for AppImage installer: %w", err)
	}

	// Validate URL and binary name
	if err := validateAppImageParameters(downloadURL, binaryName); err != nil {
		return fmt.Errorf("parameter validation failed: %w", err)
	}

	// Create AppConfig for installation checks
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "appimage",
		InstallCommand: binaryName, // Use binary name for checks
	}

	// Check if the AppImage binary is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if app is installed", err, "binaryName", binaryName)
		return fmt.Errorf("failed to check if AppImage binary is installed: %w", err)
	}
	if isInstalled {
		log.Info("AppImage already installed, skipping installation", "binaryName", binaryName)
		return nil
	}

	// Validate URL accessibility before downloading
	if err := validateURLAccessibility(downloadURL); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	// Download and install the AppImage
	if err := installAppImage(ctx, downloadURL, binaryName); err != nil {
		log.Error("Failed to install AppImage", err, "downloadURL", downloadURL)

		// Provide actionable error messages
		if strings.Contains(err.Error(), "no such host") {
			return fmt.Errorf("failed to install AppImage '%s': network error (hint: check internet connection and URL)", binaryName)
		}
		if strings.Contains(err.Error(), "permission denied") {
			return fmt.Errorf("failed to install AppImage '%s': permission denied (hint: ensure /usr/local/bin is writable or run with appropriate permissions)", binaryName)
		}
		if strings.Contains(err.Error(), "no space left") {
			return fmt.Errorf("failed to install AppImage '%s': insufficient disk space (hint: free up space in /tmp and /usr/local/bin)", binaryName)
		}
		return fmt.Errorf("failed to install AppImage '%s': %w", binaryName, err)
	}

	log.Debug("AppImage installed successfully", "binaryName", binaryName)

	// Verify installation succeeded
	verifyConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "appimage",
		InstallCommand: binaryName,
	}
	if isInstalled, err := utilities.IsAppInstalled(verifyConfig); err != nil {
		log.Warn("Failed to verify installation", "error", err, "binaryName", binaryName)
	} else if !isInstalled {
		return fmt.Errorf("AppImage installation verification failed for: %s (hint: check if binary exists in /usr/local/bin)", binaryName)
	}

	// Perform post-installation setup
	if err := performPostInstallationSetup(binaryName); err != nil {
		log.Warn("Post-installation setup failed", "binary", binaryName, "error", err)
		// Don't fail the installation, just warn
	}

	// Add the binary to the repository
	if err := repo.AddApp(binaryName); err != nil {
		log.Error("Failed to add AppImage to repository", err, "binaryName", binaryName)
		return fmt.Errorf("failed to add AppImage to repository: %w", err)
	}

	log.Debug("AppImage added to repository", "binaryName", binaryName)
	return nil
}

// Uninstall removes AppImages
func (a *AppImageInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("AppImage Installer: Starting uninstallation", "command", command)

	// Parse command to extract binary name
	_, binaryName, err := parseAppImageCommand(command)
	if err != nil {
		return fmt.Errorf("invalid command format for AppImage uninstaller: %w", err)
	}

	// Check if the AppImage is installed
	isInstalled, err := a.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if AppImage is installed", err, "binaryName", binaryName)
		return fmt.Errorf("failed to check if AppImage is installed: %w", err)
	}

	if !isInstalled {
		log.Info("AppImage not installed, skipping uninstallation", "binaryName", binaryName)
		return nil
	}

	// Remove the AppImage binary
	binaryPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)
	if err := fs.Remove(binaryPath); err != nil {
		log.Error("Failed to remove AppImage binary", err, "binaryPath", binaryPath)

		// Provide actionable error messages
		if strings.Contains(err.Error(), "permission denied") {
			return fmt.Errorf("failed to remove AppImage binary '%s': permission denied (hint: check file permissions or run with appropriate privileges)", binaryName)
		}
		return fmt.Errorf("failed to remove AppImage binary '%s': %w", binaryName, err)
	}

	log.Debug("AppImage binary removed successfully", "binaryName", binaryName)

	// Remove from repository
	if err := repo.DeleteApp(binaryName); err != nil {
		log.Error("Failed to remove AppImage from repository", err, "binaryName", binaryName)
		return fmt.Errorf("failed to remove AppImage from repository: %w", err)
	}

	log.Debug("AppImage removed from repository successfully", "binaryName", binaryName)
	return nil
}

// IsInstalled checks if an AppImage is installed
func (a *AppImageInstaller) IsInstalled(command string) (bool, error) {
	// Parse command to extract binary name
	_, binaryName, err := parseAppImageCommand(command)
	if err != nil {
		return false, fmt.Errorf("invalid command format for AppImage installer: %w", err)
	}

	// Check if the binary exists in /usr/local/bin
	binaryPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)
	exists, err := fs.Exists(binaryPath)
	if err != nil {
		return false, fmt.Errorf("failed to check if AppImage exists: %w", err)
	}

	// Also verify it's executable
	if exists {
		info, err := fs.Stat(binaryPath)
		if err != nil {
			return false, fmt.Errorf("failed to check AppImage permissions: %w", err)
		}
		// Check if file is executable
		mode := info.Mode()
		if mode&0o111 == 0 {
			log.Warn("AppImage exists but is not executable", "path", binaryPath)
			return false, nil
		}
	}

	return exists, nil
}

// PackageManager interface implementation methods

// InstallPackages installs multiple AppImages (implements PackageManager interface)
func (a *AppImageInstaller) InstallPackages(ctx context.Context, packages []string, dryRun bool) error {
	if len(packages) == 0 {
		return nil
	}

	log.Info("Installing AppImages", "packages", packages, "dryRun", dryRun)

	if dryRun {
		log.Info("DRY RUN: Would install AppImages", "packages", packages)
		return nil
	}

	// Install each AppImage individually
	for _, pkg := range packages {
		downloadURL, binaryName, err := parseAppImageCommand(pkg)
		if err != nil {
			log.Error("Failed to parse AppImage command", err, "package", pkg)
			continue
		}

		if err := installAppImage(ctx, downloadURL, binaryName); err != nil {
			// Don't fail the entire batch, just log the error
			log.Error("Failed to install AppImage", err, "package", pkg)
			continue
		}
		log.Info("Successfully installed AppImage", "package", pkg)
	}

	return nil
}

// IsAvailable checks if AppImage installation is supported
func (a *AppImageInstaller) IsAvailable(ctx context.Context) bool {
	// Check if we can write to /usr/local/bin
	testFile := "/usr/local/bin/.appimage-test"
	if err := fs.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		return false
	}
	_ = fs.Remove(testFile)

	// Check if wget or curl is available for downloads
	_, wgetErr := utils.CommandExec.RunShellCommand("which wget")
	_, curlErr := utils.CommandExec.RunShellCommand("which curl")

	return wgetErr == nil || curlErr == nil
}

// GetName returns the package manager name
func (a *AppImageInstaller) GetName() string {
	return "appimage"
}

// Helper functions

// parseAppImageCommand parses an AppImage command to extract URL and binary name
func parseAppImageCommand(command string) (string, string, error) {
	parts := strings.Fields(command)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected format: '<download_url> <binary_name>', got: %s", command)
	}

	downloadURL := parts[0]
	binaryName := parts[1]

	// Basic validation
	if downloadURL == "" || binaryName == "" {
		return "", "", fmt.Errorf("download URL and binary name cannot be empty")
	}

	return downloadURL, binaryName, nil
}

// validateAppImageParameters validates URL and binary name
func validateAppImageParameters(downloadURL, binaryName string) error {
	// Validate URL format
	if _, err := url.Parse(downloadURL); err != nil {
		return fmt.Errorf("invalid download URL '%s': %w", downloadURL, err)
	}

	// Validate binary name (no path separators, no special chars)
	if strings.ContainsAny(binaryName, "/\\") {
		return fmt.Errorf("binary name '%s' cannot contain path separators", binaryName)
	}

	// Check for potentially dangerous names
	if binaryName == "." || binaryName == ".." || binaryName == "" {
		return fmt.Errorf("invalid binary name '%s'", binaryName)
	}

	// Validate against regex for safe filenames
	validName := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !validName.MatchString(binaryName) {
		return fmt.Errorf("binary name '%s' contains invalid characters", binaryName)
	}

	return nil
}

// validateURLAccessibility checks if the URL is accessible
func validateURLAccessibility(downloadURL string) error {
	log.Debug("Validating URL accessibility", "url", downloadURL)

	// Create a HEAD request to check if URL is accessible
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "HEAD", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for URL validation: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("URL is not accessible: %w (hint: check internet connection and URL validity)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("URL returned status %d: %s (hint: check if the download link is valid)", resp.StatusCode, resp.Status)
	}

	log.Debug("URL accessibility validated", "url", downloadURL, "status", resp.StatusCode)
	return nil
}

// performPostInstallationSetup handles post-installation configuration
func performPostInstallationSetup(binaryName string) error {
	binaryPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)

	// Ensure the binary is executable
	if err := fs.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	// Try to create a desktop entry if this looks like a GUI application
	if shouldCreateDesktopEntry(binaryName) {
		if err := createDesktopEntry(binaryName, binaryPath); err != nil {
			log.Debug("Failed to create desktop entry", "error", err)
			// Non-fatal error
		}
	}

	// Update desktop database if available
	if _, err := utils.CommandExec.RunShellCommand("which update-desktop-database"); err == nil {
		if _, err := utils.CommandExec.RunShellCommand("update-desktop-database ~/.local/share/applications 2>/dev/null || true"); err != nil {
			log.Debug("Failed to update desktop database", "error", err)
		}
	}

	return nil
}

// shouldCreateDesktopEntry determines if we should create a desktop entry
func shouldCreateDesktopEntry(binaryName string) bool {
	// Simple heuristic: if it's not a command-line tool name, create desktop entry
	cliTools := []string{"grep", "find", "sed", "awk", "curl", "wget", "git", "vim", "nano", "cat", "ls", "cp", "mv"}
	for _, tool := range cliTools {
		if strings.Contains(strings.ToLower(binaryName), tool) {
			return false
		}
	}
	return true
}

// createDesktopEntry creates a basic desktop entry
func createDesktopEntry(binaryName, binaryPath string) error {
	desktopDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications")
	if err := fs.MkdirAll(desktopDir, 0o755); err != nil {
		return fmt.Errorf("failed to create applications directory: %w", err)
	}

	desktopFile := filepath.Join(desktopDir, binaryName+".desktop")
	content := fmt.Sprintf(`[Desktop Entry]
Version=1.0
Type=Application
Name=%s
Comment=AppImage application
Exec=%s
Icon=application-x-executable
Categories=Utility;
Terminal=false
`, binaryName, binaryPath)

	if err := fs.WriteFile(desktopFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write desktop file: %w", err)
	}

	log.Debug("Created desktop entry", "file", desktopFile)
	return nil
}

func installAppImage(ctx context.Context, downloadURL, binaryName string) error {
	// Check if it's a direct AppImage or tarball
	if strings.HasSuffix(strings.ToLower(downloadURL), ".appimage") {
		return installDirectAppImage(ctx, downloadURL, binaryName)
	}

	// Assume it's a tarball
	return installTarballAppImage(ctx, downloadURL, binaryName)
}

// installDirectAppImage downloads and installs a direct AppImage file
func installDirectAppImage(ctx context.Context, downloadURL, binaryName string) error {
	binaryPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)

	log.Debug("Downloading AppImage directly", "url", downloadURL, "destination", binaryPath)
	if err := utils.DownloadFileWithContext(ctx, downloadURL, binaryPath); err != nil {
		return fmt.Errorf("failed to download AppImage: %w", err)
	}

	// Set executable permissions
	if err := fs.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("failed to set permissions on AppImage: %w", err)
	}

	return nil
}

// installTarballAppImage downloads and extracts a tarball containing an AppImage
func installTarballAppImage(ctx context.Context, downloadURL, binaryName string) error {
	tarballPath := fmt.Sprintf("/tmp/%s.tar.gz", binaryName)
	binaryPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)

	log.Debug("Downloading AppImage tarball", "url", downloadURL, "destination", tarballPath)
	if err := utils.DownloadFileWithContext(ctx, downloadURL, tarballPath); err != nil {
		return fmt.Errorf("failed to download AppImage tarball: %w", err)
	}

	// Clean up temporary file
	defer func() {
		if err := fs.Remove(tarballPath); err != nil {
			log.Debug("Failed to clean up tarball", "error", err)
		}
	}()

	log.Debug("Extracting AppImage tarball", "tarballPath", tarballPath)
	if err := extractTarball(tarballPath, "/tmp"); err != nil {
		return fmt.Errorf("failed to extract AppImage: %w", err)
	}

	// Clean up extracted binary in tmp after move
	tempBinaryPath := filepath.Join("/tmp", binaryName)
	defer func() {
		if err := fs.Remove(tempBinaryPath); err != nil {
			log.Debug("Failed to clean up temporary binary", "error", err)
		}
	}()

	log.Debug("Moving AppImage binary to final location", "binaryPath", binaryPath)
	if err := fs.Rename(tempBinaryPath, binaryPath); err != nil {
		return fmt.Errorf("failed to move AppImage binary: %w", err)
	}

	log.Debug("Setting executable permissions on binary", "binaryPath", binaryPath)
	if err := fs.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("failed to set permissions on AppImage binary: %w", err)
	}

	return nil
}

func extractTarball(tarballPath, destDir string) error {
	log.Debug("Extracting tarball", "tarballPath", tarballPath, "destDir", destDir)

	file, err := fs.Open(tarballPath)
	if err != nil {
		log.Error("Failed to open tarball", err, "tarballPath", tarballPath)
		return fmt.Errorf("failed to open tarball: %w", err)
	}
	defer func(file afero.File) {
		err := file.Close()
		if err != nil {
			log.Error("Failed to close tarball", err, "tarballPath", tarballPath)
		}
	}(file)

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		log.Error("Failed to create gzip reader", err, "tarballPath", tarballPath)
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func(gzipReader *gzip.Reader) {
		err := gzipReader.Close()
		if err != nil {
			log.Error("Failed to close gzip reader", err, "tarballPath", tarballPath)
		}
	}(gzipReader)

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("Failed to read tarball header", err, "tarballPath", tarballPath)
			return fmt.Errorf("failed to read tarball header: %w", err)
		}

		// #nosec G305 -- Path traversal protection implemented below
		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			log.Error("Potential directory traversal detected", fmt.Errorf("invalid entry: %s", header.Name))
			return fmt.Errorf("tarball entry is outside the target directory: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			log.Debug("Creating directory from tarball", "target", target)
			if err := fs.MkdirAll(target, 0o755); err != nil {
				log.Error("Failed to create directory from tarball", err, "target", target)
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			log.Debug("Extracting file from tarball", "target", target)
			outFile, err := fs.Create(target)
			if err != nil {
				log.Error("Failed to create file from tarball", err, "target", target)
				return fmt.Errorf("failed to create file: %w", err)
			}

			// Security: prevent decompression bombs by limiting file size
			const maxFileSize = 500 * 1024 * 1024 // 500MB limit for AppImages
			limitedReader := io.LimitReader(tarReader, maxFileSize)

			written, err := io.Copy(outFile, limitedReader)
			if err != nil {
				_ = outFile.Close()
				log.Error("Failed to write data to file from tarball", err, "target", target)
				return fmt.Errorf("failed to write data: %w", err)
			}
			_ = outFile.Close()

			// Check if we hit the limit
			if written == maxFileSize {
				log.Error("File size exceeds maximum allowed size", fmt.Errorf("file: %s", target))
				return fmt.Errorf("file size exceeds maximum allowed size of %d bytes: %s", maxFileSize, target)
			}
		}
	}

	log.Debug("Tarball extracted successfully", "tarballPath", tarballPath, "destDir", destDir)
	return nil
}
