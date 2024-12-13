package gnome

import (
	"fmt"
	"os/exec"
)

// execCommand is a variable to allow mocking for tests
var execCommand = exec.Command

// SetBackground sets the desktop background in Gnome using gsettings
func SetBackground(imagePath string) error {
	cmd := execCommand("gsettings", "set", "org.gnome.desktop.background", "picture-uri", "file://"+imagePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set background image: %v - %s", err, string(output))
	}
	return nil
}
