package commands

import (
	"github.com/spf13/cobra"
)

func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Long: `Generate autocompletion scripts for your shell.

To enable autocompletion:
  Bash:
    devex completion bash > /etc/bash_completion.d/devex
  Zsh:
    devex completion zsh > "${fpath[1]}/_devex"
  Fish:
    devex completion fish > ~/.config/fish/completions/devex.fish
  PowerShell:
    devex completion powershell > devex.ps1`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			switch shell {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return cmd.Usage()
			}
		},
	}
	return cmd
}
