package utils

import "fmt"

type MockCommandExecutor struct {
	Commands map[string]string // Maps command strings to their output
	Errors   map[string]error  // Maps command strings to errors
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		Commands: make(map[string]string),
		Errors:   make(map[string]error),
	}
}

func (m *MockCommandExecutor) RunCommand(name string, args ...string) (string, error) {
	cmd := fmt.Sprintf("%s %v", name, args)
	if err, exists := m.Errors[cmd]; exists {
		return "", err
	}
	if output, exists := m.Commands[cmd]; exists {
		return output, nil
	}
	return "", fmt.Errorf("command not found: %s", cmd)
}
