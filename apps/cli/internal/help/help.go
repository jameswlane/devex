package help

import (
	"embed"
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/muesli/reflow/wordwrap"
)

//go:embed docs/*.md docs/commands/*.md
var helpFiles embed.FS

// HelpTopic represents a help topic with metadata
type HelpTopic struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Keywords    []string `json:"keywords"`
	RelatedTo   []string `json:"related_to"`
	FilePath    string   `json:"file_path"`
	Context     string   `json:"context"` // command, tui, error, getting-started
	Priority    int      `json:"priority"`
	Description string   `json:"description"`
}

// HelpManager manages the help system
type HelpManager struct {
	topics   map[string]*HelpTopic
	renderer *glamour.TermRenderer
	width    int
}

// HelpContext represents different contexts where help can be accessed
type HelpContext string

const (
	ContextCommand         HelpContext = "command"
	ContextTUI             HelpContext = "tui"
	ContextError           HelpContext = "error"
	ContextGettingStarted  HelpContext = "getting-started"
	ContextConfiguration   HelpContext = "configuration"
	ContextTroubleshooting HelpContext = "troubleshooting"
)

// NewHelpManager creates a new help manager
func NewHelpManager() (*HelpManager, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown renderer: %w", err)
	}

	hm := &HelpManager{
		topics:   make(map[string]*HelpTopic),
		renderer: renderer,
		width:    80,
	}

	if err := hm.loadTopics(); err != nil {
		return nil, fmt.Errorf("failed to load help topics: %w", err)
	}

	return hm, nil
}

// SetWidth sets the rendering width for the help content
func (hm *HelpManager) SetWidth(width int) {
	hm.width = width
	// Create new renderer with updated width
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err == nil {
		hm.renderer = renderer
	}
}

// GetTopic retrieves a help topic by ID
func (hm *HelpManager) GetTopic(id string) (*HelpTopic, error) {
	topic, exists := hm.topics[id]
	if !exists {
		return nil, fmt.Errorf("help topic '%s' not found", id)
	}
	return topic, nil
}

// RenderTopic renders a help topic as markdown
func (hm *HelpManager) RenderTopic(id string) (string, error) {
	topic, err := hm.GetTopic(id)
	if err != nil {
		return "", err
	}

	content, err := helpFiles.ReadFile(topic.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read help file %s: %w", topic.FilePath, err)
	}

	rendered, err := hm.renderer.Render(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to render markdown: %w", err)
	}

	return rendered, nil
}

// SearchTopics searches for help topics by keyword or title
func (hm *HelpManager) SearchTopics(query string) []*HelpTopic {
	query = strings.ToLower(query)
	var results []*HelpTopic

	for _, topic := range hm.topics {
		// Check title
		if strings.Contains(strings.ToLower(topic.Title), query) {
			results = append(results, topic)
			continue
		}

		// Check keywords
		for _, keyword := range topic.Keywords {
			if strings.Contains(strings.ToLower(keyword), query) {
				results = append(results, topic)
				break
			}
		}
	}

	return results
}

// GetTopicsByContext retrieves help topics for a specific context
func (hm *HelpManager) GetTopicsByContext(context HelpContext) []*HelpTopic {
	var results []*HelpTopic

	for _, topic := range hm.topics {
		if topic.Context == string(context) {
			results = append(results, topic)
		}
	}

	return results
}

// GetTopicsByCategory retrieves help topics by category
func (hm *HelpManager) GetTopicsByCategory(category string) []*HelpTopic {
	var results []*HelpTopic

	for _, topic := range hm.topics {
		if topic.Category == category {
			results = append(results, topic)
		}
	}

	return results
}

// GetRelatedTopics retrieves topics related to a given topic ID
func (hm *HelpManager) GetRelatedTopics(id string) []*HelpTopic {
	topic, exists := hm.topics[id]
	if !exists {
		return nil
	}

	var results []*HelpTopic
	for _, relatedID := range topic.RelatedTo {
		if relatedTopic, exists := hm.topics[relatedID]; exists {
			results = append(results, relatedTopic)
		}
	}

	return results
}

// ListTopics returns all available help topics
func (hm *HelpManager) ListTopics() []*HelpTopic {
	topics := make([]*HelpTopic, 0, len(hm.topics))
	for _, topic := range hm.topics {
		topics = append(topics, topic)
	}
	return topics
}

// GetCategories returns all available categories
func (hm *HelpManager) GetCategories() []string {
	categories := make(map[string]bool)
	for _, topic := range hm.topics {
		categories[topic.Category] = true
	}

	result := make([]string, 0, len(categories))
	for category := range categories {
		result = append(result, category)
	}
	return result
}

// FormatForTerminal formats text for terminal display with proper wrapping
func (hm *HelpManager) FormatForTerminal(text string) string {
	return wordwrap.String(text, hm.width)
}

// loadTopics loads all help topics from embedded files
func (hm *HelpManager) loadTopics() error {
	// Define topics metadata - in a real implementation, this could be loaded from a JSON file
	topics := []*HelpTopic{
		{
			ID:          "overview",
			Title:       "DevEx Overview",
			Category:    "Getting Started",
			Keywords:    []string{"overview", "introduction", "start", "begin"},
			RelatedTo:   []string{"installation", "configuration"},
			FilePath:    "docs/overview.md",
			Context:     string(ContextGettingStarted),
			Priority:    1,
			Description: "Introduction to DevEx and its core concepts",
		},
		{
			ID:          "installation",
			Title:       "Installation Guide",
			Category:    "Getting Started",
			Keywords:    []string{"install", "setup", "download"},
			RelatedTo:   []string{"overview", "quick-start"},
			FilePath:    "docs/installation.md",
			Context:     string(ContextGettingStarted),
			Priority:    2,
			Description: "How to install and set up DevEx",
		},
		{
			ID:          "quick-start",
			Title:       "Quick Start Guide",
			Category:    "Getting Started",
			Keywords:    []string{"quick", "start", "tutorial", "guide"},
			RelatedTo:   []string{"installation", "commands"},
			FilePath:    "docs/quick-start.md",
			Context:     string(ContextGettingStarted),
			Priority:    3,
			Description: "Get started with DevEx in minutes",
		},
		{
			ID:          "commands",
			Title:       "Command Reference",
			Category:    "Reference",
			Keywords:    []string{"commands", "cli", "reference"},
			RelatedTo:   []string{"init", "install", "config"},
			FilePath:    "docs/commands.md",
			Context:     string(ContextCommand),
			Priority:    4,
			Description: "Complete reference for all DevEx commands",
		},
		{
			ID:          "init",
			Title:       "devex init",
			Category:    "Commands",
			Keywords:    []string{"init", "initialize", "setup"},
			RelatedTo:   []string{"configuration", "quick-start"},
			FilePath:    "docs/commands/init.md",
			Context:     string(ContextCommand),
			Priority:    5,
			Description: "Initialize a new DevEx configuration",
		},
		{
			ID:          "install",
			Title:       "devex install",
			Category:    "Commands",
			Keywords:    []string{"install", "package", "app"},
			RelatedTo:   []string{"uninstall", "status"},
			FilePath:    "docs/commands/install.md",
			Context:     string(ContextCommand),
			Priority:    6,
			Description: "Install applications using DevEx",
		},
		{
			ID:          "config",
			Title:       "devex config",
			Category:    "Commands",
			Keywords:    []string{"config", "configuration", "settings"},
			RelatedTo:   []string{"init", "templates"},
			FilePath:    "docs/commands/config.md",
			Context:     string(ContextCommand),
			Priority:    7,
			Description: "Manage DevEx configuration",
		},
		{
			ID:          "templates",
			Title:       "Template System",
			Category:    "Configuration",
			Keywords:    []string{"templates", "preset", "team"},
			RelatedTo:   []string{"config", "init"},
			FilePath:    "docs/templates.md",
			Context:     string(ContextConfiguration),
			Priority:    8,
			Description: "Using and creating DevEx templates",
		},
		{
			ID:          "troubleshooting",
			Title:       "Troubleshooting",
			Category:    "Help",
			Keywords:    []string{"troubleshoot", "error", "problem", "issue"},
			RelatedTo:   []string{"faq", "support"},
			FilePath:    "docs/troubleshooting.md",
			Context:     string(ContextTroubleshooting),
			Priority:    9,
			Description: "Common issues and their solutions",
		},
		{
			ID:          "recover",
			Title:       "devex recover",
			Category:    "Commands",
			Keywords:    []string{"recover", "recovery", "error", "fix", "troubleshoot"},
			RelatedTo:   []string{"undo", "config", "backup"},
			FilePath:    "docs/commands/recover.md",
			Context:     string(ContextCommand),
			Priority:    8,
			Description: "Error recovery and troubleshooting assistance",
		},
		{
			ID:          "faq",
			Title:       "Frequently Asked Questions",
			Category:    "Help",
			Keywords:    []string{"faq", "questions", "answers"},
			RelatedTo:   []string{"troubleshooting", "support"},
			FilePath:    "docs/faq.md",
			Context:     string(ContextTroubleshooting),
			Priority:    10,
			Description: "Answers to common questions",
		},
	}

	for _, topic := range topics {
		hm.topics[topic.ID] = topic
	}

	return nil
}

// GetCommandHelp returns contextual help for a specific command
func (hm *HelpManager) GetCommandHelp(command string) (string, error) {
	commandTopicID := strings.ToLower(command)

	// Try to find exact command match
	if topic, exists := hm.topics[commandTopicID]; exists {
		return hm.RenderTopic(topic.ID)
	}

	// Search for command in keywords
	results := hm.SearchTopics(command)
	if len(results) > 0 {
		return hm.RenderTopic(results[0].ID)
	}

	return "", fmt.Errorf("no help found for command '%s'", command)
}

// GetContextualHelp provides help based on current context
func (hm *HelpManager) GetContextualHelp(context HelpContext, specific string) (string, error) {
	topics := hm.GetTopicsByContext(context)

	if specific != "" {
		// Look for specific topic within context
		for _, topic := range topics {
			if strings.Contains(strings.ToLower(topic.Title), strings.ToLower(specific)) ||
				contains(topic.Keywords, strings.ToLower(specific)) {
				return hm.RenderTopic(topic.ID)
			}
		}
	}

	if len(topics) > 0 {
		// Return the highest priority topic for this context
		return hm.RenderTopic(topics[0].ID)
	}

	return "", fmt.Errorf("no help found for context '%s'", context)
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.Contains(strings.ToLower(s), item) {
			return true
		}
	}
	return false
}
