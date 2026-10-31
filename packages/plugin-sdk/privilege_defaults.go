package sdk

import (
	"os"
	"os/user"
	"runtime"
)

func defaultPrivilegeResolver() PrivilegeResolver {
	return PrivilegeResolver{Elevated: defaultElevated(), CurrentUser: user.Current, LookupUser: user.Lookup, Getenv: os.Getenv}
}

// PrivilegeAdapterForOS selects the host's build-tagged default. Cross-platform
// inspection retains the Unix fallback and reports unavailable Windows elevation.
func PrivilegeAdapterForOS(goos string) PrivilegeAdapter {
	if goos == runtime.GOOS {
		return defaultPrivilegeAdapter()
	}
	if goos == "windows" {
		return unavailablePrivilegeAdapter{}
	}
	return UnixSudoAdapter{}
}
