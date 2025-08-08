package appimage

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type AppImageInstaller struct{}

func New() *AppImageInstaller {
	return &AppImageInstaller{}
}

func (a *AppImageInstaller) Install(command string, repo types.Repository) error {
	log.Debug("AppImage Installer: Starting installation", "command", command)

	// Parse command to extract download URL and binary name
	downloadURL, binaryName := parseAppImageCommand(command)
	if downloadURL == "" || binaryName == "" {
		// log.Error("Invalid command format", "command", command)
		return fmt.Errorf("invalid command format for AppImage installer")
	}

	// Check if the AppImage binary is already installed
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "appimage",
		InstallCommand: command,
	}
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if app is installed", err, "binaryName", binaryName)
		return fmt.Errorf("failed to check if AppImage binary is installed: %w", err)
	}
	if isInstalled {
		log.Info("AppImage already installed, skipping installation", "binaryName", binaryName)
		return nil
	}

	// Download and install the AppImage
	if err := installAppImage(downloadURL, binaryName); err != nil {
		log.Error("Failed to install AppImage", err, "downloadURL", downloadURL)
		return fmt.Errorf("failed to install AppImage: %w", err)
	}

	log.Debug("AppImage installed successfully", "binaryName", binaryName)

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
	_, binaryName := parseAppImageCommand(command)
	if binaryName == "" {
		return fmt.Errorf("invalid command format for AppImage uninstaller")
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
		return fmt.Errorf("failed to remove AppImage binary: %w", err)
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
	_, binaryName := parseAppImageCommand(command)
	if binaryName == "" {
		return false, fmt.Errorf("invalid command format for AppImage installer")
	}

	// Check if the binary exists in /usr/local/bin
	binaryPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)
	exists, err := fs.Exists(binaryPath)
	if err != nil {
		return false, fmt.Errorf("failed to check if AppImage exists: %w", err)
	}

	return exists, nil
}

func parseAppImageCommand(command string) (string, string) {
	parts := strings.Fields(command)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func installAppImage(downloadURL, binaryName string) error {
	tarballPath := fmt.Sprintf("/tmp/%s.tar.gz", binaryName)
	binaryPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)

	log.Debug("Downloading AppImage", "url", downloadURL, "destination", tarballPath)
	if err := utils.DownloadFile(downloadURL, tarballPath); err != nil {
		return fmt.Errorf("failed to download AppImage: %w", err)
	}

	log.Debug("Extracting AppImage tarball", "tarballPath", tarballPath)
	if err := extractTarball(tarballPath, "/tmp"); err != nil {
		return fmt.Errorf("failed to extract AppImage: %w", err)
	}

	log.Debug("Moving AppImage binary to final location", "binaryPath", binaryPath)
	if err := fs.Rename(filepath.Join("/tmp", binaryName), binaryPath); err != nil {
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
			log.Info("Extracting file from tarball", "target", target)
			outFile, err := fs.Create(target)
			if err != nil {
				log.Error("Failed to create file from tarball", err, "target", target)
				return fmt.Errorf("failed to create file: %w", err)
			}
			defer func(outFile afero.File) {
				err := outFile.Close()
				if err != nil {
					log.Error("Failed to close file from tarball", err, "target", target)
				}
			}(outFile)
			// Security: prevent decompression bombs by limiting file size
			const maxFileSize = 100 * 1024 * 1024 // 100MB limit
			limitedReader := io.LimitReader(tarReader, maxFileSize)

			written, err := io.Copy(outFile, limitedReader)
			if err != nil {
				log.Error("Failed to write data to file from tarball", err, "target", target)
				return fmt.Errorf("failed to write data: %w", err)
			}

			// Check if we hit the limit
			if written == maxFileSize {
				log.Error("File size exceeds maximum allowed size", fmt.Errorf("file: %s", target))
				return fmt.Errorf("file size exceeds maximum allowed size of %d bytes: %s", maxFileSize, target)
			}
		}
	}

	log.Info("Tarball extracted successfully", "tarballPath", tarballPath, "destDir", destDir)
	return nil
}
