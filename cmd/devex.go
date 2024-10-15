package main

import (
	"flag"
	"fmt"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/logger"
	"github.com/jameswlane/devex/pkg/steps"
	"github.com/jameswlane/devex/pkg/view"
	"golang.org/x/term"
	"os"
	"syscall"
)

func main() {
	// Dry-run flag
	var dryRun bool
	flag.BoolVar(&dryRun, "dry-run", false, "Simulate the installation process without making changes")
	flag.Parse()

	// Initialize logger and database
	db, err := datastore.InitDB(fmt.Sprintf("%s/.devex/apps.db", os.Getenv("HOME")))
	if err != nil {
		fmt.Println("Failed to initialize database:", err)
		return
	}
	defer db.Close()

	// Get terminal size
	width, height, err := term.GetSize(int(syscall.Stdout))
	if err != nil {
		width, height = 80, 24 // Fallback size
	}

	// Initialize custom logger
	log := logger.InitLogger()

	// Initialize view model with system info
	viewModel := view.NewViewModel("CPU: 2.3 GHz | RAM: 16 GB | Disk: 256 GB SSD", width, height)

	// Generate and execute steps from YAML files
	stepsList, err := steps.GenerateSteps()
	if err != nil {
		log.LogError("Failed to generate steps", err)
		return
	}

	// Execute the steps
	viewModel.ExecuteSteps(stepsList, dryRun, db, log)

	// Render the layout
	view := viewModel.Render()

	// Output the view to the terminal
	fmt.Print("\033[H\033[2J") // Clears screen
	fmt.Println(view)
}
