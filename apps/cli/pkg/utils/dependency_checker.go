package utils

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/jameswlane/devex/pkg/log"
)

type Dependency struct {
	Name    string
	Command string
}

// CheckDependencies verifies the availability of required dependencies.
func CheckDependencies(dependencies []Dependency) error {
	ctx := context.Background()
	for _, dep := range dependencies {
		if err := exec.CommandContext(ctx, "which", dep.Command).Run(); err != nil {
			log.Error("Missing dependency", err, "name", dep.Name, "command", dep.Command)
			return fmt.Errorf("missing dependency: %s (command: %s)", dep.Name, dep.Command)
		}
		log.Info("Dependency available", "name", dep.Name, "command", dep.Command)
	}
	return nil
}
