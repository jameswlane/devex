package trufflehogsetup

import (
	"fmt"
	"os"
	"os/exec"
)

// CreatePreCommitConfig generates the .pre-commit-config.yaml file
func CreatePreCommitConfig(useDocker bool) error {
	var preCommitConfig string
	if useDocker {
		preCommitConfig = `
repos:
  - repo: local
    hooks:
      - id: trufflehog
        name: TruffleHog
        description: Detect secrets in your data.
        entry: bash -c 'docker run --rm -v "$(pwd):/workdir" -i trufflesecurity/trufflehog:latest git file:///workdir --since-commit HEAD --only-verified --fail'
        language: system
        stages: ["commit", "push"]
`
	} else {
		preCommitConfig = `
repos:
  - repo: local
    hooks:
      - id: trufflehog
        name: TruffleHog
        description: Detect secrets in your data.
        entry: bash -c 'trufflehog git file://. --since-commit HEAD --only-verified --fail'
        language: system
        stages: ["commit", "push"]
`
	}

	// Write the config to .pre-commit-config.yaml
	err := os.WriteFile(".pre-commit-config.yaml", []byte(preCommitConfig), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write .pre-commit-config.yaml: %v", err)
	}

	fmt.Println(".pre-commit-config.yaml created successfully")
	return nil
}

// InstallPreCommitHook runs pre-commit install
func InstallPreCommitHook() error {
	cmd := exec.Command("pre-commit", "install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install pre-commit hook: %v", err)
	}

	fmt.Println("Pre-commit hook installed successfully")
	return nil
}
