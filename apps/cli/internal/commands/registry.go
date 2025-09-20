package commands

import (
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var registry []func(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command

// Register adds a new command to the registry.
func Register(cmdFunc func(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command) {
	registry = append(registry, cmdFunc)
}

// LoadCommands initializes and returns all registered commands.
func LoadCommands(repo types.Repository, settings config.CrossPlatformSettings) []*cobra.Command {
	cmds := make([]*cobra.Command, 0, len(registry))
	for _, cmdFunc := range registry {
		cmds = append(cmds, cmdFunc(repo, settings)) // Pass repo and settings to each command
	}
	return cmds
}
