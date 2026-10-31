//go:build unix && !darwin

package sdk

import "os"

func defaultPrivilegeAdapter() PrivilegeAdapter { return UnixSudoAdapter{} }
func defaultElevated() bool                     { return os.Geteuid() == 0 }
