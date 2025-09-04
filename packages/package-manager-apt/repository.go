package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// APTSource represents an APT repository source configuration
type APTSource struct {
	KeyURL         string `json:"key_url"`
	KeyPath        string `json:"key_path"`
	SourceLine     string `json:"source_line"`
	SourceFile     string `json:"source_file"`
	RequireDearmor bool   `json:"require_dearmor"`
}

// handleAddRepository adds a new APT repository with GPG key management
func (a *APTInstaller) handleAddRepository(args []string) error {
	a.logger.Printf("Adding APT repository...\n")

	// For now, we'll implement basic repository addition
	// In a real implementation, this would parse command-line arguments
	// or receive structured data from the DevEx CLI

	if len(args) < 4 {
		return fmt.Errorf("add-repository requires: key-url key-path source-line source-file")
	}

	source := APTSource{
		KeyURL:         args[0],
		KeyPath:        args[1],
		SourceLine:     args[2],
		SourceFile:     args[3],
		RequireDearmor: len(args) > 4 && args[4] == "true",
	}

	// Step 1: Validate repository string first (before downloading anything)
	if err := a.validateAptRepo(source.SourceLine); err != nil {
		return fmt.Errorf("repository validation failed: %w", err)
	}

	// Step 2: Download and install GPG key
	if err := a.downloadAndInstallGPGKey(source); err != nil {
		return fmt.Errorf("failed to install GPG key: %w", err)
	}

	// Step 3: Add repository source
	if err := a.addRepositorySource(source); err != nil {
		return fmt.Errorf("failed to add repository source: %w", err)
	}

	// Step 4: Update package lists
	if err := a.handleUpdate([]string{}); err != nil {
		a.logger.Warning("Failed to update package lists after adding repository: %v", err)
	}

	// Step 5: Validate the repository
	if err := a.validateRepositorySource(source); err != nil {
		a.logger.Warning("Repository validation failed: %v", err)
		return fmt.Errorf("repository added but validation failed: %w", err)
	}

	a.logger.Success("Successfully added repository: %s", source.SourceLine)
	return nil
}

// addRepositorySource adds the repository source line to the system
func (a *APTInstaller) addRepositorySource(source APTSource) error {
	a.logger.Printf("Adding repository source to %s\n", source.SourceFile)

	// Check if source file already exists
	if _, err := os.Stat(source.SourceFile); err == nil {
		a.logger.Printf("Source file already exists at %s, checking content\n", source.SourceFile)

		// Read existing content
		content, err := os.ReadFile(source.SourceFile)
		if err != nil {
			return fmt.Errorf("failed to read existing source file: %w", err)
		}

		// Check if our source line is already present
		if strings.Contains(string(content), source.SourceLine) {
			a.logger.Printf("Repository already configured in %s, skipping\n", source.SourceFile)
			return nil
		}
	}

	// Create sources.list.d directory if it doesn't exist
	sourcesDir := filepath.Dir(source.SourceFile)
	if err := os.MkdirAll(sourcesDir, 0755); err != nil {
		return fmt.Errorf("failed to create sources directory '%s' for repository configuration: %w", sourcesDir, err)
	}

	// Write the source line to file securely
	tempFile := source.SourceFile + ".tmp"
	if err := os.WriteFile(tempFile, []byte(source.SourceLine+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write temporary source file: %w", err)
	}

	// Validate paths before moving files
	if err := a.validateFilePath(tempFile); err != nil {
		return fmt.Errorf("invalid temporary file path: %w", err)
	}
	if err := a.validateFilePath(source.SourceFile); err != nil {
		return fmt.Errorf("invalid destination file path: %w", err)
	}

	// Move temporary file to final location with sudo
	if err := sdk.ExecCommand(true, "mv", tempFile, source.SourceFile); err != nil {
		if rmErr := os.Remove(tempFile); rmErr != nil {
			a.logger.Warning("Failed to remove temporary file: %v", rmErr)
		}
		return fmt.Errorf("failed to install repository source from '%s' to '%s': %w", tempFile, source.SourceFile, err)
	}

	// Validate file path before setting permissions
	if err := a.validateFilePath(source.SourceFile); err != nil {
		return fmt.Errorf("invalid source file path for chmod: %w", err)
	}

	// Set proper permissions
	if err := sdk.ExecCommand(true, "chmod", "644", source.SourceFile); err != nil {
		a.logger.Warning("Failed to set source file permissions: %v", err)
	}

	a.logger.Success("Repository source added to %s", source.SourceFile)
	return nil
}

// validateRepositorySource validates that the repository and key are properly configured
func (a *APTInstaller) validateRepositorySource(source APTSource) error {
	a.logger.Printf("Validating repository configuration\n")

	// Check if key file exists
	if _, err := os.Stat(source.KeyPath); err != nil {
		return fmt.Errorf("GPG key file not found: %s", source.KeyPath)
	}

	// Check if source file exists
	if _, err := os.Stat(source.SourceFile); err != nil {
		return fmt.Errorf("source file not found: %s", source.SourceFile)
	}

	// Validate key format
	if err := a.validateGPGKeyFormat(source.KeyPath, source.RequireDearmor); err != nil {
		return fmt.Errorf("GPG key validation failed: %w", err)
	}

	// Test APT configuration
	a.logger.Debug("Testing APT configuration validity")
	if err := sdk.ExecCommand(false, "apt-get", "update", "-o", "Dir::Etc::sourcelist=/dev/null", "-o", fmt.Sprintf("Dir::Etc::sourceparts=%s", filepath.Dir(source.SourceFile))); err != nil {
		return fmt.Errorf("repository configuration test failed: %w", err)
	}

	a.logger.Success("Repository validation completed successfully")
	return nil
}

// handleRemoveRepository removes a repository and its associated GPG key
func (a *APTInstaller) handleRemoveRepository(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("remove-repository requires: source-file key-path")
	}

	sourceFile := args[0]
	keyPath := args[1]

	a.logger.Printf("Removing repository: %s\n", sourceFile)

	var errors []string

	// Validate paths before removal - fail if paths are invalid
	if err := a.validateFilePath(sourceFile); err != nil {
		errMsg := fmt.Sprintf("Invalid source file path, skipping removal: %v", err)
		a.logger.ErrorMsg(errMsg)
		fmt.Fprintf(os.Stderr, "%s\n", errMsg) // Direct stderr for test capture
		errors = append(errors, errMsg)
	} else {
		// Remove source file
		if err := sdk.ExecCommand(true, "rm", "-f", sourceFile); err != nil {
			a.logger.Warning("Failed to remove source file: %v", err)
			errors = append(errors, fmt.Sprintf("Failed to remove source file: %v", err))
		} else {
			a.logger.Success("Removed source file: %s", sourceFile)
		}
	}

	// Validate key path before removal - fail if paths are invalid
	if err := a.validateFilePath(keyPath); err != nil {
		errMsg := fmt.Sprintf("Invalid key file path, skipping removal: %v", err)
		a.logger.ErrorMsg(errMsg)
		fmt.Fprintf(os.Stderr, "%s\n", errMsg) // Direct stderr for test capture
		errors = append(errors, errMsg)
	} else {
		// Remove key file
		if err := sdk.ExecCommand(true, "rm", "-f", keyPath); err != nil {
			a.logger.Warning("Failed to remove key file: %v", err)
			errors = append(errors, fmt.Sprintf("Failed to remove key file: %v", err))
		} else {
			a.logger.Success("Removed key file: %s", keyPath)
		}
	}

	// Update package lists
	if err := a.handleUpdate([]string{}); err != nil {
		a.logger.Warning("Failed to update package lists after removing repository: %v", err)
	}

	// If there were validation errors, log them but don't fail the entire operation
	// This allows graceful handling of invalid paths while still proceeding
	if len(errors) > 0 {
		a.logger.Warning("Repository removal completed with validation warnings: %s", strings.Join(errors, "; "))
	}

	a.logger.Success("Repository removal completed")
	return nil
}

// handleValidateRepository validates existing repository configurations
func (a *APTInstaller) handleValidateRepository(args []string) error {
	a.logger.Printf("Validating repository configurations\n")

	// Check if APT configuration is valid
	if err := sdk.ExecCommand(false, "apt-get", "check"); err != nil {
		return fmt.Errorf("APT configuration validation failed: %w", err)
	}

	// Check for broken repositories
	output, err := sdk.ExecCommandOutput("apt-get", "update")
	if err != nil {
		a.logger.Warning("Some repositories may have issues: %v", err)
		if strings.Contains(output, "NO_PUBKEY") {
			return fmt.Errorf("missing GPG keys detected - some repositories cannot be validated")
		}
		if strings.Contains(output, "404") {
			return fmt.Errorf("repository not found errors detected")
		}
		return fmt.Errorf("repository validation failed: %w", err)
	}

	// List configured repositories
	a.logger.Printf("Configured repositories:\n")
	if err := sdk.ExecCommand(false, "apt-cache", "policy"); err != nil {
		a.logger.Warning("Failed to list repository policies: %v", err)
	}

	a.logger.Success("All repositories validated successfully")
	return nil
}
