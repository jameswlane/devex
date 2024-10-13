package installers

import (
	"github.com/jameswlane/devex/pkg/installers/deb"
	"os/exec"
	"testing"
)

func TestInstallViaApt(t *testing.T) {
	app := App{
		Name:           "fzf",
		InstallMethod:  "apt",
		InstallCommand: "fzf",
		Dependencies:   []string{"apt"},
	}

	// Mock exec.Command to avoid real installation
	deb.execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "mocked command")
	}

	err := installViaApt(app)
	if err != nil {
		t.Errorf("Failed to install app via apt: %v", err)
	}
}
