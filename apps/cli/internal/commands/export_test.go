package commands

import (
	"context"
	"os"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Exported for the external commands_test package.
var (
	ValidateCleanupPath     = validateCleanupPath
	ValidateAptManagedPath  = validateAptManagedPath
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

func SetAptManagedFileHooksForTest(
	reader func(string) ([]byte, error),
	statter func(string) (os.FileInfo, error),
	remover func(context.Context, string) error,
) func() {
	prevReader := readAptManagedFile
	prevStatter := statAptManagedFile
	prevRemover := removeAptManagedFile
	if reader != nil {
		readAptManagedFile = reader
	}
	if statter != nil {
		statAptManagedFile = statter
	}
	if remover != nil {
		removeAptManagedFile = remover
	}
	return func() {
		readAptManagedFile = prevReader
		statAptManagedFile = prevStatter
		removeAptManagedFile = prevRemover
	}
}
