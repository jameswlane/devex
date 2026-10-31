//go:build windows

package sdk

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/windows"
)

func defaultPrivilegeAdapter() PrivilegeAdapter {
	return windowsPrivilegeAdapter{elevated: windowsTokenElevated}
}

func defaultElevated() bool {
	elevated, _ := windowsTokenElevated()
	return elevated
}

// Query TokenElevation rather than group membership: a filtered administrator
// token is not elevated. Keep errors so Run can fail closed if inspection fails.
func windowsTokenElevated() (bool, error) {
	var elevation [4]byte
	var returned uint32
	if err := windows.GetTokenInformation(windows.GetCurrentProcessToken(), windows.TokenElevation,
		&elevation[0], uint32(len(elevation)), &returned); err != nil {
		return false, fmt.Errorf("query Windows process token elevation: %w", err)
	}
	if returned != uint32(len(elevation)) {
		return false, fmt.Errorf("query Windows process token elevation: unexpected result size %d", returned)
	}
	return binary.LittleEndian.Uint32(elevation[:]) != 0, nil
}
