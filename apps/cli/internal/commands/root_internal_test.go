package commands

import (
	"os"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/spf13/viper"
)

func TestInitializeConfigBindsPluginFlagsToEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	t.Setenv("DEVEX_SKIP_PLUGIN_DOWNLOAD", "true")
	t.Setenv("DEVEX_OFFLINE", "true")

	cmd := NewRootCmd("test", &mocks.MockRepository{}, config.CrossPlatformSettings{})
	if err := initializeConfig(cmd); err != nil {
		t.Fatalf("initializeConfig() error = %v", err)
	}

	if !viper.GetBool("skip-plugin-download") {
		t.Fatal("DEVEX_SKIP_PLUGIN_DOWNLOAD did not set skip-plugin-download")
	}
	if !viper.GetBool("offline") {
		t.Fatal("DEVEX_OFFLINE did not set offline")
	}
}

func TestIsRunningInTestUsesExactTestMarkers(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	t.Setenv("GO_TEST", "")
	t.Setenv("TESTING", "")

	os.Args = []string{"/usr/local/bin/devex", "--interest-level=high"}
	if isRunningInTest() {
		t.Fatal("isRunningInTest() returned true for a non-test substring")
	}

	os.Args = []string{"/tmp/devex.test"}
	if !isRunningInTest() {
		t.Fatal("isRunningInTest() returned false for a Go test binary")
	}
}
