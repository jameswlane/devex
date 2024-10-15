package trufflehogsetup

import (
	"fmt"
	"os"
	"os/exec"
)

// RunTruffleHogInDocker runs TruffleHog using Docker
func RunTruffleHogInDocker() error {
	cmd := exec.Command("docker", "run", "--rm", "-v", "$(pwd):/workdir", "-i", "trufflesecurity/trufflehog:latest", "git", "file:///workdir", "--since-commit", "HEAD", "--only-verified", "--fail")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run TruffleHog via Docker: %v", err)
	}
	fmt.Println("TruffleHog run successfully via Docker")
	return nil
}
