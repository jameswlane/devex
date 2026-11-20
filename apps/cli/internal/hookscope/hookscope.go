// Package hookscope serializes guarded install hooks with declarative writes to
// user configuration. A guard restores protected files that change while its
// hook runs, so a concurrent shell_init or config_files write from another app
// would be undone and blamed on that hook.
package hookscope

import "sync"

var mu sync.RWMutex

// Hook holds the guard window (snapshot, hook, verify, restore). Hooks may run
// together; declarative writes wait for them. Call the returned func to release.
func Hook() func() {
	mu.RLock()
	return mu.RUnlock
}

// Write holds a declarative write to user configuration, excluding guarded hooks.
func Write() func() {
	mu.Lock()
	return mu.Unlock
}
