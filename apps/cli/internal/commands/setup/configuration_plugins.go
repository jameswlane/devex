package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

func gitConfigurationArgs(name, email string) []string {
	args := []string{"config"}
	if name != "" {
		args = append(args, "--name", name)
	}
	if email != "" {
		args = append(args, "--email", email)
	}
	return args
}

func desktopConfigurationArgs(theme string) [][]string {
	commands := [][]string{{"configure"}}
	if theme != "" {
		commands = append(commands, []string{"apply-theme", theme})
	}
	return commands
}

// Capture plugin output while setup owns the terminal, including plugin errors.
func runConfigurationPlugin(ctx context.Context, path string, args []string) error {
	output, runErr := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if logPath := log.GetLogFile(); logPath != "" && len(output) > 0 {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Warn("Could not open setup log for plugin output", "error", err)
		} else {
			if _, err := fmt.Fprintf(file, "\n%s %s:\n%s\n", filepath.Base(path), args[0], output); err != nil {
				log.Warn("Could not write setup plugin output", "error", err)
			}
			if err := file.Close(); err != nil {
				log.Warn("Could not close setup plugin log", "error", err)
			}
		}
	}
	if runErr != nil {
		return fmt.Errorf("%s %s: %w; %s", filepath.Base(path), args[0], runErr, strings.TrimSpace(string(output)))
	}
	return nil
}
