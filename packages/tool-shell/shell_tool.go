package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// ShellPlugin implements the Shell configuration plugin
type ShellPlugin struct {
	*sdk.BasePlugin
}

// Execute handles command execution
func (p *ShellPlugin) Execute(command string, args []string) error {
	switch command {
	case "setup":
		return p.handleSetup(args)
	case "switch":
		return p.handleSwitch(args)
	case "config":
		return p.handleConfig(args)
	case "backup":
		return p.handleBackup(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// detectCurrentShell detects the current shell from environment variables
func (p *ShellPlugin) detectCurrentShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}

	// Extract shell name from path
	parts := strings.Split(shell, "/")
	return parts[len(parts)-1]
}