//go:build windows

package platform

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func detectNativeWindowsVersion() (string, error) {
	v := windows.RtlGetVersion()
	if v == nil {
		return "", fmt.Errorf("RtlGetVersion returned nil")
	}
	return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber), nil
}
