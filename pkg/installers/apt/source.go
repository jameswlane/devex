package apt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
)

func AddAptSource(keySource string, keyName string, sourceRepo string, sourceName string, dearmor bool) error {
	log.Info("Adding APT source", "keySource", keySource, "keyName", keyName, "sourceRepo", sourceRepo, "sourceName", sourceName)

	if err := ValidateAptRepo(sourceRepo); err != nil {
		log.Error("Invalid repository string", "repo", sourceRepo, "error", err)
		return fmt.Errorf("invalid repository: %v", err)
	}

	// Download and dearmor GPG key
	if keySource != "" {
		if err := DownloadGPGKey(keySource, keyName, dearmor); err != nil {
			log.Error("Failed to download GPG key", "error", err)
			return fmt.Errorf("failed to download GPG key: %v", err)
		}
	}

	// Replace placeholders in sourceRepo
	evaluatedRepo := replaceTemplatePlaceholders(sourceRepo)

	// Ensure the directory exists
	dir := filepath.Dir(sourceName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory for source file: %v", err)
	}

	// Write evaluated repository to source file
	if err := os.WriteFile(sourceName, []byte(evaluatedRepo+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write APT source file: %v", err)
	}

	log.Info("APT source added successfully", "sourceName", sourceName)
	return nil
}

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

func getCommandOutput(command string) string {
	output, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		log.Error("Failed to evaluate placeholder", "command", command, "error", err)
		return ""
	}
	return strings.TrimSpace(string(output))
}
