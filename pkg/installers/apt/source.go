package apt

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/utils"
)

func AddAptSource(keySource, keyName, sourceRepo, sourceName string, dearmor bool) error {
	log.Info("Adding APT source", "keySource", keySource, "keyName", keyName, "sourceRepo", sourceRepo, "sourceName", sourceName)

	if err := ValidateAptRepo(sourceRepo); err != nil {
		log.Error("Invalid repository string", "repo", sourceRepo, "error", err)
		return fmt.Errorf("invalid repository: %v", err)
	}

	if keySource != "" {
		if err := DownloadGPGKey(keySource, sourceName+".gpg", dearmor); err != nil {
			log.Error("Failed to download GPG key", "keySource", keySource, "error", err)
			return fmt.Errorf("failed to download GPG key: %v", err)
		}
	}

	evaluatedRepo := replaceTemplatePlaceholders(sourceRepo)

	dir := filepath.Dir(sourceName)
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory for source file: %v", err)
	}
	exists, err := fs.FileExistsAndIsFile(sourceName)
	if err != nil {
		return fmt.Errorf("failed to check if source file exists: %v", err)
	}

	if exists {
		log.Info("APT source file already exists", "sourceName", sourceName)
		return nil
	}

	if err := fs.WriteFile(sourceName, []byte(evaluatedRepo+"\n"), 0o644); err != nil {
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

// getCommandOutput executes a command and returns its output.
func getCommandOutput(command string) string {
	output, err := utils.RunCommand("bash", "-c", command)
	if err != nil {
		log.Error("Failed to evaluate placeholder", "command", command, "error", err)
		return ""
	}
	return strings.TrimSpace(output)
}
