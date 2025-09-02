package help

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewerState represents the current state of the help viewer
type ViewerState int

const (
	StateTopicList ViewerState = iota
	StateTopicView
	StateSearch
)

// ViewerModel represents the help viewer TUI model
type ViewerModel struct {
	helpManager *HelpManager
	state       ViewerState
	width       int
	height      int

	// Topic list view
	topicList list.Model
	topics    []list.Item

	// Topic content view
	viewport     viewport.Model
	currentTopic *HelpTopic
	content      string

	// Search functionality
	searchInput textinput.Model
	searchQuery string
	searchMode  bool

	// Navigation
	breadcrumbs []string
	keys        KeyMap
}

// TopicItem implements list.Item for help topics
type TopicItem struct {
	topic *HelpTopic
}

func (i TopicItem) FilterValue() string {
	return i.topic.Title + " " + strings.Join(i.topic.Keywords, " ")
}

func (i TopicItem) Title() string       { return i.topic.Title }
func (i TopicItem) Description() string { return i.topic.Description }

// KeyMap defines key bindings for the help viewer
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Back     key.Binding
	Enter    key.Binding
	Search   key.Binding
	Help     key.Binding
	Quit     key.Binding
	Escape   key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
}

// DefaultKeyMap returns the default key map
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "back"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("backspace", "back"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back/cancel"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdown", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g", "top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G", "bottom"),
		),
	}
}

// NewViewerModel creates a new help viewer model
func NewViewerModel() (*ViewerModel, error) {
	helpManager, err := NewHelpManager()
	if err != nil {
		return nil, err
	}

	// Initialize topic list
	topics := helpManager.ListTopics()
	items := make([]list.Item, len(topics))
	for i, topic := range topics {
		items[i] = TopicItem{topic: topic}
	}

	topicList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	topicList.Title = "DevEx Help Topics"
	topicList.SetShowStatusBar(false)
	topicList.SetFilteringEnabled(true)

	// Initialize search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search help topics..."
	searchInput.CharLimit = 50

	// Initialize viewport
	vp := viewport.New(0, 0)

	model := &ViewerModel{
		helpManager: helpManager,
		state:       StateTopicList,
		topicList:   topicList,
		topics:      items,
		viewport:    vp,
		searchInput: searchInput,
		keys:        DefaultKeyMap(),
		breadcrumbs: []string{"Help"},
	}

	return model, nil
}

// Init initializes the model
func (m ViewerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m ViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.helpManager.SetWidth(msg.Width - 4) // Account for padding

		// Update component sizes
		m.topicList.SetSize(msg.Width-4, msg.Height-6)
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8
		m.searchInput.Width = msg.Width - 6

	case tea.KeyMsg:
		if m.searchMode && m.state == StateSearch {
			switch {
			case key.Matches(msg, m.keys.Escape):
				m.searchMode = false
				m.state = StateTopicList
				m.searchInput.SetValue("")
				return m, nil
			case key.Matches(msg, m.keys.Enter):
				m.searchQuery = m.searchInput.Value()
				if m.searchQuery != "" {
					return m.performSearch()
				}
				m.searchMode = false
				m.state = StateTopicList
				return m, nil
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
		}

		switch m.state {
		case StateTopicList:
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Search):
				m.state = StateSearch
				m.searchMode = true
				m.searchInput.Focus()
				return m, nil
			case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Right):
				if selectedItem := m.topicList.SelectedItem(); selectedItem != nil {
					if topicItem, ok := selectedItem.(TopicItem); ok {
						return m.viewTopic(topicItem.topic)
					}
				}
			default:
				m.topicList, cmd = m.topicList.Update(msg)
				cmds = append(cmds, cmd)
			}

		case StateTopicView:
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Left):
				m.state = StateTopicList
				m.currentTopic = nil
				m.breadcrumbs = m.breadcrumbs[:len(m.breadcrumbs)-1]
				return m, nil
			case key.Matches(msg, m.keys.PageUp):
				m.viewport.ScrollUp(m.viewport.Height / 2)
			case key.Matches(msg, m.keys.PageDown):
				m.viewport.ScrollDown(m.viewport.Height / 2)
			case key.Matches(msg, m.keys.Up):
				m.viewport.ScrollUp(1)
			case key.Matches(msg, m.keys.Down):
				m.viewport.ScrollDown(1)
			case key.Matches(msg, m.keys.Home):
				m.viewport.GotoTop()
			case key.Matches(msg, m.keys.End):
				m.viewport.GotoBottom()
			default:
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the help viewer
func (m ViewerModel) View() string {
	if m.width == 0 {
		return "Loading help..."
	}

	// Style definitions
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	breadcrumbStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262"))

	searchStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#626262")).
		Padding(1)

	// Header
	header := titleStyle.Render("DevEx Help System")
	breadcrumb := breadcrumbStyle.Render(strings.Join(m.breadcrumbs, " > "))

	var content string
	var footer string

	switch m.state {
	case StateTopicList:
		content = m.topicList.View()
		footer = m.renderFooter()

	case StateSearch:
		if m.searchMode {
			searchBox := searchStyle.Render(m.searchInput.View())
			content = fmt.Sprintf("Search Help Topics\n\n%s\n\nPress Enter to search, Esc to cancel", searchBox)
		} else {
			// Show search results
			content = m.renderSearchResults()
		}
		footer = "Enter: search • Esc: back • q: quit"

	case StateTopicView:
		if m.content != "" {
			content = contentStyle.Width(m.width - 4).Height(m.height - 8).Render(m.viewport.View())
		} else {
			content = "Loading topic content..."
		}
		footer = m.renderViewerFooter()
	}

	// Combine all parts
	view := fmt.Sprintf("%s\n%s\n\n%s\n\n%s",
		header,
		breadcrumb,
		content,
		footer,
	)

	return view
}

// viewTopic switches to viewing a specific topic
func (m *ViewerModel) viewTopic(topic *HelpTopic) (ViewerModel, tea.Cmd) {
	m.currentTopic = topic
	m.state = StateTopicView
	m.breadcrumbs = append(m.breadcrumbs, topic.Title)

	// Render topic content
	content, err := m.helpManager.RenderTopic(topic.ID)
	if err != nil {
		content = fmt.Sprintf("Error loading topic: %s", err)
	}

	m.content = content
	m.viewport.SetContent(content)
	m.viewport.GotoTop()

	return *m, nil
}

// performSearch performs a search and updates the topic list
func (m *ViewerModel) performSearch() (ViewerModel, tea.Cmd) {
	results := m.helpManager.SearchTopics(m.searchQuery)

	items := make([]list.Item, len(results))
	for i, topic := range results {
		items[i] = TopicItem{topic: topic}
	}

	m.topicList.SetItems(items)
	m.topicList.Title = fmt.Sprintf("Search Results for '%s'", m.searchQuery)
	m.state = StateTopicList
	m.searchMode = false
	m.searchInput.SetValue("")

	return *m, nil
}

// renderFooter renders the footer for the topic list
func (m ViewerModel) renderFooter() string {
	return "Enter: view topic • /: search • ?: help • q: quit"
}

// renderViewerFooter renders the footer for the topic viewer
func (m ViewerModel) renderViewerFooter() string {
	percentage := fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
	return fmt.Sprintf("↑/↓: scroll • PgUp/PgDn: page • g/G: top/bottom • ←/Esc: back • %s", percentage)
}

// renderSearchResults renders search results (currently unused but available for future)
func (m ViewerModel) renderSearchResults() string {
	if m.searchQuery == "" {
		return "Enter a search query above"
	}

	results := m.helpManager.SearchTopics(m.searchQuery)
	if len(results) == 0 {
		return fmt.Sprintf("No results found for '%s'", m.searchQuery)
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("Found %d result(s) for '%s':\n\n", len(results), m.searchQuery))

	for i, topic := range results {
		content.WriteString(fmt.Sprintf("%d. %s\n   %s\n\n", i+1, topic.Title, topic.Description))
	}

	return content.String()
}

// ShowHelp shows the help viewer as a standalone TUI
func ShowHelp() error {
	model, err := NewViewerModel()
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// ShowHelpTopic shows a specific help topic
func ShowHelpTopic(topicID string) error {
	model, err := NewViewerModel()
	if err != nil {
		return err
	}

	// Find and display the specific topic
	topic, err := model.helpManager.GetTopic(topicID)
	if err != nil {
		return err
	}

	newModel, _ := model.viewTopic(topic)
	model = &newModel

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// ShowContextualHelp shows help for a specific context
func ShowContextualHelp(context HelpContext, specific string) error {
	helpManager, err := NewHelpManager()
	if err != nil {
		return err
	}

	content, err := helpManager.GetContextualHelp(context, specific)
	if err != nil {
		return err
	}

	// Create a simple viewer model just for displaying content
	model, err := NewViewerModel()
	if err != nil {
		return err
	}

	model.state = StateTopicView
	model.content = content
	model.viewport.SetContent(content)
	model.breadcrumbs = []string{"Help", string(context)}

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
