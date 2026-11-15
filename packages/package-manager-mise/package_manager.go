package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

	// The mise.run installer puts mise in ~/.local/bin, which a fresh login
	// session often lacks on PATH. Find it there so the steps after
	// ensure-installed can run in the same DevEx invocation.
	addUserBinToPath()

	// ensure-installed must run before mise exists.
	if command == "ensure-installed" {
		return m.HandleEnsureInstalled(ctx, args)
	}

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
	case "is-installed":
		return m.HandleIsInstalled(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// addUserBinToPath prepends ~/.local/bin to PATH when mise is installed there
// but is not already reachable on PATH.
func addUserBinToPath() {
	if sdk.CommandExists("mise") {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	userBin := filepath.Join(home, ".local", "bin")
	if info, err := os.Stat(filepath.Join(userBin, "mise")); err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return
	}
	_ = os.Setenv("PATH", userBin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
