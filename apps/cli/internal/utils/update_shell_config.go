package utils

import (
	"fmt"

	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
)

func UpdateShellConfig(shellPath, homeDir string, commands []string) error {
	shellRCPath, err := GetShellRCPath(shellPath, homeDir)
	if err != nil {
		return fmt.Errorf("failed to determine shell RC path: %w", err)
	}

	_, err = managed.Merge(shellRCPath, commands)
	return err
}
