//go:build !windows

package platform

import "fmt"

func detectNativeWindowsVersion() (string, error) {
	return "", fmt.Errorf("native Windows version detection is unavailable on this platform")
}
