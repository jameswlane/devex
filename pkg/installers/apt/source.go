package apt

import (
	"fmt"
	"os"
	"os/exec"
)

// AddAptSource manages the entire process of adding an APT source, including GPG key handling
func AddAptSource(keySource string, keyName string, sourceRepo string, sourceName string) error {
	// Download and add GPG key
	if keySource != "" {
		keyPath := fmt.Sprintf("/etc/apt/trusted.gpg.d/%s.gpg", keyName)
		if err := DownloadGPGKey(keySource, keyPath); err != nil {
			return fmt.Errorf("failed to download GPG key: %v", err)
		}
		if err := AddGPGKeyToKeyring(keyPath); err != nil {
			return fmt.Errorf("failed to add GPG key to keyring: %v", err)
		}
	}

	// Write the repository definition to the source file
	sourcePath := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", sourceName)
	if err := os.WriteFile(sourcePath, []byte(sourceRepo), 0o644); err != nil {
		return fmt.Errorf("failed to write APT source to %s: %v", sourcePath, err)
	}

	// Update APT sources
	cmd := exec.Command("apt-get", "update")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update APT sources: %v - %s", err, string(output))
	}

	return nil
}
