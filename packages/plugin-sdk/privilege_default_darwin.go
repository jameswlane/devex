//go:build darwin

package sdk

import "os"

func defaultPrivilegeAdapter() PrivilegeAdapter { return macOSAuthorizationAdapter{} }
func defaultElevated() bool                     { return os.Geteuid() == 0 }
