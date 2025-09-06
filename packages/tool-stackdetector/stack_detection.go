package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// handleDetect performs basic stack detection and displays results
func (p *StackDetectorPlugin) handleDetect(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Validate directory
	if err := p.validateDirectory(dir); err != nil {
		return err
	}

	fmt.Printf("🔍 Detecting development stack in: %s\n", dir)

	// Perform detection
	technologies := p.detectStack(dir)
	if len(technologies) == 0 {
		fmt.Println("❌ No recognizable technology stack detected")
		fmt.Println("💡 This might be a new project or use uncommon technologies")
		return nil
	}

	// Display results
	p.displayDetectionResults(technologies)

	return nil
}

// validateDirectory ensures the target directory exists and is accessible
func (p *StackDetectorPlugin) validateDirectory(dir string) error {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("cannot access directory '%s': %w", dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("'%s' is not a directory", dir)
	}

	return nil
}

// displayDetectionResults formats and displays the detection results
func (p *StackDetectorPlugin) displayDetectionResults(technologies []Technology) {
	fmt.Printf("✅ Detected %d technologies:\n\n", len(technologies))

	// Group by category
	categories := make(map[string][]Technology)
	for _, tech := range technologies {
		categories[tech.Category] = append(categories[tech.Category], tech)
	}

	// Display by category
	categoryOrder := []string{"Language", "Runtime", "Framework", "Build System", "Package Manager", "Build Tool", "CSS Framework", "Containerization", "Orchestration", "Version Control", "Dependencies", "Build Output", "Virtual Environment", "Linting", "Code Formatting"}
	
	for _, category := range categoryOrder {
		if techs, exists := categories[category]; exists {
			fmt.Printf("📂 %s:\n", category)
			for _, tech := range techs {
				confidence := p.getConfidenceEmoji(tech.Confidence)
				fmt.Printf("  %s %s\n", confidence, tech.Name)
				if tech.Description != "" {
					fmt.Printf("     %s\n", tech.Description)
				}
			}
			fmt.Println()
		}
	}

	// Handle any remaining categories not in the predefined order
	for category, techs := range categories {
		inOrder := false
		for _, orderedCategory := range categoryOrder {
			if category == orderedCategory {
				inOrder = true
				break
			}
		}
		if !inOrder {
			fmt.Printf("📂 %s:\n", category)
			for _, tech := range techs {
				confidence := p.getConfidenceEmoji(tech.Confidence)
				fmt.Printf("  %s %s\n", confidence, tech.Name)
				if tech.Description != "" {
					fmt.Printf("     %s\n", tech.Description)
				}
			}
			fmt.Println()
		}
	}

	fmt.Println("💡 Use 'stackdetector analyze' for detailed project analysis")
	fmt.Println("📄 Use 'stackdetector report' to generate a comprehensive report")
}

// getConfidenceEmoji returns an emoji representing the confidence level
func (p *StackDetectorPlugin) getConfidenceEmoji(confidence int) string {
	switch {
	case confidence >= 9:
		return "🟢" // High confidence
	case confidence >= 7:
		return "🟡" // Medium confidence
	case confidence >= 5:
		return "🟠" // Low-medium confidence
	default:
		return "🔴" // Low confidence
	}
}