package commands

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

func TestRunsListSupportsTableJSONAndYAML(t *testing.T) {
	dbPath := seedRunsState(t)

	tests := []struct {
		name       string
		format     string
		wantOutput string
	}{
		{name: "table", format: "table", wantOutput: "RUN ID"},
		{name: "json", format: "json", wantOutput: `"template_id": "databases"`},
		{name: "yaml", format: "yaml", wantOutput: "template_id: databases"},
		{name: "uppercase json", format: " JSON ", wantOutput: `"template_id": "databases"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withStateDBPath(t, dbPath)
			output := captureStdout(t, func() {
				cmd := NewRunsCmd()
				cmd.SetArgs([]string{"list", "--format", tt.format})
				if err := cmd.Execute(); err != nil {
					t.Fatalf("runs list error = %v", err)
				}
			})
			if !strings.Contains(output, tt.wantOutput) {
				t.Fatalf("runs list output = %q, want %q", output, tt.wantOutput)
			}
		})
	}
}

func TestRunsShowSupportsStructuredFormats(t *testing.T) {
	dbPath := seedRunsState(t)

	t.Run("json", func(t *testing.T) {
		withStateDBPath(t, dbPath)
		output := captureStdout(t, func() {
			cmd := NewRunsCmd()
			cmd.SetArgs([]string{"show", "run-1", "--format", "json"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("runs show error = %v", err)
			}
		})

		var run state.TemplateRun
		if err := json.Unmarshal([]byte(output), &run); err != nil {
			t.Fatalf("json output did not unmarshal: %v\n%s", err, output)
		}
		if run.ID != "run-1" {
			t.Fatalf("run id = %s, want run-1", run.ID)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		withStateDBPath(t, dbPath)
		output := captureStdout(t, func() {
			cmd := NewRunsCmd()
			cmd.SetArgs([]string{"show", "run-1", "--format", "yaml"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("runs show error = %v", err)
			}
		})

		var run state.TemplateRun
		if err := yaml.Unmarshal([]byte(output), &run); err != nil {
			t.Fatalf("yaml output did not unmarshal: %v\n%s", err, output)
		}
		if run.TemplateID != "databases" {
			t.Fatalf("template id = %s, want databases", run.TemplateID)
		}
	})
}

func TestRunsResourcesAndOperationsRequireExistingRun(t *testing.T) {
	dbPath := seedRunsState(t)

	tests := []struct {
		name string
		args []string
	}{
		{name: "show", args: []string{"show", "missing"}},
		{name: "resources", args: []string{"resources", "missing"}},
		{name: "operations", args: []string{"operations", "missing"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withStateDBPath(t, dbPath)
			cmd := NewRunsCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("runs command error = nil, want missing run error")
			}
			if !strings.Contains(err.Error(), "run 'missing' not found") {
				t.Fatalf("runs command error = %v", err)
			}
		})
	}
}

func TestRunsResourcesAndOperationsOutput(t *testing.T) {
	dbPath := seedRunsState(t)

	t.Run("resources", func(t *testing.T) {
		withStateDBPath(t, dbPath)
		output := captureStdout(t, func() {
			cmd := NewRunsCmd()
			cmd.SetArgs([]string{"resources", "run-1", "--format", "table"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("runs resources error = %v", err)
			}
		})
		if !strings.Contains(output, "devex-postgres") {
			t.Fatalf("resources output = %q, want resource name", output)
		}
	})

	t.Run("operations", func(t *testing.T) {
		withStateDBPath(t, dbPath)
		output := captureStdout(t, func() {
			cmd := NewRunsCmd()
			cmd.SetArgs([]string{"operations", "run-1", "--format", "table"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("runs operations error = %v", err)
			}
		})
		if !strings.Contains(output, "package-manager-docker install postgres:16") {
			t.Fatalf("operations output = %q, want operation request", output)
		}
	})
}

func seedRunsState(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "state.db")
	store, err := state.OpenBoltStore(dbPath, 0o600)
	if err != nil {
		t.Fatalf("OpenBoltStore() error = %v", err)
	}

	completedAt := time.Unix(200, 0)
	run := state.TemplateRun{
		ID:              "run-1",
		TemplateID:      "databases",
		TemplateVersion: "1.0.0",
		Action:          templates.LifecycleInstall,
		Status:          state.RunStatusSucceeded,
		StartedAt:       time.Unix(100, 0),
		CompletedAt:     &completedAt,
		Plan: templates.ExecutionPlan{
			ID:         "plan-1",
			TemplateID: "databases",
			Action:     templates.LifecycleInstall,
			Steps: []templates.PlannedStep{{
				ID:    "postgres",
				Order: 10,
				Type:  templates.StepTypePlugin,
			}},
		},
	}
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := store.UpsertResource(context.Background(), state.ManagedResource{
		ID:           "resource-1",
		RunID:        run.ID,
		TemplateID:   run.TemplateID,
		StepID:       "postgres",
		Type:         "container",
		Name:         "devex-postgres",
		ExternalID:   "devex-postgres",
		CurrentState: state.ResourceStatePresent,
	}); err != nil {
		t.Fatalf("UpsertResource() error = %v", err)
	}
	if err := store.RecordOperation(context.Background(), state.OperationRecord{
		ID:         "operation-1",
		RunID:      run.ID,
		TemplateID: run.TemplateID,
		StepID:     "postgres",
		Action:     templates.LifecycleInstall,
		Request: templates.ExecutionRequest{
			Plugin:  "package-manager-docker",
			Command: "install",
			Args:    []string{"postgres:16"},
		},
		Status:    state.OperationStatusSucceeded,
		StartedAt: time.Unix(150, 0),
	}); err != nil {
		t.Fatalf("RecordOperation() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return dbPath
}

func withStateDBPath(t *testing.T, dbPath string) {
	t.Helper()

	viper.Reset()
	viper.Set(state.DBPathConfigKey, dbPath)
	t.Cleanup(viper.Reset)
}
