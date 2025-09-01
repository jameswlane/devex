package utils

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
)

func UpdateShellConfig(shellPath, homeDir string, commands []string) error {
	shellRCPath, err := GetShellRCPath(shellPath, homeDir)
	if err != nil {
		return fmt.Errorf("failed to determine shell RC path: %w", err)
	}

	existingContent, err := fs.ReadFileIfExists(shellRCPath)
	if err != nil {
		return fmt.Errorf("failed to read shell RC file: %w", err)
	}

	updatedContent := string(existingContent)
	for _, cmd := range commands {
		if !strings.Contains(updatedContent, cmd) {
			updatedContent += "\n" + cmd
		}
	}

	return fs.WriteFile(shellRCPath, []byte(updatedContent), 0o644)
}
