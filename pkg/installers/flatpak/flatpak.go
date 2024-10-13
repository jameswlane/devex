package flatpak

import (
	"fmt"
	"log"
	"os/exec"
)

var flatpakExecCommand = exec.Command

func Install(appID, repo string, dryRun bool) error {
	cmd := flatpakExecCommand("flatpak", "install", repo, appID, "-y")
	if dryRun {
		log.Printf("[Dry Run] Would run command: %s", cmd.String())
		return nil
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install Flatpak app: %v - %s", err, string(output))
	}
	return nil
}
