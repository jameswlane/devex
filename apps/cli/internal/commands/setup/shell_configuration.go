package setup

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// shellSetupCommand uses tool-shell's advertised setup operation and SDK stdin
// protocol. selected_shell limits configuration to that shell and does not switch
// the user's login shell.
func shellSetupCommand(ctx context.Context, path, shell string) (*exec.Cmd, error) {
	switch shell {
	case "bash", "zsh", "fish":
	default:
		return nil, fmt.Errorf("unsupported shell %q; select bash, zsh, or fish", shell)
	}
	input := sdk.PluginSetupInput{
		Command:    "setup",
		Parameters: map[string]interface{}{"selected_shell": shell},
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(input); err != nil {
		return nil, fmt.Errorf("encode shell setup input: %w", err)
	}
	cmd := exec.CommandContext(ctx, path, input.Command)
	cmd.Stdin = &stdin
	return cmd, nil
}

func runShellSetup(ctx context.Context, path, shell string) error {
	cmd, err := shellSetupCommand(ctx, path, shell)
	if err != nil {
		return err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	// The plugin emits newline-delimited SDK progress/results interspersed with
	// human-readable file updates. An SDK error can accompany exit status zero.
	scanner := bufio.NewScanner(&stdout)
	var result sdk.PluginSetupOutput
	for scanner.Scan() {
		var output sdk.PluginSetupOutput
		if json.Unmarshal(scanner.Bytes(), &output) != nil || output.Status == "" {
			continue
		}
		result = output
		if output.Status == string(sdk.SetupStatusError) {
			reason := output.Error
			if reason == "" {
				reason = output.Message
			}
			return fmt.Errorf("tool-shell setup failed: %s", reason)
		}
	}
	if runErr != nil {
		return fmt.Errorf("tool-shell setup failed: %w; %s", runErr, strings.TrimSpace(stderr.String()))
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read tool-shell setup output: %w", err)
	}
	if result.Status != string(sdk.SetupStatusSuccess) {
		return fmt.Errorf("tool-shell setup did not report success; check the plugin installation")
	}
	return nil
}

func (m *SetupModel) recordShellConfigurationFailure(shell string, err error) {
	// Shell names come from setup's supported options. Reuse the SDK protocol so
	// recovery configures the selected shell even if $SHELL differs.
	input, marshalErr := json.Marshal(sdk.PluginSetupInput{
		Command:    "setup",
		Parameters: map[string]interface{}{"selected_shell": shell},
	})
	if marshalErr != nil {
		m.installation.addError(fmt.Sprintf("Shell configuration failed: %v", err))
		return
	}
	m.installation.addError(fmt.Sprintf("Shell configuration failed: %v\n  Recover: printf '%%s\\n' %s | \"$HOME/.devex/plugins/devex-plugin-tool-shell\" setup", err, singleQuote(string(input))))
}

// singleQuote quotes s for a POSIX shell so it cannot break out of the quotes.
func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
