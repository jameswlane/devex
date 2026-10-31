package sdk

import (
	"os"
	"os/user"
	"runtime"
)

func defaultPrivilegeResolver() PrivilegeResolver {
	return PrivilegeResolver{Elevated: defaultElevated(), CurrentUser: user.Current, LookupUser: user.Lookup, Getenv: privilegeGetenv}
}

func privilegeGetenv(key string) string {
	// SUDO_USER is a Unix convention, not a Windows account provenance signal.
	if runtime.GOOS == "windows" && key == "SUDO_USER" {
		return ""
	}
	return os.Getenv(key)
}

// PrivilegeAdapterForOS selects the host's build-tagged default. Cross-platform
// inspection retains the Unix fallback; Windows inspection requires a native host.
func PrivilegeAdapterForOS(goos string) PrivilegeAdapter {
	if goos == runtime.GOOS {
		return defaultPrivilegeAdapter()
	}
	if goos == "windows" {
		return unavailablePrivilegeAdapter{}
	}
	return UnixSudoAdapter{}
}
