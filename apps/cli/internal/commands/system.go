package commands

import (
	"github.com/fatih/color"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/spf13/cobra"
)

// NewSystemCmd creates the system command with plugin redirect
func NewSystemCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Manage system settings and configurations",
		Long: `Configure and optimize your system settings for development.

The system command manages system-level configurations including:
  • Git global configuration (aliases, user settings, SSH keys)
  • Shell configuration (Bash/Zsh/Fish profiles and dotfiles)
  • Desktop environment settings (GNOME, KDE themes and preferences)
  • Terminal configuration and color schemes
  • Font installation and management

This functionality has been moved to plugins for better modularity.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSystemCommand(settings)
		},
	}

	return cmd
}

func runSystemCommand(settings config.CrossPlatformSettings) error {
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	log.Print("%s System Configuration\n\n", cyan("🔧"))
	log.Print("%s System configuration features have been moved to plugins.\n", yellow("⚠️"))
	log.Print("To manage your system settings:\n\n")
	log.Print("📝 Shell Configuration:\n")
	log.Print("  \"$HOME/.devex/plugins/devex-plugin-tool-shell\" setup\n")
	log.Print("  \"$HOME/.devex/plugins/devex-plugin-tool-shell\" config\n\n")
	log.Print("🎨 Desktop Themes:\n")
	log.Print("  devex plugin run desktop-themes apply [theme-name]\n")
	log.Print("  devex plugin run desktop-themes list\n\n")
	log.Print("🔧 Git Configuration:\n")
	log.Print("  devex plugin run tool-git setup\n")
	log.Print("  devex plugin run tool-git configure\n\n")
	log.Print("🖥️ Desktop Environment:\n")
	log.Print("  devex plugin run desktop-gnome configure    # For GNOME\n")
	log.Print("  devex plugin run desktop-kde configure      # For KDE\n\n")
	log.Print("For more information, see: https://docs.devex.sh/plugins/\n")

	return nil
}
