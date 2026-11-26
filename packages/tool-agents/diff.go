package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type diffOp struct {
	kind byte // ' ', '-', '+'
	line string
}

// UnifiedDiff produces a readable standard unified diff between before and after.
func UnifiedDiff(path, before, after string) string {
	if before == after {
		return ""
	}
	oldLines := splitDiffLines(before)
	newLines := splitDiffLines(after)
	ops := computeLineDiff(oldLines, newLines)

	var out strings.Builder
	fromLabel := "a/" + strings.TrimPrefix(filepath.ToSlash(path), "/")
	toLabel := "b/" + strings.TrimPrefix(filepath.ToSlash(path), "/")
	if before == "" {
		fromLabel = "/dev/null"
	}
	if after == "" {
		toLabel = "/dev/null"
	}
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", fromLabel, toLabel)

	const contextLines = 3
	type hunkRange struct {
		start, end int
	}
	var hunks []hunkRange
	for i, op := range ops {
		if op.kind == ' ' {
			continue
		}
		s := i - contextLines
		if s < 0 {
			s = 0
		}
		e := i + contextLines + 1
		if e > len(ops) {
			e = len(ops)
		}
		if len(hunks) > 0 && s <= hunks[len(hunks)-1].end {
			hunks[len(hunks)-1].end = e
		} else {
			hunks = append(hunks, hunkRange{start: s, end: e})
		}
	}

	for _, h := range hunks {
		oldStart, newStart := 1, 1
		for i := 0; i < h.start; i++ {
			if ops[i].kind != '+' {
				oldStart++
			}
			if ops[i].kind != '-' {
				newStart++
			}
		}
		oldCount, newCount := 0, 0
		for i := h.start; i < h.end; i++ {
			if ops[i].kind != '+' {
				oldCount++
			}
			if ops[i].kind != '-' {
				newCount++
			}
		}
		if oldCount == 0 {
			oldStart = 0
		}
		if newCount == 0 {
			newStart = 0
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for i := h.start; i < h.end; i++ {
			out.WriteByte(ops[i].kind)
			out.WriteString(ops[i].line)
			if !strings.HasSuffix(ops[i].line, "\n") {
				out.WriteByte('\n')
			}
		}
	}

	return out.String()
}

func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func computeLineDiff(a, b []string) []diffOp {
	n, m := len(a), len(b)
	// Trim common prefix and suffix to keep DP small and fast.
	prefix := 0
	for prefix < n && prefix < m && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < n-prefix && suffix < m-prefix && a[n-1-suffix] == b[m-1-suffix] {
		suffix++
	}

	var ops []diffOp
	for i := 0; i < prefix; i++ {
		ops = append(ops, diffOp{kind: ' ', line: a[i]})
	}

	midA := a[prefix : n-suffix]
	midB := b[prefix : m-suffix]
	na, nb := len(midA), len(midB)

	dp := make([][]int, na+1)
	for i := range dp {
		dp[i] = make([]int, nb+1)
	}
	for i := na - 1; i >= 0; i-- {
		for j := nb - 1; j >= 0; j-- {
			if midA[i] == midB[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	i, j := 0, 0
	for i < na || j < nb {
		switch {
		case i < na && j < nb && midA[i] == midB[j]:
			ops = append(ops, diffOp{kind: ' ', line: midA[i]})
			i++
			j++
		case i < na && (j == nb || dp[i+1][j] >= dp[i][j+1]):
			ops = append(ops, diffOp{kind: '-', line: midA[i]})
			i++
		default:
			ops = append(ops, diffOp{kind: '+', line: midB[j]})
			j++
		}
	}

	for k := n - suffix; k < n; k++ {
		ops = append(ops, diffOp{kind: ' ', line: a[k]})
	}
	return ops
}
