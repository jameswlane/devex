package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// GnomeExtension struct for gnome_extensions.yaml
type GnomeExtension struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

// ProgrammingLanguage struct for programming_languages.yaml
type ProgrammingLanguage struct {
	Name           string `yaml:"name"`
	InstallCommand string `yaml:"install_command"`
}

// LoadCustomOrDefaultFile loads a custom file from ~/.devex if available, otherwise defaults to the assets/ folder
func LoadCustomOrDefaultFile(defaultPath, assetType string) (string, error) {
	customPath := filepath.Join(os.Getenv("HOME"), ".devex", assetType, filepath.Base(defaultPath))

	// Check if the user has a custom file
	if _, err := os.Stat(customPath); err == nil {
		fmt.Printf("Using custom config: %s\n", customPath)
		return customPath, nil
	}

	// Otherwise, fallback to the default in assets/
	defaultAssetPath := filepath.Join("assets", assetType, filepath.Base(defaultPath))
	fmt.Printf("Using default config: %s\n", defaultAssetPath)

	if _, err := os.Stat(defaultAssetPath); err != nil {
		return "", fmt.Errorf("failed to load default asset: %v", err)
	}

	return defaultAssetPath, nil
}

// LoadYAMLConfig loads a YAML file into the provided interface (e.g., []GnomeExtension, []ProgrammingLanguage)
func LoadYAMLConfig(filePath string, out any) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("failed to decode YAML: %v", err)
	}

	return nil
}

// LoadGnomeExtensionsConfig loads GNOME extensions from gnome_extensions.yaml
func LoadGnomeExtensionsConfig(filename string) ([]GnomeExtension, error) {
	filePath, err := LoadCustomOrDefaultFile(filename, "gnome_extensions")
	if err != nil {
		return nil, err
	}

	var extensions []GnomeExtension
	err = LoadYAMLConfig(filePath, &extensions)
	if err != nil {
		return nil, fmt.Errorf("failed to load GNOME extensions config: %v", err)
	}

	return extensions, nil
}

// LoadProgrammingLanguagesConfig loads programming languages from programming_languages.yaml
func LoadProgrammingLanguagesConfig(filename string) ([]ProgrammingLanguage, error) {
	filePath, err := LoadCustomOrDefaultFile(filename, "programming_languages")
	if err != nil {
		return nil, err
	}

	var languages []ProgrammingLanguage
	err = LoadYAMLConfig(filePath, &languages)
	if err != nil {
		return nil, fmt.Errorf("failed to load programming languages config: %v", err)
	}

	return languages, nil
}
