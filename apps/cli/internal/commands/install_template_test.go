package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/viper"
)

var templateResolverTestMu sync.Mutex

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
				{
					Name:     "Docker",
					Category: "development",
					Linux: types.OSConfig{
						InstallMethod:  "apt",
						InstallCommand: "docker",
					},
				},
				{
					Name:     "zsh",
					Category: "development",
					Default:  true,
					Linux: types.OSConfig{
						InstallMethod:  "apt",
						InstallCommand: "zsh",
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
		Databases: config.DatabasesConfig{
			Servers: []types.CrossPlatformApp{
				{
					Name:     "PostgreSQL",
					Category: "databases",
					Linux: types.OSConfig{
						InstallMethod:  "apt",
						InstallCommand: "postgresql",
					},
				},
			},
		},
	}

	byName, err := resolveAppsByName(settings, []string{"docker", "git"})
	if err != nil {
		t.Fatalf("resolveAppsByName() error = %v", err)
	}
	if len(byName) != 2 || byName[0].Name != "Docker" || byName[1].Name != "Git" {
		t.Fatalf("resolveAppsByName() = %#v, want Docker and Git only", byName)
	}

	byCategory, err := resolveAppsByCategory(settings, []string{"databases"})
	if err != nil {
		t.Fatalf("resolveAppsByCategory() error = %v", err)
	}
	if len(byCategory) != 1 || byCategory[0].Name != "PostgreSQL" {
		t.Fatalf("resolveAppsByCategory() = %#v, want PostgreSQL only", byCategory)
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

type templateCommandFakePluginExecutor struct {
	calls []templateCommandPluginCall
}

type templateCommandPluginCall struct {
	name string
	args []string
}

func (e *templateCommandFakePluginExecutor) ExecutePlugin(_ context.Context, pluginName string, args []string) error {
	e.calls = append(e.calls, templateCommandPluginCall{
		name: pluginName,
		args: append([]string(nil), args...),
	})
	return nil
}

func TestTemplateLifecycleCommandsUseTempStateAndRecordedResources(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	dbPath := filepath.Join(tempDir, "state.db")
	templatePath := filepath.Join(tempDir, "template.yaml")
	writeTestFile(t, templatePath, []byte(`
schema_version: devex.template/v1
id: runtime-e2e
name: Runtime E2E
version: 1.0.0
inputs:
  postgres_password:
    type: text
    prompt: PostgreSQL password
    sensitive: true
    required: true
lifecycle:
  install:
    - id: postgres
      plugin:
        name: package-manager-docker
        command: install
        args:
          - postgres:16
          - --name=devex-postgres
          - --password={{ inputs.postgres_password }}
      resources:
        - type: container
          name: devex-postgres
          external_id: devex-postgres
  maintain:
    - id: postgres-pull
      plugin:
        name: package-manager-docker
        command: pull
        args:
          - postgres:16
  status:
    - id: postgres-status
      plugin:
        name: package-manager-docker
        command: status
        args:
          - devex-postgres
  remove:
    - id: postgres-remove
      plugin:
        name: package-manager-docker
        command: remove
        args:
          - devex-postgres
`))

	t.Setenv("HOME", homeDir)
	t.Setenv(state.DBPathEnvVar, dbPath)
	viper.Reset()
	t.Cleanup(viper.Reset)
	if err := viper.BindEnv(state.DBPathConfigKey, state.DBPathEnvVar); err != nil {
		t.Fatalf("BindEnv() error = %v", err)
	}

	templateResolverTestMu.Lock()
	t.Cleanup(templateResolverTestMu.Unlock)
	fakePlugin := &templateCommandFakePluginExecutor{}
	oldInstallResolver := templateInstallPluginExecutorResolver
	oldLifecycleResolver := templatePluginExecutorResolver
	templateInstallPluginExecutorResolver = func(_ pluginBootstrapProvider, dryRun bool) (templateexec.PluginExecutor, error) {
		if dryRun {
			return nil, nil
		}
		return fakePlugin, nil
	}
	templatePluginExecutorResolver = func(_ pluginBootstrapProvider) (templateexec.PluginExecutor, error) {
		return fakePlugin, nil
	}
	t.Cleanup(func() {
		templateInstallPluginExecutorResolver = oldInstallResolver
		templatePluginExecutorResolver = oldLifecycleResolver
	})

	installOpts := templateInstallOptions{
		Answers:        []string{"postgres_password=super-secret"},
		NonInteractive: true,
	}
	if err := executeTemplateInstall(context.Background(), templatePath, templateInstallOptions{
		Answers:        installOpts.Answers,
		NonInteractive: true,
		DryRun:         true,
	}, config.CrossPlatformSettings{}, nil); err != nil {
		t.Fatalf("executeTemplateInstall() dry run error = %v", err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("dry run state file err = %v, want not exist", err)
	}
	if len(fakePlugin.calls) != 0 {
		t.Fatalf("dry run plugin calls = %#v, want none", fakePlugin.calls)
	}

	if err := executeTemplateInstall(context.Background(), templatePath, installOpts, config.CrossPlatformSettings{}, nil); err != nil {
		t.Fatalf("executeTemplateInstall() error = %v", err)
	}
	installRun := latestTemplateRun(t, dbPath, "runtime-e2e", templates.LifecycleInstall)
	resources := resourcesForRun(t, dbPath, installRun.ID)
	if len(resources) != 1 || resources[0].CurrentState != state.ResourceStatePresent {
		t.Fatalf("install resources = %#v, want one present resource", resources)
	}

	if err := executeTemplateLifecycle(context.Background(), templatePath, templates.LifecycleMaintain, templateLifecycleOptions{
		Answers:        installOpts.Answers,
		NonInteractive: true,
		Run:            "latest",
	}, config.CrossPlatformSettings{}, nil); err != nil {
		t.Fatalf("executeTemplateLifecycle() maintain error = %v", err)
	}
	if got := fakePlugin.calls[len(fakePlugin.calls)-1]; got.name != "package-manager-docker" || got.args[0] != "pull" {
		t.Fatalf("latest maintain plugin call = %#v, want docker pull", got)
	}

	statusOutput := captureStdout(t, func() {
		err := executeTemplateLifecycle(context.Background(), templatePath, templates.LifecycleStatus, templateLifecycleOptions{
			Answers:        installOpts.Answers,
			NonInteractive: true,
			Format:         "table",
			Run:            "latest",
		}, config.CrossPlatformSettings{}, nil)
		if err != nil {
			t.Fatalf("executeTemplateLifecycle() status error = %v", err)
		}
	})
	if !bytes.Contains([]byte(statusOutput), []byte("devex-postgres")) {
		t.Fatalf("status output = %q, want recorded resource", statusOutput)
	}

	writeTestFile(t, templatePath, []byte(`
schema_version: devex.template/v1
id: runtime-e2e
name: Runtime E2E
version: 1.0.1
lifecycle:
  install:
    - id: unrelated
      plugin:
        name: package-manager-docker
        command: install
        args:
          - redis:7
`))
	if err := executeTemplateLifecycle(context.Background(), templatePath, templates.LifecycleRemove, templateLifecycleOptions{
		NonInteractive: true,
		Run:            "latest",
	}, config.CrossPlatformSettings{}, nil); err != nil {
		t.Fatalf("executeTemplateLifecycle() remove after template change error = %v", err)
	}
	resources = resourcesForRun(t, dbPath, installRun.ID)
	if len(resources) != 1 || resources[0].CurrentState != state.ResourceStateRemoved {
		t.Fatalf("removed resources = %#v, want one removed resource", resources)
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile() state DB error = %v", err)
	}
	if bytes.Contains(data, []byte("super-secret")) {
		t.Fatal("state DB contains sensitive plaintext")
	}
}

func latestTemplateRun(t *testing.T, dbPath string, templateID string, action templates.LifecycleAction) state.TemplateRun {
	t.Helper()
	store, err := state.OpenBoltStore(dbPath, 0o600)
	if err != nil {
		t.Fatalf("OpenBoltStore() error = %v", err)
	}
	defer store.Close()
	runs, err := store.ListRuns(context.Background(), state.RunFilter{
		TemplateID: templateID,
		Action:     action,
		Status:     state.RunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("no successful %s run found for template %s", action, templateID)
	}
	return runs[len(runs)-1]
}

func resourcesForRun(t *testing.T, dbPath string, runID string) []state.ManagedResource {
	t.Helper()
	store, err := state.OpenBoltStore(dbPath, 0o600)
	if err != nil {
		t.Fatalf("OpenBoltStore() error = %v", err)
	}
	defer store.Close()
	resources, err := store.ListResourcesByRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("ListResourcesByRun() error = %v", err)
	}
	return resources
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
