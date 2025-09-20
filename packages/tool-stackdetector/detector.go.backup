package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// TechnologyStack represents a detected technology stack
type TechnologyStack struct {
	Name        string                 `json:"name"`
	Category    string                 `json:"category"`
	Confidence  float64                `json:"confidence"`
	Evidence    []Evidence             `json:"evidence"`
	Suggestions []Suggestion           `json:"suggestions"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Evidence represents proof of a technology's presence
type Evidence struct {
	Type        string  `json:"type"`        // file, content, dependency, etc.
	Path        string  `json:"path"`        // file path
	Description string  `json:"description"` // human readable description
	Confidence  float64 `json:"confidence"`  // 0.0 to 1.0
}

// Suggestion represents recommended configurations or tools
type Suggestion struct {
	Type        string                 `json:"type"`        // application, environment, system
	Action      string                 `json:"action"`      // install, configure, setup
	Target      string                 `json:"target"`      // what to install/configure
	Description string                 `json:"description"` // why this is recommended
	Priority    string                 `json:"priority"`    // critical, recommended, optional
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DetectionRule defines how to detect a specific technology
type DetectionRule struct {
	Name        string             `yaml:"name"`
	Category    string             `yaml:"category"`
	Files       []FilePattern      `yaml:"files,omitempty"`
	Content     []ContentPattern   `yaml:"content,omitempty"`
	Directories []DirectoryPattern `yaml:"directories,omitempty"`
	Commands    []CommandPattern   `yaml:"commands,omitempty"`
	Suggestions []SuggestionRule   `yaml:"suggestions,omitempty"`
}

// FilePattern defines file-based detection
type FilePattern struct {
	Path       string  `yaml:"path"`
	Pattern    string  `yaml:"pattern,omitempty"`
	Required   bool    `yaml:"required,omitempty"`
	Confidence float64 `yaml:"confidence"`
}

// ContentPattern defines content-based detection
type ContentPattern struct {
	FilePattern  string  `yaml:"file_pattern"`
	ContentRegex string  `yaml:"content_regex"`
	Description  string  `yaml:"description"`
	Confidence   float64 `yaml:"confidence"`
}

// DirectoryPattern defines directory-based detection
type DirectoryPattern struct {
	Path       string  `yaml:"path"`
	Confidence float64 `yaml:"confidence"`
}

// CommandPattern defines command-based detection
type CommandPattern struct {
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args,omitempty"`
	Description string   `yaml:"description"`
	Confidence  float64  `yaml:"confidence"`
}

// SuggestionRule defines what to suggest when technology is detected
type SuggestionRule struct {
	Type        string                 `yaml:"type"`
	Action      string                 `yaml:"action"`
	Target      string                 `yaml:"target"`
	Description string                 `yaml:"description"`
	Priority    string                 `yaml:"priority"`
	Condition   string                 `yaml:"condition,omitempty"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty"`
}

// StackDetector analyzes project directories for technology stacks
type StackDetector struct {
	rules       []DetectionRule
	workingDir  string
	maxDepth    int
	excludeDirs []string
}

// NewStackDetector creates a new stack detector
func NewStackDetector(workingDir string) *StackDetector {
	if workingDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workingDir = wd
		} else {
			workingDir = "."
		}
	}

	return &StackDetector{
		rules:       getBuiltinRules(),
		workingDir:  workingDir,
		maxDepth:    3, // Don't go too deep to avoid performance issues
		excludeDirs: []string{".git", "node_modules", "vendor", "target", "build", ".next", "dist", "__pycache__", ".venv"},
	}
}

// LoadRulesFromFile loads detection rules from a YAML file
func (sd *StackDetector) LoadRulesFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	var rules []DetectionRule
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("failed to parse rules file: %w", err)
	}

	sd.rules = append(sd.rules, rules...)
	return nil
}

// DetectStack analyzes the working directory and returns detected technologies
func (sd *StackDetector) DetectStack() ([]TechnologyStack, error) {
	var detectedStacks []TechnologyStack

	// Scan the project directory
	fileMap, err := sd.scanDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	// Apply detection rules
	for _, rule := range sd.rules {
		stack := sd.applyRule(rule, fileMap)
		if stack != nil {
			detectedStacks = append(detectedStacks, *stack)
		}
	}

	// Sort by confidence using simple bubble sort (highest first)
	// Note: Using bubble sort for simplicity since stack count is typically small (<20)
	// For larger datasets, consider using sort.Slice with custom comparator
	for i := 0; i < len(detectedStacks); i++ {
		for j := i + 1; j < len(detectedStacks); j++ {
			if detectedStacks[j].Confidence > detectedStacks[i].Confidence {
				// Swap stacks to maintain descending confidence order
				detectedStacks[i], detectedStacks[j] = detectedStacks[j], detectedStacks[i]
			}
		}
	}

	return detectedStacks, nil
}

// scanDirectory recursively scans the directory and builds a file map
func (sd *StackDetector) scanDirectory() (map[string][]byte, error) {
	fileMap := make(map[string][]byte)

	err := sd.walkDirectory(sd.workingDir, 0, fileMap)
	if err != nil {
		return nil, err
	}

	return fileMap, nil
}

// walkDirectory recursively walks the directory tree
// Uses depth-first traversal with configurable maximum depth to prevent
// performance issues in deeply nested projects. Maintains a file map
// for efficient content-based pattern matching.
func (sd *StackDetector) walkDirectory(dir string, depth int, fileMap map[string][]byte) error {
	// Depth limiting prevents exponential time complexity in deep directory trees
	if depth > sd.maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // Skip directories we can't read
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		relPath, _ := filepath.Rel(sd.workingDir, path)

		if entry.IsDir() {
			// Skip excluded directories (node_modules, .git, etc.) to improve performance
			// and avoid scanning irrelevant build artifacts or version control data
			if sd.isExcludedDir(entry.Name()) {
				continue
			}
			// Recursively scan subdirectories with incremented depth counter
			if err := sd.walkDirectory(path, depth+1, fileMap); err != nil {
				// Continue scanning other directories even if one fails (resilient traversal)
				continue
			}
		} else if entry.Type().IsRegular() {
			// File filtering: read regular files, skip hidden files unless they're important
			// (like .gitignore, .nvmrc, etc.) for technology detection
			if entry.Name() != "" && !strings.HasPrefix(entry.Name(), ".") || sd.isImportantFile(entry.Name()) {
				// Memory protection: only read files under 1MB to prevent OOM on large binaries
				if info, err := entry.Info(); err == nil && info.Size() < 1024*1024 { // Max 1MB
					if data, err := os.ReadFile(path); err == nil {
						// Store file content in map using relative path as key for pattern matching
						fileMap[relPath] = data
					}
				}
			}
		}
	}

	return nil
}

// isExcludedDir checks if a directory should be excluded from scanning
func (sd *StackDetector) isExcludedDir(name string) bool {
	for _, excluded := range sd.excludeDirs {
		if name == excluded {
			return true
		}
	}
	return false
}

// isImportantFile checks if a file is important for detection (even if it starts with .)
// This function implements a whitelist approach for dotfiles that contain technology indicators
func (sd *StackDetector) isImportantFile(name string) bool {
	// Explicit list of configuration files that indicate specific technologies
	// These files are critical for accurate stack detection despite being hidden
	importantFiles := []string{
		".gitignore", ".gitattributes", ".dockerignore", ".env", ".env.example",
		".nvmrc", ".node-version", ".python-version", ".ruby-version", ".go-version",
		".babelrc", ".eslintrc", ".prettierrc", ".editorconfig", ".browserslistrc",
	}

	// O(n) linear search is acceptable here since list is small (<20 items)
	for _, important := range importantFiles {
		if name == important {
			return true
		}
	}

	// Include dotfiles with extensions (e.g., .eslintrc.json, .prettierrc.yml)
	// Pattern: starts with dot, contains another dot, longer than 1 char
	if strings.HasPrefix(name, ".") && strings.Contains(name, ".") && len(name) > 1 {
		return true
	}

	return false
}

// applyRule applies a detection rule to the file map
func (sd *StackDetector) applyRule(rule DetectionRule, fileMap map[string][]byte) *TechnologyStack {
	var evidence []Evidence
	totalConfidence := 0.0
	evidenceCount := 0

	// Check file patterns
	for _, filePattern := range rule.Files {
		if sd.matchFilePattern(filePattern, fileMap) {
			evidence = append(evidence, Evidence{
				Type:        "file",
				Path:        filePattern.Path,
				Description: fmt.Sprintf("Found %s file", filePattern.Path),
				Confidence:  filePattern.Confidence,
			})
			totalConfidence += filePattern.Confidence
			evidenceCount++
		}
	}

	// Check content patterns
	for _, contentPattern := range rule.Content {
		if matches := sd.matchContentPattern(contentPattern, fileMap); len(matches) > 0 {
			for _, match := range matches {
				evidence = append(evidence, Evidence{
					Type:        "content",
					Path:        match.Path,
					Description: match.Description,
					Confidence:  contentPattern.Confidence,
				})
				totalConfidence += contentPattern.Confidence
				evidenceCount++
			}
		}
	}

	// Check directory patterns
	for _, dirPattern := range rule.Directories {
		if sd.matchDirectoryPattern(dirPattern) {
			evidence = append(evidence, Evidence{
				Type:        "directory",
				Path:        dirPattern.Path,
				Description: fmt.Sprintf("Found %s directory", dirPattern.Path),
				Confidence:  dirPattern.Confidence,
			})
			totalConfidence += dirPattern.Confidence
			evidenceCount++
		}
	}

	// If no evidence found, don't detect this technology
	if evidenceCount == 0 {
		return nil
	}

	// Calculate average confidence across all evidence pieces
	// This normalizes confidence scores regardless of evidence count,
	// preventing technologies with many weak indicators from scoring higher
	// than those with fewer strong indicators
	avgConfidence := totalConfidence / float64(evidenceCount)

	// Generate suggestions
	suggestions := make([]Suggestion, 0, len(rule.Suggestions))
	for _, suggestionRule := range rule.Suggestions {
		suggestions = append(suggestions, Suggestion{
			Type:        suggestionRule.Type,
			Action:      suggestionRule.Action,
			Target:      suggestionRule.Target,
			Description: suggestionRule.Description,
			Priority:    suggestionRule.Priority,
			Metadata:    suggestionRule.Metadata,
		})
	}

	return &TechnologyStack{
		Name:        rule.Name,
		Category:    rule.Category,
		Confidence:  avgConfidence,
		Evidence:    evidence,
		Suggestions: suggestions,
	}
}

// matchFilePattern checks if a file pattern matches any files
func (sd *StackDetector) matchFilePattern(pattern FilePattern, fileMap map[string][]byte) bool {
	if pattern.Pattern != "" {
		// Use regex pattern
		regex, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			return false
		}

		for path := range fileMap {
			if regex.MatchString(path) {
				return true
			}
		}
	} else {
		// Exact file match
		_, exists := fileMap[pattern.Path]
		return exists
	}

	return false
}

// matchContentPattern performs regex-based content analysis across filtered files
// This is the most computationally expensive operation in stack detection,
// using compiled regex patterns to scan file contents for technology indicators
func (sd *StackDetector) matchContentPattern(pattern ContentPattern, fileMap map[string][]byte) []struct {
	Path        string
	Description string
} {
	var matches []struct {
		Path        string
		Description string
	}

	fileRegex, err := regexp.Compile(pattern.FilePattern)
	if err != nil {
		return matches
	}

	contentRegex, err := regexp.Compile(pattern.ContentRegex)
	if err != nil {
		return matches
	}

	// Dual-phase matching: first filter files by path regex, then scan content
	// This optimization reduces regex operations on file content (expensive)
	// by pre-filtering based on file paths (cheap)
	for path, content := range fileMap {
		if fileRegex.MatchString(path) && contentRegex.Match(content) {
			// Create match record with descriptive context for debugging
			matches = append(matches, struct {
				Path        string
				Description string
			}{
				Path:        path,
				Description: fmt.Sprintf("%s in %s", pattern.Description, path),
			})
		}
	}

	return matches
}

// matchDirectoryPattern checks if a directory pattern matches
func (sd *StackDetector) matchDirectoryPattern(pattern DirectoryPattern) bool {
	dirPath := filepath.Join(sd.workingDir, pattern.Path)
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		return true
	}
	return false
}

// GetDetectionSummary returns a summary of detected technologies
func (sd *StackDetector) GetDetectionSummary(stacks []TechnologyStack) map[string]interface{} {
	summary := map[string]interface{}{
		"total_technologies": len(stacks),
		"categories":         make(map[string]int),
		"high_confidence":    0,
		"medium_confidence":  0,
		"low_confidence":     0,
	}

	categories, ok := summary["categories"].(map[string]int)
	if !ok {
		categories = make(map[string]int)
	}

	for _, stack := range stacks {
		// Count by category
		categories[stack.Category]++

		// Count by confidence level
		switch {
		case stack.Confidence >= 0.8:
			if val, ok := summary["high_confidence"].(int); ok {
				summary["high_confidence"] = val + 1
			}
		case stack.Confidence >= 0.5:
			if val, ok := summary["medium_confidence"].(int); ok {
				summary["medium_confidence"] = val + 1
			}
		default:
			if val, ok := summary["low_confidence"].(int); ok {
				summary["low_confidence"] = val + 1
			}
		}
	}

	return summary
}

// SaveResults saves detection results to a JSON file
func (sd *StackDetector) SaveResults(stacks []TechnologyStack, outputPath string) error {
	data, err := json.MarshalIndent(stacks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	return nil
}
