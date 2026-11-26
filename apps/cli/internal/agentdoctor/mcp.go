package agentdoctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// MCPStatus reports whether an existing MCP config file parsed cleanly.
type MCPStatus string

const (
	MCPStatusOK      MCPStatus = "ok"
	MCPStatusInvalid MCPStatus = "invalid"
)

// MCPConfigResult holds the validation outcome for a single MCP config file.
// Error messages never quote file contents — only sanitized error positions.
type MCPConfigResult struct {
	Path   string    `json:"path"`
	Format MCPFormat `json:"format"`
	Status MCPStatus `json:"status"`
	Error  string    `json:"error,omitempty"`
}

var yamlLineRegexp = regexp.MustCompile(`line\s+(\d+)`)

// validateMCPConfig reads and parses a single existing MCP config file.
// It never includes file contents or raw parser excerpts in the returned error.
func validateMCPConfig(probe Probe, spec MCPConfigSpec, resolvedPath string) MCPConfigResult {
	res := MCPConfigResult{
		Path:   resolvedPath,
		Format: spec.Format,
	}
	data, err := probe.ReadFile(resolvedPath)
	if err != nil {
		res.Status = MCPStatusInvalid
		res.Error = "invalid: unreadable file"
		return res
	}

	var parseErr string
	switch spec.Format {
	case MCPFormatJSON:
		parseErr = parseJSONMCP(data, spec.ServersKey)
	case MCPFormatJSONC, MCPFormatJSONComments:
		stripped, ok := stripJSONC(data, spec.Format == MCPFormatJSONC)
		if !ok {
			parseErr = "invalid: unterminated block comment"
		} else {
			parseErr = parseJSONMCP(stripped, spec.ServersKey)
		}
	case MCPFormatTOML:
		parseErr = parseTOMLMCP(data, spec.ServersKey)
	case MCPFormatYAML:
		parseErr = parseYAMLMCP(data, spec.ServersKey)
	default:
		parseErr = "invalid: unsupported format"
	}

	if parseErr != "" {
		res.Status = MCPStatusInvalid
		res.Error = parseErr
		return res
	}
	res.Status = MCPStatusOK
	return res
}

func parseJSONMCP(data []byte, serversKey string) string {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			offset := syntaxErr.Offset
			if offset > 0 && offset < int64(len(data)) {
				// Offset counts the offending byte; point at it, not past it.
				offset--
			}
			line, col := offsetToLineCol(data, offset)
			return fmt.Sprintf("invalid: syntax error at line %d, column %d", line, col)
		}
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			line, col := offsetToLineCol(data, typeErr.Offset)
			return fmt.Sprintf("invalid: unexpected value type at line %d, column %d", line, col)
		}
		return "invalid: malformed JSON"
	}
	if serversKey != "" && doc != nil {
		if raw, exists := doc[serversKey]; exists && raw != nil {
			if _, ok := raw.(map[string]any); !ok {
				return fmt.Sprintf("invalid: %s must be an object", serversKey)
			}
		}
	}
	return ""
}

func parseTOMLMCP(data []byte, serversKey string) string {
	var doc map[string]any
	if _, err := toml.Decode(string(data), &doc); err != nil {
		var parseErr toml.ParseError
		if errors.As(err, &parseErr) && parseErr.Position.Line > 0 {
			if parseErr.Position.Col > 0 {
				return fmt.Sprintf("invalid: syntax error at line %d, column %d", parseErr.Position.Line, parseErr.Position.Col)
			}
			return fmt.Sprintf("invalid: syntax error at line %d", parseErr.Position.Line)
		}
		return "invalid: malformed TOML"
	}
	if serversKey != "" && doc != nil {
		if raw, exists := doc[serversKey]; exists && raw != nil {
			if _, ok := raw.(map[string]any); !ok {
				return fmt.Sprintf("invalid: %s must be a table", serversKey)
			}
		}
	}
	return ""
}

func parseYAMLMCP(data []byte, serversKey string) string {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		if m := yamlLineRegexp.FindStringSubmatch(err.Error()); len(m) == 2 {
			return fmt.Sprintf("invalid: syntax error at line %s", m[1])
		}
		return "invalid: malformed YAML"
	}
	if len(node.Content) == 0 {
		return ""
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode && root.Kind != 0 {
		return fmt.Sprintf("invalid: expected mapping at line %d", root.Line)
	}
	if serversKey != "" && root.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(root.Content); i += 2 {
			k := root.Content[i]
			v := root.Content[i+1]
			if k.Value == serversKey && v.Kind != yaml.MappingNode && v.Tag != "!!null" {
				return fmt.Sprintf("invalid: %s must be a mapping at line %d", serversKey, v.Line)
			}
		}
	}
	return ""
}

func offsetToLineCol(data []byte, offset int64) (int, int) {
	line := 1
	col := 1
	limit := int(offset)
	if limit > len(data) {
		limit = len(data)
	}
	if limit < 0 {
		limit = 0
	}
	for i := 0; i < limit; i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// stripJSONC blanks comments (and trailing commas when allowed) outside strings with spaces,
// keeping newlines so parser error positions still match the original file.
func stripJSONC(data []byte, trailingCommas bool) ([]byte, bool) {
	out := make([]byte, len(data))
	copy(out, data)
	inString := false
	for i := 0; i < len(out); i++ {
		c := out[i]
		if inString {
			switch c {
			case '\\':
				i++
			case '"':
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
		case c == '/' && i+1 < len(out) && out[i+1] == '/':
			for ; i < len(out) && out[i] != '\n'; i++ {
				out[i] = ' '
			}
		case c == '/' && i+1 < len(out) && out[i+1] == '*':
			out[i], out[i+1] = ' ', ' '
			closed := false
			for i += 2; i < len(out); i++ {
				if out[i] == '*' && i+1 < len(out) && out[i+1] == '/' {
					out[i], out[i+1] = ' ', ' '
					i++
					closed = true
					break
				}
				if out[i] != '\n' {
					out[i] = ' '
				}
			}
			if !closed {
				return nil, false
			}
		case c == ',' && trailingCommas:
			if next := nextSignificant(out, i+1); next == '}' || next == ']' {
				out[i] = ' '
			}
		}
	}
	return out, true
}

// nextSignificant returns the next byte after start that is not whitespace or
// the start of a comment, or 0 at end of input.
func nextSignificant(data []byte, start int) byte {
	for i := start; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\r', '\n':
			continue
		case '/':
			if i+1 < len(data) && data[i+1] == '/' {
				for ; i < len(data) && data[i] != '\n'; i++ {
				}
				continue
			}
			if i+1 < len(data) && data[i+1] == '*' {
				for i += 2; i+1 < len(data) && (data[i] != '*' || data[i+1] != '/'); i++ {
				}
				i++
				continue
			}
		}
		return data[i]
	}
	return 0
}
