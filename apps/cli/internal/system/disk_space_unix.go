//go:build unix

package system

import (
	"fmt"
	"syscall"
)

// getDiskSpaceInfo returns disk space information for Unix systems
func getDiskSpaceInfo(path string) (availableMB int, err error) {
	var stat syscall.Statfs_t
	err = syscall.Statfs(path, &stat)
	if err != nil {
		return 0, fmt.Errorf("failed to get disk space information: %w", err)
	}

	// Calculate available space in MB
	// Use safe integer arithmetic to prevent overflow
	blockSize := uint64(stat.Bsize) // #nosec G115 - Bsize is always positive in practice
	if blockSize > 0 && stat.Bavail > 0 {
		availableBytes := stat.Bavail * blockSize
		availableMB = int(availableBytes / (1024 * 1024)) // #nosec G115 - Division result is safe for int conversion
	} else {
		availableMB = 0
	}

	return availableMB, nil
}
