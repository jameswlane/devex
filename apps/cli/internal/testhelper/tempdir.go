package testhelper

import (
	"os"
	"path/filepath"
	"runtime"
)

// ResolveTempDir points the platform's temporary directory environment
// variables at the symlink-free, long-path form of os.TempDir(). On macOS it
// lives under /var -> /private/var, and on Windows runners TMP/TEMP often use
// 8.3 short names (e.g. RUNNER~1) that filepath.EvalSymlinks expands; without
// this, resolved paths would not match paths specs construct under TempDir().
// Call it from the suite's Test function, before RunSpecs; it affects the whole
// test process.
func ResolveTempDir() {
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err == nil {
		_ = os.Setenv("TMPDIR", resolved)
		if runtime.GOOS == "windows" {
			_ = os.Setenv("TMP", resolved)
			_ = os.Setenv("TEMP", resolved)
		}
	}
}
