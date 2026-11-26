package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// OpenCodeRule mirrors one rule object in OpenCode V2's `permissions` array.
type OpenCodeRule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

// AgentManagedState records the list entries DevEx added for one agent so
// entries dropped from DevEx config can be removed without touching user entries.
type AgentManagedState struct {
	Allow       []string       `json:"allow,omitempty"`
	Deny        []string       `json:"deny,omitempty"`
	Permissions []OpenCodeRule `json:"permissions,omitempty"`
	// PresetPermissions are the OpenCode preset defaults DevEx put first.
	PresetPermissions []OpenCodeRule `json:"preset_permissions,omitempty"`
}

// ManagedState is stored at <DEVEX_STATE_DIR>/agents/managed.json.
type ManagedState struct {
	Version int                          `json:"version"`
	Agents  map[string]AgentManagedState `json:"agents"`
}

func newManagedState() ManagedState {
	return ManagedState{
		Version: 1,
		Agents:  map[string]AgentManagedState{},
	}
}

func (s ManagedState) clone() ManagedState {
	out := newManagedState()
	out.Version = s.Version
	for k, v := range s.Agents {
		out.Agents[k] = AgentManagedState{
			Allow:             slices.Clone(v.Allow),
			Deny:              slices.Clone(v.Deny),
			Permissions:       slices.Clone(v.Permissions),
			PresetPermissions: slices.Clone(v.PresetPermissions),
		}
	}
	return out
}

// LoadManagedState loads managed.json if it exists; absent files return an
// empty state without touching the filesystem.
func LoadManagedState(path string) (ManagedState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return newManagedState(), nil
	}
	if err != nil {
		return ManagedState{}, fmt.Errorf("read managed state %q: %w", path, err)
	}
	var state ManagedState
	if err := json.Unmarshal(data, &state); err != nil {
		return ManagedState{}, fmt.Errorf("parse managed state %q: %w", path, err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Agents == nil {
		state.Agents = map[string]AgentManagedState{}
	}
	return state, nil
}

// SaveManagedState writes managed.json atomically with mode 0600.
func SaveManagedState(path string, state ManagedState) error {
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Agents == nil {
		state.Agents = map[string]AgentManagedState{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode managed state: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create managed state directory: %w", err)
	}
	return writeFileAtomic(path, data, 0o600)
}

// MergeStringList preserves all user-owned entries (entries not in prevManaged),
// drops entries that were previously managed by DevEx but are no longer in
// desired, and appends newly desired entries without duplicates.
func MergeStringList(existing, prevManaged, desired []string) []string {
	prevSet := make(map[string]bool, len(prevManaged))
	for _, s := range prevManaged {
		prevSet[s] = true
	}
	desiredSet := make(map[string]bool, len(desired))
	for _, s := range desired {
		desiredSet[s] = true
	}

	out := make([]string, 0, len(existing)+len(desired))
	seen := make(map[string]bool, len(existing)+len(desired))

	for _, item := range existing {
		if prevSet[item] && !desiredSet[item] {
			continue
		}
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}

	for _, item := range desired {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

// OwnedEntries returns the desired entries DevEx owns after a merge: entries it
// already managed, plus entries it adds now. A desired entry the user already
// had is left user-owned, so dropping it from DevEx config never removes it.
func OwnedEntries(existing, prevManaged, desired []string) []string {
	out := make([]string, 0, len(desired))
	for _, item := range uniqueStrings(desired) {
		if slices.Contains(prevManaged, item) || !slices.Contains(existing, item) {
			out = append(out, item)
		}
	}
	return out
}

// union returns a state owning every entry either state owns. applyPlan saves
// it before writing, so neither an entry being added nor one being removed
// loses its owner if a write fails part-way through.
func (s ManagedState) union(other ManagedState) ManagedState {
	out := s.clone()
	for name, b := range other.Agents {
		a := out.Agents[name]
		out.Agents[name] = AgentManagedState{
			Allow:             uniqueStrings(append(a.Allow, b.Allow...)),
			Deny:              uniqueStrings(append(a.Deny, b.Deny...)),
			Permissions:       uniqueOpenCodeRules(append(a.Permissions, b.Permissions...)),
			PresetPermissions: uniqueOpenCodeRules(append(a.PresetPermissions, b.PresetPermissions...)),
		}
	}
	return out
}

func (a AgentManagedState) equal(b AgentManagedState) bool {
	return slices.Equal(a.Allow, b.Allow) && slices.Equal(a.Deny, b.Deny) &&
		slices.Equal(a.Permissions, b.Permissions) && slices.Equal(a.PresetPermissions, b.PresetPermissions)
}
