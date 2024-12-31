package cmd

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jameswlane/devex/ui"
)

func loadProgrammingLanguages(shared map[string][]string) *ui.PopupModel {
	popup := &ui.PopupModel{}
	popup.Reset(
		"Select Programming Languages",
		"Programming Languages",
		[]string{"Python", "Go", "Ruby", "Node.js"},
		func() *ui.PopupModel { return loadDatabases(shared) },
	)
	popup.Shared = shared
	return popup
}

func loadDatabases(shared map[string][]string) *ui.PopupModel {
	popup := &ui.PopupModel{}
	popup.Reset(
		"Select Databases",
		"Databases",
		[]string{"PostgreSQL", "MySQL", "SQLite"},
		func() *ui.PopupModel { return loadThemes(shared) },
	)
	popup.Shared = shared
	return popup
}

func loadThemes(shared map[string][]string) *ui.PopupModel {
	popup := &ui.PopupModel{}
	popup.Reset(
		"Select Themes",
		"Themes",
		[]string{"Light", "Dark", "Solarized"},
		nil, // No further popups
	)
	popup.Shared = shared
	return popup
}

func Execute() {
	// Initialize the program model
	model := ui.Model{
		Layout:  ui.Layout{},
		TaskLog: "Starting installation...\n",
		Dialog: &ui.Dialog{
			Title:   "Confirm Exit",
			Message: "Are you sure you want to quit?",
			Buttons: []string{"Yes", "No"},
			Active:  0,
		},
	}

	// Run the program and capture the final model and error
	finalModel, err := tea.NewProgram(&model).Run()
	if err != nil {
		log.Fatalf("Error starting program: %v", err)
	}

	// Process the final model
	log.Printf("Final model state: %+v\n", finalModel)
	log.Println("Program exited successfully.")
}

var dummyInstallHooks = []string{
	"Validate system requirements",
	"Backup existing files",
	"Remove conflicting packages",
	"Run pre-install commands",
	"Set up environment",
	"Handle dependencies",
	"Handle APT sources",
	"Run apt-get update to refresh package lists",
	"Process config files",
	"Process themes",
	"Execute the installation",
	"Run post-install commands",
	"Perform cleanup",
}

func printSelections(popup *ui.PopupModel) {
	for popup != nil {
		fmt.Printf("%s:\n", popup.Title)
		for i, selected := range popup.Selected {
			if selected {
				fmt.Printf("- %s\n", popup.Options[i])
			}
		}
		// Safely transition to the next category
		if popup.Next != nil {
			popup = popup.Next()
		} else {
			popup = nil
		}
	}
}
