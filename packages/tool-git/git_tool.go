package main

import (
	"context"
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

	ctx := context.Background()

	switch command {
	case "config":
		return p.HandleConfig(ctx, args)
	case "aliases":
		return p.HandleAliases(ctx, args)
	case "status":
		return p.HandleStatus(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}
