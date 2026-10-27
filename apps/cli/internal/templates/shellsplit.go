package templates

import (
	"fmt"
	"strings"
)

func splitShellFields(command string) ([]string, error) {
	var fields []string
	var current strings.Builder
	var quote rune
	escaped := false
	inField := false

	for _, r := range command {
		if escaped {
			current.WriteRune(r)
			escaped = false
			inField = true
			continue
		}
		if r == '\\' {
			escaped = true
			inField = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			inField = true
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			inField = true
		case ' ', '\t', '\n', '\r':
			if inField {
				fields = append(fields, current.String())
				current.Reset()
				inField = false
			}
		default:
			current.WriteRune(r)
			inField = true
		}
	}
	if escaped {
		return nil, fmt.Errorf("unterminated escape sequence")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if inField {
		fields = append(fields, current.String())
	}
	return fields, nil
}
