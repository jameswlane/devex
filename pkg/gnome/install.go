package gnome

import (
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/utils"
)

type GnomeExtension struct {
	ID          string       `yaml:"id"`
	SchemaFiles []SchemaFile `yaml:"schema_files"`
}

type SchemaFile struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
}

// LoadGnomeExtensions loads the GNOME extensions from the YAML configuration.
func LoadGnomeExtensions(filename string) ([]GnomeExtension, error) {
	log.Info("Loading GNOME extensions from configuration", "filename", filename)

	data, err := fs.ReadFile(filename)
	if err != nil {
		log.Error("Failed to read GNOME extensions YAML file", err, "filename", filename)
		return nil, fmt.Errorf("failed to read GNOME extensions YAML file: %w", err)
	}

	var extensions []GnomeExtension
	if err := yaml.Unmarshal(data, &extensions); err != nil {
		log.Error("Failed to parse GNOME extensions YAML", err, "filename", filename)
		return nil, fmt.Errorf("failed to parse GNOME extensions YAML: %w", err)
	}

	log.Info("GNOME extensions loaded successfully", "count", len(extensions))
	return extensions, nil
}

// InstallGnomeExtension installs a GNOME extension using gext and handles schema files.
func InstallGnomeExtension(extension GnomeExtension) error {
	log.Info("Installing GNOME extension", "id", extension.ID)

	// Install the extension using gext
	installCommand := fmt.Sprintf("gext install %s", extension.ID)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install GNOME extension", err, "id", extension.ID)
		return fmt.Errorf("failed to install GNOME extension %s: %w", extension.ID, err)
	}

	// Handle schema files
	for _, schema := range extension.SchemaFiles {
		if err := copySchemaFile(schema); err != nil {
			log.Error("Failed to copy schema file", err, "id", extension.ID, "source", schema.Source, "destination", schema.Destination)
			return fmt.Errorf("failed to copy schema file for %s: %w", extension.ID, err)
		}
	}

	log.Info("GNOME extension installed successfully", "id", extension.ID)
	return nil
}

// copySchemaFile copies schema files to the appropriate destination.
func copySchemaFile(schema SchemaFile) error {
	log.Info("Copying schema file", "source", schema.Source, "destination", schema.Destination)

	// Ensure destination directory exists
	if err := fs.EnsureDir(schema.Destination, 0o755); err != nil {
		log.Error("Failed to create destination directory", err, "destination", schema.Destination)
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy the schema file
	destinationFile := filepath.Join(schema.Destination, filepath.Base(schema.Source))
	if err := utils.CopyFile(schema.Source, destinationFile); err != nil {
		log.Error("Failed to copy schema file", err, "source", schema.Source, "destination", destinationFile)
		return fmt.Errorf("failed to copy schema file: %w", err)
	}

	log.Info("Schema file copied successfully", "source", schema.Source, "destination", destinationFile)
	return nil
}

// CompileSchemas runs glib-compile-schemas after installing extensions.
func CompileSchemas() error {
	log.Info("Compiling schemas")

	compileCommand := "sudo glib-compile-schemas /usr/share/glib-2.0/schemas/"
	if _, err := utils.CommandExec.RunShellCommand(compileCommand); err != nil {
		log.Error("Failed to compile schemas", err)
		return fmt.Errorf("failed to compile schemas: %w", err)
	}

	log.Info("Schemas compiled successfully")
	return nil
}
