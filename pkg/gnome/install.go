package gnome

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/jameswlane/devex/pkg/log"
)

type GnomeExtension struct {
	ID          string       `yaml:"id"`
	SchemaFiles []SchemaFile `yaml:"schema_files"`
}

type SchemaFile struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
}

// LoadGnomeExtensions loads the GNOME extensions from the YAML configuration
func LoadGnomeExtensions(filename string) ([]GnomeExtension, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read GNOME extensions YAML file: %v", err)
	}

	var extensions []GnomeExtension
	err = yaml.Unmarshal(data, &extensions)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal GNOME extensions YAML: %v", err)
	}

	return extensions, nil
}

// InstallGnomeExtension installs a GNOME extension using gnome-extensions-cli
func InstallGnomeExtension(extension GnomeExtension) error {
	log.Info(fmt.Sprintf("Installing GNOME extension: id=%s", extension.ID))

	cmd := exec.Command("gext", "install", extension.ID)
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(fmt.Sprintf("Failed to install GNOME extension: id=%s", extension.ID), err)
		return err
	}

	// Handle schema files
	for _, schema := range extension.SchemaFiles {
		if err := copySchemaFile(schema); err != nil {
			return fmt.Errorf("failed to copy schema file for %s: %v", extension.ID, err)
		}
	}

	return nil
}

// copySchemaFile copies schema files to the appropriate destination
func copySchemaFile(schema SchemaFile) error {
	log.Info(fmt.Sprintf("Copying schema file: source=%s, destination=%s", schema.Source, schema.Destination))

	// Ensure destination directory exists
	if err := os.MkdirAll(schema.Destination, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	// Copy the schema file
	sourceFile, err := os.Open(schema.Source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	destinationFile := filepath.Join(schema.Destination, filepath.Base(schema.Source))
	destFile, err := os.Create(destinationFile)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	return nil
}

// CompileSchemas runs glib-compile-schemas after installing extensions
func CompileSchemas() error {
	log.Info("Compiling schemas...")

	cmd := exec.Command("sudo", "glib-compile-schemas", "/usr/share/glib-2.0/schemas/")
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to compile schemas", err)
		return err
	}

	log.Info("Schemas compiled successfully")
	return nil
}
