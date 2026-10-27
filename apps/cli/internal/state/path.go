package state

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DBPathConfigKey  = "state.db_path"
	DBPathEnvVar     = "DEVEX_STATE_DB"
	DBPathAutoEnvVar = "DEVEX_STATE_DB_PATH"
)

func DefaultDBPath(homeDir string) string {
	return filepath.Join(homeDir, ".devex", "state.db")
}

func ConfiguredDBPath() (string, error) {
	if path := viper.GetString(DBPathConfigKey); path != "" {
		return path, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return DefaultDBPath(homeDir), nil
}
