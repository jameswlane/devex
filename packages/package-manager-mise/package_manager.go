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

// Execute handles command execution
func (m *MisePlugin) Execute(command string, args []string) error {
	// Ensure Mise is available
	m.EnsureAvailable()

	switch command {
	case "install":
		return m.handleInstall(args)
	case "remove":
		return m.handleRemove(args)
	case "update":
		return m.handleUpdate(args)
	case "search":
		return m.handleSearch(args)
	case "list":
		return m.handleList(args)
	case "ensure-installed":
		return m.handleEnsureInstalled(args)
	case "is-installed":
		return m.handleIsInstalled(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}
