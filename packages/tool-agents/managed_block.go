package main

import (
	"fmt"
	"strings"
)

const (
	BlockStart   = "<!-- >>> devex agents >>> -->"
	BlockComment = "<!-- Managed by DevEx. Changes inside this block are overwritten. -->"
	BlockEnd     = "<!-- <<< devex agents <<< -->"
)

// RenderManagedBlock updates or removes the single DevEx-managed instructions
// block while keeping every byte outside the block intact.
func RenderManagedBlock(existing string, exists bool, instructions string) (updated string, changed bool, err error) {
	if strings.Contains(instructions, BlockStart) || strings.Contains(instructions, BlockEnd) {
		return "", false, fmt.Errorf("instructions must not contain managed block markers")
	}

	prefix, suffix, found, err := splitManagedBlock(existing)
	if err != nil {
		return "", false, err
	}

	trimmed := strings.TrimRight(instructions, "\r\n")
	if strings.TrimSpace(trimmed) == "" {
		if !exists || !found {
			return existing, false, nil
		}
		removed := joinWithoutBlock(prefix, suffix)
		return removed, removed != existing, nil
	}

	block := BlockStart + "\n" + BlockComment + "\n" + trimmed + "\n" + BlockEnd + "\n"

	if found {
		updated = prefix + block + suffix
		return updated, updated != existing, nil
	}

	if !exists || existing == "" {
		return block, true, nil
	}

	sep := "\n\n"
	if strings.HasSuffix(existing, "\n\n") {
		sep = ""
	} else if strings.HasSuffix(existing, "\n") {
		sep = "\n"
	}
	updated = existing + sep + block
	return updated, updated != existing, nil
}

// splitManagedBlock splits content around a single well-formed managed block.
// Any malformed marker arrangement returns an error without modifying content.
func splitManagedBlock(content string) (prefix, suffix string, found bool, err error) {
	lines := strings.SplitAfter(content, "\n")
	startIdx := -1
	endIdx := -1
	inBlock := false
	blockCount := 0

	offset := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case BlockStart:
			if inBlock {
				return "", "", false, fmt.Errorf("nested DevEx agents block marker")
			}
			if blockCount > 0 {
				return "", "", false, fmt.Errorf("duplicate DevEx agents block marker")
			}
			inBlock = true
			startIdx = offset
		case BlockEnd:
			if !inBlock {
				return "", "", false, fmt.Errorf("unmatched DevEx agents end marker")
			}
			inBlock = false
			blockCount++
			endIdx = offset + len(line)
		}
		offset += len(line)
	}

	if inBlock {
		return "", "", false, fmt.Errorf("unterminated DevEx agents block")
	}
	if blockCount == 0 {
		return content, "", false, nil
	}
	return content[:startIdx], content[endIdx:], true, nil
}

func joinWithoutBlock(prefix, suffix string) string {
	switch {
	case prefix == "" && suffix == "":
		return ""
	case prefix == "":
		return strings.TrimLeft(suffix, "\n")
	case suffix == "":
		if strings.HasSuffix(prefix, "\n\n") {
			return strings.TrimSuffix(prefix, "\n")
		}
		return prefix
	default:
		if strings.HasSuffix(prefix, "\n\n") && strings.HasPrefix(suffix, "\n") {
			return strings.TrimSuffix(prefix, "\n") + suffix
		}
		if strings.HasSuffix(prefix, "\n\n") {
			return strings.TrimSuffix(prefix, "\n") + suffix
		}
		return prefix + suffix
	}
}
