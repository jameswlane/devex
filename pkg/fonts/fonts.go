package fonts

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/pkg/log"
)

type Font struct {
	Name        string `yaml:"name"`
	Method      string `yaml:"method"`
	URL         string `yaml:"url,omitempty"`
	ExtractPath string `yaml:"extract_path,omitempty"`
	Destination string `yaml:"destination,omitempty"`
}

// LoadFonts loads the font configuration from a YAML file
func LoadFonts(filename string) ([]Font, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read fonts YAML file: %w", err)
	}

	var fonts []Font
	if err := yaml.Unmarshal(data, &fonts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fonts YAML: %w", err)
	}

	return fonts, nil
}

// InstallFont installs the font based on the method (url, oh-my-posh, or homebrew)
func InstallFont(font Font) error {
	switch font.Method {
	case "url":
		return installFromURL(font)
	case "oh-my-posh":
		return runCommand("oh-my-posh", []string{"font", "install", font.Name})
	case "homebrew":
		return runCommand("brew", []string{"install", "--cask", "font-" + font.Name + "-nerd-font"})
	default:
		return fmt.Errorf("unsupported install method: %s", font.Method)
	}
}

// installFromURL installs a font by downloading and extracting it from a URL
func installFromURL(font Font) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(font.Destination, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create font destination: %w", err)
	}

	// Download the font archive
	zipFile, err := downloadFile(font.URL)
	if err != nil {
		return fmt.Errorf("failed to download font: %w", err)
	}
	defer os.Remove(zipFile)

	// Extract the font files
	if err := unzipAndMove(zipFile, font.ExtractPath, font.Destination); err != nil {
		return fmt.Errorf("failed to extract font: %w", err)
	}

	// Refresh font cache
	if err := runCommand("fc-cache", []string{"-f", "-v"}); err != nil {
		return fmt.Errorf("failed to refresh font cache: %w", err)
	}

	return nil
}

// downloadFile downloads a file from the given URL to a temporary file
func downloadFile(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp("", "font-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to save downloaded file: %w", err)
	}
	tmpFile.Close()

	return tmpFile.Name(), nil
}

// unzipAndMove extracts the fonts from the zip and moves them to the destination
func unzipAndMove(zipFile, extractPath, dest string) error {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, extractPath) && strings.HasSuffix(file.Name, ".ttf") {
			rc, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open font file: %w", err)
			}
			defer rc.Close()

			destFile := filepath.Join(dest, filepath.Base(file.Name))
			outFile, err := os.Create(destFile)
			if err != nil {
				return fmt.Errorf("failed to create font file: %w", err)
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, rc); err != nil {
				return fmt.Errorf("failed to copy font file: %w", err)
			}
			log.Info(fmt.Sprintf("Installed font: file=%s", destFile))
		}
	}

	return nil
}

// runCommand runs a command with the specified arguments
func runCommand(cmd string, args []string) error {
	log.Info(fmt.Sprintf("Running command: %s %s", cmd, strings.Join(args, " ")))
	command := exec.Command(cmd, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
