package main

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/utils"
)

// ProcessGPGKey dearmors and saves a GPG key to the desired location.
func ProcessGPGKey(tempFile, destination string) error {
	log.Info("Processing GPG key", "tempFile", tempFile, "destination", destination)

	// Validate that the temporary file exists and is not empty
	if err := validateGPGKeyFile(tempFile); err != nil {
		return fmt.Errorf("GPG key validation failed: %w", err)
	}

	// Execute the gpg --dearmor command
	command := fmt.Sprintf("gpg --dearmor -o %s %s", destination, tempFile)
	if _, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to dearmor GPG key", err, "command", command)
		return fmt.Errorf("failed to dearmor GPG key: %w", err)
	}

	// Verify the dearmored key was created successfully
	if err := validateGPGKeyFile(destination); err != nil {
		return fmt.Errorf("dearmored GPG key validation failed: %w", err)
	}

	// Remove the temporary file
	if err := fs.Remove(tempFile); err != nil {
		log.Warn("Failed to remove temporary file after dearmor", err, "tempFile", tempFile)
	} else {
		log.Info("Temporary file removed successfully", "tempFile", tempFile)
	}

	log.Info("GPG key processed successfully", "destination", destination)
	return nil
}

// validateGPGKeyFile validates that a GPG key file exists and has reasonable content
func validateGPGKeyFile(filePath string) error {
	// Check if file exists
	exists, err := fs.FileExistsAndIsFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to check GPG key file: %w", err)
	}
	if !exists {
		return fmt.Errorf("GPG key file does not exist: %s", filePath)
	}

	// Check file size (GPG keys should be at least a few hundred bytes)
	info, err := fs.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get GPG key file info: %w", err)
	}
	if info.Size() < 100 {
		return fmt.Errorf("GPG key file appears to be too small: %d bytes", info.Size())
	}
	if info.Size() > 50*1024 { // 50KB should be more than enough for a GPG key
		return fmt.Errorf("GPG key file appears to be too large: %d bytes", info.Size())
	}

	log.Info("GPG key file validated", "file", filePath, "size", info.Size())
	return nil
}
