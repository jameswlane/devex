package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleFreeze generates a requirements.txt file from installed packages
func (p *PipPlugin) handleFreeze(ctx context.Context, args []string) error {
	p.logger.Printf("Generating requirements.txt...\n")

	// Generate requirements and save to file
	output, err := sdk.ExecCommandOutputWithContext(ctx, "pip", "freeze")
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
	defer func() {
		if err := file.Close(); err != nil {
			p.logger.Warning("Failed to close file: %v", err)
		}
	}()

	if _, err := file.WriteString(output); err != nil {
		return fmt.Errorf("failed to write requirements: %w", err)
	}

	p.logger.Success("Requirements saved to %s", filePath)
	return nil
}

// installFromRequirements installs packages from a requirements.txt file
func (p *PipPlugin) installFromRequirements(ctx context.Context, files []string) error {
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
	return sdk.ExecCommandWithContext(ctx, true, "pip", "install", "-r", requirementsFile)
}
