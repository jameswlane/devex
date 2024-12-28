package apt

import (
	"fmt"
	"os"
	"os/exec"
)

// ProcessGPGKey dearmors and saves a GPG key to the desired location.
func ProcessGPGKey(tempFile, destination string) error {
	cmd := exec.Command("gpg", "--dearmor", "-o", destination, tempFile)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to dearmor GPG key: %v", err)
	}
	os.Remove(tempFile)
	return nil
}
