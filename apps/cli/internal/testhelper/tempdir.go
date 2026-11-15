package testhelper

import (
	"os"
	"path/filepath"
)

// ResolveTempDir points TMPDIR at the symlink-free form of the temporary
// directory. On macOS it lives under /var, a symlink to /private/var, so
// paths that code resolves would otherwise not match the paths specs expect
// there, unlike on Linux. Call it from the suite's
// Test function, before RunSpecs; it affects the whole test process.
func ResolveTempDir() {
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err == nil {
		_ = os.Setenv("TMPDIR", resolved)
	}
}
