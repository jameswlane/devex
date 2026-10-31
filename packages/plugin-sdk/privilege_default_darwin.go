//go:build darwin

package sdk

import "os"

// Native Authorization Services replaces this default in #126.
func defaultPrivilegeAdapter() PrivilegeAdapter { return UnixSudoAdapter{} }
func defaultElevated() bool                     { return os.Geteuid() == 0 }
