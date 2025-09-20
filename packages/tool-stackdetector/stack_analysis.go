package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectAnalysis represents comprehensive project analysis results
type ProjectAnalysis struct {
	Technologies    []Technology
	Dependencies    []Dependency
	ProjectSize     ProjectSize
	Recommendations []string
	Issues          []string
}

// Dependency represents a project dependency
type Dependency struct {
	Name    string
	Version string
	Type    string // "direct", "dev", "peer"
	Source  string // file where found
}

// ProjectSize represents project size metrics
type ProjectSize struct {
	TotalFiles    int
	CodeFiles     int
	ConfigFiles   int
	DocumentFiles int
	LinesOfCode   int
}

// handleAnalyze performs deep project analysis
func (p *StackDetectorPlugin) handleAnalyze(args []string) error {
	dir := "."
	verbose := false

	// Parse arguments
	for i, arg := range args {
		if arg == "--verbose" {
			verbose = true
		} else if i == 0 && !strings.HasPrefix(arg, "--") {
			dir = arg
		}
	}

	// Validate directory
	if err := p.ValidateDirectory(dir); err != nil {
		return err
	}

	fmt.Printf("🔬 Analyzing project structure in: %s\n", dir)

	// Perform comprehensive analysis
	analysis, err := p.performProjectAnalysis(dir)
	if err != nil {
		return fmt.Errorf("failed to analyze project: %w", err)
	}

	// Display results
	p.displayAnalysisResults(analysis, verbose)

	return nil
}

// performProjectAnalysis conducts comprehensive project analysis
func (p *StackDetectorPlugin) performProjectAnalysis(dir string) (*ProjectAnalysis, error) {
	analysis := &ProjectAnalysis{}

	// Technology detection
	analysis.Technologies = p.DetectStack(dir)

	// Dependency analysis
	var err error
	analysis.Dependencies, err = p.analyzeDependencies(dir)
	if err != nil {
		return nil, fmt.Errorf("dependency analysis failed: %w", err)
	}

	// Project size analysis
	analysis.ProjectSize, err = p.analyzeProjectSize(dir)
	if err != nil {
		return nil, fmt.Errorf("project size analysis failed: %w", err)
	}

	// Generate recommendations
	analysis.Recommendations = p.generateRecommendations(analysis)

	// Identify issues
	analysis.Issues = p.identifyIssues(dir, analysis)

	return analysis, nil
}

// analyzeDependencies extracts and analyzes project dependencies
func (p *StackDetectorPlugin) analyzeDependencies(dir string) ([]Dependency, error) {
	var dependencies []Dependency

	// Analyze different dependency files
	depAnalyzers := map[string]func(string) ([]Dependency, error){
		"package.json":     p.analyzePackageJson,
		"requirements.txt": p.analyzeRequirementsTxt,
		"go.mod":           p.analyzeGoMod,
		"Cargo.toml":       p.analyzeCargoToml,
		"Gemfile":          p.analyzeGemfile,
	}

	for file, analyzer := range depAnalyzers {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			deps, err := analyzer(path)
			if err != nil {
				fmt.Printf("Warning: failed to analyze %s: %v\n", file, err)
				continue
			}
			dependencies = append(dependencies, deps...)
		}
	}

	return dependencies, nil
}

// analyzePackageJson extracts dependencies from package.json
func (p *StackDetectorPlugin) analyzePackageJson(path string) ([]Dependency, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var dependencies []Dependency

	// Simple parsing - in a real implementation, use JSON parsing
	contentStr := string(content)

	// Extract dependencies section (simplified)
	if strings.Contains(contentStr, "\"dependencies\"") {
		// This is a simplified approach - proper JSON parsing would be better
		dependencies = append(dependencies, Dependency{
			Name:   "package.json dependencies",
			Type:   "direct",
			Source: "package.json",
		})
	}

	return dependencies, nil
}

// analyzeRequirementsTxt extracts dependencies from requirements.txt
func (p *StackDetectorPlugin) analyzeRequirementsTxt(path string) ([]Dependency, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var dependencies []Dependency
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			parts := strings.Split(line, "==")
			name := parts[0]
			version := ""
			if len(parts) > 1 {
				version = parts[1]
			}

			dependencies = append(dependencies, Dependency{
				Name:    name,
				Version: version,
				Type:    "direct",
				Source:  "requirements.txt",
			})
		}
	}

	return dependencies, nil
}

// analyzeGoMod extracts dependencies from go.mod
func (p *StackDetectorPlugin) analyzeGoMod(path string) ([]Dependency, error) {
	// Simplified implementation
	return []Dependency{{
		Name:   "go.mod dependencies",
		Type:   "module",
		Source: "go.mod",
	}}, nil
}

// analyzeCargoToml extracts dependencies from Cargo.toml
func (p *StackDetectorPlugin) analyzeCargoToml(path string) ([]Dependency, error) {
	// Simplified implementation
	return []Dependency{{
		Name:   "Cargo.toml dependencies",
		Type:   "crate",
		Source: "Cargo.toml",
	}}, nil
}

// analyzeGemfile extracts dependencies from Gemfile
func (p *StackDetectorPlugin) analyzeGemfile(path string) ([]Dependency, error) {
	// Simplified implementation
	return []Dependency{{
		Name:   "Gemfile dependencies",
		Type:   "gem",
		Source: "Gemfile",
	}}, nil
}

// analyzeProjectSize calculates project size metrics
func (p *StackDetectorPlugin) analyzeProjectSize(dir string) (ProjectSize, error) {
	size := ProjectSize{}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		// Skip hidden directories and common ignore patterns
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") ||
				name == "node_modules" ||
				name == "vendor" ||
				name == "target" ||
				name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		size.TotalFiles++

		ext := filepath.Ext(info.Name())
		switch ext {
		case ".js", ".ts", ".py", ".go", ".rs", ".rb", ".java", ".kt", ".c", ".cpp", ".h", ".hpp":
			size.CodeFiles++
		case ".json", ".yaml", ".yml", ".toml", ".xml", ".ini", ".cfg":
			size.ConfigFiles++
		case ".md", ".txt", ".rst", ".doc", ".docx":
			size.DocumentFiles++
		}

		return nil
	})

	return size, err
}

// generateRecommendations creates recommendations based on analysis
func (p *StackDetectorPlugin) generateRecommendations(analysis *ProjectAnalysis) []string {
	var recommendations []string

	// Analyze technology combinations
	hasDocker := false
	hasCI := false
	hasLinting := false

	for _, tech := range analysis.Technologies {
		if strings.Contains(strings.ToLower(tech.Name), "docker") {
			hasDocker = true
		}
		if strings.Contains(strings.ToLower(tech.Name), "eslint") ||
			strings.Contains(strings.ToLower(tech.Name), "prettier") {
			hasLinting = true
		}
	}

	// Generate recommendations
	if !hasDocker && analysis.ProjectSize.CodeFiles > 10 {
		recommendations = append(recommendations, "Consider containerizing your application with Docker")
	}

	if !hasLinting && analysis.ProjectSize.CodeFiles > 5 {
		recommendations = append(recommendations, "Add code linting tools (ESLint, Prettier, etc.) for better code quality")
	}

	if !hasCI {
		recommendations = append(recommendations, "Set up continuous integration (CI/CD) pipeline")
	}

	if len(analysis.Technologies) > 5 {
		recommendations = append(recommendations, "Consider simplifying your tech stack to reduce complexity")
	}

	return recommendations
}

// identifyIssues identifies potential issues in the project
func (p *StackDetectorPlugin) identifyIssues(dir string, analysis *ProjectAnalysis) []string {
	var issues []string

	// Check for common issues
	if analysis.ProjectSize.TotalFiles > 1000 {
		issues = append(issues, "Large project size may impact build and development performance")
	}

	// Check for dependency conflicts
	depFiles := 0
	for _, tech := range analysis.Technologies {
		if tech.Category == "Package Manager" {
			depFiles++
		}
	}

	if depFiles > 2 {
		issues = append(issues, "Multiple package managers detected - this may cause dependency conflicts")
	}

	// Check for missing essential files
	essentialFiles := []string{"README.md", ".gitignore"}
	for _, file := range essentialFiles {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err != nil {
			issues = append(issues, fmt.Sprintf("Missing %s file", file))
		}
	}

	return issues
}

// displayAnalysisResults formats and displays the analysis results
func (p *StackDetectorPlugin) displayAnalysisResults(analysis *ProjectAnalysis, verbose bool) {
	fmt.Printf("📊 Analysis Results:\n\n")

	// Project size
	fmt.Printf("📏 Project Size:\n")
	fmt.Printf("  Total Files: %d\n", analysis.ProjectSize.TotalFiles)
	fmt.Printf("  Code Files: %d\n", analysis.ProjectSize.CodeFiles)
	fmt.Printf("  Config Files: %d\n", analysis.ProjectSize.ConfigFiles)
	fmt.Printf("  Documentation: %d\n", analysis.ProjectSize.DocumentFiles)
	fmt.Println()

	// Technologies (summary if not verbose)
	if verbose {
		fmt.Printf("🔧 Technologies (%d detected):\n", len(analysis.Technologies))
		for _, tech := range analysis.Technologies {
			confidence := p.getConfidenceEmoji(tech.Confidence)
			fmt.Printf("  %s %s (%s)\n", confidence, tech.Name, tech.Category)
			if tech.Description != "" {
				fmt.Printf("     %s\n", tech.Description)
			}
		}
	} else {
		fmt.Printf("🔧 Technologies: %d detected (use --verbose for details)\n", len(analysis.Technologies))
	}
	fmt.Println()

	// Dependencies
	if len(analysis.Dependencies) > 0 {
		fmt.Printf("📦 Dependencies (%d found):\n", len(analysis.Dependencies))
		if verbose {
			for _, dep := range analysis.Dependencies {
				fmt.Printf("  - %s", dep.Name)
				if dep.Version != "" {
					fmt.Printf(" (%s)", dep.Version)
				}
				fmt.Printf(" [%s]\n", dep.Source)
			}
		} else {
			fmt.Printf("  Use --verbose to see detailed dependency information\n")
		}
		fmt.Println()
	}

	// Issues
	if len(analysis.Issues) > 0 {
		fmt.Printf("⚠️  Issues Found:\n")
		for _, issue := range analysis.Issues {
			fmt.Printf("  - %s\n", issue)
		}
		fmt.Println()
	}

	// Recommendations
	if len(analysis.Recommendations) > 0 {
		fmt.Printf("💡 Recommendations:\n")
		for _, rec := range analysis.Recommendations {
			fmt.Printf("  - %s\n", rec)
		}
		fmt.Println()
	}

	fmt.Println("📄 Use 'stackdetector report' to generate a comprehensive report")
}
