package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func TestResolveAppsByNameAndCategory(t *testing.T) {
	settings := config.CrossPlatformSettings{
		Terminal: config.TerminalApplicationsConfig{
			Development: []types.CrossPlatformApp{
				{
					Name:     "Git",
					Category: "development",
					Linux: types.OSConfig{
						InstallMethod:  "apt",
						InstallCommand: "git",
					},
				},
			},
			Utilities: []types.CrossPlatformApp{
				{
					Name:     "jq",
					Category: "utilities",
					Linux: types.OSConfig{
						InstallMethod:  "apt",
						InstallCommand: "jq",
					},
				},
			},
		},
	}

	byName, err := resolveAppsByName(settings, []string{"git"})
	if err != nil {
		t.Fatalf("resolveAppsByName() error = %v", err)
	}
	if len(byName) != 1 || byName[0].Name != "Git" {
		t.Fatalf("resolveAppsByName() = %#v, want Git", byName)
	}

	byCategory, err := resolveAppsByCategory(settings, []string{"utilities"})
	if err != nil {
		t.Fatalf("resolveAppsByCategory() error = %v", err)
	}
	if len(byCategory) != 1 || byCategory[0].Name != "jq" {
		t.Fatalf("resolveAppsByCategory() = %#v, want jq", byCategory)
	}

	_, err = resolveAppsByCategory(settings, []string{"development", "typo"})
	if err == nil {
		t.Fatal("resolveAppsByCategory() error = nil, want unknown category error")
	}
}

func TestExecuteTemplateInstallDryRunDoesNotRequirePluginRuntime(t *testing.T) {
	templatePath := filepath.Join(t.TempDir(), "template.yaml")
	writeTestFile(t, templatePath, []byte(`
schema_version: devex.template/v1
id: dry-run-template
name: Dry Run Template
version: 1.0.0
lifecycle:
  install:
    - id: echo
      plugin:
        name: tool-shell
        command: echo
        args:
          - hello
`))

	err := executeTemplateInstall(context.Background(), templatePath, templateInstallOptions{
		DryRun:         true,
		NonInteractive: true,
	}, config.CrossPlatformSettings{}, nil)
	if err != nil {
		t.Fatalf("executeTemplateInstall() dry run error = %v", err)
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
