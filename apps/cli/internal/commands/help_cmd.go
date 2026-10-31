package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/help"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var isTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// isInteractiveHelp determines if interactive help should be shown
func isInteractiveHelp(cmd *cobra.Command) bool {
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
	if nonInteractive {
		return false
	}
	if !interaction.IsInteractiveMode() {
		return false
	}
	if !isTerminal() {
		return false
	}
	return true
}

// NewHelpCmd creates a new help command
func NewHelpCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help [command|topic]",
		Short: "Show contextual help with interactive viewer",
		Long: `Display help information with an interactive terminal viewer.

Without arguments, launches an interactive help browser where you can:
  • Browse help topics by category
  • Search for specific topics
  • Navigate with keyboard shortcuts
  • View detailed markdown documentation

Without a TTY or in non-interactive mode, prints the standard command list.

When given a command name (e.g., 'devex help install'), it displays help for that
command directly. If no matching command is found, it searches help topics.

Examples:
  devex help              # Interactive help browser (or command list without TTY)
  devex help install      # Help for install command
  devex help templates    # Template system documentation
  devex help --search git # Search for git-related topics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			searchQuery, _ := cmd.Flags().GetString("search")

			// Handle search mode
			if searchQuery != "" {
				return runHelpSearch(cmd, searchQuery)
			}

			// Handle list mode
			listFlag, _ := cmd.Flags().GetBool("list")
			if listFlag {
				return runHelpList(cmd)
			}

			// Handle specific command or topic
			if len(args) > 0 {
				if cmd.Root() != nil {
					targetCmd, remainingArgs, err := cmd.Root().Find(args)
					if err == nil && targetCmd != nil && targetCmd != cmd.Root() && len(remainingArgs) == 0 {
						targetCmd.SetOut(cmd.OutOrStdout())
						targetCmd.SetErr(cmd.ErrOrStderr())
						targetCmd.InitDefaultHelpFlag()
						return targetCmd.Help()
					}
				}

				topic := strings.Join(args, " ")
				return runHelpTopic(cmd, topic)
			}

			// Launch interactive help browser or print root help if non-interactive / no TTY
			if !isInteractiveHelp(cmd) {
				if root := cmd.Root(); root != nil {
					// Cobra adds these flags lazily when the root itself executes.
					root.InitDefaultHelpFlag()
					root.InitDefaultVersionFlag()
					return root.Help()
				}
				return cmd.Help()
			}

			return runInteractiveHelp()
		},
	}

	// Add flags
	cmd.Flags().StringP("search", "s", "", "Search help topics")
	cmd.Flags().BoolP("list", "l", false, "List all available topics")
	cmd.Flags().Bool("non-interactive", false, "Run in non-interactive mode")

	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))

	return cmd
}

// runInteractiveHelp launches the interactive help browser
func runInteractiveHelp() error {
	return help.ShowHelp()
}

// runHelpTopic shows help for a specific topic
func runHelpTopic(cmd *cobra.Command, topic string) error {
	hm, err := help.NewHelpManager()
	if err != nil {
		return fmt.Errorf("failed to initialize help system: %w", err)
	}

	_, err = hm.GetTopic(topic)
	if err != nil {
		// If specific topic not found, try searching
		fmt.Fprintf(cmd.OutOrStdout(), "Topic '%s' not found. Searching for related topics...\n\n", topic)
		return runHelpSearchWithManager(cmd, hm, topic)
	}

	if isInteractiveHelp(cmd) {
		return help.ShowHelpTopic(topic)
	}

	rendered, err := hm.RenderTopic(topic)
	if err != nil {
		return fmt.Errorf("failed to render help topic: %w", err)
	}

	fmt.Fprint(cmd.OutOrStdout(), rendered)
	return nil
}

// runHelpSearch searches and displays help topics
func runHelpSearch(cmd *cobra.Command, query string) error {
	hm, err := help.NewHelpManager()
	if err != nil {
		return fmt.Errorf("failed to initialize help system: %w", err)
	}
	return runHelpSearchWithManager(cmd, hm, query)
}

func runHelpSearchWithManager(cmd *cobra.Command, hm *help.HelpManager, query string) error {
	results := hm.SearchTopics(query)
	if len(results) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No help topics found for '%s'\n\n", query)
		fmt.Fprintln(cmd.OutOrStdout(), "Available topics:")
		return runHelpListWithManager(cmd, hm)
	}

	if len(results) == 1 {
		// If only one result, show it directly
		if isInteractiveHelp(cmd) {
			return help.ShowHelpTopic(results[0].ID)
		}
		rendered, err := hm.RenderTopic(results[0].ID)
		if err != nil {
			return fmt.Errorf("failed to render help topic: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), rendered)
		return nil
	}

	// Multiple results - show list and let user choose
	fmt.Fprintf(cmd.OutOrStdout(), "Found %d topics for '%s':\n\n", len(results), query)
	for i, topic := range results {
		fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n   %s\n\n", i+1, topic.Title, topic.Description)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Use 'devex help <topic-id>' to view a specific topic or 'devex help' for interactive browser.")
	return nil
}

// runHelpList lists all available help topics
func runHelpList(cmd *cobra.Command) error {
	hm, err := help.NewHelpManager()
	if err != nil {
		return fmt.Errorf("failed to initialize help system: %w", err)
	}
	return runHelpListWithManager(cmd, hm)
}

func runHelpListWithManager(cmd *cobra.Command, hm *help.HelpManager) error {
	topics := hm.ListTopics()
	categories := make(map[string][]*help.HelpTopic)

	// Group by category
	for _, topic := range topics {
		categories[topic.Category] = append(categories[topic.Category], topic)
	}

	// Display by category
	for category, categoryTopics := range categories {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n", category)
		for _, topic := range categoryTopics {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s\n", topic.ID, topic.Description)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nUse 'devex help <topic>' to view detailed help or 'devex help' for interactive browser.")
	return nil
}

// AddContextualHelp adds contextual help flags to any command
func AddContextualHelp(cmd *cobra.Command, context help.HelpContext, specific string) {
	originalRunE := cmd.RunE
	originalRun := cmd.Run

	// Add help flag
	cmd.Flags().Bool("help-context", false, "Show contextual help for this command")

	// Wrap the existing run function to check for contextual help
	cmd.RunE = func(c *cobra.Command, args []string) error {
		showContextualHelp, _ := c.Flags().GetBool("help-context")
		if showContextualHelp {
			if !isInteractiveHelp(c) {
				hm, err := help.NewHelpManager()
				if err != nil {
					return err
				}
				content, err := hm.GetContextualHelp(context, specific)
				if err != nil {
					return err
				}
				fmt.Fprint(c.OutOrStdout(), content)
				return nil
			}
			return help.ShowContextualHelp(context, specific)
		}

		if originalRunE != nil {
			return originalRunE(c, args)
		}

		if originalRun != nil {
			originalRun(c, args)
		}

		return nil
	}
}

// GetCommandHelp returns quick help text for a command
func GetCommandHelp(command string) string {
	helpManager, err := help.NewHelpManager()
	if err != nil {
		return fmt.Sprintf("Help system unavailable: %s", err)
	}

	content, err := helpManager.GetCommandHelp(command)
	if err != nil {
		// Fallback to basic help
		return fmt.Sprintf("No specific help available for '%s'. Use 'devex help' for interactive help browser.", command)
	}

	// Strip markdown formatting for quick display
	lines := strings.Split(content, "\n")
	var quickHelp []string
	inCodeBlock := false

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			quickHelp = append(quickHelp, "  "+line)
			continue
		}

		// Remove markdown formatting
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "*", "")
		line = strings.ReplaceAll(line, "`", "")

		if strings.TrimSpace(line) != "" {
			quickHelp = append(quickHelp, line)
		}

		// Limit to first few lines for quick help
		if len(quickHelp) >= 10 {
			break
		}
	}

	return strings.Join(quickHelp, "\n")
}
