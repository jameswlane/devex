package main

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

// EditTOMLTopLevelStrings updates only the owned top-level string keys in a
// TOML document while preserving comments, key ordering, and all tables.
// New keys are inserted before the first table header.
// If any owned key is defined in a non-scalar/table form, an error is returned.
func EditTOMLTopLevelStrings(existing string, updates map[string]string, orderedKeys []string) (string, bool, error) {
	if len(updates) == 0 {
		return existing, false, nil
	}

	if strings.TrimSpace(existing) != "" {
		var parsed map[string]any
		if _, err := toml.Decode(existing, &parsed); err != nil {
			return "", false, fmt.Errorf("parse TOML: %w", err)
		}
		for _, key := range orderedKeys {
			if _, want := updates[key]; !want {
				continue
			}
			val, exists := parsed[key]
			if !exists {
				continue
			}
			if _, ok := val.(string); !ok {
				return "", false, fmt.Errorf("TOML key %q is defined in non-scalar/table form; DevEx will not overwrite it", key)
			}
		}
	}

	lines := splitLinesPreserve(existing)
	foundKeys := make(map[string]bool, len(updates))
	firstTableIdx := len(lines)
	inMultiline := ""

	for i, line := range lines {
		if inMultiline != "" {
			if strings.Contains(line, inMultiline) {
				inMultiline = ""
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			firstTableIdx = i
			break
		}

		key, rawVal, comment, isAssign := splitTOMLAssignment(line)
		if isAssign {
			// Detect multiline string delimiter if this is another key.
			if delim := openMultilineDelimiter(rawVal); delim != "" {
				if _, owned := updates[key]; owned {
					return "", false, fmt.Errorf("TOML key %q uses a multiline string; DevEx will not overwrite it", key)
				}
				inMultiline = delim
				continue
			}
		}

		if !isAssign {
			continue
		}
		newVal, owned := updates[key]
		if !owned {
			continue
		}

		valTrim := strings.TrimSpace(rawVal)
		if strings.HasPrefix(valTrim, "{") || strings.HasPrefix(valTrim, "[") {
			return "", false, fmt.Errorf("TOML key %q is defined in non-scalar/table form; DevEx will not overwrite it", key)
		}

		foundKeys[key] = true
		indent := leadingWhitespace(line)
		eol := lineEnding(line)
		if eol == "" {
			eol = "\n"
		}
		rebuilt := indent + key + " = " + tomlQuoteString(newVal)
		if comment != "" {
			rebuilt += " " + comment
		}
		lines[i] = rebuilt + eol
	}

	var missing []string
	for _, key := range orderedKeys {
		val, want := updates[key]
		if !want || foundKeys[key] {
			continue
		}
		missing = append(missing, key+" = "+tomlQuoteString(val)+"\n")
	}

	if len(missing) > 0 {
		insertAt := firstTableIdx
		// Keep blank lines immediately before the first table header after the inserted keys.
		for insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) == "" {
			insertAt--
		}
		// Ensure line right before insertion ends with a newline.
		if insertAt > 0 && !strings.HasSuffix(lines[insertAt-1], "\n") {
			lines[insertAt-1] += "\n"
		}
		// If inserting before a table header and there is no blank line separating
		// the top-level keys from the table header, add one.
		if firstTableIdx < len(lines) && insertAt == firstTableIdx {
			missing = append(missing, "\n")
		}
		newLines := make([]string, 0, len(lines)+len(missing))
		newLines = append(newLines, lines[:insertAt]...)
		newLines = append(newLines, missing...)
		newLines = append(newLines, lines[insertAt:]...)
		lines = newLines
	}

	result := strings.Join(lines, "")
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	// Prove the resulting document is valid TOML and has the exact expected values.
	var verified map[string]any
	if _, err := toml.Decode(result, &verified); err != nil {
		return "", false, fmt.Errorf("validate updated TOML: %w", err)
	}
	for _, key := range orderedKeys {
		expected, want := updates[key]
		if !want {
			continue
		}
		got, ok := verified[key].(string)
		if !ok || got != expected {
			return "", false, fmt.Errorf("validate updated TOML: key %q resolved to %v, expected %q", key, verified[key], expected)
		}
	}

	return result, result != existing, nil
}

func splitLinesPreserve(s string) []string {
	if s == "" {
		return nil
	}
	return strings.SplitAfter(s, "\n")
}

func leadingWhitespace(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}

func lineEnding(line string) string {
	if strings.HasSuffix(line, "\r\n") {
		return "\r\n"
	}
	if strings.HasSuffix(line, "\n") {
		return "\n"
	}
	return ""
}

// splitTOMLAssignment parses a simple top-level bare-key assignment line
// `key = value [# comment]`.
func splitTOMLAssignment(line string) (key, rawVal, comment string, ok bool) {
	body := strings.TrimRight(line, "\r\n")
	trimmed := strings.TrimLeft(body, " \t")
	eq := strings.IndexByte(trimmed, '=')
	if eq <= 0 {
		return "", "", "", false
	}
	k := strings.TrimSpace(trimmed[:eq])
	if k == "" || strings.ContainsAny(k, " \t.\"'[]#") {
		return "", "", "", false
	}
	rest := trimmed[eq+1:]
	valPart, commentPart := splitTOMLInlineComment(rest)
	return k, strings.TrimSpace(valPart), strings.TrimSpace(commentPart), true
}

func splitTOMLInlineComment(s string) (val, comment string) {
	inBasic := false
	inLiteral := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inBasic {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inBasic = false
			}
			continue
		}
		if inLiteral {
			if c == '\'' {
				inLiteral = false
			}
			continue
		}
		switch c {
		case '"':
			inBasic = true
		case '\'':
			inLiteral = true
		case '#':
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func openMultilineDelimiter(val string) string {
	for _, delim := range []string{`"""`, `'''`} {
		if strings.HasPrefix(val, delim) {
			rest := val[len(delim):]
			if !strings.Contains(rest, delim) {
				return delim
			}
		}
	}
	return ""
}

func tomlQuoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
