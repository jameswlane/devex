package appimage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/types"
)

type AppImageInstaller struct{}

func New() *AppImageInstaller {
	return &AppImageInstaller{}
}

func (a *AppImageInstaller) Install(command string, repo repository.Repository) error {
	log.Info("AppImage Installer: Starting installation", "downloadURL", command)

	// Parse command to extract download URL and binary name
	downloadURL, binaryName := parseAppImageCommand(command)
	if downloadURL == "" || binaryName == "" {
		log.Error("AppImage Installer: Invalid command format", "command", command)
		return fmt.Errorf("invalid command format for AppImage installer")
	}

	// Wrap the command into a types.AppConfig object for the utilities function
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "appimage",
		InstallCommand: command,
	}

	// Check if the AppImage binary is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("AppImage Installer: Failed to check if app is installed", "binaryName", binaryName, "error", err)
		return fmt.Errorf("failed to check if AppImage binary is installed: %v", err)
	}

	if isInstalled {
		log.Info("AppImage Installer: App already installed, skipping", "binaryName", binaryName)
		return nil
	}

	// Download and install AppImage
	err = installAppImage(downloadURL, binaryName)
	if err != nil {
		log.Error("AppImage Installer: Failed to install AppImage", "downloadURL", downloadURL, "error", err)
		return fmt.Errorf("failed to install AppImage: %v", err)
	}

	log.Info("AppImage Installer: Installation successful", "binaryName", binaryName)

	// Add to repository
	if err := repo.AddApp(binaryName); err != nil {
		log.Error("AppImage Installer: Failed to add app to repository", "binaryName", binaryName, "error", err)
		return fmt.Errorf("failed to add AppImage to repository: %v", err)
	}

	log.Info("AppImage Installer: App added to repository", "binaryName", binaryName)
	return nil
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

	log.Info("AppImage Installer: Downloading AppImage", "url", downloadURL, "destination", tarballPath)
	if err := downloadFile(downloadURL, tarballPath); err != nil {
		return fmt.Errorf("failed to download AppImage: %v", err)
	}

	log.Info("AppImage Installer: Extracting AppImage", "tarballPath", tarballPath, "destination", "/tmp")
	if err := extractTarball(tarballPath, "/tmp"); err != nil {
		return fmt.Errorf("failed to extract AppImage: %v", err)
	}

	log.Info("AppImage Installer: Moving binary", "binaryPath", binaryPath)
	if err := moveFile(fmt.Sprintf("/tmp/%s", binaryName), binaryPath); err != nil {
		return fmt.Errorf("failed to move AppImage binary: %v", err)
	}

	log.Info("AppImage Installer: Setting executable permissions", "binaryPath", binaryPath)
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("failed to set permissions on AppImage binary: %v", err)
	}

	return nil
}

func downloadFile(url, dest string) error {
	log.Info("Starting downloadFile", "url", url, "dest", dest)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Error("Failed to create request", "url", url, "error", err)
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Download the file
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Failed to download file", "url", url, "error", err)
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error("Unexpected status code", "url", url, "statusCode", resp.StatusCode)
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		log.Error("Failed to create destination file", "dest", dest, "error", err)
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Error("Failed to save file", "dest", dest, "error", err)
		return fmt.Errorf("failed to save file: %v", err)
	}

	log.Info("Downloaded file successfully", "url", url, "dest", dest)
	return nil
}

func extractTarball(tarballPath, destDir string) error {
	log.Info("Starting extractTarball", "tarballPath", tarballPath, "destDir", destDir)

	file, err := os.Open(tarballPath)
	if err != nil {
		log.Error("Failed to open tarball", "tarballPath", tarballPath, "error", err)
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		log.Error("Failed to create gzip reader", "tarballPath", tarballPath, "error", err)
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("Failed to read tarball", "tarballPath", tarballPath, "error", err)
			return err
		}

		target := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			log.Info("Creating directory", "target", target)
			err = os.MkdirAll(target, 0o755)
			if err != nil {
				log.Error("Failed to create directory", "target", target, "error", err)
				return fmt.Errorf("failed to create directory: %v", err)
			}
		case tar.TypeReg:
			log.Info("Creating file", "target", target)
			outFile, err := os.Create(target)
			if err != nil {
				log.Error("Failed to create file", "target", target, "error", err)
				return err
			}
			defer outFile.Close()
			_, err = io.Copy(outFile, tarReader)
			if err != nil {
				log.Error("Failed to copy data", "target", target, "error", err)
				return fmt.Errorf("failed to copy data: %v", err)
			}
		}
	}

	log.Info("Extracted tarball successfully", "tarballPath", tarballPath, "destDir", destDir)
	return nil
}

func moveFile(src, dest string) error {
	log.Info("Starting moveFile", "src", src, "dest", dest)
	err := os.Rename(src, dest)
	if err != nil {
		log.Error("Failed to move file", "src", src, "dest", dest, "error", err)
	}
	return err
}
