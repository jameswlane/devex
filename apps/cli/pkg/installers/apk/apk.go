package apk

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// ApkInstaller implements the Installer interface for Alpine Package Keeper (apk)
type ApkInstaller struct{}

// NewApkInstaller creates a new ApkInstaller instance
func NewApkInstaller() *ApkInstaller {
	return &ApkInstaller{}
}

// Install installs packages using apk (Alpine Package Keeper)
func (a *ApkInstaller) Install(command string, repo types.Repository) error {
	log.Info("APK installer not yet implemented")
	log.Info("Would run: apk add %s", command)
	return fmt.Errorf("apk installer not yet implemented")
}
