package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

// AppItem represents an application item for the list component
type AppItem struct {
	app         types.AppConfig
	isInstalled bool
}

func (i AppItem) FilterValue() string { return i.app.Name }
func (i AppItem) Title() string       { return i.app.Name }
func (i AppItem) Description() string {
	desc := i.app.Description
	if i.isInstalled {
		desc += " (Already in config)"
	}
	return desc
}

// AddModel represents the TUI state for the add command
type AddModel struct {
	list     list.Model
	choice   string
	quitting bool
	settings config.CrossPlatformSettings
	width    int
	height   int
}

// NewAddModel creates a new add command TUI model
func NewAddModel(settings config.CrossPlatformSettings) *AddModel {
	// Get all available applications
	availableApps := settings.GetApplications()

	configuredApps := make(map[string]bool)
	apps, _ := configuredEditableApplications(settings)
	for _, app := range apps {
		configuredApps[strings.ToLower(app.Name)] = true
	}

	// Create list items
	items := make([]list.Item, 0, len(availableApps))
	for _, app := range availableApps {
		items = append(items, AppItem{
			app:         app,
			isInstalled: configuredApps[strings.ToLower(app.Name)],
		})
	}

	// Sort items by name
	sort.Slice(items, func(i, j int) bool {
		itemI, okI := items[i].(AppItem)
		itemJ, okJ := items[j].(AppItem)
		if !okI || !okJ {
			return false
		}
		return itemI.app.Name < itemJ.app.Name
	})

	l := list.New(items, AppItemDelegate{}, 80, 14)
	l.Title = "Select Applications to Add"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	return &AddModel{
		list:     l,
		settings: settings,
	}
}

// AppItemDelegate renders list items
type AppItemDelegate struct{}

func (d AppItemDelegate) Height() int                             { return 1 }
func (d AppItemDelegate) Spacing() int                            { return 0 }
func (d AppItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d AppItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(AppItem)
	if !ok {
		return
	}

	str := i.app.Name

	if i.isInstalled {
		str += " ✓"
	}

	fn := itemStyle.Render
	if index == m.Index() {
		fn = selectedItemStyle.Render
	}

	fmt.Fprint(w, fn(str))
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170"))

	paginationStyle = list.DefaultStyles().PaginationStyle.
			PaddingLeft(4)

	helpStyle = list.DefaultStyles().HelpStyle.
			PaddingLeft(4).
			PaddingBottom(1)

	quitTextStyle = lipgloss.NewStyle().
			Margin(1, 0, 2, 4)
)

func (m AddModel) Init() tea.Cmd {
	return nil
}

func (m AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			item, ok := m.list.SelectedItem().(AppItem)
			if ok {
				// Check if app is already installed
				if item.isInstalled {
					m.choice = fmt.Sprintf("Application '%s' is already in your configuration", item.app.Name)
				} else {
					// Add the application to user config
					if err := m.addAppToConfig(item.app); err != nil {
						m.choice = fmt.Sprintf("Error adding %s: %v", item.app.Name, err)
					} else {
						m.choice = fmt.Sprintf("Successfully added %s to your configuration", item.app.Name)
					}
				}
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m AddModel) View() string {
	if m.choice != "" {
		return quitTextStyle.Render(m.choice + "\n")
	}
	if m.quitting {
		return quitTextStyle.Render("Cancelled adding applications.\n")
	}
	return "\n" + m.list.View()
}

// addAppToConfig adds an application to the user's configuration
func (m *AddModel) addAppToConfig(app types.AppConfig) error {
	userConfigDir := m.settings.GetConfigDir()
	apps, err := configuredEditableApplications(m.settings)
	if err != nil {
		return err
	}
	for _, existing := range apps {
		if strings.EqualFold(existing.Name, app.Name) {
			return fmt.Errorf("application %q is already in your configuration", app.Name)
		}
	}
	if err := migrateApplicationConfig(userConfigDir); err != nil {
		return err
	}
	path, err := applicationConfigPath(userConfigDir, "applications", app.Category, app.Name)
	if err != nil {
		return err
	}

	// Create undo operation before modification
	baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
	undoManager := undo.NewUndoManager(baseDir)

	metadata := map[string]interface{}{
		"app_name":        app.Name,
		"app_category":    app.Category,
		"app_description": app.Description,
	}

	undoOp, err := undoManager.RecordOperation("add",
		fmt.Sprintf("Added application: %s", app.Name),
		app.Name,
		metadata)
	if err != nil {
		// Log warning but don't block the operation
		fmt.Fprintf(os.Stderr, "Warning: Failed to record undo operation: %v\n", err)
	}

	// Keep every platform definition rather than saving only the current OS.
	definition := config.CrossPlatformAppFromLegacy(app)
	for _, candidate := range m.settings.GetAllApps() {
		if strings.EqualFold(candidate.Name, app.Name) {
			definition = candidate
			break
		}
	}
	if err := writeYAMLConfig(definition, path, ""); err != nil {
		return err
	}

	// Create new version after successful configuration change
	vm := version.NewVersionManager(baseDir)
	_, versionErr := vm.UpdateVersion(
		fmt.Sprintf("Added application: %s", app.Name),
		[]string{fmt.Sprintf("Added %s to applications configuration", app.Name)},
	)
	if versionErr != nil {
		// Log warning but don't fail the operation
		fmt.Fprintf(os.Stderr, "Warning: Failed to create version: %v\n", versionErr)
	}

	// Update undo operation with completion info
	if undoOp != nil {
		if updateErr := undoManager.UpdateOperation(undoOp.ID); updateErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to update undo operation: %v\n", updateErr)
		}
	}

	return nil
}

// NewAddCmd creates a new add command
func NewAddCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		category string
		search   string
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "add [application-name]",
		Short: "Add applications to your DevEx configuration",
		Long: `Add applications to your DevEx configuration interactively.

This command allows you to:
  • Browse and search available applications
  • Filter by category
  • Add applications to your personal configuration
  • Validate configuration before saving

Examples:
  # Interactive application browser
  devex add
  
  # Add a specific application by name
  devex add git
  
  # Browse applications in a specific category
  devex add --category development
  
  # Search for applications
  devex add --search docker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If specific app name provided, add it directly
			if len(args) > 0 {
				return addSpecificApp(args[0], settings, dryRun)
			}

			// Interactive mode
			model := NewAddModel(settings)

			// Apply filters if provided
			if category != "" {
				model.filterByCategory(category)
			}
			if search != "" {
				model.list.SetFilteringEnabled(true)
			}

			p := tea.NewProgram(model, tea.WithAltScreen())
			finalModel, err := p.Run()
			if err != nil {
				return fmt.Errorf("failed to run TUI: %w", err)
			}

			// Handle the result
			if addModel, ok := finalModel.(AddModel); ok && addModel.choice != "" {
				fmt.Println(addModel.choice)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", "Filter by application category")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Search for applications")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be added without making changes")

	return cmd
}

// filterByCategory filters the list by a specific category
func (m *AddModel) filterByCategory(category string) {
	allItems := m.list.Items()
	filteredItems := make([]list.Item, 0)

	for _, item := range allItems {
		if appItem, ok := item.(AppItem); ok {
			if strings.EqualFold(appItem.app.Category, category) {
				filteredItems = append(filteredItems, item)
			}
		}
	}

	m.list.SetItems(filteredItems)
	m.list.Title = fmt.Sprintf("Applications in category: %s", category)
}

// addSpecificApp adds a specific application by name
func addSpecificApp(appName string, settings config.CrossPlatformSettings, dryRun bool) error {
	// Find the application
	availableApps := settings.GetApplications()
	var targetApp *types.AppConfig

	for _, app := range availableApps {
		if strings.EqualFold(app.Name, appName) {
			targetApp = &app
			break
		}
	}

	if targetApp == nil {
		return fmt.Errorf("application '%s' not found", appName)
	}

	apps, err := configuredEditableApplications(settings)
	if err != nil {
		return err
	}
	for _, app := range apps {
		if strings.EqualFold(app.Name, targetApp.Name) {
			return fmt.Errorf("application '%s' is already in your configuration", appName)
		}
	}

	if dryRun {
		fmt.Printf("Would add application '%s' to configuration\n", targetApp.Name)
		fmt.Printf("Description: %s\n", targetApp.Description)
		fmt.Printf("Category: %s\n", targetApp.Category)
		return nil
	}

	// Add to configuration
	model := &AddModel{settings: settings}
	if err := model.addAppToConfig(*targetApp); err != nil {
		return fmt.Errorf("failed to add application: %w", err)
	}

	fmt.Printf("Successfully added '%s' to your configuration\n", targetApp.Name)
	return nil
}
