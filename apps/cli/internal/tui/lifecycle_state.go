package tui

import (
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// lifecycleKeyPrefix stores whether DevEx finished an app's install lifecycle
// (pre-install, install, config files, post-install hooks and registration).
// Installed packages alone do not show that the hooks ran.
const lifecycleKeyPrefix = "installed-app-lifecycle:"

const (
	lifecycleComplete   = "complete"
	lifecycleIncomplete = "incomplete"
)

func lifecycleKey(name string) string {
	return lifecycleKeyPrefix + strings.ToLower(strings.TrimSpace(name))
}

// LifecycleUnfinished reports whether DevEx started the app's install
// lifecycle and did not finish it, for example because a post-install hook
// failed or the run was interrupted. Such an app is not skipped as installed,
// even when its packages are present, so its hooks and registration are retried.
// Apps DevEx never started, such as ones installed outside DevEx, are not
// unfinished.
func LifecycleUnfinished(repo types.Repository, name string) bool {
	if repo == nil {
		return false
	}
	marker, err := repo.Get(lifecycleKey(name))
	return err == nil && marker == lifecycleIncomplete
}

// RecordLifecycle stores whether the app's install lifecycle finished. Install
// paths record it as unfinished before the first hook runs and as finished
// after registration, so a failed or interrupted run is retried.
func RecordLifecycle(repo types.Repository, name string, complete bool) error {
	if repo == nil {
		return nil
	}
	value := lifecycleIncomplete
	if complete {
		value = lifecycleComplete
	}
	return repo.Set(lifecycleKey(name), value)
}

// ClearLifecycle forgets the lifecycle state when an app is removed.
func ClearLifecycle(repo types.Repository, name string) error {
	if repo == nil {
		return nil
	}
	return repo.Set(lifecycleKey(name), "")
}

// SkippableInstalledApps drops installed apps whose DevEx lifecycle is
// unfinished, so they run it again instead of being skipped.
func SkippableInstalledApps(repo types.Repository, installed map[string]bool) map[string]bool {
	result := make(map[string]bool, len(installed))
	for name, present := range installed {
		if present && !LifecycleUnfinished(repo, name) {
			result[name] = true
		}
	}
	return result
}
