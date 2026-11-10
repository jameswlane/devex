package types

import (
	"fmt"
	"regexp"
	"strings"
)

// Match redirection targets, including quoted HOME and tilde paths.
var rawRCAppend = regexp.MustCompile(`>>\s*["']?(?:~|\$HOME|\$\{HOME\})["']?/(?:\.bashrc|\.zshrc|\.profile|\.bash_profile|(?:\.config/)?fish/config\.fish|config\.fish)(?:["'\s;]|$)`)

// Protected user configuration must use declarative actions. Reject even reads in
// raw hooks: shell text cannot establish whether an indirect command writes it.
var protectedUserConfig = regexp.MustCompile(`(?i)(?:~|\$HOME|\$\{HOME\}|\$env:USERPROFILE|%USERPROFILE%)["']?[/\\]\.(?:config|[a-z0-9_-]+rc|profile|bash_profile|gitconfig|git-credentials|ssh)(?:[/\\"'\s;]|$)|(?:/home/[^/\s]+|/Users/[^/\s]+|/root)/\.(?:config|[a-z0-9_-]+rc|profile|bash_profile|gitconfig|ssh)(?:[/"'\s;]|$)|\$(?:XDG_CONFIG_HOME|\{XDG_CONFIG_HOME\})`)

// Validate rejects shell writes that bypass the managed rc editor.
func (command InstallCommand) Validate() error {
	if rawRCAppend.MatchString(command.Shell) || rawRCAppend.MatchString(command.Command) {
		return fmt.Errorf("raw shell rc appends are not allowed; use declarative shell_init or path post-install actions")
	}
	for _, raw := range []string{command.Shell, command.Command, command.UpdateShellConfig} {
		if protectedUserConfig.MatchString(raw) {
			return fmt.Errorf("raw hooks may not access user configuration outside managed actions; use declarative shell_init, path or config_files")
		}
	}
	if command.Copy != nil && protectedUserConfig.MatchString(command.Copy.Destination) {
		return fmt.Errorf("copy hooks may not overwrite user configuration; use backed-up config_files")
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
	if len(config.Packages) > 0 || len(config.Options) > 0 {
		if _, err := ResolvePackageInstall(config.InstallMethod, config.InstallCommand, config.Packages, config.Options); err != nil {
			return err
		}
	}
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
