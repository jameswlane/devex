package main

import (
	"context"

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
