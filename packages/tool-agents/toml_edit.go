package main

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
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

// EditTOMLMCPServers adds, replaces, or removes DevEx-managed
// [mcp_servers.<name>] tables (including [mcp_servers.<name>.*] subtables) in a
// TOML document while preserving all unrelated text, comments, and tables
// byte-for-byte. Unmanaged tables with a colliding name return an error unless
// semantically identical, in which case they are adopted.
func EditTOMLMCPServers(
	existing string,
	desired map[string]CodexMCPTable,
	desiredOrder []string,
	prevManaged []string,
) (result string, changed bool, owned []string, err error) {
	if len(desired) == 0 && len(prevManaged) == 0 {
		return existing, false, nil, nil
	}

	parsed := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		if _, err := toml.Decode(existing, &parsed); err != nil {
			return "", false, nil, fmt.Errorf("parse TOML: %w", err)
		}
	}

	existingServers := map[string]map[string]any{}
	if raw, ok := parsed["mcp_servers"]; ok {
		m, isMap := raw.(map[string]any)
		if !isMap {
			return "", false, nil, fmt.Errorf(`TOML key "mcp_servers" is not a table; DevEx will not overwrite it`)
		}
		for k, v := range m {
			if sub, isSub := v.(map[string]any); isSub {
				existingServers[k] = sub
			} else if _, want := desired[k]; want || slices.Contains(prevManaged, k) {
				return "", false, nil, fmt.Errorf(`TOML key "mcp_servers.%s" is not a table; DevEx will not overwrite it`, k)
			}
		}
	}

	desiredParsed := make(map[string]map[string]any, len(desired))
	desiredRendered := make(map[string]string, len(desired))
	for _, name := range desiredOrder {
		rendered := renderCodexMCPTable(name, desired[name])
		desiredRendered[name] = rendered
		p, parseErr := decodeSingleCodexMCPTable(name, rendered)
		if parseErr != nil {
			return "", false, nil, parseErr
		}
		desiredParsed[name] = p
	}

	toAdd := []string{}
	toReplace := map[string]bool{}
	toRemove := map[string]bool{}
	owned = make([]string, 0, len(desiredOrder))

	for _, name := range desiredOrder {
		exTbl, exists := existingServers[name]
		wasManaged := slices.Contains(prevManaged, name)
		if exists {
			if codexMCPServerEqual(exTbl, desiredParsed[name]) {
				owned = append(owned, name)
				continue
			}
			if !wasManaged {
				return "", false, nil, fmt.Errorf("MCP server %q already exists in config.toml and is not managed by DevEx (remove it or match its configuration)", name)
			}
			toReplace[name] = true
			owned = append(owned, name)
			continue
		}
		toAdd = append(toAdd, name)
		owned = append(owned, name)
	}

	for _, prevName := range prevManaged {
		if _, want := desired[prevName]; want {
			continue
		}
		if _, exists := existingServers[prevName]; exists {
			toRemove[prevName] = true
		}
	}

	if len(toAdd) == 0 && len(toReplace) == 0 && len(toRemove) == 0 {
		return existing, false, owned, nil
	}

	lines := splitLinesPreserve(existing)
	sections := splitTOMLSections(lines)

	// Confirm every server we must replace or remove has explicit table headers
	// (not inline `mcp_servers = { ... }` or dotted keys under `[mcp_servers]`).
	foundHeaderFor := map[string]bool{}
	for _, sec := range sections {
		if srvName, isMCP := mcpServerTableOwner(sec.path); isMCP {
			foundHeaderFor[srvName] = true
		}
	}
	for name := range toReplace {
		if !foundHeaderFor[name] {
			return "", false, nil, fmt.Errorf("MCP server %q is defined in inline or dotted TOML form; DevEx will not rewrite it", name)
		}
	}
	for name := range toRemove {
		if !foundHeaderFor[name] {
			return "", false, nil, fmt.Errorf("MCP server %q is defined in inline or dotted TOML form; DevEx will not rewrite it", name)
		}
	}

	replacedDone := map[string]bool{}
	var outLines []string
	for idx, sec := range sections {
		srvName, isMCP := mcpServerTableOwner(sec.path)
		if isMCP && (toRemove[srvName] || toReplace[srvName]) {
			// Preserve any trailing comment block that introduces the following section.
			_, preservedTail := splitSectionPreservedTail(sec.lines, idx < len(sections)-1)
			if toReplace[srvName] && !replacedDone[srvName] {
				replacedDone[srvName] = true
				block := desiredRendered[srvName]
				if len(preservedTail) > 0 || idx < len(sections)-1 {
					if len(preservedTail) == 0 {
						block += "\n"
					} else if strings.TrimSpace(preservedTail[0]) != "" {
						block += "\n"
					}
				}
				outLines = append(outLines, splitLinesPreserve(block)...)
			}
			if len(preservedTail) > 0 {
				outLines = append(outLines, preservedTail...)
			} else if toRemove[srvName] {
				// Avoid leaving trailing blank lines when removing a section at EOF
				// or collapsing duplicate blank lines when removing a middle section.
				trimTrailingBlankLineBeforeRemoval(&outLines, idx == len(sections)-1)
			}
			continue
		}
		outLines = append(outLines, sec.lines...)
	}

	// Clean up trailing blank lines if all sections were removed.
	for len(toAdd) == 0 && len(outLines) > 0 && strings.TrimSpace(outLines[len(outLines)-1]) == "" {
		// Only trim if the original file didn't end with a blank line or we removed sections.
		if len(toRemove) > 0 {
			outLines = outLines[:len(outLines)-1]
		} else {
			break
		}
	}

	for _, name := range toAdd {
		if len(outLines) > 0 {
			if !strings.HasSuffix(outLines[len(outLines)-1], "\n") {
				outLines[len(outLines)-1] += "\n"
			}
			if strings.TrimSpace(outLines[len(outLines)-1]) != "" {
				outLines = append(outLines, "\n")
			}
		}
		outLines = append(outLines, splitLinesPreserve(desiredRendered[name])...)
	}

	result = strings.Join(outLines, "")
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	// Verify the updated TOML parses cleanly and matches every managed server.
	verified := map[string]any{}
	if strings.TrimSpace(result) != "" {
		if _, err := toml.Decode(result, &verified); err != nil {
			return "", false, nil, fmt.Errorf("validate updated TOML: %w", err)
		}
	}
	verifiedServers, _ := verified["mcp_servers"].(map[string]any)
	for _, name := range desiredOrder {
		got, ok := verifiedServers[name].(map[string]any)
		if !ok || !codexMCPServerEqual(got, desiredParsed[name]) {
			return "", false, nil, fmt.Errorf("validate updated TOML: mcp_servers.%s did not match expected configuration", name)
		}
	}
	for name := range toRemove {
		if _, stillThere := verifiedServers[name]; stillThere {
			return "", false, nil, fmt.Errorf("validate updated TOML: removed mcp_servers.%s is still present", name)
		}
	}

	return result, result != existing, owned, nil
}

type tomlSection struct {
	path  []string // nil for top-level preamble before first table header
	lines []string
}

func splitTOMLSections(lines []string) []tomlSection {
	var sections []tomlSection
	cur := tomlSection{}
	inMultiline := ""

	for _, line := range lines {
		if inMultiline != "" {
			cur.lines = append(cur.lines, line)
			if strings.Contains(line, inMultiline) {
				inMultiline = ""
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if path, ok := parseTOMLTableHeader(trimmed); ok {
			if len(cur.lines) > 0 || cur.path != nil {
				sections = append(sections, cur)
			}
			cur = tomlSection{
				path:  path,
				lines: []string{line},
			}
			continue
		}

		cur.lines = append(cur.lines, line)
		if _, rawVal, _, isAssign := splitTOMLAssignment(line); isAssign {
			if delim := openMultilineDelimiter(rawVal); delim != "" {
				inMultiline = delim
			}
		}
	}
	if len(cur.lines) > 0 || cur.path != nil {
		sections = append(sections, cur)
	}
	return sections
}

// parseTOMLTableHeader parses `[a.b."c"]` or `[[a.b]]` into its path segments.
func parseTOMLTableHeader(trimmed string) ([]string, bool) {
	if !strings.HasPrefix(trimmed, "[") {
		return nil, false
	}
	headerPart, _ := splitTOMLInlineComment(trimmed)
	headerPart = strings.TrimSpace(headerPart)
	isArrayTable := strings.HasPrefix(headerPart, "[[") && strings.HasSuffix(headerPart, "]]")
	var inner string
	switch {
	case isArrayTable && len(headerPart) >= 4:
		inner = strings.TrimSpace(headerPart[2 : len(headerPart)-2])
	case strings.HasPrefix(headerPart, "[") && strings.HasSuffix(headerPart, "]") && len(headerPart) >= 2:
		inner = strings.TrimSpace(headerPart[1 : len(headerPart)-1])
	default:
		return nil, false
	}
	if inner == "" {
		return nil, false
	}
	return parseTOMLDottedKey(inner)
}

func parseTOMLDottedKey(s string) ([]string, bool) {
	var parts []string
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		switch s[i] {
		case '"':
			i++
			var b strings.Builder
			closed := false
			for i < len(s) {
				if s[i] == '\\' && i+1 < len(s) {
					i++
					b.WriteByte(s[i])
					i++
					continue
				}
				if s[i] == '"' {
					i++
					closed = true
					break
				}
				b.WriteByte(s[i])
				i++
			}
			if !closed {
				return nil, false
			}
			parts = append(parts, b.String())
		case '\'':
			i++
			start := i
			for i < len(s) && s[i] != '\'' {
				i++
			}
			if i >= len(s) {
				return nil, false
			}
			parts = append(parts, s[start:i])
			i++
		default:
			start := i
			for i < len(s) && s[i] != '.' && s[i] != ' ' && s[i] != '\t' {
				i++
			}
			seg := s[start:i]
			if seg == "" {
				return nil, false
			}
			parts = append(parts, seg)
		}
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i < len(s) {
			if s[i] != '.' {
				return nil, false
			}
			i++
		}
	}
	return parts, len(parts) > 0
}

func mcpServerTableOwner(path []string) (string, bool) {
	if len(path) >= 2 && path[0] == "mcp_servers" {
		return path[1], true
	}
	return "", false
}

// splitSectionPreservedTail separates a section's own lines from any trailing
// comment block (after a blank line) that introduces the following section.
func splitSectionPreservedTail(lines []string, hasNext bool) (owned, tail []string) {
	if !hasNext || len(lines) <= 1 {
		return lines, nil
	}
	lastCode := 0
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lastCode = i
			break
		}
	}
	// Look for a blank line followed by a comment after lastCode.
	for i := lastCode + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			for j := i + 1; j < len(lines); j++ {
				if strings.HasPrefix(strings.TrimSpace(lines[j]), "#") {
					return lines[:i+1], lines[j:]
				}
				if strings.TrimSpace(lines[j]) != "" {
					break
				}
			}
		}
	}
	return lines, nil
}

func trimTrailingBlankLineBeforeRemoval(outLines *[]string, atEOF bool) {
	if atEOF {
		for len(*outLines) > 0 && strings.TrimSpace((*outLines)[len(*outLines)-1]) == "" {
			*outLines = (*outLines)[:len(*outLines)-1]
		}
	}
}

func renderCodexMCPTable(name string, tbl CodexMCPTable) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", formatTOMLKey(name))
	if tbl.Command != "" {
		fmt.Fprintf(&b, "command = %s\n", tomlQuoteString(tbl.Command))
	}
	if len(tbl.Args) > 0 {
		fmt.Fprintf(&b, "args = %s\n", formatTOMLStringArray(tbl.Args))
	}
	if len(tbl.EnvVars) > 0 {
		fmt.Fprintf(&b, "env_vars = %s\n", formatTOMLStringArray(tbl.EnvVars))
	}
	if len(tbl.Env) > 0 {
		fmt.Fprintf(&b, "env = %s\n", formatTOMLInlineStringMap(tbl.Env))
	}
	if tbl.URL != "" {
		fmt.Fprintf(&b, "url = %s\n", tomlQuoteString(tbl.URL))
	}
	if tbl.BearerTokenEnvVar != "" {
		fmt.Fprintf(&b, "bearer_token_env_var = %s\n", tomlQuoteString(tbl.BearerTokenEnvVar))
	}
	if len(tbl.EnvHTTPHeaders) > 0 {
		fmt.Fprintf(&b, "env_http_headers = %s\n", formatTOMLInlineStringMap(tbl.EnvHTTPHeaders))
	}
	if len(tbl.HTTPHeaders) > 0 {
		fmt.Fprintf(&b, "http_headers = %s\n", formatTOMLInlineStringMap(tbl.HTTPHeaders))
	}
	return b.String()
}

func formatTOMLKey(k string) string {
	if k != "" && isBareTOMLKey(k) {
		return k
	}
	return tomlQuoteString(k)
}

func isBareTOMLKey(k string) bool {
	for i := 0; i < len(k); i++ {
		c := k[i]
		isValid := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
		if !isValid {
			return false
		}
	}
	return true
}

func formatTOMLStringArray(items []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(tomlQuoteString(item))
	}
	b.WriteByte(']')
	return b.String()
}

func formatTOMLInlineStringMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{ ")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s = %s", formatTOMLKey(k), tomlQuoteString(m[k]))
	}
	b.WriteString(" }")
	return b.String()
}

func decodeSingleCodexMCPTable(name, rendered string) (map[string]any, error) {
	var parsed map[string]any
	if _, err := toml.Decode(rendered, &parsed); err != nil {
		return nil, fmt.Errorf("encode Codex MCP server %q: %w", name, err)
	}
	servers, ok := parsed["mcp_servers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("encode Codex MCP server %q: missing mcp_servers table", name)
	}
	tbl, ok := servers[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("encode Codex MCP server %q: missing server table", name)
	}
	return tbl, nil
}

func codexMCPServerEqual(a, b map[string]any) bool {
	return reflect.DeepEqual(normalizeCodexMCPMap(a), normalizeCodexMCPMap(b))
}

func normalizeCodexMCPMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch x := v.(type) {
		case []any:
			if len(x) == 0 {
				continue
			}
			out[k] = x
		case map[string]any:
			if len(x) == 0 {
				continue
			}
			out[k] = x
		default:
			out[k] = v
		}
	}
	return out
}
