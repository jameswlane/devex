package main

import (
	"context"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// MisePlugin implements Mise development tool version manager functionality as a plugin
type MisePlugin struct {
	*sdk.PackageManagerPlugin
	logger sdk.Logger
}

// InitLogger sets the logger for the plugin
func (m *MisePlugin) InitLogger(logger sdk.Logger) {
	m.logger = logger
}

// Execute handles command execution
func (m *MisePlugin) Execute(command string, args []string) error {
	ctx := context.Background()

	// Ensure Mise is available
	m.EnsureAvailable()

	switch command {
	case "install":
		return m.HandleInstall(ctx, args)
	case "remove":
		return m.HandleRemove(ctx, args)
	case "update":
		return m.HandleUpdate(ctx, args)
	case "search":
		return m.HandleSearch(ctx, args)
	case "list":
		return m.HandleList(ctx, args)
	case "ensure-installed":
		return m.HandleEnsureInstalled(ctx, args)
	case "is-installed":
		return m.HandleIsInstalled(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}
