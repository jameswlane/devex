//go:build windows

package system

import (
	"fmt"
	"syscall"
	"unsafe"
)

// getDiskSpaceInfo returns disk space information for Windows systems
func getDiskSpaceInfo(path string) (availableMB int, err error) {
	kernel32, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		return 0, fmt.Errorf("failed to load kernel32.dll: %w", err)
	}
	defer syscall.FreeLibrary(kernel32)

	getDiskFreeSpaceEx, err := syscall.GetProcAddress(kernel32, "GetDiskFreeSpaceExW")
	if err != nil {
		return 0, fmt.Errorf("failed to get GetDiskFreeSpaceExW proc: %w", err)
	}

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("failed to convert path to UTF16: %w", err)
	}

	var freeBytesAvailable uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64

	ret, _, callErr := syscall.Syscall6(getDiskFreeSpaceEx,
		4,
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
		0,
		0)

	if ret == 0 {
		return 0, fmt.Errorf("GetDiskFreeSpaceExW failed: %v", callErr)
	}

	// Convert bytes to MB
	availableMB = int(freeBytesAvailable / (1024 * 1024))

	return availableMB, nil
}
