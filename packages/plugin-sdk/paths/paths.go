// Package paths exposes DevEx's shared directory policy to CLI and plugin hosts.
// The SDK owns its implementation so its storage layer uses exactly the same policy.
package paths

import sdk "github.com/jameswlane/devex/packages/plugin-sdk"

type Dirs = sdk.Paths

func Resolve() (Dirs, error)   { return sdk.ResolvePaths() }
func ForHome(home string) Dirs { return sdk.PathsForHome(home) }
func ForPlatform(platform, home string, getenv func(string) string) Dirs {
	return sdk.ResolvePathsFor(platform, home, getenv)
}
