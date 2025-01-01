package apt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/executil"
	"github.com/jameswlane/devex/pkg/fs"
)

// AddAptSource adds a new APT source repository and optional GPG key.
func AddAptSource(keySource, keyName, sourceRepo, sourceName string, dearmor bool) error {
	log.Info("Adding APT source", "keySource", keySource, "keyName", keyName, "sourceRepo", sourceRepo, "sourceName", sourceName)

	// Validate repository string
	if err := ValidateAptRepo(sourceRepo); err != nil {
		log.Error("Invalid repository string", "repo", sourceRepo, "error", err)
		return fmt.Errorf("invalid repository: %v", err)
	}

	// Download GPG key if provided
	if keySource != "" {
		if err := DownloadGPGKey(keySource, keyName, dearmor); err != nil {
			log.Error("Failed to download GPG key", "keySource", keySource, "error", err)
			return fmt.Errorf("failed to download GPG key: %v", err)
		}
	}

	// Replace placeholders in the source repository string
	evaluatedRepo := replaceTemplatePlaceholders(sourceRepo)

	// Ensure the target directory exists
	dir := filepath.Dir(sourceName)
	if err := fs.MkdirAll(dir, os.FileMode(0o755)); err != nil {
		return fmt.Errorf("failed to create directory for source file: %v", err)
	}

	// Write the repository to the source file
	if err := fs.WriteFile(sourceName, []byte(evaluatedRepo+"\n"), os.FileMode(0o644)); err != nil {
		return fmt.Errorf("failed to write APT source file: %v", err)
	}

	log.Info("APT source added successfully", "sourceName", sourceName)
	return nil
}

// replaceTemplatePlaceholders replaces placeholders in the repository template.
func replaceTemplatePlaceholders(template string) string {
	placeholders := map[string]string{
		"%ARCHITECTURE%": getCommandOutput("dpkg --print-architecture"),
		"%CODENAME%":     getCommandOutput("bash -c '. /etc/os-release && echo $VERSION_CODENAME'"),
	}

	for placeholder, value := range placeholders {
		template = strings.ReplaceAll(template, placeholder, value)
	}

	return template
}

// getCommandOutput executes a command and returns its output.
func getCommandOutput(command string) string {
	output, err := executil.RunCommand("bash", "-c", command)
	if err != nil {
		log.Error("Failed to evaluate placeholder", "command", command, "error", err)
		return ""
	}
	return strings.TrimSpace(output)
}
