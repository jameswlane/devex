package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// StackReport represents a comprehensive stack report
type StackReport struct {
	GeneratedAt     time.Time     `json:"generated_at"`
	ProjectPath     string        `json:"project_path"`
	Technologies    []Technology  `json:"technologies"`
	Dependencies    []Dependency  `json:"dependencies"`
	ProjectSize     ProjectSize   `json:"project_size"`
	Recommendations []string      `json:"recommendations"`
	Issues          []string      `json:"issues"`
	Summary         ReportSummary `json:"summary"`
}

// ReportSummary provides high-level project insights
type ReportSummary struct {
	PrimaryLanguage   string `json:"primary_language"`
	ProjectType       string `json:"project_type"`
	ComplexityLevel   string `json:"complexity_level"`
	MaturityLevel     string `json:"maturity_level"`
	RecommendedAction string `json:"recommended_action"`
}

// handleReport generates comprehensive stack reports in various formats
func (p *StackDetectorPlugin) handleReport(args []string) error {
	dir := "."
	format := "text"
	outputPath := ""

	// Parse arguments
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(args) {
				outputPath = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "--") && dir == "." {
				dir = args[i]
			}
		}
	}

	// Validate format
	validFormats := []string{"text", "json", "yaml"}
	if !p.isValidFormat(format, validFormats) {
		return fmt.Errorf("invalid format '%s'. Valid formats: %s", format, strings.Join(validFormats, ", "))
	}

	// Validate directory
	if err := p.ValidateDirectory(dir); err != nil {
		return err
	}

	fmt.Printf("📄 Generating stack report for: %s\n", dir)

	// Perform analysis
	analysis, err := p.performProjectAnalysis(dir)
	if err != nil {
		return fmt.Errorf("failed to analyze project: %w", err)
	}

	// Generate report
	report := p.generateStackReport(dir, analysis)

	// Output report
	if err := p.outputReport(report, format, outputPath); err != nil {
		return fmt.Errorf("failed to output report: %w", err)
	}

	return nil
}

// isValidFormat checks if the provided format is valid
func (p *StackDetectorPlugin) isValidFormat(format string, validFormats []string) bool {
	for _, valid := range validFormats {
		if format == valid {
			return true
		}
	}
	return false
}

// generateStackReport creates a comprehensive stack report
func (p *StackDetectorPlugin) generateStackReport(dir string, analysis *ProjectAnalysis) *StackReport {
	report := &StackReport{
		GeneratedAt:     time.Now(),
		ProjectPath:     dir,
		Technologies:    analysis.Technologies,
		Dependencies:    analysis.Dependencies,
		ProjectSize:     analysis.ProjectSize,
		Recommendations: analysis.Recommendations,
		Issues:          analysis.Issues,
		Summary:         p.generateReportSummary(analysis),
	}

	return report
}

// generateReportSummary creates high-level project insights
func (p *StackDetectorPlugin) generateReportSummary(analysis *ProjectAnalysis) ReportSummary {
	summary := ReportSummary{}

	// Determine primary language
	summary.PrimaryLanguage = p.determinePrimaryLanguage(analysis.Technologies)

	// Determine project type
	summary.ProjectType = p.determineProjectType(analysis.Technologies)

	// Assess complexity level
	summary.ComplexityLevel = p.assessComplexityLevel(analysis)

	// Assess maturity level
	summary.MaturityLevel = p.assessMaturityLevel(analysis)

	// Generate recommended action
	summary.RecommendedAction = p.generateRecommendedAction(analysis)

	return summary
}

// determinePrimaryLanguage identifies the primary programming language
func (p *StackDetectorPlugin) determinePrimaryLanguage(technologies []Technology) string {
	languagePriority := map[string]int{
		"TypeScript": 10,
		"JavaScript": 9,
		"Python":     8,
		"Go":         8,
		"Rust":       8,
		"Java":       7,
		"PHP":        6,
		"Ruby":       6,
		"C++":        5,
		"C":          5,
	}

	var primaryLang string
	highestPriority := 0

	for _, tech := range technologies {
		if tech.Category == "Language" {
			if priority, exists := languagePriority[tech.Name]; exists && priority > highestPriority {
				highestPriority = priority
				primaryLang = tech.Name
			}
		}
	}

	if primaryLang == "" {
		return "Unknown"
	}

	return primaryLang
}

// determineProjectType identifies the type of project
func (p *StackDetectorPlugin) determineProjectType(technologies []Technology) string {
	// Check for web frameworks
	webFrameworks := []string{"React", "Vue.js", "Angular", "Next.js", "Nuxt.js", "Express.js"}
	for _, tech := range technologies {
		for _, framework := range webFrameworks {
			if strings.Contains(tech.Name, framework) {
				return "Web Application"
			}
		}
	}

	// Check for mobile frameworks
	mobileFrameworks := []string{"Flutter", "React Native"}
	for _, tech := range technologies {
		for _, framework := range mobileFrameworks {
			if strings.Contains(tech.Name, framework) {
				return "Mobile Application"
			}
		}
	}

	// Check for containerization
	for _, tech := range technologies {
		if strings.Contains(tech.Name, "Docker") {
			return "Containerized Application"
		}
	}

	// Default categorization
	for _, tech := range technologies {
		switch tech.Category {
		case "Language":
			return "Software Library"
		case "Framework":
			return "Application"
		}
	}

	return "Unknown"
}

// assessComplexityLevel determines project complexity
func (p *StackDetectorPlugin) assessComplexityLevel(analysis *ProjectAnalysis) string {
	score := 0

	// Technology diversity
	if len(analysis.Technologies) > 10 {
		score += 3
	} else if len(analysis.Technologies) > 5 {
		score += 2
	} else if len(analysis.Technologies) > 2 {
		score += 1
	}

	// Project size
	if analysis.ProjectSize.CodeFiles > 100 {
		score += 3
	} else if analysis.ProjectSize.CodeFiles > 50 {
		score += 2
	} else if analysis.ProjectSize.CodeFiles > 10 {
		score += 1
	}

	// Dependency count
	if len(analysis.Dependencies) > 50 {
		score += 2
	} else if len(analysis.Dependencies) > 20 {
		score += 1
	}

	switch {
	case score >= 6:
		return "High"
	case score >= 3:
		return "Medium"
	default:
		return "Low"
	}
}

// assessMaturityLevel determines project maturity
func (p *StackDetectorPlugin) assessMaturityLevel(analysis *ProjectAnalysis) string {
	score := 0

	// Check for essential files and tools
	essentialTools := []string{"Git", "Docker", "ESLint", "Prettier"}
	for _, tech := range analysis.Technologies {
		for _, tool := range essentialTools {
			if strings.Contains(tech.Name, tool) {
				score++
				break
			}
		}
	}

	// Check for build systems
	buildSystems := []string{"Webpack", "Vite", "Maven", "Gradle", "Make", "CMake"}
	for _, tech := range analysis.Technologies {
		for _, build := range buildSystems {
			if strings.Contains(tech.Name, build) {
				score++
				break
			}
		}
	}

	// Penalize for issues
	score -= len(analysis.Issues)

	switch {
	case score >= 4:
		return "Mature"
	case score >= 2:
		return "Developing"
	default:
		return "Early Stage"
	}
}

// generateRecommendedAction suggests next steps
func (p *StackDetectorPlugin) generateRecommendedAction(analysis *ProjectAnalysis) string {
	if len(analysis.Issues) > 3 {
		return "Address critical issues and improve project structure"
	}

	if len(analysis.Technologies) < 3 {
		return "Consider adding essential development tools and CI/CD"
	}

	if len(analysis.Dependencies) == 0 && analysis.ProjectSize.CodeFiles > 10 {
		return "Review and document project dependencies"
	}

	return "Continue development with current stack"
}

// outputReport outputs the report in the specified format
func (p *StackDetectorPlugin) outputReport(report *StackReport, format, outputPath string) error {
	var content string
	var err error

	switch format {
	case "json":
		content, err = p.formatReportJSON(report)
	case "yaml":
		content, err = p.formatReportYAML(report)
	default: // text
		content, err = p.formatReportText(report), nil
	}

	if err != nil {
		return err
	}

	// Output to file or stdout
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write report to file: %w", err)
		}
		fmt.Printf("✅ Report saved to: %s\n", outputPath)
	} else {
		fmt.Print(content)
	}

	return nil
}

// formatReportJSON formats the report as JSON
func (p *StackDetectorPlugin) formatReportJSON(report *StackReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// formatReportYAML formats the report as YAML (simplified)
func (p *StackDetectorPlugin) formatReportYAML(report *StackReport) (string, error) {
	// Simplified YAML formatting - in a real implementation, use a YAML library
	yaml := fmt.Sprintf(`generated_at: %s
project_path: %s
summary:
  primary_language: %s
  project_type: %s
  complexity_level: %s
  maturity_level: %s
  recommended_action: %s
technologies:
`,
		report.GeneratedAt.Format(time.RFC3339),
		report.ProjectPath,
		report.Summary.PrimaryLanguage,
		report.Summary.ProjectType,
		report.Summary.ComplexityLevel,
		report.Summary.MaturityLevel,
		report.Summary.RecommendedAction)

	for _, tech := range report.Technologies {
		yaml += fmt.Sprintf("  - name: %s\n    category: %s\n    confidence: %d\n",
			tech.Name, tech.Category, tech.Confidence)
	}

	return yaml, nil
}

// formatReportText formats the report as human-readable text
func (p *StackDetectorPlugin) formatReportText(report *StackReport) string {
	var text strings.Builder

	text.WriteString("🚀 DevEx Stack Report\n")
	text.WriteString("====================\n\n")

	text.WriteString(fmt.Sprintf("📅 Generated: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	text.WriteString(fmt.Sprintf("📂 Project: %s\n\n", report.ProjectPath))

	// Summary
	text.WriteString("📋 Project Summary\n")
	text.WriteString("------------------\n")
	text.WriteString(fmt.Sprintf("Primary Language: %s\n", report.Summary.PrimaryLanguage))
	text.WriteString(fmt.Sprintf("Project Type: %s\n", report.Summary.ProjectType))
	text.WriteString(fmt.Sprintf("Complexity Level: %s\n", report.Summary.ComplexityLevel))
	text.WriteString(fmt.Sprintf("Maturity Level: %s\n", report.Summary.MaturityLevel))
	text.WriteString(fmt.Sprintf("Recommended Action: %s\n\n", report.Summary.RecommendedAction))

	// Project size
	text.WriteString("📏 Project Metrics\n")
	text.WriteString("------------------\n")
	text.WriteString(fmt.Sprintf("Total Files: %d\n", report.ProjectSize.TotalFiles))
	text.WriteString(fmt.Sprintf("Code Files: %d\n", report.ProjectSize.CodeFiles))
	text.WriteString(fmt.Sprintf("Config Files: %d\n", report.ProjectSize.ConfigFiles))
	text.WriteString(fmt.Sprintf("Documentation Files: %d\n\n", report.ProjectSize.DocumentFiles))

	// Technologies
	text.WriteString(fmt.Sprintf("🔧 Technologies (%d detected)\n", len(report.Technologies)))
	text.WriteString("--------------------------------\n")
	for _, tech := range report.Technologies {
		confidence := ""
		switch {
		case tech.Confidence >= 9:
			confidence = "🟢 High"
		case tech.Confidence >= 7:
			confidence = "🟡 Medium"
		default:
			confidence = "🟠 Low"
		}
		text.WriteString(fmt.Sprintf("%s - %s (%s)\n", confidence, tech.Name, tech.Category))
	}
	text.WriteString("\n")

	// Issues
	if len(report.Issues) > 0 {
		text.WriteString(fmt.Sprintf("⚠️  Issues (%d found)\n", len(report.Issues)))
		text.WriteString("-------------------\n")
		for _, issue := range report.Issues {
			text.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		text.WriteString("\n")
	}

	// Recommendations
	if len(report.Recommendations) > 0 {
		text.WriteString(fmt.Sprintf("💡 Recommendations (%d suggestions)\n", len(report.Recommendations)))
		text.WriteString("----------------------------------\n")
		for _, rec := range report.Recommendations {
			text.WriteString(fmt.Sprintf("- %s\n", rec))
		}
		text.WriteString("\n")
	}

	text.WriteString("Generated by DevEx Stack Detector\n")

	return text.String()
}
