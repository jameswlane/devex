package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleCreateVenv creates a Python virtual environment
func (p *PipPlugin) handleCreateVenv(ctx context.Context, args []string) error {
	venvName := "venv"

	// Process arguments for custom venv name
	for i, arg := range args {
		if arg == "--name" && i+1 < len(args) {
			if err := p.validateVenvName(args[i+1]); err != nil {
				return fmt.Errorf("invalid virtual environment name: %w", err)
			}
			venvName = args[i+1]
			break
		}
	}

	// Validate the venv name
	if err := p.validateVenvName(venvName); err != nil {
		return fmt.Errorf("invalid virtual environment name: %w", err)
	}

	p.logger.Printf("Creating virtual environment: %s\n", venvName)

	// Check if venv already exists
	if _, err := os.Stat(venvName); err == nil {
		return fmt.Errorf("virtual environment '%s' already exists", venvName)
	}

	// Create virtual environment
	if err := sdk.ExecCommandWithContext(ctx, true, "python3", "-m", "venv", venvName); err != nil {
		// Try python if python3 is not available
		if err2 := sdk.ExecCommandWithContext(ctx, true, "python", "-m", "venv", venvName); err2 != nil {
			return fmt.Errorf("failed to create virtual environment: %w (also tried python: %v)", err, err2)
		}
	}

	p.logger.Success("Virtual environment '%s' created successfully", venvName)
	p.logger.Printf("To activate: source %s/bin/activate (Linux/Mac) or %s\\Scripts\\activate (Windows)\n", venvName, venvName)
	return nil
}

// isVirtualEnvActive checks if a virtual environment is currently active
func (p *PipPlugin) isVirtualEnvActive(ctx context.Context) bool {
	// Check VIRTUAL_ENV environment variable
	if os.Getenv("VIRTUAL_ENV") != "" {
		return true
	}

	// Check CONDA_DEFAULT_ENV for conda environments
	if os.Getenv("CONDA_DEFAULT_ENV") != "" {
		return true
	}

	// Check if pip executable is in a virtual environment path
	if output, err := sdk.ExecCommandOutputWithContext(ctx, "which", "pip"); err == nil {
		if strings.Contains(output, "venv") || strings.Contains(output, "virtualenv") || strings.Contains(output, "conda") {
			return true
		}
	}

	return false
}
