package templates

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const DefaultInputEnvPrefix = "DEVEX_INPUT_"

type InputResolverOptions struct {
	Answers        []string
	AnswerFiles    []string
	EnvPrefix      string
	NonInteractive bool
	Prompt         InputPromptFunc
}

type InputPromptFunc func(ctx context.Context, key string, input TemplateInput) (any, error)

func ResolveInputs(ctx context.Context, spec TemplateSpec, opts InputResolverOptions) (map[string]any, error) {
	values := make(map[string]any, len(spec.Inputs))
	sources := make(map[string]string, len(spec.Inputs))

	envPrefix := opts.EnvPrefix
	if envPrefix == "" {
		envPrefix = DefaultInputEnvPrefix
	}
	for key, input := range spec.Inputs {
		if _, ok := values[key]; !ok && input.Default != nil {
			values[key] = input.Default
			sources[key] = "default"
		}
	}

	for key := range spec.Inputs {
		if value, ok := os.LookupEnv(envPrefix + envKey(key)); ok {
			values[key] = value
			sources[key] = "environment"
		}
	}

	fileAnswers, err := loadAnswerFiles(opts.AnswerFiles)
	if err != nil {
		return nil, err
	}
	for key, value := range fileAnswers {
		values[key] = value
		sources[key] = "answers file"
	}

	cliAnswers, err := parseCLIAnswers(opts.Answers)
	if err != nil {
		return nil, err
	}
	for key, value := range cliAnswers {
		values[key] = value
		sources[key] = "cli"
	}

	prompt := opts.Prompt
	if prompt == nil {
		prompt = terminalPrompt
	}

	for key, input := range spec.Inputs {
		value, ok := values[key]
		if !ok || isEmptyInputValue(value) {
			if input.Required {
				if opts.NonInteractive {
					return nil, fmt.Errorf("required input '%s' is missing in non-interactive mode", key)
				}
				value, err = prompt(ctx, key, input)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve input '%s': %w", key, err)
				}
				values[key] = value
				sources[key] = "interactive"
			}
		}
	}

	resolved := make(map[string]any, len(values))
	for key, input := range spec.Inputs {
		value, ok := values[key]
		if !ok {
			continue
		}
		coerced, err := coerceInputValue(input, value)
		if err != nil {
			return nil, fmt.Errorf("invalid input '%s' from %s: %w", key, sources[key], err)
		}
		if err := validateInputValue(key, input, coerced); err != nil {
			return nil, err
		}
		resolved[key] = coerced
	}

	return resolved, nil
}

func loadAnswerFiles(paths []string) (map[string]any, error) {
	answers := make(map[string]any)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read answers file %s: %w", path, err)
		}
		fileAnswers := make(map[string]any)
		if err := yaml.Unmarshal(data, &fileAnswers); err != nil {
			return nil, fmt.Errorf("failed to parse answers file %s: %w", path, err)
		}
		for key, value := range fileAnswers {
			answers[key] = value
		}
	}
	return answers, nil
}

func parseCLIAnswers(raw []string) (map[string]any, error) {
	answers := make(map[string]any, len(raw))
	for _, item := range raw {
		key, value, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("answer must use key=value format: %s", item)
		}
		answers[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return answers, nil
}

func coerceInputValue(input TemplateInput, value any) (any, error) {
	switch input.Type {
	case "text", "select":
		return fmt.Sprint(value), nil
	case "bool":
		switch typed := value.(type) {
		case bool:
			return typed, nil
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
			if err != nil {
				return nil, err
			}
			return parsed, nil
		default:
			return nil, fmt.Errorf("expected bool")
		}
	case "multiselect":
		return coerceStringSlice(value)
	default:
		return nil, fmt.Errorf("unsupported input type '%s'", input.Type)
	}
}

func coerceStringSlice(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
		return values, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return []string{}, nil
		}
		parts := strings.Split(typed, ",")
		values := make([]string, 0, len(parts))
		for _, part := range parts {
			values = append(values, strings.TrimSpace(part))
		}
		return values, nil
	default:
		return nil, fmt.Errorf("expected string list")
	}
}

func validateInputValue(key string, input TemplateInput, value any) error {
	if input.Required && isEmptyInputValue(value) {
		return fmt.Errorf("required input '%s' is empty", key)
	}
	if len(input.Options) > 0 {
		allowed := make(map[string]bool, len(input.Options))
		for _, option := range input.Options {
			allowed[option.Value] = true
		}
		values, ok := value.([]string)
		if !ok {
			values = []string{fmt.Sprint(value)}
		}
		for _, item := range values {
			if item != "" && !allowed[item] {
				return fmt.Errorf("input '%s' has unsupported option '%s'", key, item)
			}
		}
	}
	if input.Validation == nil {
		return nil
	}
	text := fmt.Sprint(value)
	if input.Validation.Pattern != "" {
		matched, err := regexp.MatchString(input.Validation.Pattern, text)
		if err != nil {
			return fmt.Errorf("input '%s' has invalid validation pattern: %w", key, err)
		}
		if !matched {
			if input.Validation.Message != "" {
				return fmt.Errorf("input '%s': %s", key, input.Validation.Message)
			}
			return fmt.Errorf("input '%s' does not match required pattern", key)
		}
	}
	if input.Validation.Min != nil && len(text) < *input.Validation.Min {
		return fmt.Errorf("input '%s' is shorter than %d characters", key, *input.Validation.Min)
	}
	if input.Validation.Max != nil && len(text) > *input.Validation.Max {
		return fmt.Errorf("input '%s' is longer than %d characters", key, *input.Validation.Max)
	}
	return nil
}

func isEmptyInputValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

func envKey(key string) string {
	key = strings.ToUpper(key)
	return strings.NewReplacer("-", "_", ".", "_").Replace(key)
}

func terminalPrompt(ctx context.Context, key string, input TemplateInput) (any, error) {
	prompt := input.Prompt
	if prompt == "" {
		prompt = key
	}
	fmt.Printf("%s: ", prompt)

	type readResult struct {
		value string
		err   error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		if input.Sensitive && term.IsTerminal(int(os.Stdin.Fd())) {
			value, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			resultCh <- readResult{value: string(value), err: err}
			return
		}

		reader := bufio.NewReader(os.Stdin)
		value, err := reader.ReadString('\n')
		resultCh <- readResult{value: value, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.err != nil {
			return nil, result.err
		}
		return strings.TrimSpace(result.value), nil
	}
}
