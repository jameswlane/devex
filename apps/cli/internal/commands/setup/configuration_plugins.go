package setup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

type configurationPluginExecutor interface {
	ExecutePluginWithIO(context.Context, string, []string, sdk.OperationIO) error
}

// The SDK wraps stderr separately, so stdout and stderr can write concurrently.
type configurationOutput struct {
	buffer bytes.Buffer
	mu     sync.Mutex
}

func (b *configurationOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *configurationOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

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
func runConfigurationPlugin(ctx context.Context, manager configurationPluginExecutor, pluginName string, args []string) error {
	var buf configurationOutput
	runErr := manager.ExecutePluginWithIO(ctx, pluginName, args, sdk.OperationIO{Stdout: &buf, Stderr: &buf})
	output := buf.String()
	if logPath := log.GetLogFile(); logPath != "" && len(output) > 0 {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Warn("Could not open setup log for plugin output", "error", err)
		} else {
			if _, err := fmt.Fprintf(file, "\n%s %s:\n%s\n", pluginName, args[0], output); err != nil {
				log.Warn("Could not write setup plugin output", "error", err)
			}
			if err := file.Close(); err != nil {
				log.Warn("Could not close setup plugin log", "error", err)
			}
		}
	}
	if runErr != nil {
		const maxErrorOutput = 4096
		if len(output) > maxErrorOutput {
			output = output[len(output)-maxErrorOutput:]
		}
		return fmt.Errorf("%s %s: %w; %s", pluginName, args[0], runErr, strings.TrimSpace(output))
	}
	return nil
}
