package agentinit

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// ProjectInfo holds offline-detected repository metadata used to scaffold
// project agent instructions.
type ProjectInfo struct {
	ProjectName  string
	Languages    []string
	TaskRunner   string
	TestCommands []string
	LintCommands []string
}

var (
	justRecipeRe = regexp.MustCompile(`^@?([A-Za-z_][A-Za-z0-9_-]*)(?:\s+[^:=]*?)?:\s*(?:[^=]|$)`)
	railsGemRe   = regexp.MustCompile(`(?m)^\s*gem\s+['"]rails['"]`)
)

// Detect inspects top-level marker files in fsys to identify languages,
// task runners, and test/lint commands without network or plugin dependencies.
func Detect(fsys fs.FS, projectName string) ProjectInfo {
	info := ProjectInfo{ProjectName: projectName}
	if fsys == nil {
		return info
	}

	info.Languages = detectLanguages(fsys)
	runner, runnerTest, runnerLint := detectTaskRunner(fsys)
	info.TaskRunner = runner

	if runnerTest != "" {
		info.TestCommands = []string{runnerTest}
	} else {
		info.TestCommands = defaultTestCommands(fsys, info.Languages)
	}

	if runnerLint != "" {
		info.LintCommands = []string{runnerLint}
	} else {
		info.LintCommands = defaultLintCommands(fsys, info.Languages)
	}

	return info
}

func hasTopLevelFile(fsys fs.FS, name string) bool {
	st, err := fs.Stat(fsys, name)
	return err == nil && !st.IsDir()
}

func hasTopLevelDir(fsys fs.FS, name string) bool {
	st, err := fs.Stat(fsys, name)
	return err == nil && st.IsDir()
}

func detectLanguages(fsys fs.FS) []string {
	var langs []string
	if hasTopLevelFile(fsys, "go.mod") {
		langs = append(langs, "Go")
	}
	if hasTopLevelFile(fsys, "package.json") {
		if hasTopLevelFile(fsys, "tsconfig.json") {
			langs = append(langs, "TypeScript")
		} else {
			langs = append(langs, "JavaScript")
		}
	}
	if hasTopLevelFile(fsys, "Cargo.toml") {
		langs = append(langs, "Rust")
	}
	if hasTopLevelFile(fsys, "pyproject.toml") || hasTopLevelFile(fsys, "requirements.txt") || hasTopLevelFile(fsys, "setup.py") {
		langs = append(langs, "Python")
	}
	if hasTopLevelFile(fsys, "Gemfile") {
		langs = append(langs, "Ruby")
	}
	return langs
}

func detectTaskRunner(fsys fs.FS) (runner, testCmd, lintCmd string) {
	for _, name := range []string{"Taskfile.yml", "Taskfile.yaml"} {
		if hasTopLevelFile(fsys, name) {
			hasTest, hasLint := parseTaskfileTargets(fsys, name)
			if hasTest {
				testCmd = "task test"
			}
			if hasLint {
				lintCmd = "task lint"
			}
			return "task", testCmd, lintCmd
		}
	}

	for _, name := range []string{"mise.toml", ".mise.toml"} {
		if hasTopLevelFile(fsys, name) {
			hasTasks, hasTest, hasLint := parseMiseTasks(fsys, name)
			if hasTasks {
				if hasTest {
					testCmd = "mise run test"
				}
				if hasLint {
					lintCmd = "mise run lint"
				}
				return "mise", testCmd, lintCmd
			}
		}
	}

	for _, name := range []string{"justfile", "Justfile"} {
		if hasTopLevelFile(fsys, name) {
			hasTest, hasLint := parseJustfileRecipes(fsys, name)
			if hasTest {
				testCmd = "just test"
			}
			if hasLint {
				lintCmd = "just lint"
			}
			return "just", testCmd, lintCmd
		}
	}

	if hasTopLevelFile(fsys, "Makefile") {
		hasTest, hasLint := parseMakefileTargets(fsys, "Makefile")
		if hasTest {
			testCmd = "make test"
		}
		if hasLint {
			lintCmd = "make lint"
		}
		return "make", testCmd, lintCmd
	}

	if hasTopLevelFile(fsys, "package.json") {
		hasScripts, hasTest, hasLint := parsePackageJSONScripts(fsys, "package.json")
		if hasScripts {
			pm := detectJSManager(fsys)
			if hasTest {
				testCmd = formatJSScriptCommand(pm, "test")
			}
			if hasLint {
				lintCmd = formatJSScriptCommand(pm, "lint")
			}
			return pm, testCmd, lintCmd
		}
	}

	return "", "", ""
}

func parseTaskfileTargets(fsys fs.FS, name string) (hasTest, hasLint bool) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return false, false
	}
	var doc struct {
		Tasks map[string]any `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, false
	}
	_, hasTest = doc.Tasks["test"]
	_, hasLint = doc.Tasks["lint"]
	return hasTest, hasLint
}

func parseMiseTasks(fsys fs.FS, name string) (hasTasks, hasTest, hasLint bool) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return false, false, false
	}
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return false, false, false
	}
	tasksVal, ok := raw["tasks"]
	if !ok {
		return false, false, false
	}
	tasksMap, ok := tasksVal.(map[string]any)
	if !ok {
		return false, false, false
	}
	_, hasTest = tasksMap["test"]
	_, hasLint = tasksMap["lint"]
	return true, hasTest, hasLint
}

func parseJustfileRecipes(fsys fs.FS, name string) (hasTest, hasLint bool) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return false, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "#") {
			continue
		}
		m := justRecipeRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		switch m[1] {
		case "test":
			hasTest = true
		case "lint":
			hasLint = true
		}
	}
	return hasTest, hasLint
}

func parseMakefileTargets(fsys fs.FS, name string) (hasTest, hasLint bool) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return false, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "#") {
			continue
		}
		lhs, rhs, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		// Skip variable assignments (:=, ::=)
		if strings.HasPrefix(rhs, "=") || strings.HasPrefix(rhs, ":=") {
			continue
		}
		for _, target := range strings.Fields(lhs) {
			switch target {
			case "test":
				hasTest = true
			case "lint":
				hasLint = true
			}
		}
	}
	return hasTest, hasLint
}

func parsePackageJSONScripts(fsys fs.FS, name string) (hasScripts, hasTest, hasLint bool) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return false, false, false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false, false, false
	}
	if len(pkg.Scripts) == 0 {
		return false, false, false
	}
	if v, ok := pkg.Scripts["test"]; ok && strings.TrimSpace(v) != "" {
		hasTest = true
	}
	if v, ok := pkg.Scripts["lint"]; ok && strings.TrimSpace(v) != "" {
		hasLint = true
	}
	return true, hasTest, hasLint
}

func detectJSManager(fsys fs.FS) string {
	switch {
	case hasTopLevelFile(fsys, "pnpm-lock.yaml"):
		return "pnpm"
	case hasTopLevelFile(fsys, "yarn.lock"):
		return "yarn"
	case hasTopLevelFile(fsys, "bun.lockb") || hasTopLevelFile(fsys, "bun.lock"):
		return "bun"
	default:
		return "npm"
	}
}

func formatJSScriptCommand(pm, script string) string {
	switch pm {
	case "pnpm", "yarn":
		return pm + " " + script
	case "bun":
		return "bun run " + script
	default:
		if script == "test" {
			return "npm test"
		}
		return "npm run " + script
	}
}

func defaultTestCommands(fsys fs.FS, langs []string) []string {
	var cmds []string
	for _, lang := range langs {
		switch lang {
		case "Go":
			cmds = appendUnique(cmds, "go test ./...")
		case "Rust":
			cmds = appendUnique(cmds, "cargo test")
		case "Python":
			cmds = appendUnique(cmds, "pytest")
		case "Ruby":
			if hasTopLevelDir(fsys, "spec") {
				cmds = appendUnique(cmds, "bundle exec rspec")
			} else if isRailsProject(fsys) {
				cmds = appendUnique(cmds, "bundle exec rails test")
			}
		}
	}
	return cmds
}

func defaultLintCommands(fsys fs.FS, langs []string) []string {
	var cmds []string
	for _, lang := range langs {
		switch lang {
		case "Go":
			if hasTopLevelFile(fsys, ".golangci.yml") || hasTopLevelFile(fsys, ".golangci.yaml") {
				cmds = appendUnique(cmds, "golangci-lint run")
			} else {
				cmds = appendUnique(cmds, "go vet ./...")
			}
		case "Rust":
			cmds = appendUnique(cmds, "cargo clippy")
		case "Python":
			if isRuffConfigured(fsys) {
				cmds = appendUnique(cmds, "ruff check .")
			}
		case "Ruby":
			if hasTopLevelFile(fsys, ".rubocop.yml") {
				cmds = appendUnique(cmds, "bundle exec rubocop")
			}
		}
	}
	return cmds
}

func isRuffConfigured(fsys fs.FS) bool {
	if hasTopLevelFile(fsys, "ruff.toml") || hasTopLevelFile(fsys, ".ruff.toml") {
		return true
	}
	data, err := fs.ReadFile(fsys, "pyproject.toml")
	if err != nil {
		return false
	}
	var doc struct {
		Tool map[string]any `toml:"tool"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return false
	}
	_, ok := doc.Tool["ruff"]
	return ok
}

func isRailsProject(fsys fs.FS) bool {
	data, err := fs.ReadFile(fsys, "Gemfile")
	if err != nil {
		return false
	}
	return railsGemRe.Match(data)
}

func appendUnique(list []string, item string) []string {
	if slices.Contains(list, item) {
		return list
	}
	return append(list, item)
}
