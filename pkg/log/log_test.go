package log_test

import (
	"os"
	"testing"

	logger "github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/log"
)

func TestLogger(t *testing.T) {
	t.Parallel()
	log.ConfigureLogger(logger.DebugLevel, os.Stdout)
	log.Info("Test Info Message", "key", "value")
	log.Debug("Test Debug Message", "key", "value")
	log.Error("Test Error Message", "key", "value")
}
