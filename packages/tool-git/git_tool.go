package main

import (
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// GitPlugin implements the Git configuration plugin
type GitPlugin struct {
	*sdk.BasePlugin
}

// Execute handles command execution
func (p *GitPlugin) Execute(command string, args []string) error {
	// Ensure git is available
	if !sdk.CommandExists("git") {
		return fmt.Errorf("git is not installed on this system")
	}

	switch command {
	case "config":
		return p.handleConfig(args)
	case "aliases":
		return p.handleAliases(args)
	case "status":
		return p.handleStatus(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}