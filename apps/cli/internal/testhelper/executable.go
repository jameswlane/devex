package testhelper

import (
	"path/filepath"
	"runtime"
)

// Executable returns the path of a binary named name in dir, adding the
// ".exe" suffix Windows needs to run it. Use it for binaries specs build with
// "go build -o", so the spec can exec them on every platform.
func Executable(dir, name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}
