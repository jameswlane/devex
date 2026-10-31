//go:build windows

package sdk

// Native UAC adapter and token-elevation detection are deferred to #133.
func defaultPrivilegeAdapter() PrivilegeAdapter { return unavailablePrivilegeAdapter{} }
func defaultElevated() bool                     { return false }
