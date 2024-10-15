package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/pkg/themes"
	"os"
	"os/exec"
	"path/filepath"
)

type menuState int

const (
	mainMenu menuState = iota
	themeMenu
)

type model struct {
	state        menuState
	cursor       int
	submenuItems []string
	themeList    []themes.Theme
}

// Main menu options
var mainMenuOptions = []string{"Theme", "Quit"}

func initialModel(themeList []themes.Theme) model {
	return model{
		state:     mainMenu,
		cursor:    0,
		themeList: themeList, // Assign the loaded themes to the model
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.submenuItems)-1 {
				m.cursor++
			}
		case "enter":
			if m.state == themeMenu {
				if m.submenuItems[m.cursor] == "Back" {
					m.state = mainMenu
					m.cursor = 0
					return m, nil
				}
				selectedTheme := m.themeList[m.cursor]
				applyGnomeAndNeovimSettings(selectedTheme.ThemeColor, selectedTheme.ThemeBackground, selectedTheme.NeovimColorscheme)
			}
			return handleMainMenuSelection(m)
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.state {
	case mainMenu:
		return mainMenuView(m)
	case themeMenu:
		return submenuView(m)
	default:
		return "Unknown state!"
	}
}

func mainMenuView(m model) string {
	s := "Main Menu:\n\n"
	for i, option := range mainMenuOptions {
		cursor := " "
		if m.cursor == i {
			cursor = ">" // cursor to indicate selection
		}
		s += fmt.Sprintf("%s %s\n", cursor, option)
	}
	return s + "\nUse arrow keys to navigate, Enter to select."
}

func submenuView(m model) string {
	s := "Submenu:\n\n"
	for i, option := range m.submenuItems {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, option)
	}
	return s + "\nUse arrow keys to navigate, Enter to select."
}

func handleMainMenuSelection(m model) (model, tea.Cmd) {
	switch mainMenuOptions[m.cursor] {
	case "Theme":
		m.state = themeMenu
		// Populate submenuItems based on available themes
		m.submenuItems = make([]string, len(m.themeList))
		for i, theme := range m.themeList {
			m.submenuItems[i] = theme.Name
		}
		m.submenuItems = append(m.submenuItems, "Back") // Add Back option
		m.cursor = 0
	case "Quit":
		return m, tea.Quit // Trigger the tea.Quit command to exit the program
	}
	return m, nil
}

func applyGnomeAndNeovimSettings(themeColor, themeBackground, neovimColorscheme string) error {
	// Set the GNOME interface and background
	commands := []string{
		fmt.Sprintf("gsettings set org.gnome.desktop.interface color-scheme 'prefer-dark'"),
		fmt.Sprintf("gsettings set org.gnome.desktop.interface cursor-theme 'Yaru'"),
		fmt.Sprintf("gsettings set org.gnome.desktop.interface gtk-theme 'Yaru-%s-dark'", themeColor),
		fmt.Sprintf("gsettings set org.gnome.desktop.interface icon-theme 'Yaru-%s'", themeColor),
	}

	backgroundOrgPath := filepath.Join(os.Getenv("HOME"), ".local/share/devex/themes", themeBackground)
	backgroundDestDir := filepath.Join(os.Getenv("HOME"), ".local/share/backgrounds")
	backgroundDestPath := filepath.Join(backgroundDestDir, filepath.Base(themeBackground))

	commands = append(commands, fmt.Sprintf("mkdir -p %s", backgroundDestDir))
	commands = append(commands, fmt.Sprintf("cp %s %s", backgroundOrgPath, backgroundDestPath))
	commands = append(commands, fmt.Sprintf("gsettings set org.gnome.desktop.background picture-uri 'file://%s'", backgroundDestPath))
	commands = append(commands, fmt.Sprintf("gsettings set org.gnome.desktop.background picture-uri-dark 'file://%s'", backgroundDestPath))
	commands = append(commands, "gsettings set org.gnome.desktop.background picture-options 'zoom'")

	// Execute each command
	for _, cmdStr := range commands {
		cmd := exec.Command("bash", "-c", cmdStr)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to execute command %s: %v", cmdStr, err)
		}
	}

	// Handle Neovim theme copy
	if neovimColorscheme != "" {
		neovimDestDir := filepath.Join(os.Getenv("HOME"), ".config/nvim/lua/plugins/")
		neovimDestPath := filepath.Join(neovimDestDir, "theme.lua")
		neovimSourcePath := filepath.Join(os.Getenv("HOME"), ".local/share/devex", neovimColorscheme)

		err := os.MkdirAll(neovimDestDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create Neovim directory: %v", err)
		}

		// Copy the Neovim colorscheme
		cmd := exec.Command("cp", neovimSourcePath, neovimDestPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to copy Neovim colorscheme: %v", err)
		}

		fmt.Println("Neovim theme applied successfully.")
	}

	return nil
}

func main() {
	// Load themes from YAML file
	themeList, err := themes.LoadThemes("config/themes.yaml")
	if err != nil {
		fmt.Printf("Error loading themes: %v\n", err)
		return
	}

	// Start the Bubble Tea program with the loaded themes
	p := tea.NewProgram(initialModel(themeList))
	if err := p.Start(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
}
