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
	// Check if the app is already installed on the system
	isInstalledOnSystem, err := check_install.IsAppInstalled(binary)
	if err != nil {
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
	err = downloadFileFunc(downloadURL, tarballPath)
	if err != nil {
		return fmt.Errorf("failed to download AppImage: %v", err)
	}

	err = extractTarballFunc(tarballPath, "/tmp")
	if err != nil {
		return fmt.Errorf("failed to extract AppImage tarball: %v", err)
	}

	binaryPath := filepath.Join("/tmp", binary)
	err = moveFile(binaryPath, filepath.Join(installDir, binary))
	if err != nil {
		return fmt.Errorf("failed to move AppImage binary: %v", err)
	}

	err = os.Chmod(filepath.Join(installDir, binary), 0o755)
	if err != nil {
		return fmt.Errorf("failed to set executable permissions: %v", err)
	}

	// Add to the repository
	err = repo.AddApp(appName)
	if err != nil {
		return fmt.Errorf("failed to add %s to repository: %v", appName, err)
	}

	return nil
}

func downloadFile(url, dest string) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Download the file
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}

func extractTarball(tarballPath, destDir string) error {
	file, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
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
			return err
		}

		target := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o755)
			if err != nil {
				return fmt.Errorf("failed to create directory: %v", err)
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			defer outFile.Close()
			_, err = io.Copy(outFile, tarReader)
			if err != nil {
				return fmt.Errorf("failed to copy data: %v", err)
			}
		}
	}
	return nil
}

func moveFile(src, dest string) error {
	return os.Rename(src, dest)
}
