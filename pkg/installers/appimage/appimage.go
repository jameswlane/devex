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
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
)

var (
	downloadFileFunc   = downloadFile
	extractTarballFunc = extractTarball
)

func Install(appName, downloadURL, installDir, binary string, dryRun bool, repo repository.Repository) error {
	log.Info("Starting Install", "appName", appName, "downloadURL", downloadURL, "installDir", installDir, "binary", binary, "dryRun", dryRun)

	// Check if the app is already installed on the system
	isInstalledOnSystem, err := check_install.IsAppInstalled(binary)
	if err != nil {
		log.Error("Failed to check if app is installed on system", "binary", binary, "error", err)
		return fmt.Errorf("failed to check if app is installed on system: %v", err)
	}

	if isInstalledOnSystem {
		log.Info(fmt.Sprintf("%s is already installed on the system, skipping installation", appName))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would download file from URL: %s", downloadURL))
		log.Info(fmt.Sprintf("[Dry Run] Would extract tarball to: %s", "/tmp"))
		log.Info(fmt.Sprintf("[Dry Run] Would move binary to: %s", filepath.Join(installDir, binary)))
		log.Info(fmt.Sprintf("[Dry Run] Would set executable permissions for: %s", filepath.Join(installDir, binary)))
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
		return nil
	}

	// Download and install
	tarballPath := "/tmp/appimage.tar.gz"
	log.Info("Downloading file", "url", downloadURL, "dest", tarballPath)
	err = downloadFileFunc(downloadURL, tarballPath)
	if err != nil {
		log.Error("Failed to download AppImage", "url", downloadURL, "dest", tarballPath, "error", err)
		return fmt.Errorf("failed to download AppImage: %v", err)
	}

	log.Info("Extracting tarball", "tarballPath", tarballPath, "destDir", "/tmp")
	err = extractTarballFunc(tarballPath, "/tmp")
	if err != nil {
		log.Error("Failed to extract AppImage tarball", "tarballPath", tarballPath, "destDir", "/tmp", "error", err)
		return fmt.Errorf("failed to extract AppImage tarball: %v", err)
	}

	binaryPath := filepath.Join("/tmp", binary)
	log.Info("Moving binary", "src", binaryPath, "dest", filepath.Join(installDir, binary))
	err = moveFile(binaryPath, filepath.Join(installDir, binary))
	if err != nil {
		log.Error("Failed to move AppImage binary", "src", binaryPath, "dest", filepath.Join(installDir, binary), "error", err)
		return fmt.Errorf("failed to move AppImage binary: %v", err)
	}

	log.Info("Setting executable permissions", "file", filepath.Join(installDir, binary))
	err = os.Chmod(filepath.Join(installDir, binary), 0o755)
	if err != nil {
		log.Error("Failed to set executable permissions", "file", filepath.Join(installDir, binary), "error", err)
		return fmt.Errorf("failed to set executable permissions: %v", err)
	}

	// Add to the repository
	log.Info("Adding app to repository", "appName", appName)
	err = repo.AddApp(appName)
	if err != nil {
		log.Error("Failed to add app to repository", "appName", appName, "error", err)
		return fmt.Errorf("failed to add %s to repository: %v", appName, err)
	}

	log.Info(fmt.Sprintf("%s installed successfully", appName))
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
