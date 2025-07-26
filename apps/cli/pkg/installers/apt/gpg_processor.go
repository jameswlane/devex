package apt

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/utils"
)

// ProcessGPGKey dearmors and saves a GPG key to the desired location.
func ProcessGPGKey(tempFile, destination string) error {
	log.Info("Processing GPG key", "tempFile", tempFile, "destination", destination)

	// Execute the gpg --dearmor command
	command := fmt.Sprintf("gpg --dearmor -o %s %s", destination, tempFile)
	if _, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to dearmor GPG key", err, "command", command)
		return fmt.Errorf("failed to dearmor GPG key: %w", err)
	}

	// Remove the temporary file
	if err := fs.Remove(tempFile); err != nil {
		log.Warn("Failed to remove temporary file after dearmor", err, "tempFile", tempFile)
	} else {
		log.Info("Temporary file removed successfully", "tempFile", tempFile)
	}

	log.Info("GPG key processed successfully", "destination", destination)
	return nil
}
