package commands

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Exported for the external commands_test package.
var (
	ValidateCleanupPath     = validateCleanupPath
	RemoveFileIfExists      = removeFileIfExists
	ValidateEnvironmentName = validateEnvironmentName
	ResolveUninstallApp     = resolveUninstallApp
	StatusErrorSummary      = statusErrorSummary
)

func SetUninstallInstallerForTest(getter func(context.Context, string) types.BaseInstaller) func() {
	previous := getUninstallInstaller
	getUninstallInstaller = getter
	return func() { getUninstallInstaller = previous }
}

func SetOrphanRemovalForTest(remover func(context.Context)) func() {
	previous := removeOrphanedPackages
	removeOrphanedPackages = remover
	return func() { removeOrphanedPackages = previous }
}

func SetDockerDataRemovalForTest(remover func(context.Context) error) func() {
	previous := removeDockerDataFiles
	removeDockerDataFiles = remover
	return func() { removeDockerDataFiles = previous }
}
