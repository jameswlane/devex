package types

import (
	"fmt"
	"regexp"
	"strings"
)

// Match redirection targets, including quoted HOME and tilde paths.
var rawRCAppend = regexp.MustCompile(`>>\s*["']?(?:~|\$HOME|\$\{HOME\})["']?/(?:\.bashrc|\.zshrc|\.profile|\.bash_profile|(?:\.config/)?fish/config\.fish|config\.fish)(?:["'\s;]|$)`)

// Validate rejects shell writes that bypass the managed rc editor.
func (command InstallCommand) Validate() error {
	if rawRCAppend.MatchString(command.Shell) || rawRCAppend.MatchString(command.Command) {
		return fmt.Errorf("raw shell rc appends are not allowed; use declarative shell_init or path post-install actions")
	}
	for shell, line := range command.ShellInit {
		if shell != "bash" && shell != "zsh" && shell != "fish" {
			return fmt.Errorf("unsupported shell_init shell %q", shell)
		}
		if strings.ContainsAny(line, "\r\n") {
			return fmt.Errorf("shell_init for %s must be a single line", shell)
		}
	}
	return nil
}

// ValidateShellActions validates hooks on every platform and alternative.
func (config OSConfig) ValidateShellActions() error {
	for _, commands := range [][]InstallCommand{config.PreInstall, config.PostInstall} {
		for _, command := range commands {
			if err := command.Validate(); err != nil {
				return err
			}
		}
	}
	for _, alternative := range config.Alternatives {
		if err := alternative.ValidateShellActions(); err != nil {
			return err
		}
	}
	return nil
}
