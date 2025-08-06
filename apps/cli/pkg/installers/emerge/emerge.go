package emerge

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// EmergeInstaller implements the Installer interface for Portage (Gentoo)
type EmergeInstaller struct{}

// NewEmergeInstaller creates a new EmergeInstaller instance
func NewEmergeInstaller() *EmergeInstaller {
	return &EmergeInstaller{}
}

// Install installs packages using emerge
func (e *EmergeInstaller) Install(command string, repo types.Repository) error {
	log.Info("Emerge installer not yet implemented")
	log.Info("Would run: emerge %s", command)
	return fmt.Errorf("emerge installer not yet implemented")
}
