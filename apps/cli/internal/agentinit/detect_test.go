package agentinit_test

import (
	"io/fs"
	"testing/fstest"

	"github.com/jameswlane/devex/apps/cli/internal/agentinit"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func mapFS(files map[string]string) fstest.MapFS {
	m := make(fstest.MapFS, len(files))
	for name, body := range files {
		if body == "<dir>" {
			m[name] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}
			continue
		}
		m[name] = &fstest.MapFile{Data: []byte(body), Mode: 0o644}
	}
	return m
}

var _ = Describe("Project detection", func() {
	DescribeTable("detects languages, task runners, and test/lint commands from top-level files",
		func(files map[string]string, want agentinit.ProjectInfo) {
			got := agentinit.Detect(mapFS(files), want.ProjectName)
			Expect(got).To(Equal(want))
		},
		Entry("Go + Taskfile with test and lint tasks",
			map[string]string{
				"go.mod": "module example.com/demo\n\ngo 1.26\n",
				"Taskfile.yml": `version: '3'
tasks:
  test:
    cmds: [go test ./...]
  lint:
    cmds: [golangci-lint run]
`,
			},
			agentinit.ProjectInfo{
				ProjectName:  "go-task-app",
				Languages:    []string{"Go"},
				TaskRunner:   "task",
				TestCommands: []string{"task test"},
				LintCommands: []string{"task lint"},
			},
		),
		Entry("Go without task runner uses go vet when no golangci config exists",
			map[string]string{
				"go.mod": "module example.com/demo\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "go-plain",
				Languages:    []string{"Go"},
				TestCommands: []string{"go test ./..."},
				LintCommands: []string{"go vet ./..."},
			},
		),
		Entry("Go with .golangci.yml uses golangci-lint run",
			map[string]string{
				"go.mod":        "module example.com/demo\n",
				".golangci.yml": "version: '2'\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "go-lint",
				Languages:    []string{"Go"},
				TestCommands: []string{"go test ./..."},
				LintCommands: []string{"golangci-lint run"},
			},
		),
		Entry("Node TypeScript with pnpm lockfile and scripts",
			map[string]string{
				"package.json":   `{"name":"web","scripts":{"test":"vitest","lint":"biome check"}}`,
				"tsconfig.json":  `{"compilerOptions":{}}`,
				"pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "ts-pnpm",
				Languages:    []string{"TypeScript"},
				TaskRunner:   "pnpm",
				TestCommands: []string{"pnpm test"},
				LintCommands: []string{"pnpm lint"},
			},
		),
		Entry("Node JavaScript with npm fallback",
			map[string]string{
				"package.json": `{"name":"js-app","scripts":{"test":"jest","lint":"eslint ."}}`,
			},
			agentinit.ProjectInfo{
				ProjectName:  "js-npm",
				Languages:    []string{"JavaScript"},
				TaskRunner:   "npm",
				TestCommands: []string{"npm test"},
				LintCommands: []string{"npm run lint"},
			},
		),
		Entry("Node with yarn lockfile",
			map[string]string{
				"package.json": `{"name":"yarn-app","scripts":{"test":"jest"}}`,
				"yarn.lock":    "# yarn lockfile v1\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "js-yarn",
				Languages:    []string{"JavaScript"},
				TaskRunner:   "yarn",
				TestCommands: []string{"yarn test"},
			},
		),
		Entry("Node with bun.lockb",
			map[string]string{
				"package.json": `{"name":"bun-app","scripts":{"test":"vitest","lint":"biome check"}}`,
				"bun.lockb":    "binary",
			},
			agentinit.ProjectInfo{
				ProjectName:  "js-bun",
				Languages:    []string{"JavaScript"},
				TaskRunner:   "bun",
				TestCommands: []string{"bun run test"},
				LintCommands: []string{"bun run lint"},
			},
		),
		Entry("Rust project defaults",
			map[string]string{
				"Cargo.toml": "[package]\nname = \"demo\"\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "rust-app",
				Languages:    []string{"Rust"},
				TestCommands: []string{"cargo test"},
				LintCommands: []string{"cargo clippy"},
			},
		),
		Entry("Python project without ruff omits lint",
			map[string]string{
				"pyproject.toml": "[project]\nname = \"py-app\"\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "py-app",
				Languages:    []string{"Python"},
				TestCommands: []string{"pytest"},
			},
		),
		Entry("Python project with [tool.ruff] in pyproject.toml includes ruff check",
			map[string]string{
				"pyproject.toml": "[project]\nname = \"py-ruff\"\n\n[tool.ruff]\nline-length = 100\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "py-ruff",
				Languages:    []string{"Python"},
				TestCommands: []string{"pytest"},
				LintCommands: []string{"ruff check ."},
			},
		),
		Entry("Python project with requirements.txt and ruff.toml",
			map[string]string{
				"requirements.txt": "pytest\n",
				"ruff.toml":        "line-length = 88\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "py-req",
				Languages:    []string{"Python"},
				TestCommands: []string{"pytest"},
				LintCommands: []string{"ruff check ."},
			},
		),
		Entry("Ruby project with spec/ and .rubocop.yml",
			map[string]string{
				"Gemfile":      "source 'https://rubygems.org'\n",
				"spec":         "<dir>",
				".rubocop.yml": "AllCops:\n  NewCops: enable\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "ruby-rspec",
				Languages:    []string{"Ruby"},
				TestCommands: []string{"bundle exec rspec"},
				LintCommands: []string{"bundle exec rubocop"},
			},
		),
		Entry("Ruby Rails project without spec/ uses rails test",
			map[string]string{
				"Gemfile": "source 'https://rubygems.org'\ngem 'rails', '~> 7.1'\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "ruby-rails",
				Languages:    []string{"Ruby"},
				TestCommands: []string{"bundle exec rails test"},
			},
		),
		Entry("Ruby plain Gemfile without spec/ or rails omits test and lint",
			map[string]string{
				"Gemfile": "source 'https://rubygems.org'\ngem 'sinatra'\n",
			},
			agentinit.ProjectInfo{
				ProjectName: "ruby-plain",
				Languages:   []string{"Ruby"},
			},
		),
		Entry("Makefile with test and lint targets overrides language defaults",
			map[string]string{
				"Cargo.toml": "[package]\nname = \"demo\"\n",
				"Makefile":   ".PHONY: test lint\ntest:\n\tcargo nextest run\nlint:\n\tcargo clippy -- -D warnings\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "rust-make",
				Languages:    []string{"Rust"},
				TaskRunner:   "make",
				TestCommands: []string{"make test"},
				LintCommands: []string{"make lint"},
			},
		),
		Entry("justfile with test recipe falls back to language default for lint",
			map[string]string{
				"go.mod":   "module example.com/demo\n",
				"justfile": "test:\n  go test -race ./...\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "go-just",
				Languages:    []string{"Go"},
				TaskRunner:   "just",
				TestCommands: []string{"just test"},
				LintCommands: []string{"go vet ./..."},
			},
		),
		Entry("mise.toml with [tasks] table detects mise runner and targets",
			map[string]string{
				"go.mod":    "module example.com/demo\n",
				"mise.toml": "[tools]\ngo = \"1.26\"\n\n[tasks.test]\nrun = \"go test ./...\"\n\n[tasks.lint]\nrun = \"golangci-lint run\"\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "go-mise",
				Languages:    []string{"Go"},
				TaskRunner:   "mise",
				TestCommands: []string{"mise run test"},
				LintCommands: []string{"mise run lint"},
			},
		),
		Entry("mise.toml without [tasks] is ignored as a task runner",
			map[string]string{
				"go.mod":    "module example.com/demo\n",
				"mise.toml": "[tools]\ngo = \"1.26\"\n",
			},
			agentinit.ProjectInfo{
				ProjectName:  "go-mise-tools-only",
				Languages:    []string{"Go"},
				TestCommands: []string{"go test ./..."},
				LintCommands: []string{"go vet ./..."},
			},
		),
		Entry("empty directory reports no languages, runner, or commands",
			map[string]string{},
			agentinit.ProjectInfo{
				ProjectName: "empty-repo",
			},
		),
		Entry("ignores nested marker files (top-level only)",
			map[string]string{
				"subdir/go.mod": "module example.com/nested\n",
			},
			agentinit.ProjectInfo{
				ProjectName: "nested-only",
			},
		),
	)
})
