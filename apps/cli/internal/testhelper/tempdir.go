package testhelper

import (
	"os"
	"path/filepath"
)

// ResolveTempDir points TMPDIR at the symlink-free form of the temporary
// directory. On macOS it lives under /var, a symlink to /private/var, and
// DevEx storage refuses symlinked directories, so a temporary HOME would
// otherwise behave differently there than on Linux. Call it from the suite's
// Test function, before RunSpecs; it affects the whole test process.
func ResolveTempDir() {
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err == nil {
		_ = os.Setenv("TMPDIR", resolved)
	}
}
