package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleSwitch changes the default shell to the specified target shell
func (p *ShellPlugin) handleSwitch(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("shell switch requires a target shell (bash, zsh, fish)")
	}

	targetShell := args[0]
	fmt.Printf("Switching to %s shell...\n", targetShell)

	// Validate target shell
	if err := p.ValidateShell(targetShell); err != nil {
		return err
	}

	// Check if the target shell is installed
	shellPath, err := p.getShellPath(targetShell)
	if err != nil {
		return err
	}

	// Change default shell
	if err := p.changeDefaultShell(ctx, shellPath); err != nil {
		return err
	}

	fmt.Printf("Successfully switched to %s shell\n", targetShell)
	fmt.Println("Please log out and log back in for the change to take effect")

	return nil
}

// ValidateShell validates that the target shell is supported
func (p *ShellPlugin) ValidateShell(shell string) error {
	validShells := []string{"bash", "zsh", "fish"}
	for _, validShell := range validShells {
		if shell == validShell {
			return nil
		}
	}
	return fmt.Errorf("invalid shell: %s. Valid options are: %s", shell, strings.Join(validShells, ", "))
}

// getShellPath finds the installation path of the specified shell
func (p *ShellPlugin) getShellPath(shell string) (string, error) {
	ctx := context.Background()
	shellPath, err := sdk.ExecCommandOutputWithContext(ctx, "which", shell)
	if err != nil {
		return "", fmt.Errorf("shell %s is not installed", shell)
	}
	return strings.TrimSpace(shellPath), nil
}

// changeDefaultShell changes the user's default shell using chsh
func (p *ShellPlugin) changeDefaultShell(ctx context.Context, shellPath string) error {
	fmt.Printf("Changing default shell to %s...\n", shellPath)
	if err := sdk.ExecCommandWithContext(ctx, true, "chsh", "-s", shellPath); err != nil {
		return fmt.Errorf("failed to change shell: %w", err)
	}
	return nil
}
