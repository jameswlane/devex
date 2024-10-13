package mise

import (
	"fmt"
	"log"
	"os/exec"
)

// Package-level variable for exec.Command to allow mocking in Mise
var miseExecCommand = exec.Command

// InstallMiseLanguage installs a programming language via Mise
func Install(language string, dryRun bool) error {
	cmd := miseExecCommand("mise", "use", "--global", language)
	if dryRun {
		log.Printf("[Dry Run] Would run command: %s", cmd.String())
		return nil
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install language via Mise: %v - %s", err, string(output))
	}
	return nil
}

// RunPostInstallCommand runs any post-install commands defined in the YAML
func RunPostInstallCommand(command string) error {
	cmd := miseExecCommand("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run post-install command: %v - %s", err, string(output))
	}
	return nil
}
