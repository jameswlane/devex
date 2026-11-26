package skillpacks

import (
	"fmt"
	"sort"
	"strings"
)

// FileChangeKind describes whether a file was added, removed, or modified.
type FileChangeKind string

const (
	FileAdded    FileChangeKind = "added"
	FileRemoved  FileChangeKind = "removed"
	FileModified FileChangeKind = "modified"
)

// FileDiff describes one file-level difference between two snapshots of a skill.
// For files under scripts/, UnifiedDiff contains the full line-level diff.
type FileDiff struct {
	Path        string
	Kind        FileChangeKind
	OldMode     string
	NewMode     string
	UnifiedDiff string
}

// SkillDiff summarizes all file changes for a single skill during an update.
type SkillDiff struct {
	Skill   string
	Changes []FileDiff
}

// HasChanges reports whether any files changed.
func (d SkillDiff) HasChanges() bool {
	return len(d.Changes) > 0
}

// DiffSnapshots compares oldSnap (may be nil) against newSnap and computes
// file-level changes plus full line-level unified diffs for files under scripts/.
func DiffSnapshots(skill string, oldSnap, newSnap *SkillSnapshot) SkillDiff {
	oldFiles := map[string]SnapshotFile{}
	if oldSnap != nil {
		oldFiles = oldSnap.Files
	}
	newFiles := map[string]SnapshotFile{}
	if newSnap != nil {
		newFiles = newSnap.Files
	}

	pathSet := map[string]bool{}
	for p := range oldFiles {
		pathSet[p] = true
	}
	for p := range newFiles {
		pathSet[p] = true
	}
	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	diff := SkillDiff{Skill: skill}
	for _, p := range paths {
		oldF, hadOld := oldFiles[p]
		newF, hasNew := newFiles[p]
		isScript := strings.HasPrefix(p, "scripts/")
		switch {
		case !hadOld && hasNew:
			fd := FileDiff{
				Path:    p,
				Kind:    FileAdded,
				NewMode: FormatFileMode(newF.Mode),
			}
			if isScript {
				fd.UnifiedDiff = UnifiedDiff("/dev/null", "b/"+p, "", string(newF.Data))
			}
			diff.Changes = append(diff.Changes, fd)
		case hadOld && !hasNew:
			fd := FileDiff{
				Path:    p,
				Kind:    FileRemoved,
				OldMode: FormatFileMode(oldF.Mode),
			}
			if isScript {
				fd.UnifiedDiff = UnifiedDiff("a/"+p, "/dev/null", string(oldF.Data), "")
			}
			diff.Changes = append(diff.Changes, fd)
		default:
			oldMode := FormatFileMode(oldF.Mode)
			newMode := FormatFileMode(newF.Mode)
			if oldF.SHA256 == newF.SHA256 && oldMode == newMode {
				continue
			}
			fd := FileDiff{
				Path:    p,
				Kind:    FileModified,
				OldMode: oldMode,
				NewMode: newMode,
			}
			if isScript && oldF.SHA256 != newF.SHA256 {
				fd.UnifiedDiff = UnifiedDiff("a/"+p, "b/"+p, string(oldF.Data), string(newF.Data))
			}
			diff.Changes = append(diff.Changes, fd)
		}
	}
	return diff
}

// UnifiedDiff produces a unified line diff between oldText and newText.
func UnifiedDiff(oldName, newName, oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	ops := diffLines(oldLines, newLines)

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n", oldName)
	fmt.Fprintf(&b, "+++ %s\n", newName)

	const contextLines = 3
	type hunkRange struct {
		start int
		end   int
	}
	var ranges []hunkRange
	for i, op := range ops {
		if op.kind == ' ' {
			continue
		}
		start := i - contextLines
		if start < 0 {
			start = 0
		}
		end := i + contextLines + 1
		if end > len(ops) {
			end = len(ops)
		}
		if len(ranges) > 0 && start <= ranges[len(ranges)-1].end {
			if end > ranges[len(ranges)-1].end {
				ranges[len(ranges)-1].end = end
			}
		} else {
			ranges = append(ranges, hunkRange{start: start, end: end})
		}
	}

	for _, r := range ranges {
		oldStart, newStart := 1, 1
		for i := 0; i < r.start; i++ {
			switch ops[i].kind {
			case ' ':
				oldStart++
				newStart++
			case '-':
				oldStart++
			case '+':
				newStart++
			}
		}
		oldCount, newCount := 0, 0
		for i := r.start; i < r.end; i++ {
			switch ops[i].kind {
			case ' ':
				oldCount++
				newCount++
			case '-':
				oldCount++
			case '+':
				newCount++
			}
		}
		if oldCount == 0 {
			oldStart = 0
		}
		if newCount == 0 {
			newStart = 0
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for i := r.start; i < r.end; i++ {
			fmt.Fprintf(&b, "%c%s\n", ops[i].kind, ops[i].line)
		}
	}
	return b.String()
}

type diffOp struct {
	kind byte
	line string
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

func diffLines(a, b []string) []diffOp {
	// Trim common prefix and suffix to keep the DP table compact for real scripts.
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}

	ops := make([]diffOp, 0, len(a)+len(b))
	for i := 0; i < prefix; i++ {
		ops = append(ops, diffOp{kind: ' ', line: a[i]})
	}

	midA := a[prefix : len(a)-suffix]
	midB := b[prefix : len(b)-suffix]
	m, n := len(midA), len(midB)

	// Guard against pathological giant inputs: fall back to full delete+insert.
	if m*n > 2_000_000 {
		for _, line := range midA {
			ops = append(ops, diffOp{kind: '-', line: line})
		}
		for _, line := range midB {
			ops = append(ops, diffOp{kind: '+', line: line})
		}
	} else {
		dp := make([][]int, m+1)
		for i := range dp {
			dp[i] = make([]int, n+1)
		}
		for i := m - 1; i >= 0; i-- {
			for j := n - 1; j >= 0; j-- {
				switch {
				case midA[i] == midB[j]:
					dp[i][j] = dp[i+1][j+1] + 1
				case dp[i+1][j] >= dp[i][j+1]:
					dp[i][j] = dp[i+1][j]
				default:
					dp[i][j] = dp[i][j+1]
				}
			}
		}
		i, j := 0, 0
		for i < m || j < n {
			switch {
			case i < m && j < n && midA[i] == midB[j]:
				ops = append(ops, diffOp{kind: ' ', line: midA[i]})
				i++
				j++
			case j < n && (i == m || dp[i][j+1] >= dp[i+1][j]):
				ops = append(ops, diffOp{kind: '+', line: midB[j]})
				j++
			default:
				ops = append(ops, diffOp{kind: '-', line: midA[i]})
				i++
			}
		}
	}

	for i := len(a) - suffix; i < len(a); i++ {
		ops = append(ops, diffOp{kind: ' ', line: a[i]})
	}
	return ops
}
