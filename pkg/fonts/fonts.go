package fonts

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/utils"
)

type Font struct {
	Name        string `yaml:"name"`
	Method      string `yaml:"method"`
	URL         string `yaml:"url,omitempty"`
	ExtractPath string `yaml:"extract_path,omitempty"`
	Destination string `yaml:"destination,omitempty"`
}

var utilsInstance utils.Interface = &utils.OSCommandExecutor{}

func SetUtils(mock utils.Interface) {
	utilsInstance = mock
}

func SetDownloadFile(mock func(url string) (string, error)) {
	downloadFile = mock
}

func LoadFonts(filename string) ([]Font, error) {
	log.Info("Loading fonts from file", "filename", filename)

	data, err := fs.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read fonts YAML file: %w", err)
	}

	var fonts []Font
	if err := yaml.Unmarshal(data, &fonts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fonts YAML: %w", err)
	}

	return fonts, nil
}

func InstallFont(font Font) error {
	log.Info("Installing font", "font", font.Name, "method", font.Method)

	switch font.Method {
	case "url":
		return installFromURL(font)
	case "oh-my-posh":
		_, err := utilsInstance.RunShellCommand(fmt.Sprintf("oh-my-posh font install %s", font.Name))
		return err
	case "homebrew":
		_, err := utilsInstance.RunShellCommand(fmt.Sprintf("brew install --cask font-%s-nerd-font", font.Name))
		return err
	default:
		return fmt.Errorf("unsupported install method: %s", font.Method)
	}
}

func installFromURL(font Font) error {
	if err := fs.MkdirAll(font.Destination, 0o755); err != nil {
		return fmt.Errorf("failed to create font destination: %w", err)
	}

	zipFile, err := downloadFile(font.URL)
	if err != nil {
		return fmt.Errorf("failed to download font: %w", err)
	}
	defer func() {
		if err := fs.AppFs.Remove(zipFile); err != nil {
			log.Error("failed to remove zip file", err)
		}
	}()

	if err := UnzipAndMove(zipFile, font.ExtractPath, font.Destination); err != nil {
		return fmt.Errorf("failed to extract font: %w", err)
	}

	if _, err := utilsInstance.RunShellCommand("fc-cache -f -v"); err != nil {
		return fmt.Errorf("failed to refresh font cache: %w", err)
	}

	return nil
}

var downloadFile = func(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpFile, err := fs.TempFile("", "font-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if err := utilsInstance.DownloadFileWithContext(ctx, url, tmpFile.Name()); err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	return tmpFile.Name(), nil
}

func UnzipAndMove(zipFile, extractPath, dest string) error {
	file, err := fs.AppFs.Open(zipFile)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get zip file info: %w", err)
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return fmt.Errorf("failed to read zip file: %w", err)
	}

	for _, f := range reader.File {
		destPath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			err = fs.MkdirAll(destPath, 0o755)
		} else {
			err = extractFile(f, destPath)
		}
		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	return nil
}

func extractFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	file, err := fs.AppFs.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, rc)
	return err
}
