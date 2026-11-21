//go:build windows

package system

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// getDiskSpaceInfo returns disk space information for Windows systems
func getDiskSpaceInfo(path string) (availableMB int, err error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("failed to convert path to UTF16: %w", err)
	}

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes); err != nil {
		return 0, fmt.Errorf("GetDiskFreeSpaceExW failed for %s: %w", path, err)
	}

	// Convert bytes to MB
	availableMB = int(freeBytesAvailable / (1024 * 1024))

	return availableMB, nil
}
