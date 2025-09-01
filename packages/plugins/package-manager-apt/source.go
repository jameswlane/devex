package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/utils"
)

func AddAptSource(keySource, keyName, sourceRepo, sourceName string, dearmor bool) error {
	log.Info("Adding APT source", "keySource", keySource, "keyName", keyName, "sourceRepo", sourceRepo, "sourceName", sourceName)

	// Validate the repository
	if err := ValidateAptRepo(sourceRepo); err != nil {
		log.Error("Invalid repository string", err, "repo", sourceRepo)
		return fmt.Errorf("invalid repository: %w", err)
	}

	// Download and optionally dearmor the GPG key
	if keySource != "" {
		if err := DownloadGPGKey(keySource, keyName, dearmor); err != nil {
			log.Error("Failed to download GPG key", err, "keySource", keySource)
			return fmt.Errorf("failed to download GPG key: %w", err)
		}
	}

	// Replace template placeholders in the repository string
	placeholders := map[string]string{
		"%ARCHITECTURE%": getCommandOutput("dpkg --print-architecture"),
		"%CODENAME%":     getCommandOutput("bash -c '. /etc/os-release && echo $VERSION_CODENAME'"),
	}
	evaluatedRepo := utils.ReplacePlaceholders(sourceRepo, placeholders)

	// Ensure the directory for the source file exists
	dir := filepath.Dir(sourceName)
	if err := fs.EnsureDir(dir, 0o755); err != nil {
		log.Error("Failed to create directory", err, "directory", dir)
		return fmt.Errorf("failed to create directory for source file: %w", err)
	}

	// Check if the source file already exists
	exists, err := fs.FileExistsAndIsFile(sourceName)
	if err != nil {
		log.Error("Failed to check if source file exists", err, "sourceName", sourceName)
		return fmt.Errorf("failed to check if source file exists: %w", err)
	}
	if exists {
		log.Info("APT source file already exists", "sourceName", sourceName)
		return nil
	}

	// Write the repository string to the source file
	if err := fs.WriteFile(sourceName, []byte(evaluatedRepo+"\n"), 0o644); err != nil {
		log.Error("Failed to write APT source file", err, "sourceName", sourceName)
		return fmt.Errorf("failed to write APT source file: %w", err)
	}

	log.Info("APT source added successfully", "sourceName", sourceName)
	return nil
}

// getCommandOutput executes a command and returns its output.
func getCommandOutput(command string) string {
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		log.Error("Failed to evaluate placeholder", err, "command", command)
		return ""
	}
	return strings.TrimSpace(output)
}
