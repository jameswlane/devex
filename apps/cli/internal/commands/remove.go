package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

// RemoveAppItem represents an application item for removal
type RemoveAppItem struct {
	app           types.AppConfig
	hasDependents bool
	dependentApps []string
	canRemove     bool
	removalRisks  []string
}

func (i RemoveAppItem) FilterValue() string { return i.app.Name }
func (i RemoveAppItem) Title() string       { return i.app.Name }
func (i RemoveAppItem) Description() string {
	desc := i.app.Description
	if i.hasDependents {
		desc += fmt.Sprintf(" (Required by: %s)", strings.Join(i.dependentApps, ", "))
	}
	if !i.canRemove {
		desc += " ⚠️ Cannot remove safely"
	}
	return desc
}

// RemoveModel represents the TUI state for the remove command
type RemoveModel struct {
	list     list.Model
	choice   string
	quitting bool
	settings config.CrossPlatformSettings
	width    int
	height   int
}

// BackupInfo contains information about a configuration backup
type BackupInfo struct {
	Timestamp time.Time `yaml:"timestamp"`
	Apps      []string  `yaml:"apps"`
	FilePath  string    `yaml:"file_path"`
}

// NewRemoveModel creates a new remove command TUI model
func NewRemoveModel(settings config.CrossPlatformSettings) *RemoveModel {
	// Get currently configured applications
	userConfigDir := settings.GetConfigDir()
	userAppsPath := filepath.Join(userConfigDir, "applications.yaml")
	var configuredApps []types.AppConfig

	if data, err := os.ReadFile(userAppsPath); err == nil {
		var userConfig struct {
			Applications []types.AppConfig `yaml:"applications"`
		}
		if yaml.Unmarshal(data, &userConfig) == nil {
			configuredApps = userConfig.Applications
		}
	}

	// Create dependency map
	dependencyMap := buildDependencyMap(configuredApps)

	// Create list items
	items := make([]list.Item, 0, len(configuredApps))
	for _, app := range configuredApps {
		dependentApps := getDependentApps(app.Name, dependencyMap)
		risks := calculateRemovalRisks(app, dependentApps)

		items = append(items, RemoveAppItem{
			app:           app,
			hasDependents: len(dependentApps) > 0,
			dependentApps: dependentApps,
			canRemove:     len(dependentApps) == 0 || isOptionalDependency(app),
			removalRisks:  risks,
		})
	}

	// Sort items by name
	sort.Slice(items, func(i, j int) bool {
		itemI, okI := items[i].(RemoveAppItem)
		itemJ, okJ := items[j].(RemoveAppItem)
		if !okI || !okJ {
			return false
		}
		return itemI.app.Name < itemJ.app.Name
	})

	l := list.New(items, RemoveItemDelegate{}, 80, 14)
	l.Title = "Select Applications to Remove"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = removeStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	return &RemoveModel{
		list:     l,
		settings: settings,
	}
}

// RemoveItemDelegate renders list items for removal
type RemoveItemDelegate struct{}

func (d RemoveItemDelegate) Height() int                             { return 2 }
func (d RemoveItemDelegate) Spacing() int                            { return 1 }
func (d RemoveItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d RemoveItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(RemoveAppItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("📦 %s", i.app.Name)

	if i.hasDependents {
		str += " ⚠️"
	}

	var style lipgloss.Style
	if index == m.Index() {
		if i.canRemove {
			style = selectedRemovableStyle
		} else {
			style = selectedBlockedStyle
		}
	} else {
		if i.canRemove {
			style = removableStyle
		} else {
			style = blockedStyle
		}
	}

	fmt.Fprint(w, style.Render(str))

	// Add warning line if needed
	if i.hasDependents {
		warningText := fmt.Sprintf("  Dependencies: %s", strings.Join(i.dependentApps, ", "))
		if index == m.Index() {
			fmt.Fprint(w, "\n"+warningStyle.Render(warningText))
		} else {
			fmt.Fprint(w, "\n"+mutedStyle.Render(warningText))
		}
	}
}

var (
	removeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#FF6B6B")).
			Padding(0, 1)

	removableStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color("#90EE90"))

	blockedStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color("#FFB6C1"))

	selectedRemovableStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("#00FF00")).
				Bold(true)

	selectedBlockedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("#FF4444")).
				Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Italic(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080")).
			Italic(true)
)

func (m RemoveModel) Init() tea.Cmd {
	return nil
}

func (m RemoveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			item, ok := m.list.SelectedItem().(RemoveAppItem)
			if ok {
				if !item.canRemove {
					m.choice = fmt.Sprintf("Cannot remove '%s': required by %s",
						item.app.Name, strings.Join(item.dependentApps, ", "))
				} else {
					// Remove the application from user config
					if err := m.removeAppFromConfig(item.app, true); err != nil {
						m.choice = fmt.Sprintf("Error removing %s: %v", item.app.Name, err)
					} else {
						m.choice = fmt.Sprintf("Successfully removed %s from your configuration", item.app.Name)
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

func (m RemoveModel) View() string {
	if m.choice != "" {
		return quitTextStyle.Render(m.choice + "\n")
	}
	if m.quitting {
		return quitTextStyle.Render("Cancelled removing applications.\n")
	}

	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Render("Green = Safe to remove, Red = Has dependencies, ⚠️ = Warning")

	return instructions + "\n\n" + m.list.View()
}

// removeAppFromConfig removes an application from the user's configuration
func (m *RemoveModel) removeAppFromConfig(app types.AppConfig, createBackup bool) error {
	userConfigDir := m.settings.GetConfigDir()
	userAppsPath := filepath.Join(userConfigDir, "applications.yaml")

	// Read existing configuration
	var userConfig struct {
		Applications []types.AppConfig `yaml:"applications"`
	}

	data, err := os.ReadFile(userAppsPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &userConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Create undo operation before modification
	baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
	undoManager := undo.NewUndoManager(baseDir)

	metadata := map[string]interface{}{
		"app_name":        app.Name,
		"app_category":    app.Category,
		"app_description": app.Description,
		"create_backup":   createBackup,
	}

	undoOp, err := undoManager.RecordOperation("remove",
		fmt.Sprintf("Removed application: %s", app.Name),
		app.Name,
		metadata)
	if err != nil {
		// Log warning but don't block the operation
		fmt.Fprintf(os.Stderr, "Warning: Failed to record undo operation: %v\n", err)
	}

	// Find and remove the application
	newApps := make([]types.AppConfig, 0, len(userConfig.Applications))
	found := false
	for _, existingApp := range userConfig.Applications {
		if existingApp.Name != app.Name {
			newApps = append(newApps, existingApp)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("application not found in configuration")
	}

	userConfig.Applications = newApps

	// Write back to file
	newData, err := yaml.Marshal(userConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(userAppsPath, newData, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Create new version after successful configuration change
	vm := version.NewVersionManager(baseDir)
	_, versionErr := vm.UpdateVersion(
		fmt.Sprintf("Removed application: %s", app.Name),
		[]string{fmt.Sprintf("Removed %s from applications configuration", app.Name)},
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

// createBackup creates a backup of the current configuration
func (m *RemoveModel) createBackup(apps []types.AppConfig) error {
	userConfigDir := m.settings.GetConfigDir()
	backupDir := filepath.Join(userConfigDir, "backups")

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("applications_%s.yaml", timestamp))

	// Create backup data
	backupData := struct {
		Applications []types.AppConfig `yaml:"applications"`
		BackupInfo   BackupInfo        `yaml:"backup_info"`
	}{
		Applications: apps,
		BackupInfo: BackupInfo{
			Timestamp: time.Now(),
			Apps:      getAppNames(apps),
			FilePath:  backupFile,
		},
	}

	data, err := yaml.Marshal(backupData)
	if err != nil {
		return fmt.Errorf("failed to marshal backup data: %w", err)
	}

	if err := os.WriteFile(backupFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// buildDependencyMap creates a map of applications and their dependencies
func buildDependencyMap(apps []types.AppConfig) map[string][]string {
	depMap := make(map[string][]string)
	for _, app := range apps {
		depMap[app.Name] = app.Dependencies
	}
	return depMap
}

// getDependentApps finds applications that depend on the given app
func getDependentApps(appName string, dependencyMap map[string][]string) []string {
	var dependents []string
	for app, deps := range dependencyMap {
		for _, dep := range deps {
			if dep == appName {
				dependents = append(dependents, app)
				break
			}
		}
	}
	return dependents
}

// calculateRemovalRisks assesses the risks of removing an application
func calculateRemovalRisks(app types.AppConfig, dependentApps []string) []string {
	var risks []string

	if len(dependentApps) > 0 {
		risks = append(risks, "Has dependent applications")
	}

	// Check for system-critical apps
	criticalApps := []string{"git", "curl", "wget", "ssh"}
	for _, critical := range criticalApps {
		if app.Name == critical {
			risks = append(risks, "System-critical application")
			break
		}
	}

	return risks
}

// isOptionalDependency checks if the app is marked as optional
func isOptionalDependency(app types.AppConfig) bool {
	// This could be extended to check for optional dependency markers
	return !app.Default
}

// getAppNames extracts app names from a slice of AppConfig
func getAppNames(apps []types.AppConfig) []string {
	names := make([]string, len(apps))
	for i, app := range apps {
		names[i] = app.Name
	}
	return names
}

// NewRemoveCmd creates a new remove command
func NewRemoveCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		force    bool
		noBackup bool
		cascade  bool
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "remove [application-name]",
		Short: "Remove applications from your DevEx configuration",
		Long: `Remove applications from your DevEx configuration safely.

This command allows you to:
  • Browse and select configured applications for removal
  • Check for dependency conflicts before removal
  • Create automatic backups before making changes
  • Force removal of applications with dependencies
  • Cascade removal of dependent applications

Examples:
  # Interactive application removal browser
  devex remove
  
  # Remove a specific application by name
  devex remove git
  
  # Remove without creating backup
  devex remove git --no-backup
  
  # Force removal even if dependencies exist
  devex remove git --force
  
  # Remove with cascade (remove dependents too)
  devex remove docker --cascade`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If specific app name provided, remove it directly
			if len(args) > 0 {
				return removeSpecificApp(args[0], settings, force, noBackup, cascade, dryRun)
			}

			// Interactive mode
			model := NewRemoveModel(settings)

			p := tea.NewProgram(model, tea.WithAltScreen())
			finalModel, err := p.Run()
			if err != nil {
				return fmt.Errorf("failed to run TUI: %w", err)
			}

			// Handle the result
			if removeModel, ok := finalModel.(RemoveModel); ok && removeModel.choice != "" {
				fmt.Println(removeModel.choice)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force removal even if dependencies exist")
	cmd.Flags().BoolVar(&noBackup, "no-backup", false, "Don't create backup before removal")
	cmd.Flags().BoolVar(&cascade, "cascade", false, "Remove dependent applications as well")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be removed without making changes")

	return cmd
}

// removeSpecificApp removes a specific application by name
func removeSpecificApp(appName string, settings config.CrossPlatformSettings, force, noBackup, cascade, dryRun bool) error {
	userConfigDir := settings.GetConfigDir()
	userAppsPath := filepath.Join(userConfigDir, "applications.yaml")

	// Read current configuration
	var userConfig struct {
		Applications []types.AppConfig `yaml:"applications"`
	}

	data, err := os.ReadFile(userAppsPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &userConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Find the target application
	var targetApp *types.AppConfig
	for _, app := range userConfig.Applications {
		if strings.EqualFold(app.Name, appName) {
			targetApp = &app
			break
		}
	}

	if targetApp == nil {
		return fmt.Errorf("application '%s' not found in configuration", appName)
	}

	// Check dependencies
	dependencyMap := buildDependencyMap(userConfig.Applications)
	dependentApps := getDependentApps(targetApp.Name, dependencyMap)

	if len(dependentApps) > 0 && !force && !cascade {
		return fmt.Errorf("cannot remove '%s': required by %s (use --force or --cascade)",
			targetApp.Name, strings.Join(dependentApps, ", "))
	}

	if dryRun {
		fmt.Printf("Would remove application '%s'\n", targetApp.Name)
		if len(dependentApps) > 0 {
			if cascade {
				fmt.Printf("Would also remove dependent apps: %s\n", strings.Join(dependentApps, ", "))
			} else if force {
				fmt.Printf("Warning: This will break dependencies for: %s\n", strings.Join(dependentApps, ", "))
			}
		}
		return nil
	}

	// Create backup
	if !noBackup {
		model := &RemoveModel{settings: settings}
		if err := model.createBackup(userConfig.Applications); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		fmt.Println("✅ Backup created successfully")
	}

	// Remove the application
	model := &RemoveModel{settings: settings}
	if err := model.removeAppFromConfig(*targetApp, false); err != nil { // Don't double-backup
		return fmt.Errorf("failed to remove application: %w", err)
	}

	fmt.Printf("✅ Successfully removed '%s' from your configuration\n", targetApp.Name)

	// Handle cascade removal
	if cascade && len(dependentApps) > 0 {
		fmt.Printf("🔄 Removing dependent applications...\n")
		for _, depApp := range dependentApps {
			if err := removeSpecificApp(depApp, settings, true, true, false, false); err != nil {
				fmt.Printf("⚠️ Failed to remove dependent app '%s': %v\n", depApp, err)
			} else {
				fmt.Printf("✅ Removed dependent app '%s'\n", depApp)
			}
		}
	}

	return nil
}
