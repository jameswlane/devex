package gnome

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/utils"
)

// SetGSetting sets a value for a given Gnome schema and key using gsettings.
func SetGSetting(schema, key, value string) error {
	log.Info("Setting GSetting value", "schema", schema, "key", key, "value", value)

	// Construct the gsettings command
	command := fmt.Sprintf("gsettings set %s %s '%s'", schema, key, value)

	// Execute the command
	if _, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to set GSetting", err, "schema", schema, "key", key, "value", value)
		return fmt.Errorf("failed to set GSetting: %w", err)
	}

	log.Info("GSetting value set successfully", "schema", schema, "key", key, "value", value)
	return nil
}
