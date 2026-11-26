package skillpacks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	skillinstall "github.com/jameswlane/devex/apps/cli/internal/agentskills/install"
)

// SkillAction describes the planned or executed action for one skill.
type SkillAction string

const (
	ActionInstall          SkillAction = "install"
	ActionReplace          SkillAction = "replace"
	ActionAdopt            SkillAction = "adopt"
	ActionAlreadyInstalled SkillAction = "already-installed"
	ActionSkipEdited       SkillAction = "skip-edited"
	ActionSkipLocalCopy    SkillAction = "skip-local-copy"
	ActionSkipAdoptable    SkillAction = "skip-adoptable"
	ActionSkipSharedCodex  SkillAction = "skip-shared-codex"
	ActionRemove           SkillAction = "remove"
	ActionBackupAndRemove  SkillAction = "backup-and-remove"
)

// Manager orchestrates skill pack installation, listing, updates, and removal.
type Manager struct {
	Catalog      *Catalog
	Fetcher      Fetcher
	Requirements RequirementChecker
	Now          func() time.Time
}

func (m *Manager) withDefaults() (*Manager, error) {
	out := Manager{}
	if m != nil {
		out = *m
	}
	if out.Catalog == nil {
		cat, err := LoadEmbeddedCatalog()
		if err != nil {
			return nil, err
		}
		out.Catalog = cat
	}
	if out.Fetcher == nil {
		out.Fetcher = DefaultFetcher()
	}
	out.Requirements = out.Requirements.withDefaults()
	if out.Now == nil {
		out.Now = time.Now
	}
	return &out, nil
}

// AddOptions configures Manager.Add.
type AddOptions struct {
	PackSpec        string
	Skills          []string
	Agent           string
	Scope           string
	ProjectDir      string
	SkillsPath      string
	DryRun          bool
	Overwrite       bool
	Adopt           bool
	AllowUnreviewed bool
}

// SkillPlanItem records the planned/executed outcome for a single skill.
type SkillPlanItem struct {
	Skill       string
	Destination string
	Action      SkillAction
	Reason      string
	BackupPath  string
	KeepBackup  bool
	Diff        *SkillDiff
	OldSHA      string
	NewSHA      string
}

// LegacyCopyNotice reports a skill copy found under the legacy ~/.codex/skills path.
type LegacyCopyNotice struct {
	Skill      string
	Path       string
	MatchesPin bool
}

// AddResult summarizes the outcome (or dry-run plan) of Manager.Add.
type AddResult struct {
	Pack                string
	Source              string
	Ref                 string
	SHA                 string
	Agent               string
	Scope               string
	LockPath            string
	Unreviewed          bool
	DryRun              bool
	Items               []SkillPlanItem
	Warnings            []string
	Notes               map[string][]string
	MissingRequirements []MissingRequirement
	LegacyCodexCopies   []LegacyCopyNotice
}

// Add installs (or previews installing) a curated or raw skill pack.
func (m *Manager) Add(ctx context.Context, opts AddOptions) (*AddResult, error) {
	mgr, err := m.withDefaults()
	if err != nil {
		return nil, err
	}
	if err := skillinstall.ValidateAgent(opts.Agent); err != nil {
		return nil, err
	}
	if err := skillinstall.ValidateScope(opts.Scope, opts.ProjectDir); err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.PackSpec) == "" {
		return nil, fmt.Errorf("pack name or raw git+https:// source is required")
	}

	nameOrSource, specRef, isRaw := ParsePackSpec(strings.TrimSpace(opts.PackSpec))

	var (
		packName       string
		sourceURL      string
		skillsPath     string
		targetRef      string
		targetSHA      string
		selectedSkills []string
		packNotes      map[string][]string
		packReqs       []Requirement
		unreviewed     bool
		warnings       []string
	)

	if isRaw {
		if !opts.AllowUnreviewed {
			return nil, fmt.Errorf("raw source %q is unreviewed; inspect the repository and re-run with --allow-unreviewed", opts.PackSpec)
		}
		var rawRef string
		sourceURL, rawRef, packName, err = ParseRawSource(nameOrSource)
		if err != nil {
			return nil, err
		}
		skillsPath = opts.SkillsPath
		if skillsPath == "" {
			skillsPath = "skills"
		}
		if err := validateRelativeSkillsPath(skillsPath); err != nil {
			return nil, fmt.Errorf("invalid --skills-path %q: %w", skillsPath, err)
		}
		targetRef = rawRef
		if targetRef == "" {
			targetRef = "HEAD"
		}
		unreviewed = true
		warnings = append(warnings, fmt.Sprintf("Warning: unreviewed source %q — skills contain instructions and scripts your coding agent may run; review them before use.", sourceURL))
		targetSHA, err = mgr.Fetcher.ResolveRef(ctx, sourceURL, targetRef)
		if err != nil {
			return nil, err
		}
	} else {
		pack, ok := mgr.Catalog.Lookup(nameOrSource)
		if !ok {
			return nil, fmt.Errorf("unknown skill pack %q (available packs: %s)", nameOrSource, strings.Join(mgr.Catalog.Names(), ", "))
		}
		packName = pack.Name
		sourceURL = pack.Source
		skillsPath = pack.Path
		packReqs = pack.Requirements

		selectedSkills, err = pack.ResolveClosure(opts.Skills)
		if err != nil {
			return nil, err
		}
		packNotes = pack.NotesFor(selectedSkills)

		if specRef != "" {
			targetRef = specRef
			targetSHA, err = mgr.Fetcher.ResolveRef(ctx, sourceURL, specRef)
			if err != nil {
				return nil, err
			}
		} else {
			targetSHA = pack.SHA
			targetRef = pack.Ref
			if targetRef == "" {
				targetRef = pack.SHA
			}
		}
	}

	checkoutDir, cleanup, err := mgr.Fetcher.Fetch(ctx, sourceURL, targetSHA)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if isRaw {
		if len(opts.Skills) > 0 {
			seen := map[string]bool{}
			for _, rawSkill := range opts.Skills {
				s := strings.TrimSpace(rawSkill)
				if s == "" {
					continue
				}
				if err := skillinstall.ValidateSkillName(s); err != nil {
					return nil, err
				}
				if !seen[s] {
					seen[s] = true
					selectedSkills = append(selectedSkills, s)
				}
			}
			sort.Strings(selectedSkills)
		}
		if len(selectedSkills) == 0 {
			selectedSkills, err = DiscoverSkills(checkoutDir, skillsPath)
			if err != nil {
				return nil, err
			}
		}
	}

	snapshots := make(map[string]*SkillSnapshot, len(selectedSkills))
	for _, skillName := range selectedSkills {
		snap, readErr := ReadSkillSnapshot(checkoutDir, skillsPath, skillName)
		if readErr != nil {
			return nil, readErr
		}
		snapshots[skillName] = snap
	}

	lockPath, err := LockPath(opts.Scope, opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	lock, err := LoadLock(lockPath)
	if err != nil {
		return nil, err
	}

	result := &AddResult{
		Pack:       packName,
		Source:     sourceURL,
		Ref:        targetRef,
		SHA:        targetSHA,
		Agent:      opts.Agent,
		Scope:      opts.Scope,
		LockPath:   lockPath,
		Unreviewed: unreviewed,
		DryRun:     opts.DryRun,
		Warnings:   warnings,
		Notes:      packNotes,
	}

	type plannedWork struct {
		item SkillPlanItem
		dest skillinstall.Destination
		snap *SkillSnapshot
	}
	work := make([]plannedWork, 0, len(selectedSkills))

	for _, skillName := range selectedSkills {
		snap := snapshots[skillName]
		dest, err := skillinstall.ResolveDestination(opts.Agent, opts.Scope, opts.ProjectDir, skillName)
		if err != nil {
			return nil, err
		}

		// For Codex user scope, inspect legacy ~/.codex/skills/<skill> read-only.
		if opts.Agent == "codex" && opts.Scope == "user" {
			legacyDest, legacyErr := skillinstall.ResolveLegacyCodexDestination(skillName)
			if legacyErr == nil {
				if legacyExists, legacyMatches, checkErr := DirMatchesSnapshot(legacyDest.Path(), snap); checkErr == nil && legacyExists {
					result.LegacyCodexCopies = append(result.LegacyCodexCopies, LegacyCopyNotice{
						Skill:      skillName,
						Path:       legacyDest.Path(),
						MatchesPin: legacyMatches,
					})
				}
			}
		}

		skipShared, conflictWarn, err := skillinstall.CheckCodexKimiConflict(opts.Agent, opts.Scope, opts.ProjectDir, skillName, fmt.Sprintf("skill %q", skillName))
		if err != nil {
			return nil, err
		}
		if conflictWarn != "" {
			result.Warnings = append(result.Warnings, strings.TrimRight(conflictWarn, "\n"))
		}

		key := DestinationKey(dest.Path())
		locked, isManaged := lock.Skills[key]
		if isManaged {
			if _, err := validateRecord(locked, lockPath, opts.ProjectDir); err != nil {
				return nil, err
			}
		}

		root, rootExists, err := skillinstall.OpenRoot(dest, opts.Scope, true)
		if err != nil {
			return nil, err
		}
		existsOnDisk := false
		if rootExists {
			validateErr := skillinstall.ValidatePath(root, dest.Relative)
			if validateErr == nil {
				info, statErr := root.Lstat(dest.Relative)
				switch {
				case statErr == nil:
					if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
						_ = root.Close()
						return nil, fmt.Errorf("skill destination %q is not a regular directory", dest.Path())
					}
					existsOnDisk = true
				case !errors.Is(statErr, os.ErrNotExist):
					_ = root.Close()
					return nil, fmt.Errorf("inspect skill destination %q: %w", dest.Path(), statErr)
				}
			}
			_ = root.Close()
			if validateErr != nil {
				return nil, validateErr
			}
		}

		item := SkillPlanItem{
			Skill:       skillName,
			Destination: dest.Path(),
			NewSHA:      targetSHA,
		}

		switch {
		case isManaged:
			item.OldSHA = locked.SHA
			if !existsOnDisk {
				item.Action = ActionInstall
			} else {
				edited, editErr := CheckLocalEdits(locked)
				if editErr != nil && !errors.Is(editErr, os.ErrNotExist) {
					return nil, fmt.Errorf("check local edits for %q: %w", dest.Path(), editErr)
				}
				switch {
				case edited && !opts.Overwrite:
					item.Action = ActionSkipEdited
					item.Reason = "locally edited; pass --overwrite to replace and retain a backup"
				case edited && opts.Overwrite:
					item.Action = ActionReplace
					item.KeepBackup = true
					item.Reason = "locally edited; will replace and retain a backup"
				case locked.Pack == packName && locked.Source == sourceURL && locked.SHA == targetSHA && snap.MatchesRecords(locked.Files):
					item.Action = ActionAlreadyInstalled
				default:
					item.Action = ActionReplace
					item.KeepBackup = locked.Pack != packName
				}
			}
		case existsOnDisk:
			_, matches, matchErr := DirMatchesSnapshot(dest.Path(), snap)
			if matchErr != nil {
				return nil, fmt.Errorf("inspect existing copy %q: %w", dest.Path(), matchErr)
			}
			switch {
			case matches && opts.Adopt:
				item.Action = ActionAdopt
				item.Reason = fmt.Sprintf("matches %s@%s; adopting into lock without rewriting files", packName, ShortSHA(targetSHA))
			case matches:
				item.Action = ActionSkipAdoptable
				item.Reason = fmt.Sprintf("matches %s@%s; pass --adopt to manage it", packName, ShortSHA(targetSHA))
			default:
				item.Action = ActionSkipLocalCopy
				item.Reason = "existing unmanaged local copy differs from pinned snapshot; left untouched"
			}
		case skipShared:
			item.Action = ActionSkipSharedCodex
			item.Reason = "covered by existing Codex skill in .agents/skills"
		default:
			item.Action = ActionInstall
		}

		work = append(work, plannedWork{item: item, dest: dest, snap: snap})
	}

	if len(packReqs) > 0 {
		result.MissingRequirements = CheckRequirements(ctx, mgr.Requirements, packReqs, selectedSkills, !opts.DryRun)
	}

	if opts.DryRun {
		for _, w := range work {
			result.Items = append(result.Items, w.item)
		}
		return result, nil
	}

	lockDirty := false
	now := mgr.Now().UTC()
	for i := range work {
		w := &work[i]
		switch w.item.Action {
		case ActionInstall, ActionReplace:
			root, _, err := skillinstall.OpenRoot(w.dest, opts.Scope, false)
			if err != nil {
				return nil, saveAfterFailure(lockPath, lock, lockDirty, err)
			}
			if err := skillinstall.ValidatePath(root, w.dest.Relative); err != nil {
				_ = root.Close()
				return nil, saveAfterFailure(lockPath, lock, lockDirty, err)
			}
			existing := w.item.Action == ActionReplace
			backupPath, err := skillinstall.StageAndPublish(ctx, root, w.dest, w.snap.FS(), existing, w.item.KeepBackup)
			_ = root.Close()
			if err != nil {
				return nil, saveAfterFailure(lockPath, lock, lockDirty, err)
			}
			w.item.BackupPath = backupPath
			lock.Skills[DestinationKey(w.dest.Path())] = InstalledSkill{
				Pack:        packName,
				Skill:       w.item.Skill,
				Agent:       opts.Agent,
				Scope:       opts.Scope,
				Destination: w.dest.Path(),
				Source:      sourceURL,
				SkillsPath:  skillsPath,
				Ref:         targetRef,
				SHA:         targetSHA,
				InstalledAt: now,
				Files:       w.snap.FileRecords(),
				Unreviewed:  unreviewed,
			}
			lockDirty = true
		case ActionAdopt:
			lock.Skills[DestinationKey(w.dest.Path())] = InstalledSkill{
				Pack:        packName,
				Skill:       w.item.Skill,
				Agent:       opts.Agent,
				Scope:       opts.Scope,
				Destination: w.dest.Path(),
				Source:      sourceURL,
				SkillsPath:  skillsPath,
				Ref:         targetRef,
				SHA:         targetSHA,
				InstalledAt: now,
				Files:       w.snap.FileRecords(),
				Unreviewed:  unreviewed,
			}
			lockDirty = true
		}
		result.Items = append(result.Items, w.item)
	}

	if lockDirty {
		if err := SaveLock(lockPath, lock); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// ListOptions filters Manager.List.
type ListOptions struct {
	Agent      string
	Scope      string
	ProjectDir string
}

// ListEntry represents one installed skill row for `devex agent skills list`.
type ListEntry struct {
	Pack        string `json:"pack"`
	Skill       string `json:"skill"`
	Agent       string `json:"agent"`
	Scope       string `json:"scope"`
	SHA         string `json:"sha"`
	SHA7        string `json:"sha7"`
	Destination string `json:"destination"`
	Edited      bool   `json:"edited"`
	Missing     bool   `json:"missing,omitempty"`
	Unreviewed  bool   `json:"unreviewed,omitempty"`
}

// List reads the applicable lock file(s) and inspects local-edit status for each skill.
func (m *Manager) List(opts ListOptions) ([]ListEntry, error) {
	if opts.Agent != "" {
		if err := skillinstall.ValidateAgent(opts.Agent); err != nil {
			return nil, err
		}
	}
	if opts.Scope != "" {
		if err := skillinstall.ValidateScope(opts.Scope, opts.ProjectDir); err != nil {
			return nil, err
		}
	}
	lockPaths, err := resolveScopeLockPaths(opts.Scope, opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	var entries []ListEntry
	for _, lp := range lockPaths {
		lock, err := LoadLock(lp)
		if err != nil {
			return nil, err
		}
		for _, skill := range lock.SortedEntries() {
			if opts.Agent != "" && skill.Agent != opts.Agent {
				continue
			}
			if opts.Scope != "" && skill.Scope != opts.Scope {
				continue
			}
			edited, editErr := CheckLocalEdits(skill)
			missing := errors.Is(editErr, os.ErrNotExist)
			entries = append(entries, ListEntry{
				Pack:        skill.Pack,
				Skill:       skill.Skill,
				Agent:       skill.Agent,
				Scope:       skill.Scope,
				SHA:         skill.SHA,
				SHA7:        ShortSHA(skill.SHA),
				Destination: skill.Destination,
				Edited:      edited,
				Missing:     missing,
				Unreviewed:  skill.Unreviewed,
			})
		}
	}
	return entries, nil
}

// UpdateOptions configures Manager.Update.
type UpdateOptions struct {
	PackSpec   string
	Agent      string
	Scope      string
	ProjectDir string
	DryRun     bool
	Overwrite  bool
	// AllowUnreviewed permits updating skills installed from raw sources.
	AllowUnreviewed bool
	// ExpectedTargets maps UpdatePackResult.ID to the commit SHA a dry run
	// showed. A group that now resolves to another commit fails before it is
	// fetched, so the applied files are always the ones the plan displayed.
	ExpectedTargets map[string]string
}

// UpdatePackResult summarizes update results for one pack within one lock scope.
type UpdatePackResult struct {
	// ID identifies the update group; pass it in UpdateOptions.ExpectedTargets
	// to apply exactly the commit a previous dry run planned.
	ID         string
	Unreviewed bool
	Pack       string
	Source     string
	Scope      string
	LockPath   string
	Target     string
	Items      []SkillPlanItem
}

// UpdateResult summarizes Manager.Update across all matched packs/scopes.
type UpdateResult struct {
	DryRun bool
	Packs  []UpdatePackResult
}

// Update updates installed skill packs to the embedded catalog SHA (or an explicit
// `<pack>@ref`), computing file-level and scripts/ line-level diffs before applying.
func (m *Manager) Update(ctx context.Context, opts UpdateOptions) (*UpdateResult, error) {
	mgr, err := m.withDefaults()
	if err != nil {
		return nil, err
	}
	if opts.Agent != "" {
		if err := skillinstall.ValidateAgent(opts.Agent); err != nil {
			return nil, err
		}
	}
	if opts.Scope != "" {
		if err := skillinstall.ValidateScope(opts.Scope, opts.ProjectDir); err != nil {
			return nil, err
		}
	}

	var filterPack, overrideRef string
	if strings.TrimSpace(opts.PackSpec) != "" {
		name, ref, isRaw := ParsePackSpec(strings.TrimSpace(opts.PackSpec))
		if isRaw {
			_, rawRef, derivedPack, parseErr := ParseRawSource(name)
			if parseErr != nil {
				return nil, parseErr
			}
			filterPack = derivedPack
			overrideRef = rawRef
		} else {
			filterPack = name
			overrideRef = ref
		}
	}

	lockPaths, err := resolveScopeLockPaths(opts.Scope, opts.ProjectDir)
	if err != nil {
		return nil, err
	}

	type groupKey struct {
		lockPath   string
		pack       string
		source     string
		skills     string
		scope      string
		ref        string
		unreviewed bool
	}
	type lockBundle struct {
		lock  *LockFile
		dirty bool
	}

	locks := make(map[string]*lockBundle, len(lockPaths))
	groups := map[groupKey][]InstalledSkill{}
	var groupOrder []groupKey

	for _, lp := range lockPaths {
		lock, err := LoadLock(lp)
		if err != nil {
			return nil, err
		}
		locks[lp] = &lockBundle{lock: lock}
		for _, entry := range lock.SortedEntries() {
			if opts.Agent != "" && entry.Agent != opts.Agent {
				continue
			}
			if opts.Scope != "" && entry.Scope != opts.Scope {
				continue
			}
			if filterPack != "" && entry.Pack != filterPack {
				continue
			}
			if _, err := validateRecord(entry, lp, opts.ProjectDir); err != nil {
				return nil, err
			}
			// Never trust the source recorded in a lock file on its own: curated
			// packs fetch from the catalog, and raw sources need the same
			// explicit opt-in as `add`.
			gk := groupKey{lockPath: lp, pack: entry.Pack, scope: entry.Scope}
			if entry.Unreviewed {
				if !opts.AllowUnreviewed {
					return nil, fmt.Errorf("skill %q was installed from unreviewed source %q; review it and re-run with --allow-unreviewed", entry.Skill, entry.Source)
				}
				if err := validateHTTPSURL(entry.Source); err != nil {
					return nil, fmt.Errorf("lock entry for %q has an invalid source %q: %w", entry.Destination, entry.Source, err)
				}
				if err := validateRelativeSkillsPath(entry.SkillsPath); err != nil {
					return nil, fmt.Errorf("lock entry for %q has an invalid skills path %q: %w", entry.Destination, entry.SkillsPath, err)
				}
				gk.source, gk.skills, gk.ref, gk.unreviewed = entry.Source, entry.SkillsPath, entry.Ref, true
			} else {
				catPack, ok := mgr.Catalog.Lookup(entry.Pack)
				if !ok {
					return nil, fmt.Errorf("skill %q belongs to pack %q, which is not in the catalog", entry.Skill, entry.Pack)
				}
				gk.source, gk.skills = catPack.Source, catPack.Path
			}
			if _, seen := groups[gk]; !seen {
				groupOrder = append(groupOrder, gk)
			}
			groups[gk] = append(groups[gk], entry)
		}
	}

	if len(groupOrder) == 0 {
		if filterPack != "" {
			return nil, fmt.Errorf("skill pack %q is not installed", filterPack)
		}
		return &UpdateResult{DryRun: opts.DryRun}, nil
	}

	result := &UpdateResult{DryRun: opts.DryRun}
	now := mgr.Now().UTC()

	// saveDirtyLocks keeps each lock in step with the skills already changed
	// before err stopped the update.
	saveDirtyLocks := func(err error) error {
		if opts.DryRun {
			return err
		}
		for lp, lb := range locks {
			err = saveAfterFailure(lp, lb.lock, lb.dirty, err)
		}
		return err
	}

	for _, gk := range groupOrder {
		entries := groups[gk]
		var targetSHA, targetRef string
		skillsPath := gk.skills
		if skillsPath == "" {
			skillsPath = "skills"
		}

		if overrideRef != "" {
			targetRef = overrideRef
			targetSHA, err = mgr.Fetcher.ResolveRef(ctx, gk.source, overrideRef)
			if err != nil {
				return nil, saveDirtyLocks(err)
			}
		} else if catPack, ok := mgr.Catalog.Lookup(gk.pack); ok && !gk.unreviewed {
			targetSHA = catPack.SHA
			targetRef = catPack.Ref
			if targetRef == "" {
				targetRef = catPack.SHA
			}
		} else {
			ref := gk.ref
			if ref == "" {
				ref = "HEAD"
			}
			targetRef = ref
			targetSHA, err = mgr.Fetcher.ResolveRef(ctx, gk.source, ref)
			if err != nil {
				return nil, saveDirtyLocks(err)
			}
		}

		groupID := strings.Join([]string{gk.lockPath, gk.pack, gk.scope, gk.source, gk.skills, gk.ref}, "\x00")
		if expected, ok := opts.ExpectedTargets[groupID]; opts.ExpectedTargets != nil && !ok {
			return nil, saveDirtyLocks(fmt.Errorf("%s (%s scope) was not in the planned update; re-run the update to review it", gk.pack, gk.scope))
		} else if ok && expected != targetSHA {
			return nil, saveDirtyLocks(fmt.Errorf("%s now resolves to %s, not the planned %s; re-run the update to review the new changes", gk.pack, ShortSHA(targetSHA), ShortSHA(expected)))
		}

		checkoutDir, cleanup, err := mgr.Fetcher.Fetch(ctx, gk.source, targetSHA)
		if err != nil {
			return nil, saveDirtyLocks(err)
		}

		packResult := UpdatePackResult{
			ID:         groupID,
			Pack:       gk.pack,
			Source:     gk.source,
			Scope:      gk.scope,
			LockPath:   gk.lockPath,
			Target:     targetSHA,
			Unreviewed: gk.unreviewed,
		}

		lb := locks[gk.lockPath]
		updateErr := func() error {
			defer cleanup()
			for _, entry := range entries {
				newSnap, snapErr := ReadSkillSnapshot(checkoutDir, skillsPath, entry.Skill)
				if snapErr != nil {
					return snapErr
				}
				edited, editErr := CheckLocalEdits(entry)
				missingOnDisk := errors.Is(editErr, os.ErrNotExist)
				if editErr != nil && !missingOnDisk {
					return fmt.Errorf("check local edits for %q: %w", entry.Destination, editErr)
				}

				var oldSnap *SkillSnapshot
				if !missingOnDisk {
					oldSnap, _ = SnapshotFromDir(entry.Destination, entry.Skill)
				}
				diff := DiffSnapshots(entry.Skill, oldSnap, newSnap)

				item := SkillPlanItem{
					Skill:       entry.Skill,
					Destination: entry.Destination,
					OldSHA:      entry.SHA,
					NewSHA:      targetSHA,
				}
				if diff.HasChanges() {
					diffCopy := diff
					item.Diff = &diffCopy
				}

				switch {
				case edited && !opts.Overwrite:
					item.Action = ActionSkipEdited
					item.Reason = "locally edited; pass --overwrite to replace and retain a backup"
				case edited && opts.Overwrite:
					item.Action = ActionReplace
					item.KeepBackup = true
					item.Reason = "locally edited; replacing and retaining backup"
				case !missingOnDisk && entry.SHA == targetSHA && !diff.HasChanges():
					item.Action = ActionAlreadyInstalled
				default:
					if missingOnDisk {
						item.Action = ActionInstall
					} else {
						item.Action = ActionReplace
					}
				}

				if !opts.DryRun && (item.Action == ActionInstall || item.Action == ActionReplace) {
					dest, destErr := validateRecord(entry, gk.lockPath, opts.ProjectDir)
					if destErr != nil {
						return destErr
					}
					root, _, openErr := skillinstall.OpenRoot(dest, entry.Scope, false)
					if openErr != nil {
						return openErr
					}
					if err := skillinstall.ValidatePath(root, dest.Relative); err != nil {
						_ = root.Close()
						return err
					}
					backupPath, pubErr := skillinstall.StageAndPublish(ctx, root, dest, newSnap.FS(), item.Action == ActionReplace, item.KeepBackup)
					_ = root.Close()
					if pubErr != nil {
						return pubErr
					}
					item.BackupPath = backupPath
					updated := entry
					updated.Source = gk.source
					updated.SkillsPath = skillsPath
					updated.Ref = targetRef
					updated.SHA = targetSHA
					updated.InstalledAt = now
					updated.Files = newSnap.FileRecords()
					lb.lock.Skills[DestinationKey(entry.Destination)] = updated
					lb.dirty = true
				}
				packResult.Items = append(packResult.Items, item)
			}
			return nil
		}()
		if updateErr != nil {
			return nil, saveDirtyLocks(updateErr)
		}
		result.Packs = append(result.Packs, packResult)
	}

	if !opts.DryRun {
		for lp, lb := range locks {
			if lb.dirty {
				if err := SaveLock(lp, lb.lock); err != nil {
					return nil, err
				}
			}
		}
	}
	return result, nil
}

// RemoveOptions configures Manager.Remove.
type RemoveOptions struct {
	Pack       string
	Skills     []string
	Agent      string
	Scope      string
	ProjectDir string
	Force      bool
	DryRun     bool
}

// RemoveResult summarizes Manager.Remove.
type RemoveResult struct {
	Pack     string
	Agent    string
	Scope    string
	LockPath string
	DryRun   bool
	Items    []SkillPlanItem
}

// Remove uninstalls lock-managed skills for pack/agent/scope while enforcing
// dependency and local-edit guards unless --force is passed. Locally edited
// skills forced out are moved to a .devex-backup-* sibling rather than deleted.
func (m *Manager) Remove(opts RemoveOptions) (*RemoveResult, error) {
	mgr, err := m.withDefaults()
	if err != nil {
		return nil, err
	}
	if err := skillinstall.ValidateAgent(opts.Agent); err != nil {
		return nil, err
	}
	if err := skillinstall.ValidateScope(opts.Scope, opts.ProjectDir); err != nil {
		return nil, err
	}
	packName := strings.TrimSpace(opts.Pack)
	if packName == "" {
		return nil, fmt.Errorf("pack name is required")
	}

	lockPath, err := LockPath(opts.Scope, opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	lock, err := LoadLock(lockPath)
	if err != nil {
		return nil, err
	}

	installedBySkill := map[string]InstalledSkill{}
	installedSet := map[string]bool{}
	for _, entry := range lock.SortedEntries() {
		if entry.Pack == packName && entry.Agent == opts.Agent && entry.Scope == opts.Scope {
			if _, err := validateRecord(entry, lockPath, opts.ProjectDir); err != nil {
				return nil, err
			}
			installedBySkill[entry.Skill] = entry
			installedSet[entry.Skill] = true
		}
	}
	if len(installedBySkill) == 0 {
		return nil, fmt.Errorf("pack %q has no installed skills for agent %q (%s scope)", packName, opts.Agent, opts.Scope)
	}

	var targetSkills []string
	removeSet := map[string]bool{}
	if len(opts.Skills) == 0 {
		for s := range installedBySkill {
			targetSkills = append(targetSkills, s)
			removeSet[s] = true
		}
	} else {
		installedNames := make([]string, 0, len(installedBySkill))
		for s := range installedBySkill {
			installedNames = append(installedNames, s)
		}
		sort.Strings(installedNames)
		for _, raw := range opts.Skills {
			s := strings.TrimSpace(raw)
			if s == "" {
				continue
			}
			if _, ok := installedBySkill[s]; !ok {
				return nil, fmt.Errorf("skill %q from pack %q is not installed for agent %q (%s scope); installed skills: %s", s, packName, opts.Agent, opts.Scope, strings.Join(installedNames, ", "))
			}
			if !removeSet[s] {
				removeSet[s] = true
				targetSkills = append(targetSkills, s)
			}
		}
	}
	sort.Strings(targetSkills)

	// Dependency guard.
	if !opts.Force {
		if catPack, ok := mgr.Catalog.Lookup(packName); ok {
			blocked := catPack.DependentsOf(removeSet, installedSet)
			if len(blocked) > 0 {
				var parts []string
				deps := make([]string, 0, len(blocked))
				for d := range blocked {
					deps = append(deps, d)
				}
				sort.Strings(deps)
				for _, d := range deps {
					parts = append(parts, fmt.Sprintf("%s (needed by %s)", d, strings.Join(blocked[d], ", ")))
				}
				return nil, fmt.Errorf("cannot remove skills still required by other installed skills: %s; pass --force to override", strings.Join(parts, "; "))
			}
		}
	}

	// Local-edit guard.
	editedSkills := map[string]bool{}
	var editedNames []string
	for _, s := range targetSkills {
		entry := installedBySkill[s]
		edited, editErr := CheckLocalEdits(entry)
		if editErr != nil && !errors.Is(editErr, os.ErrNotExist) {
			return nil, fmt.Errorf("check local edits for %q: %w", entry.Destination, editErr)
		}
		if edited {
			editedSkills[s] = true
			editedNames = append(editedNames, s)
		}
	}
	if len(editedNames) > 0 && !opts.Force {
		return nil, fmt.Errorf("refusing to remove locally edited skills (%s); pass --force to move them to a backup instead of deleting", strings.Join(editedNames, ", "))
	}

	result := &RemoveResult{
		Pack:     packName,
		Agent:    opts.Agent,
		Scope:    opts.Scope,
		LockPath: lockPath,
		DryRun:   opts.DryRun,
	}

	removed := false
	for _, s := range targetSkills {
		entry := installedBySkill[s]
		edited := editedSkills[s]
		item := SkillPlanItem{
			Skill:       s,
			Destination: entry.Destination,
			OldSHA:      entry.SHA,
			Action:      ActionRemove,
		}
		if edited {
			item.Action = ActionBackupAndRemove
			item.Reason = "locally edited; moved to backup rather than deleted"
		}
		if !opts.DryRun {
			dest, destErr := validateRecord(entry, lockPath, opts.ProjectDir)
			if destErr != nil {
				return nil, saveAfterFailure(lockPath, lock, removed, destErr)
			}
			_, statErr := os.Lstat(entry.Destination)
			if statErr == nil {
				root, _, openErr := skillinstall.OpenRoot(dest, entry.Scope, false)
				if openErr != nil {
					return nil, saveAfterFailure(lockPath, lock, removed, openErr)
				}
				if err := skillinstall.ValidatePath(root, dest.Relative); err != nil {
					_ = root.Close()
					return nil, saveAfterFailure(lockPath, lock, removed, err)
				}
				if edited {
					backupPath, backupErr := skillinstall.BackupExisting(root, dest)
					_ = root.Close()
					if backupErr != nil {
						return nil, saveAfterFailure(lockPath, lock, removed, backupErr)
					}
					item.BackupPath = backupPath
				} else {
					rmErr := root.RemoveAll(dest.Relative)
					_ = root.Close()
					if rmErr != nil {
						return nil, saveAfterFailure(lockPath, lock, removed, fmt.Errorf("remove skill %q: %w", entry.Destination, rmErr))
					}
				}
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return nil, saveAfterFailure(lockPath, lock, removed, fmt.Errorf("inspect skill %q: %w", entry.Destination, statErr))
			}
			delete(lock.Skills, DestinationKey(entry.Destination))
			removed = true
		}
		result.Items = append(result.Items, item)
	}

	if !opts.DryRun {
		if err := SaveLock(lockPath, lock); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func resolveScopeLockPaths(scope, projectDir string) ([]string, error) {
	switch scope {
	case "user":
		p, err := UserLockPath()
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	case "project":
		p, err := ProjectLockPath(projectDir)
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	default:
		if projectDir != "" {
			p, err := ProjectLockPath(projectDir)
			if err != nil {
				return nil, err
			}
			return []string{p}, nil
		}
		var paths []string
		if userPath, err := UserLockPath(); err == nil {
			paths = append(paths, userPath)
		}
		if projPath, err := ProjectLockPath(""); err == nil {
			if _, statErr := os.Stat(projPath); statErr == nil {
				paths = append(paths, projPath)
			}
		}
		return paths, nil
	}
}

// validateRecord checks that a lock entry describes exactly the destination
// DevEx would resolve for its agent, scope and skill, and that it was read from
// the lock file of that scope. A project lock comes from a checkout and is not
// trusted on its own: a record that names any other path is rejected, so update
// and remove never act on directories outside the resolved skill destination.
func validateRecord(entry InstalledSkill, lockPath, projectDir string) (skillinstall.Destination, error) {
	invalid := func(reason string) (skillinstall.Destination, error) {
		return skillinstall.Destination{}, fmt.Errorf("lock file %q has an invalid entry for %q: %s; remove the entry or reinstall the skill", lockPath, entry.Destination, reason)
	}
	if err := skillinstall.ValidateAgent(entry.Agent); err != nil {
		return invalid(err.Error())
	}
	if err := skillinstall.ValidateSkillName(entry.Skill); err != nil {
		return invalid(err.Error())
	}
	if entry.Scope != "user" && entry.Scope != "project" {
		return invalid(fmt.Sprintf("unsupported scope %q", entry.Scope))
	}
	scopeProjectDir := ""
	if entry.Scope == "project" {
		scopeProjectDir = projectDir
	}
	expectedLock, err := LockPath(entry.Scope, scopeProjectDir)
	if err != nil {
		return skillinstall.Destination{}, err
	}
	if DestinationKey(expectedLock) != DestinationKey(lockPath) {
		return invalid(fmt.Sprintf("%s-scope entries belong in %q", entry.Scope, expectedLock))
	}
	resolved, err := skillinstall.ResolveDestination(entry.Agent, entry.Scope, scopeProjectDir, entry.Skill)
	if err != nil {
		return skillinstall.Destination{}, err
	}
	// Exact match: a cleaned comparison would accept "a/../b" style paths
	// whose symlinked segments resolve somewhere else on disk.
	if entry.Destination != resolved.Path() {
		return invalid(fmt.Sprintf("expected destination %q", resolved.Path()))
	}
	return resolved, nil
}

// saveAfterFailure records the changes that completed before err so the lock
// keeps matching the skill directories on disk, then returns err.
func saveAfterFailure(lockPath string, lock *LockFile, dirty bool, err error) error {
	if !dirty {
		return err
	}
	if saveErr := SaveLock(lockPath, lock); saveErr != nil {
		return errors.Join(err, fmt.Errorf("record completed changes in %q: %w", lockPath, saveErr))
	}
	return err
}
