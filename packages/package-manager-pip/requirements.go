package main

import (
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleFreeze generates a requirements.txt file from installed packages
func (p *PipPlugin) handleFreeze(args []string) error {
	p.logger.Printf("Generating requirements.txt...\n")
	
	// Generate requirements and save to file
	output, err := sdk.ExecCommandOutput("pip", "freeze")
	if err != nil {
		return fmt.Errorf("failed to generate requirements: %w", err)
	}
	
	// Write to requirements.txt
	filePath := "requirements.txt"
	if len(args) > 0 {
		if err := p.validateFilePath(args[0]); err != nil {
			return fmt.Errorf("invalid file path: %w", err)
		}
		filePath = args[0]
	}
	
	file, err := os.Create(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("failed to create requirements file: %w", err)
	}
	defer file.Close()
	
	if _, err := file.WriteString(output); err != nil {
		return fmt.Errorf("failed to write requirements: %w", err)
	}
	
	p.logger.Success("Requirements saved to %s", filePath)
	return nil
}

// installFromRequirements installs packages from a requirements.txt file
func (p *PipPlugin) installFromRequirements(files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("no requirements file specified")
	}
	
	requirementsFile := files[0]
	if err := p.validateFilePath(requirementsFile); err != nil {
		return fmt.Errorf("invalid requirements file path: %w", err)
	}
	
	if _, err := os.Stat(requirementsFile); os.IsNotExist(err) {
		return fmt.Errorf("requirements file not found: %s", requirementsFile)
	}
	
	p.logger.Printf("Installing from %s...\n", requirementsFile)
	return sdk.ExecCommand(true, "pip", "install", "-r", requirementsFile)
}

// findRequirementsFiles searches for requirements files in common locations
func (p *PipPlugin) findRequirementsFiles() []string {
	var files []string
	
	commonNames := []string{
		"requirements.txt",
		"requirements-dev.txt",
		"requirements-test.txt",
		"dev-requirements.txt",
		"test-requirements.txt",
	}
	
	for _, name := range commonNames {
		if _, err := os.Stat(name); err == nil {
			files = append(files, name)
		}
	}
	
	return files
}

// validateRequirementsFile validates the format of a requirements file
func (p *PipPlugin) validateRequirementsFile(filePath string) error {
	if err := p.validateFilePath(filePath); err != nil {
		return err
	}
	
	// Check if file exists and is readable
	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("cannot read requirements file: %w", err)
	}
	defer file.Close()
	
	// Basic validation - check file size
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("cannot get file info: %w", err)
	}
	
	// Check if file is too large (> 1MB)
	const maxFileSize = 1024 * 1024
	if stat.Size() > maxFileSize {
		return fmt.Errorf("requirements file is too large (max 1MB)")
	}
	
	return nil
}