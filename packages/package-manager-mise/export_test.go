package main

import (
	"context"
	"debug/elf"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// InstallMiseFrom exposes the verified installer flow to tests.
func InstallMiseFrom(m *MisePlugin, ctx context.Context, scriptURL string, verification sdk.ArtifactVerification) error {
	return m.installMiseFrom(ctx, scriptURL, verification)
}

// ConfigureBashIntegration exposes the .bashrc setup to tests.
func ConfigureBashIntegration(m *MisePlugin, homeDir string) error {
	return m.configureBashIntegration(homeDir)
}

// ConfigureZshIntegration exposes the .zshrc setup to tests.
func ConfigureZshIntegration(m *MisePlugin, homeDir string) error {
	return m.configureZshIntegration(homeDir)
}

// ConfigureFishIntegration exposes the config.fish setup to tests.
func ConfigureFishIntegration(m *MisePlugin, homeDir string) error {
	return m.configureFishIntegration(homeDir)
}

// SetTargetOS overrides the platform for Linux-only behavior and returns a restore func.
func SetTargetOS(goos string) func() {
	previous := targetOS
	targetOS = goos
	return func() { targetOS = previous }
}

// SetLibatomicPatterns overrides where libatomic.so.1 is searched for.
func SetLibatomicPatterns(patterns []string) func() {
	previous := libatomicPatterns
	libatomicPatterns = patterns
	return func() { libatomicPatterns = previous }
}

// SetLdconfigPaths overrides the loader-cache tools that are queried.
func SetLdconfigPaths(paths []string) func() {
	previous := ldconfigPaths
	ldconfigPaths = paths
	return func() { ldconfigPaths = previous }
}

// SetNativeELFMachine overrides the architecture libatomic must be built for.
func SetNativeELFMachine(machine elf.Machine) func() {
	previous := nativeELFMachine
	nativeELFMachine = machine
	return func() { nativeELFMachine = previous }
}
