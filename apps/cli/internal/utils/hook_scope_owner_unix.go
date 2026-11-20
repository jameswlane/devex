//go:build unix

package utils

import (
	"os"
	"syscall"
)

type fileOwnership struct {
	uid, gid int
	known    bool
}

func ownershipOf(info os.FileInfo) fileOwnership {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileOwnership{}
	}
	return fileOwnership{uid: int(stat.Uid), gid: int(stat.Gid), known: true}
}

// restoreOwnership puts back the original owner. Only root can change it, and
// only root (DevEx under sudo) creates files the user would not own.
func restoreOwnership(path string, owner fileOwnership, link bool) error {
	if !owner.known || os.Geteuid() != 0 {
		return nil
	}
	if link {
		return os.Lchown(path, owner.uid, owner.gid)
	}
	return os.Chown(path, owner.uid, owner.gid)
}
