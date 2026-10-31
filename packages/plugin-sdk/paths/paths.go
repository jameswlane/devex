// Package paths exposes DevEx's shared directory policy to CLI and plugin hosts.
// The SDK owns its implementation so its storage layer uses exactly the same policy.
package paths

import sdk "github.com/jameswlane/devex/packages/plugin-sdk"

type Dirs = sdk.Paths

var Resolve = sdk.ResolvePaths
var ForHome = sdk.PathsForHome
var ForPlatform = sdk.ResolvePathsFor
