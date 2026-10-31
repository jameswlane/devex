package state

import (
	"fmt"
	"path/filepath"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/spf13/viper"
)

const (
	DBPathConfigKey  = "state.db_path"
	DBPathEnvVar     = "DEVEX_STATE_DB"
	DBPathAutoEnvVar = "DEVEX_STATE_DB_PATH"
)

func DefaultDBPath(homeDir string) string {
	return devexpaths.ForHome(homeDir).ReadPath(filepath.Join(devexpaths.ForHome(homeDir).State, "state.db"))
}

func ConfiguredDBPath() (string, error) {
	if path := viper.GetString(DBPathConfigKey); path != "" {
		return path, nil
	}
	homeDir, err := devexHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return DefaultDBPath(homeDir), nil
}
