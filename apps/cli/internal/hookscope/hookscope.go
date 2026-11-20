// Package hookscope serializes guarded install hooks with each other and with
// declarative writes to user configuration. A guard restores protected files
// that change while its hook runs, so an overlapping write (another app's
// shell_init or config_files, or another hook) would be undone and blamed on
// that hook, or a hook's snapshot could capture another hook's violation and
// put it back.
package hookscope

import "sync"

var mu sync.Mutex

// Hook holds the guard window (snapshot, hook, verify, restore). Call the
// returned func to release. Package installs outside hooks still run in parallel.
func Hook() func() {
	mu.Lock()
	return mu.Unlock
}

// Write holds a declarative write to user configuration.
func Write() func() {
	mu.Lock()
	return mu.Unlock
}
