package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Execute handles command execution
func (p *ShellPlugin) Execute(command string, args []string) error {
	ctx := context.Background()

	switch command {
	case "setup":
		return p.handleSetup(ctx, args)
	case "switch":
		return p.handleSwitch(ctx, args)
	case "config":
		return p.handleConfig(ctx, args)
	case "backup":
		return p.handleBackup(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// DetectCurrentShell detects the current shell from environment variables
func (p *ShellPlugin) DetectCurrentShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}

	// Extract shell name from path
	parts := strings.Split(shell, "/")
	return parts[len(parts)-1]
}
