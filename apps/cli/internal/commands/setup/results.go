package setup

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// Preserve one error per failed app from the streaming installer's joined result.
func (m *SetupModel) recordInstallationErrors(err error) {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, failure := range joined.Unwrap() {
			m.recordInstallationErrors(failure)
		}
		return
	}
	m.installation.addError(err.Error())
}

// CompletionError reports every failed item and the log path to the command caller.
func (m *SetupModel) CompletionError() error {
	failures := m.installation.getErrors()
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("setup completed with %d errors:\n%s\nInstallation logs: %s", len(failures), strings.Join(failures, "\n"), log.GetLogFile())
}

func (m *SetupModel) logCompletion() {
	if err := m.CompletionError(); err != nil {
		log.Error(fmt.Sprintf("Setup completed with %d errors", len(m.installation.getErrors())), err, "log", log.GetLogFile())
	}
}
