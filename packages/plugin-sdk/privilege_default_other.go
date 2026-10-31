//go:build !unix && !windows

package sdk

func defaultPrivilegeAdapter() PrivilegeAdapter { return unavailablePrivilegeAdapter{} }
func defaultElevated() bool                     { return false }
