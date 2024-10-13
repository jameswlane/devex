package brew

import (
	"fmt"
	"log"
	"os/exec"
)

var brewExecCommand = exec.Command

func Install(packageName string, dryRun bool) error {
	cmd := brewExecCommand("brew", "install", packageName)
	if dryRun {
		log.Printf("[Dry Run] Would run command: %s", cmd.String())
		return nil
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install %s: %v - %s", packageName, err, string(output))
	}
	return nil
}
