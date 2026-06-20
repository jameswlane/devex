package templates

import (
	"fmt"
	"regexp"
	"strings"
)

type EvaluationContext struct {
	Inputs    map[string]any
	Resources map[string]ResourceDeclaration
	System    map[string]string
}

func EvaluateCondition(expr string, ctx EvaluationContext) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}
	parser := conditionParser{tokens: tokenizeCondition(expr), ctx: ctx}
	value, err := parser.parseOr()
	if err != nil {
		return false, err
	}
	if parser.pos != len(parser.tokens) {
		return false, fmt.Errorf("unexpected token %q", parser.peek())
	}
	return value, nil
}

type conditionParser struct {
	tokens []string
	pos    int
	ctx    EvaluationContext
}

func (p *conditionParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for p.match("||") {
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

func (p *conditionParser) parseAnd() (bool, error) {
	left, err := p.parseUnary()
	if err != nil {
		return false, err
	}
	for p.match("&&") {
		right, err := p.parseUnary()
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

func (p *conditionParser) parseUnary() (bool, error) {
	if p.match("!") {
		value, err := p.parseUnary()
		return !value, err
	}
	return p.parsePrimary()
}

func (p *conditionParser) parsePrimary() (bool, error) {
	if p.match("(") {
		value, err := p.parseOr()
		if err != nil {
			return false, err
		}
		if !p.match(")") {
			return false, fmt.Errorf("missing closing parenthesis")
		}
		return value, nil
	}
	return p.parseFunction()
}

func (p *conditionParser) parseFunction() (bool, error) {
	name := p.next()
	if name == "" {
		return false, fmt.Errorf("expected condition function")
	}
	if !p.match("(") {
		return false, fmt.Errorf("expected '(' after %s", name)
	}

	args := make([]any, 0, 2)
	for !p.match(")") {
		if p.peek() == "" {
			return false, fmt.Errorf("missing closing parenthesis for %s", name)
		}
		arg, err := p.parseValue()
		if err != nil {
			return false, err
		}
		args = append(args, arg)
		if p.match(")") {
			break
		}
		if !p.match(",") {
			return false, fmt.Errorf("expected ',' in %s arguments", name)
		}
	}

	return evalFunction(name, args, p.ctx)
}

func (p *conditionParser) parseValue() (any, error) {
	token := p.next()
	if token == "" {
		return nil, fmt.Errorf("expected value")
	}
	if strings.HasPrefix(token, "\"") && strings.HasSuffix(token, "\"") {
		return strings.Trim(token, "\""), nil
	}
	if token == "true" {
		return true, nil
	}
	if token == "false" {
		return false, nil
	}
	return resolvePath(token, p.ctx), nil
}

func (p *conditionParser) match(token string) bool {
	if p.peek() != token {
		return false
	}
	p.pos++
	return true
}

func (p *conditionParser) next() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	token := p.tokens[p.pos]
	p.pos++
	return token
}

func (p *conditionParser) peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos]
}

func tokenizeCondition(expr string) []string {
	tokens := make([]string, 0)
	for i := 0; i < len(expr); {
		ch := expr[i]
		if ch == ' ' || ch == '\t' || ch == '\n' {
			i++
			continue
		}
		if i+1 < len(expr) && (expr[i:i+2] == "&&" || expr[i:i+2] == "||") {
			tokens = append(tokens, expr[i:i+2])
			i += 2
			continue
		}
		if strings.ContainsRune("(),!", rune(ch)) {
			tokens = append(tokens, string(ch))
			i++
			continue
		}
		if ch == '"' {
			j := i + 1
			for j < len(expr) && expr[j] != '"' {
				j++
			}
			if j < len(expr) {
				j++
			}
			tokens = append(tokens, expr[i:j])
			i = j
			continue
		}
		j := i
		for j < len(expr) && !strings.ContainsRune(" \t\n(),!", rune(expr[j])) {
			if j+1 < len(expr) && (expr[j:j+2] == "&&" || expr[j:j+2] == "||") {
				break
			}
			j++
		}
		tokens = append(tokens, expr[i:j])
		i = j
	}
	return tokens
}

func evalFunction(name string, args []any, ctx EvaluationContext) (bool, error) {
	switch name {
	case "equals":
		if len(args) != 2 {
			return false, fmt.Errorf("equals requires 2 arguments")
		}
		return fmt.Sprint(args[0]) == fmt.Sprint(args[1]), nil
	case "contains":
		if len(args) != 2 {
			return false, fmt.Errorf("contains requires 2 arguments")
		}
		return containsValue(args[0], args[1]), nil
	case "exists":
		if len(args) != 1 {
			return false, fmt.Errorf("exists requires 1 argument")
		}
		return !isEmptyInputValue(args[0]), nil
	case "platform":
		if len(args) != 1 {
			return false, fmt.Errorf("platform requires 1 argument")
		}
		return ctx.System["os"] == fmt.Sprint(args[0]), nil
	case "has_resource":
		if len(args) != 1 {
			return false, fmt.Errorf("has_resource requires 1 argument")
		}
		_, ok := ctx.Resources[fmt.Sprint(args[0])]
		return ok, nil
	default:
		return false, fmt.Errorf("unsupported condition function '%s'", name)
	}
}

func containsValue(list any, needle any) bool {
	switch typed := list.(type) {
	case []string:
		for _, item := range typed {
			if item == fmt.Sprint(needle) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if fmt.Sprint(item) == fmt.Sprint(needle) {
				return true
			}
		}
	case string:
		return typed == fmt.Sprint(needle)
	}
	return false
}

var interpolationPattern = regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)

func interpolateString(value string, ctx EvaluationContext) string {
	if value == "" {
		return value
	}
	return interpolationPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := interpolationPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		resolved := resolvePath(strings.TrimSpace(parts[1]), ctx)
		return fmt.Sprint(resolved)
	})
}

func interpolateStringSlice(values []string, ctx EvaluationContext) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, interpolateString(value, ctx))
	}
	return result
}

func interpolateStringMap(values map[string]string, ctx EvaluationContext) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[interpolateString(key, ctx)] = interpolateString(value, ctx)
	}
	return result
}

func interpolateResources(values []TemplateResource, ctx EvaluationContext) []ResourceDeclaration {
	result := make([]ResourceDeclaration, 0, len(values))
	for _, value := range values {
		result = append(result, ResourceDeclaration{
			Type:       interpolateString(value.Type, ctx),
			Name:       interpolateString(value.Name, ctx),
			ExternalID: interpolateString(value.ExternalID, ctx),
			Metadata:   value.Metadata,
		})
	}
	return result
}

func resolvePath(path string, ctx EvaluationContext) any {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}
	switch parts[0] {
	case "inputs":
		if len(parts) == 2 {
			return ctx.Inputs[parts[1]]
		}
	case "system":
		if len(parts) == 2 {
			return ctx.System[parts[1]]
		}
	case "resource":
		if len(parts) == 3 {
			resource, ok := ctx.Resources[parts[1]]
			if !ok {
				return nil
			}
			switch parts[2] {
			case "type":
				return resource.Type
			case "name":
				return resource.Name
			case "external_id":
				return resource.ExternalID
			}
		}
	default:
		return nil
	}
	return nil
}
