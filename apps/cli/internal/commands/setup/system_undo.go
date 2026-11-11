package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/setupundo"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

// systemUndoRunner reads and restores native settings; tests replace it.
var systemUndoRunner setupundo.Runner = setupundo.ExecRunner

// systemUndoState collects what setup changes outside DevEx's configuration.
type systemUndoState struct {
	rc      setupundo.RCSnapshot
	changes []setupundo.Change
}

func (m *SetupModel) homeDir() string {
	if m.settings.HomeDir != "" {
		return m.settings.HomeDir
	}
	home, _ := os.UserHomeDir()
	return home
}

// beginSystemUndo snapshots shell rc files before anything can change them.
func (m *SetupModel) beginSystemUndo() {
	if home := m.homeDir(); home != "" {
		m.systemUndo.rc = setupundo.SnapshotShellRC(setupundo.ShellRCPaths(home))
	}
}

// captureGitIdentity records the global Git identity keys setup is about to set.
func (m *SetupModel) captureGitIdentity(ctx context.Context, name, email string) []setupundo.Change {
	var changes []setupundo.Change
	for key, value := range map[string]string{"user.name": name, "user.email": email} {
		if value == "" {
			continue
		}
		change, err := setupundo.GitConfigChange(ctx, systemUndoRunner, key)
		if err != nil {
			log.Warn("Git identity change cannot be undone", "key", key, "error", err)
			m.installation.addChange(fmt.Sprintf("Global Git %s cannot be undone: %v", key, err))
			continue
		}
		changes = append(changes, change)
	}
	slices.SortFunc(changes, func(a, b setupundo.Change) int { return strings.Compare(a.Key, b.Key) })
	return changes
}

// captureDesktopSettings records the current value of every setting the
// accepted desktop plan writes. Settings it cannot read stay applied but are
// reported as not undoable.
func (m *SetupModel) captureDesktopSettings(ctx context.Context) []setupundo.Change {
	var plan [][]string
	if err := json.Unmarshal([]byte(m.selections.desktopPreview), &plan); err != nil {
		m.installation.addChange("Desktop settings cannot be undone: the accepted plan could not be read")
		return nil
	}
	changes, err := setupundo.DesktopChanges(ctx, systemUndoRunner, plan)
	if err != nil {
		log.Warn("Some desktop settings cannot be undone", "error", err)
		m.installation.addChange(fmt.Sprintf("Some desktop settings cannot be undone: %v", err))
	}
	return changes
}

// finishSystemUndo records one `devex undo` operation for every system change
// this run made and tells the user how to reverse them.
func (m *SetupModel) finishSystemUndo() {
	changes := append(m.systemUndo.rc.Changes(), m.systemUndo.changes...)
	if len(changes) == 0 {
		return
	}
	configDir := devexpaths.ForHome(m.settings.HomeDir).Config
	encoded, err := setupundo.Encode(changes)
	if err == nil {
		// A first setup runs before DevEx has a configuration directory.
		err = os.MkdirAll(filepath.Join(configDir, "config"), 0o750)
	}
	if err == nil {
		manager := undo.NewUndoManager(configDir)
		var operation *undo.UndoOperation
		operation, err = manager.RecordOperation(setupundo.Operation,
			fmt.Sprintf("devex setup: %d shell, Git and desktop changes", len(changes)), "",
			map[string]interface{}{setupundo.MetadataKey: encoded})
		if err == nil {
			m.installation.addChange(fmt.Sprintf("Undo the shell, Git and desktop changes with: devex undo %s", operation.ID))
			return
		}
	}
	log.Warn("Could not record setup changes for undo", "error", err)
	m.installation.addChange(fmt.Sprintf("Setup changes could not be recorded for undo: %v", err))
}
