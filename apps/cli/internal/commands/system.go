package commands

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/jameswlane/devex/apps/cli/internal/config"
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

	fmt.Printf("%s System Configuration\n\n", cyan("🔧"))
	fmt.Printf("%s System configuration features have been moved to plugins.\n", yellow("⚠️"))
	fmt.Printf("To manage your system settings:\n\n")
	fmt.Printf("📝 Shell Configuration:\n")
	fmt.Printf("  devex plugin run tool-shell setup\n")
	fmt.Printf("  devex plugin run tool-shell configure bash|zsh|fish\n\n")
	fmt.Printf("🎨 Desktop Themes:\n")
	fmt.Printf("  devex plugin run desktop-themes apply [theme-name]\n")
	fmt.Printf("  devex plugin run desktop-themes list\n\n")
	fmt.Printf("🔧 Git Configuration:\n")
	fmt.Printf("  devex plugin run tool-git setup\n")
	fmt.Printf("  devex plugin run tool-git configure\n\n")
	fmt.Printf("🖥️ Desktop Environment:\n")
	fmt.Printf("  devex plugin run desktop-gnome configure    # For GNOME\n")
	fmt.Printf("  devex plugin run desktop-kde configure      # For KDE\n\n")
	fmt.Printf("For more information, see: https://docs.devex.sh/plugins/\n")

	return nil
}
