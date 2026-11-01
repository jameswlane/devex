package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/state"
)

// TestMain isolates the whole test binary, including plain `go test` functions
// that don't run the Ginkgo suite hooks: installs record audit events, and the
// state path otherwise resolves to the real user's home.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "devex-tui-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	_ = os.Setenv("DEVEX_STATE_DIR", "")
	viper.Set(state.DBPathConfigKey, filepath.Join(dir, "state.db"))
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
