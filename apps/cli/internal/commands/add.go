package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
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
				m.choice = item.app.Name
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
	plan, err := planAddition(context.Background(), m.settings, app.Name)
	if err != nil {
		return err
	}
	return saveAddition(m.settings, plan)
}

func saveAddition(settings config.CrossPlatformSettings, plan []types.CrossPlatformApp) error {
	return saveApplications(settings, plan, false)
}

// saveApplications backs up the editable tree, then writes plan. With replace
// the last definition supersedes any existing definition of the same name;
// otherwise an existing file is never overwritten.
func saveApplications(settings config.CrossPlatformSettings, plan []types.CrossPlatformApp, replace bool) error {
	userConfigDir := settings.GetConfigDir()
	app := plan[len(plan)-1]

	// Back up the editable tree before migration or writing any application.
	// The legacy undo manager backs up the user override tier, not this tree.
	if err := os.MkdirAll(userConfigDir, 0750); err != nil {
		return err
	}
	if _, err := backup.NewBackupManager(devexpaths.ForHome(settings.HomeDir).Data).CreateBackup(backup.BackupOptions{Description: "Before adding " + app.Name, Type: "pre-add", Compress: true}); err != nil {
		return fmt.Errorf("backup configuration before adding: %w", err)
	}
	// Create undo operation before modification
	baseDir := devexpaths.ForHome(settings.HomeDir).Config
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
		fmt.Fprintf(os.Stderr, "Warning: Failed to record undo operation: %v\n", err)
	}

	if err := migrateApplicationConfig(userConfigDir); err != nil {
		return err
	}
	for _, definition := range plan {
		path, err := applicationConfigPath(userConfigDir, "applications", definition.Category, definition.Name)
		if err != nil {
			return err
		}
		if replace {
			existing, err := applicationConfigFiles(userConfigDir, definition.Name)
			if err != nil {
				return err
			}
			if err := replaceApplicationConfig(definition, path, existing); err != nil {
				return err
			}
			continue
		}
		if err := writeNewApplicationConfig(definition, path); err != nil {
			return err
		}
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
		yes      bool
		format   string
	)

	cmd := &cobra.Command{
		Use:          "add [application-name]",
		Annotations:  map[string]string{catalogAnnotation: catalogRequired},
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Short:        "Add applications to your DevEx configuration",
		Long: `Add applications to your DevEx configuration interactively.

This command allows you to:
  • Browse and search available applications
  • Filter by category
  • Add applications to your personal configuration
  • Resolve catalog dependencies and preview configuration changes
  • Validate configuration before saving

Custom tools: use --custom with NAME and per-platform flags
(--linux-method/--linux-package, --macos-..., --windows-..., --all-...) plus
optional --<platform>-pre-install/--<platform>-post-install hook commands.
Methods must have a DevEx package-manager plugin; packages are single
identifiers, never shell text. Missing fields are prompted for when a terminal
is attached. --force replaces an existing definition after a backup.

Confirm the preview interactively, or use --yes for unattended changes.
Use --dry-run to validate and preview without saving. Unknown dependencies
remain native packages; they are not added as DevEx application definitions.

Examples:
  # Interactive application browser
  devex add
  
  # Add a specific application by name
  devex add git
  
  # Browse applications in a specific category
  devex add --category development
  
  # Search for applications
  devex add --search docker

  # Define a custom tool that is not in the catalog (preview, then write)
  devex add mytool --custom --description "My internal tool" \
    --linux-method apt --linux-package mytool \
    --macos-method brew --macos-package acme/tap/mytool \
    --linux-post-install "mytool --version" --dry-run
  devex add mytool --custom --linux-method apt --linux-package mytool --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if custom, _ := cmd.Flags().GetBool("custom"); custom {
				return runAddCustom(cmd, args, settings, customAddOptions{
					DryRun: viper.GetBool("add.dry-run"),
					Yes:    viper.GetBool("add.yes"),
					Force:  viper.GetBool("add.force"),
					Format: viper.GetString("add.format"),
				})
			}
			if err := rejectCustomFlagsWithoutCustom(cmd); err != nil {
				return err
			}
			// If specific app name provided, add it directly
			if len(args) > 0 {
				return runAdd(cmd, args[0], settings, viper.GetBool("add.dry-run"), viper.GetBool("add.yes"), viper.GetString("add.format"))
			}

			if !interaction.IsInteractiveMode() || viper.GetString("add.format") == "json" {
				return fmt.Errorf("specify an application name in unattended mode")
			}
			// Interactive mode
			model := NewAddModel(settings)

			// Apply filters if provided
			if viper.GetString("add.category") != "" {
				model.filterByCategory(viper.GetString("add.category"))
			}
			if viper.GetString("add.search") != "" {
				model.list.SetFilteringEnabled(true)
			}

			p := tea.NewProgram(model, tea.WithAltScreen())
			finalModel, err := p.Run()
			if err != nil {
				return fmt.Errorf("failed to run TUI: %w", err)
			}

			// Handle the result
			if addModel, ok := finalModel.(AddModel); ok && addModel.choice != "" {
				return runAdd(cmd, addModel.choice, settings, viper.GetBool("add.dry-run"), viper.GetBool("add.yes"), viper.GetString("add.format"))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", "Filter by application category (with --custom: the new application's category)")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Search for applications")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be added without making changes")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Accept the preview and add dependencies")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	registerCustomFlags(cmd)
	_ = viper.BindPFlag("add.dry-run", cmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("add.yes", cmd.Flags().Lookup("yes"))
	_ = viper.BindPFlag("add.format", cmd.Flags().Lookup("format"))
	_ = viper.BindPFlag("add.category", cmd.Flags().Lookup("category"))
	_ = viper.BindPFlag("add.search", cmd.Flags().Lookup("search"))

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
	plan, err := planAddition(context.Background(), settings, appName)
	if err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	return saveAddition(settings, plan)
}

func runAdd(cmd *cobra.Command, name string, settings config.CrossPlatformSettings, dryRun, yes bool, format string) error {
	if format != "table" && format != "json" {
		return fmt.Errorf("unsupported format %q; use table or json", format)
	}
	plan, err := planAddition(cmd.Context(), settings, name)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(plan))
	for _, app := range plan {
		names = append(names, app.Name)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Configuration preview: add %s\n", strings.Join(names, ", "))
	for _, app := range plan {
		for platform, definition := range map[string]types.OSConfig{"linux": app.Linux, "macos": app.MacOS, "windows": app.Windows, "all_platforms": app.AllPlatforms} {
			if len(definition.Dependencies) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s %s dependencies: %s (uncatalogued names remain native packages)\n", app.Name, platform, strings.Join(definition.Dependencies, ", "))
			}
		}
	}
	if !dryRun && !yes {
		if !interaction.IsInteractiveMode() || format == "json" {
			return fmt.Errorf("confirmation required; review with --dry-run, then use --yes to add applications and dependencies")
		}
		fmt.Fprint(cmd.ErrOrStderr(), "Add these applications and dependencies? [y/N]: ")
		answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if err != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
			return fmt.Errorf("addition cancelled; configuration unchanged")
		}
	}
	if err := cmd.Context().Err(); err != nil {
		return err
	}
	if !dryRun {
		if err := saveAddition(settings, plan); err != nil {
			return err
		}
	}
	if format == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
			Added  []string `json:"added"`
			DryRun bool     `json:"dry_run"`
		}{names, dryRun})
	}
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Would add: %s\n", strings.Join(names, ", "))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Added: %s\n", strings.Join(names, ", "))
	}
	return nil
}
