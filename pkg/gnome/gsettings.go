package gnome

import (
	"fmt"
	"os/exec"
)

// gsettingsExecCommand is a variable to allow mocking for tests
var gsettingsExecCommand = exec.Command

// SetGSetting sets a value for a given Gnome schema and key using gsettings
func SetGSetting(schema, key, value string) error {
	cmd := gsettingsExecCommand("gsettings", "set", schema, key, value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set GSetting: %v - %s", err, string(output))
	}
	return nil
}
