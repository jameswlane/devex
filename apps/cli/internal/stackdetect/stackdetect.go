// Package stackdetect runs the tool-stackdetector plugin and turns its
// structured report into filtered results and catalog application suggestions.
package stackdetect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// PluginName is the plugin that performs detection.
const PluginName = "tool-stackdetector"

// MaxReportBytes bounds the plugin output the CLI reads.
const MaxReportBytes = 8 << 20

// Executor runs a plugin command with caller-owned streams.
type Executor interface {
	ExecutePluginWithIO(ctx context.Context, pluginName string, args []string, streams sdk.OperationIO) error
}

// Technology is one detected technology. Confidence is on the plugin's 1-10 scale.
type Technology struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Confidence  int      `json:"confidence"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// Dependency is one declared project dependency.
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Source  string `json:"source"`
}

// ProjectSize summarizes the scanned files.
type ProjectSize struct {
	TotalFiles    int `json:"total_files"`
	CodeFiles     int `json:"code_files"`
	ConfigFiles   int `json:"config_files"`
	DocumentFiles int `json:"document_files"`
	LinesOfCode   int `json:"lines_of_code"`
}

// Summary is the plugin's high-level project assessment.
type Summary struct {
	PrimaryLanguage   string `json:"primary_language"`
	ProjectType       string `json:"project_type"`
	ComplexityLevel   string `json:"complexity_level"`
	MaturityLevel     string `json:"maturity_level"`
	RecommendedAction string `json:"recommended_action"`
}

// Report is the plugin's machine-readable stack report.
type Report struct {
	ProjectPath     string       `json:"project_path"`
	Technologies    []Technology `json:"technologies"`
	Dependencies    []Dependency `json:"dependencies"`
	ProjectSize     ProjectSize  `json:"project_size"`
	Recommendations []string     `json:"recommendations"`
	Issues          []string     `json:"issues"`
	Summary         Summary      `json:"summary"`
}

// ResolveDir returns the absolute directory to analyze, defaulting to the
// working directory, and checks that it is an existing directory.
func ResolveDir(dir string) (string, error) {
	if dir == "" {
		dir = "."
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve directory %q: %w", dir, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("directory %q is not accessible: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", dir)
	}
	return absolute, nil
}

type limitedBuffer struct {
	bytes.Buffer
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > MaxReportBytes {
		b.exceeded = true
		return 0, errors.New("stack report exceeds the size limit")
	}
	return b.Buffer.Write(p)
}

// Run asks the plugin for a JSON report of dir. Plugin diagnostics go to stderr.
func Run(ctx context.Context, executor Executor, dir string, stderr io.Writer) (Report, error) {
	if executor == nil {
		return Report{}, fmt.Errorf("the %s plugin is unavailable; run 'devex plugin install %s' and retry", PluginName, PluginName)
	}
	var stdout limitedBuffer
	err := executor.ExecutePluginWithIO(ctx, PluginName, []string{"report", "--format", "json", dir}, sdk.OperationIO{Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: stderr})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Report{}, ctxErr
	}
	if stdout.exceeded {
		return Report{}, fmt.Errorf("the %s report exceeds %d bytes", PluginName, MaxReportBytes)
	}
	if err != nil {
		return Report{}, fmt.Errorf("%s failed to analyze %s: %w", PluginName, dir, err)
	}
	return ParseReport(stdout.Bytes())
}

// ParseReport decodes a plugin report. Releases before the plugin moved its
// progress line to stderr print it ahead of the JSON, so text before the first
// line that starts a JSON object is skipped.
func ParseReport(output []byte) (Report, error) {
	start := bytes.Index(output, []byte("\n{"))
	switch {
	case bytes.HasPrefix(bytes.TrimLeft(output, " \t\r\n"), []byte("{")):
		start = 0
	case start >= 0:
		start++
	default:
		return Report{}, fmt.Errorf("the %s plugin did not return a JSON report; update it with 'devex plugin update %s'", PluginName, PluginName)
	}
	var report Report
	decoder := json.NewDecoder(bytes.NewReader(output[start:]))
	if err := decoder.Decode(&report); err != nil {
		return Report{}, fmt.Errorf("decode the %s report: %w", PluginName, err)
	}
	if report.Technologies == nil {
		report.Technologies = []Technology{}
	}
	if report.Dependencies == nil {
		report.Dependencies = []Dependency{}
	}
	if report.Recommendations == nil {
		report.Recommendations = []string{}
	}
	if report.Issues == nil {
		report.Issues = []string{}
	}
	return report, nil
}

// Filter keeps technologies with at least minConfidence (0.0-1.0, compared
// against the plugin's 1-10 scale) and, when category is set, that category.
func Filter(technologies []Technology, minConfidence float64, category string) []Technology {
	kept := []Technology{}
	for _, technology := range technologies {
		if float64(technology.Confidence)/10 < minConfidence {
			continue
		}
		if category != "" && !strings.EqualFold(technology.Category, category) {
			continue
		}
		kept = append(kept, technology)
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].Confidence != kept[j].Confidence {
			return kept[i].Confidence > kept[j].Confidence
		}
		return kept[i].Name < kept[j].Name
	})
	return kept
}

// ValidateConfidence checks a 0.0-1.0 threshold.
func ValidateConfidence(value float64) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("--confidence must be between 0.0 and 1.0, got %g", value)
	}
	return nil
}

// technologyApps maps detected technologies to catalog application names.
// Frameworks and tools that run on a runtime map to that runtime.
var technologyApps = map[string]string{
	"Go":                   "Go",
	"Node.js":              "Node.js",
	"Node.js (npm)":        "Node.js",
	"Node.js (Yarn)":       "Node.js",
	"TypeScript":           "Node.js",
	"React":                "Node.js",
	"Vue.js":               "Node.js",
	"Angular":              "Node.js",
	"Next.js":              "Node.js",
	"Nuxt.js":              "Node.js",
	"Express.js":           "Node.js",
	"Vite":                 "Node.js",
	"Webpack":              "Node.js",
	"Rollup":               "Node.js",
	"ESLint":               "Node.js",
	"Prettier":             "Node.js",
	"Tailwind CSS":         "Node.js",
	"Python":               "Python",
	"Python (Poetry)":      "Python",
	"Python (Pipenv)":      "Python",
	"Python (venv)":        "Python",
	"Python (virtualenv)":  "Python",
	"Rust":                 "Rust",
	"Ruby":                 "Ruby",
	"Elixir":               "Elixir",
	"Java (Maven)":         "Java",
	"Java/Kotlin (Gradle)": "Java",
	"Docker":               "Docker",
	"Docker Compose":       "Docker",
	"Git":                  "git",
	"Make":                 "Build Essential",
	"CMake":                "Build Essential",
}

// Suggestion is a catalog application recommended by detected technologies.
type Suggestion struct {
	App          string   `json:"app"`
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	Confidence   float64  `json:"confidence"`
	Technologies []string `json:"technologies"`
	Configured   bool     `json:"configured"`
}

// Suggest maps technologies to catalog applications. An application suggested
// by several technologies is listed once with the highest confidence. Mapped
// applications missing from the catalog are skipped; configured names the
// applications already in the user's configuration.
func Suggest(technologies []Technology, catalog []types.CrossPlatformApp, configured map[string]bool) []Suggestion {
	apps := map[string]types.CrossPlatformApp{}
	for _, app := range catalog {
		apps[strings.ToLower(app.Name)] = app
	}
	byApp := map[string]*Suggestion{}
	for _, technology := range technologies {
		name, ok := technologyApps[technology.Name]
		if !ok {
			continue
		}
		app, ok := apps[strings.ToLower(name)]
		if !ok {
			continue
		}
		key := strings.ToLower(app.Name)
		suggestion := byApp[key]
		if suggestion == nil {
			suggestion = &Suggestion{App: app.Name, Category: app.Category, Description: app.Description, Configured: configured[key]}
			byApp[key] = suggestion
		}
		suggestion.Technologies = append(suggestion.Technologies, technology.Name)
		if confidence := float64(technology.Confidence) / 10; confidence > suggestion.Confidence {
			suggestion.Confidence = confidence
		}
	}
	suggestions := make([]Suggestion, 0, len(byApp))
	for _, suggestion := range byApp {
		sort.Strings(suggestion.Technologies)
		suggestions = append(suggestions, *suggestion)
	}
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Confidence != suggestions[j].Confidence {
			return suggestions[i].Confidence > suggestions[j].Confidence
		}
		return suggestions[i].App < suggestions[j].App
	})
	return suggestions
}
