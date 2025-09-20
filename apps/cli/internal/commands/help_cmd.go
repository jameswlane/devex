package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/help"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// NewHelpCmd creates a new help command
func NewHelpCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help [topic]",
		Short: "Show contextual help with interactive viewer",
		Long: `Display help information with an interactive terminal viewer.

Without arguments, launches an interactive help browser where you can:
  • Browse help topics by category
  • Search for specific topics
  • Navigate with keyboard shortcuts
  • View detailed markdown documentation

With a topic argument, shows help for that specific topic.

Examples:
  devex help              # Interactive help browser  
  devex help install      # Help for install command
  devex help templates    # Template system documentation
  devex help --search git # Search for git-related topics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			searchQuery, _ := cmd.Flags().GetString("search")

			// Handle search mode
			if searchQuery != "" {
				return runHelpSearch(searchQuery)
			}

			// Handle specific topic
			if len(args) > 0 {
				topic := args[0]
				return runHelpTopic(topic)
			}

			// Launch interactive help browser
			return runInteractiveHelp()
		},
	}

	// Add flags
	cmd.Flags().StringP("search", "s", "", "Search help topics")
	cmd.Flags().BoolP("list", "l", false, "List all available topics")

	return cmd
}

// runInteractiveHelp launches the interactive help browser
func runInteractiveHelp() error {
	return help.ShowHelp()
}

// runHelpTopic shows help for a specific topic
func runHelpTopic(topic string) error {
	// Try to show specific topic
	err := help.ShowHelpTopic(topic)
	if err != nil {
		// If specific topic not found, try searching
		fmt.Printf("Topic '%s' not found. Searching for related topics...\n\n", topic)
		return runHelpSearch(topic)
	}
	return nil
}

// runHelpSearch searches and displays help topics
func runHelpSearch(query string) error {
	helpManager, err := help.NewHelpManager()
	if err != nil {
		return fmt.Errorf("failed to initialize help system: %w", err)
	}

	results := helpManager.SearchTopics(query)
	if len(results) == 0 {
		fmt.Printf("No help topics found for '%s'\n\n", query)
		fmt.Println("Available topics:")
		return runHelpList()
	}

	if len(results) == 1 {
		// If only one result, show it directly
		return help.ShowHelpTopic(results[0].ID)
	}

	// Multiple results - show list and let user choose
	fmt.Printf("Found %d topics for '%s':\n\n", len(results), query)
	for i, topic := range results {
		fmt.Printf("%d. %s\n   %s\n\n", i+1, topic.Title, topic.Description)
	}

	fmt.Println("Use 'devex help <topic-id>' to view a specific topic or 'devex help' for interactive browser.")
	return nil
}

// runHelpList lists all available help topics
func runHelpList() error {
	helpManager, err := help.NewHelpManager()
	if err != nil {
		return fmt.Errorf("failed to initialize help system: %w", err)
	}

	topics := helpManager.ListTopics()
	categories := make(map[string][]*help.HelpTopic)

	// Group by category
	for _, topic := range topics {
		categories[topic.Category] = append(categories[topic.Category], topic)
	}

	// Display by category
	for category, categoryTopics := range categories {
		fmt.Printf("\n%s:\n", category)
		for _, topic := range categoryTopics {
			fmt.Printf("  %-20s %s\n", topic.ID, topic.Description)
		}
	}

	fmt.Println("\nUse 'devex help <topic>' to view detailed help or 'devex help' for interactive browser.")
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
