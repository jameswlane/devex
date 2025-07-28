package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
	"github.com/spf13/cobra"
)

func init() {
	Register(NewShellCmd)
}

// NewShellCmd creates the shell command
func NewShellCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	return &cobra.Command{
		Use:   "shell [shell-name]",
		Short: "Switch your default shell",
		Long: `Switch your default shell interactively with password prompt.

This command changes your system's default shell using the 'chsh' command.
You'll be prompted for your password for security verification.

Supported shells:
  • bash - Bourne Again Shell
  • zsh - Z Shell (recommended)
  • fish - Friendly Interactive Shell

Examples:
  devex shell zsh      # Switch to zsh
  devex shell bash     # Switch to bash
  devex shell fish     # Switch to fish`,
		Args: cobra.ExactArgs(1),
		Run:  runShellSwitch,
	}
}

func runShellSwitch(cmd *cobra.Command, args []string) {
	targetShell := args[0]

	log.Info("Starting shell switch process", "targetShell", targetShell)

	// Validate shell name
	supportedShells := []string{"bash", "zsh", "fish"}
	isSupported := false
	for _, shell := range supportedShells {
		if shell == targetShell {
			isSupported = true
			break
		}
	}

	if !isSupported {
		log.Error("Unsupported shell", fmt.Errorf("shell %s is not supported", targetShell), "shell", targetShell, "supported", strings.Join(supportedShells, ", "))
		fmt.Printf("Error: '%s' is not a supported shell.\n", targetShell)
		fmt.Printf("Supported shells: %s\n", strings.Join(supportedShells, ", "))
		os.Exit(1)
	}

	// Check if shell is available
	shellPath, err := exec.LookPath(targetShell)
	if err != nil {
		log.Error("Shell not found on system", err, "shell", targetShell)
		fmt.Printf("Error: '%s' is not installed on your system.\n", targetShell)
		fmt.Printf("Please install it first using your package manager.\n")
		fmt.Printf("For example: sudo apt install %s\n", targetShell)
		os.Exit(1)
	}

	// Get current user
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		log.Error("Cannot determine current user", fmt.Errorf("USER environment variable not set"))
		fmt.Printf("Error: Cannot determine current user.\n")
		os.Exit(1)
	}

	// Check current shell
	currentShell, err := utils.GetUserShell(currentUser)
	switch {
	case err != nil:
		log.Warn("Could not detect current shell", "error", err, "user", currentUser)
		fmt.Printf("Warning: Could not detect your current shell.\n")
	case currentShell == shellPath:
		log.Info("Shell already set", "shell", targetShell, "path", shellPath, "user", currentUser)
		fmt.Printf("✓ Your shell is already set to %s (%s)\n", targetShell, shellPath)
		return
	default:
		log.Info("Current shell differs from target", "current", currentShell, "target", shellPath, "user", currentUser)
		fmt.Printf("Current shell: %s\n", currentShell)
		fmt.Printf("Target shell:  %s\n", shellPath)
		fmt.Printf("\n")
	}

	// Inform user about the process
	fmt.Printf("🔐 Switching to %s requires your password for security verification.\n", targetShell)
	fmt.Printf("This uses the system 'chsh' command to change your default shell.\n")
	fmt.Printf("\n")

	// Use chsh interactively so user can enter password
	ctx := context.Background()
	chshCmd := exec.CommandContext(ctx, "chsh", "-s", shellPath)

	// Connect stdin/stdout/stderr so user can interact with password prompt
	chshCmd.Stdin = os.Stdin
	chshCmd.Stdout = os.Stdout
	chshCmd.Stderr = os.Stderr

	log.Info("Executing interactive shell change", "shell", targetShell, "path", shellPath, "user", currentUser)

	err = chshCmd.Run()
	if err != nil {
		log.Error("Failed to change shell", err, "shell", targetShell, "path", shellPath, "user", currentUser)
		fmt.Printf("\n❌ Failed to change shell: %v\n", err)
		fmt.Printf("Please ensure you entered the correct password.\n")
		os.Exit(1)
	}

	log.Info("Shell changed successfully", "shell", targetShell, "path", shellPath, "user", currentUser)
	fmt.Printf("\n✅ Successfully changed shell to %s!\n", targetShell)
	fmt.Printf("Please log out and log back in (or restart your terminal) for the change to take effect.\n")
}
