package utils

import (
	"fmt"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// InstallCommand represents a single installation command.
type InstallCommand struct {
	Shell string // The shell command to execute
}

// ProcessInstallCommands processes a list of installation commands concurrently.
func ProcessInstallCommands(commands []InstallCommand) error {
	log.Info("Starting installation commands processing", "totalCommands", len(commands))

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for _, cmd := range commands {
		wg.Add(1)

		go func(command InstallCommand) {
			defer wg.Done()

			log.Info("Executing install command", "command", command.Shell)
			_, err := CommandExec.RunShellCommand(command.Shell)
			if err != nil {
				log.Error("Failed to execute install command", err, "command", command.Shell)
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to execute command '%s': %w", command.Shell, err))
				mu.Unlock()
				return
			}

			log.Info("Install command executed successfully", "command", command.Shell)
		}(cmd)
	}

	wg.Wait()

	if len(errors) > 0 {
		// log.Error("One or more commands failed during installation")
		return fmt.Errorf("installation errors: %v", errors)
	}

	log.Info("All installation commands executed successfully")
	return nil
}
