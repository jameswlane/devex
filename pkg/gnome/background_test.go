package gnome

import (
	"github.com/jameswlane/devex/pkg/testutils"
	"os/exec"
	"testing"
)

func TestSetBackground(t *testing.T) {
	execCommand = testutils.MockExecCommand
	defer func() { execCommand = exec.Command }() // Reset after test

	err := SetBackground("/path/to/image.jpg")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestSetBackground_Error(t *testing.T) {
	execCommand = testutils.MockCommandWithError
	defer func() { execCommand = exec.Command }() // Reset after test

	err := SetBackground("/path/to/image.jpg")
	if err == nil {
		t.Errorf("Expected error, but got none")
	}
}
