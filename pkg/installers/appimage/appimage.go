package appimage

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Assignable functions for easier testing
var downloadFileFunc = downloadFile
var extractTarballFunc = extractTarball

// InstallAppImage handles the installation of AppImages
func Install(downloadURL, installDir, binary string, dryRun bool) error {
	// Step 1: Download the tarball
	tarballPath := "/tmp/appimage.tar.gz"
	if dryRun {
		log.Printf("[Dry Run] Would download file from URL: %s", downloadURL)
		log.Printf("[Dry Run] Would extract tarball to: %s", "/tmp")
		log.Printf("[Dry Run] Would move binary to: %s", filepath.Join(installDir, binary))
		log.Printf("[Dry Run] Would set executable permissions for: %s", filepath.Join(installDir, binary))
		return nil
	}

	err := downloadFileFunc(downloadURL, tarballPath)
	if err != nil {
		return fmt.Errorf("failed to download AppImage: %v", err)
	}

	// Step 2: Extract the tarball
	err = extractTarballFunc(tarballPath, "/tmp")
	if err != nil {
		return fmt.Errorf("failed to extract AppImage tarball: %v", err)
	}

	// Step 3: Move the binary to the install directory
	binaryPath := filepath.Join("/tmp", binary)
	err = moveFile(binaryPath, filepath.Join(installDir, binary))
	if err != nil {
		return fmt.Errorf("failed to move AppImage binary: %v", err)
	}

	// Step 4: Make the binary executable
	err = os.Chmod(filepath.Join(installDir, binary), 0755)
	if err != nil {
		return fmt.Errorf("failed to set executable permissions: %v", err)
	}

	return nil
}

// Helper to download a file
func downloadFile(url, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// Helper to extract tarball
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
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			defer outFile.Close()
			io.Copy(outFile, tarReader)
		}
	}
	return nil
}

// Helper to move a file
func moveFile(src, dest string) error {
	return os.Rename(src, dest)
}
