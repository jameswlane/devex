package fonts

import (
	"archive/zip"
	"fmt"
	"github.com/jameswlane/devex/pkg/logger"
	"gopkg.in/yaml.v2"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Font struct {
	Name        string `yaml:"name"`
	Method      string `yaml:"method"`
	URL         string `yaml:"url,omitempty"`
	ExtractPath string `yaml:"extract_path,omitempty"`
	Destination string `yaml:"destination,omitempty"`
}

var log = logger.InitLogger()

// LoadFonts loads the font configuration from a YAML file
func LoadFonts(filename string) ([]Font, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read fonts YAML file: %v", err)
	}

	var fonts []Font
	err = yaml.Unmarshal(data, &fonts)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal fonts YAML: %v", err)
	}

	return fonts, nil
}

// InstallFont installs the font based on the method (url, oh-my-posh, or homebrew)
func InstallFont(font Font) error {
	switch font.Method {
	case "url":
		return installFromURL(font)
	case "oh-my-posh":
		return installWithOhMyPosh(font.Name)
	case "homebrew":
		return installWithHomebrew(font.Name)
	default:
		return fmt.Errorf("unsupported install method: %s", font.Method)
	}
}

// installFromURL installs a font by downloading and extracting it from a URL
func installFromURL(font Font) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(font.Destination, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create font destination: %v", err)
	}

	// Download the file
	log.LogInfo(fmt.Sprintf("Downloading font: url=%s", font.URL))
	resp, err := http.Get(font.URL)
	if err != nil {
		return fmt.Errorf("failed to download font: %v", err)
	}
	defer resp.Body.Close()

	// Create a temporary file for the downloaded zip
	tmpFile, err := os.CreateTemp("", "font-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the downloaded content to the temp file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save downloaded font: %v", err)
	}

	// Extract the font files
	err = unzipAndMove(tmpFile.Name(), font.ExtractPath, font.Destination)
	if err != nil {
		return fmt.Errorf("failed to extract font: %v", err)
	}

	// Refresh font cache
	log.LogInfo("Refreshing font cache...")
	if err := exec.Command("fc-cache", "-f", "-v").Run(); err != nil {
		return fmt.Errorf("failed to refresh font cache: %v", err)
	}

	return nil
}

// unzipAndMove extracts the fonts from the zip and moves them to the destination
func unzipAndMove(zipFile, extractPath, dest string) error {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %v", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, extractPath) && strings.HasSuffix(file.Name, ".ttf") {
			rc, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open font file: %v", err)
			}
			defer rc.Close()

			// Destination for extracted file
			destFile := filepath.Join(dest, filepath.Base(file.Name))
			outFile, err := os.Create(destFile)
			if err != nil {
				return fmt.Errorf("failed to create font file: %v", err)
			}
			defer outFile.Close()

			// Copy the font file
			_, err = io.Copy(outFile, rc)
			if err != nil {
				return fmt.Errorf("failed to copy font file: %v", err)
			}
			log.LogInfo(fmt.Sprintf("Installed font: file=%s", destFile))
		}
	}

	return nil
}

// installWithOhMyPosh installs the font via Oh-my-posh
func installWithOhMyPosh(fontName string) error {
	log.LogInfo(fmt.Sprintf("Installing font with Oh-my-posh: font=%s", fontName))
	cmd := exec.Command("oh-my-posh", "font", "install", fontName)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install font with Oh-my-posh: %v", err)
	}
	return nil
}

// installWithHomebrew installs the font via Homebrew
func installWithHomebrew(fontName string) error {
	log.LogInfo(fmt.Sprintf("Installing font with Homebrew: font=%s", fontName))
	cmd := exec.Command("brew", "install", "--cask", "font-"+fontName+"-nerd-font")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install font with Homebrew: %v", err)
	}
	return nil
}
