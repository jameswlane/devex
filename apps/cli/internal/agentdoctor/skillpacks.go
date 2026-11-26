package agentdoctor

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/skillpacks"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

// SkillPackReport summarizes one installed skill pack for a given agent and scope.
type SkillPackReport struct {
	Pack  string `json:"pack"`
	Agent string `json:"agent"`
	Scope string `json:"scope"`
	// PinnedSHA is the pin shared by every skill in the group, or "mixed"
	// when an update skipped some skills (see PinnedSHAs).
	PinnedSHA       string   `json:"pinned_sha"`
	PinnedSHAs      []string `json:"pinned_shas"`
	CatalogSHA      string   `json:"catalog_sha,omitempty"`
	UpdateAvailable bool     `json:"update_available"`
	Skills          []string `json:"skills"`
	LocallyEdited   []string `json:"locally_edited,omitempty"`
	Unreviewed      bool     `json:"unreviewed,omitempty"`
}

func checkSkillPacks(probe Probe, agents []AgentSpec) []SkillPackReport {
	allowedAgents := make(map[string]bool, len(agents))
	for _, ag := range agents {
		allowedAgents[ag.ID] = true
	}

	lockPaths := candidateSkillPackLockPaths(probe)
	if len(lockPaths) == 0 {
		return nil
	}

	catalog := probe.SkillPackCatalog
	if catalog == nil {
		catalog, _ = skillpacks.LoadEmbeddedCatalog()
	}

	type groupKey struct {
		pack  string
		agent string
		scope string
	}
	type groupState struct {
		pinnedSHAs    map[string]bool
		unreviewed    bool
		skills        []string
		locallyEdited []string
	}
	groups := map[groupKey]*groupState{}

	seenLockPaths := map[string]bool{}
	for _, lp := range lockPaths {
		clean := filepath.Clean(lp)
		if clean == "" || seenLockPaths[clean] {
			continue
		}
		seenLockPaths[clean] = true

		info, err := probe.Stat(clean)
		if err != nil || info == nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := probe.ReadFile(clean)
		if err != nil {
			continue
		}
		lock, err := skillpacks.ParseLock(data, clean)
		if err != nil {
			continue
		}
		for _, entry := range lock.SortedEntries() {
			if len(allowedAgents) > 0 && !allowedAgents[entry.Agent] {
				continue
			}
			gk := groupKey{pack: entry.Pack, agent: entry.Agent, scope: entry.Scope}
			st, ok := groups[gk]
			if !ok {
				st = &groupState{
					pinnedSHAs: map[string]bool{},
					unreviewed: entry.Unreviewed,
				}
				groups[gk] = st
			}
			st.skills = append(st.skills, entry.Skill)
			if entry.SHA != "" {
				st.pinnedSHAs[strings.ToLower(entry.SHA)] = true
			}
			if edited, editErr := skillpacks.CheckLocalEdits(entry); editErr == nil && edited {
				st.locallyEdited = append(st.locallyEdited, entry.Skill)
			}
		}
	}

	if len(groups) == 0 {
		return nil
	}

	reports := make([]SkillPackReport, 0, len(groups))
	for gk, st := range groups {
		sort.Strings(st.skills)
		sort.Strings(st.locallyEdited)
		rep := SkillPackReport{
			Pack:          gk.pack,
			Agent:         gk.agent,
			Scope:         gk.scope,
			PinnedSHAs:    make([]string, 0, len(st.pinnedSHAs)),
			Skills:        st.skills,
			LocallyEdited: st.locallyEdited,
			Unreviewed:    st.unreviewed,
		}
		if catalog != nil {
			if catPack, ok := catalog.Lookup(gk.pack); ok {
				rep.CatalogSHA = catPack.SHA
				for sha := range st.pinnedSHAs {
					if !strings.EqualFold(sha, catPack.SHA) {
						rep.UpdateAvailable = true
					}
				}
			}
		}
		for sha := range st.pinnedSHAs {
			rep.PinnedSHAs = append(rep.PinnedSHAs, sha)
		}
		sort.Strings(rep.PinnedSHAs)
		switch len(rep.PinnedSHAs) {
		case 0:
		case 1:
			rep.PinnedSHA = rep.PinnedSHAs[0]
		default:
			rep.PinnedSHA = "mixed"
		}
		reports = append(reports, rep)
	}

	sort.Slice(reports, func(i, j int) bool {
		if reports[i].Pack != reports[j].Pack {
			return reports[i].Pack < reports[j].Pack
		}
		if reports[i].Scope != reports[j].Scope {
			return reports[i].Scope < reports[j].Scope
		}
		return reports[i].Agent < reports[j].Agent
	})
	return reports
}

func candidateSkillPackLockPaths(probe Probe) []string {
	var paths []string
	if p := strings.TrimSpace(probe.SkillPackUserLockPath); p != "" {
		paths = append(paths, p)
	} else if stateDir := resolveProbeStateDir(probe); stateDir != "" {
		paths = append(paths, filepath.Join(stateDir, skillpacks.LockFileName))
	}

	if p := strings.TrimSpace(probe.SkillPackProjectLockPath); p != "" {
		paths = append(paths, p)
	} else if probe.Getwd != nil {
		if cwd, err := probe.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
			paths = append(paths, filepath.Join(cwd, ".devex", skillpacks.LockFileName))
		}
	}
	return paths
}

func resolveProbeStateDir(probe Probe) string {
	if explicit := strings.TrimSpace(probe.Getenv("DEVEX_STATE_DIR")); explicit != "" {
		return explicit
	}
	if xdg := strings.TrimSpace(probe.Getenv("XDG_STATE_HOME")); xdg != "" {
		return filepath.Join(xdg, "devex")
	}
	if home := probe.homeDir(); home != "" {
		return devexpaths.ForHome(home).State
	}
	return ""
}
