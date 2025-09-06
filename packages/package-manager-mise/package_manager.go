package main

import (
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
	// Ensure Mise is available
	m.EnsureAvailable()

	switch command {
	case "install":
		return m.HandleInstall(args)
	case "remove":
		return m.HandleRemove(args)
	case "update":
		return m.HandleUpdate(args)
	case "search":
		return m.HandleSearch(args)
	case "list":
		return m.HandleList(args)
	case "ensure-installed":
		return m.HandleEnsureInstalled(args)
	case "is-installed":
		return m.HandleIsInstalled(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}
