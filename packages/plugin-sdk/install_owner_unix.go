//go:build unix

package sdk

import (
	"fmt"
	"os"
	"syscall"
)

func checkDirectoryOwner(info os.FileInfo, uid int) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || uint64(stat.Uid) != uint64(uid) {
		return fmt.Errorf("plugin storage directory is not owned by intended UID %d", uid)
	}
	return nil
}
