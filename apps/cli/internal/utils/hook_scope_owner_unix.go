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

type fileIdentity struct {
	dev, ino uint64
	known    bool
}

func ownershipOf(info os.FileInfo) fileOwnership {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileOwnership{}
	}
	return fileOwnership{uid: int(stat.Uid), gid: int(stat.Gid), known: true}
}

func identityOf(info os.FileInfo) fileIdentity {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileIdentity{}
	}
	return fileIdentity{dev: uint64(stat.Dev), ino: stat.Ino, known: true} //nolint:unconvert // Dev is int32 on darwin
}

// introducedHardLink reports a different inode that has other names too.
func introducedHardLink(info os.FileInfo, before fileIdentity) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !before.known {
		return false
	}
	current := identityOf(info)
	return current != before && uint64(stat.Nlink) > 1 //nolint:unconvert // Nlink is uint16 on darwin
}

// restoreOwnership puts back the original owner. Root restores both; a regular
// user can restore only the group of a file it owns.
func restoreOwnership(path string, owner fileOwnership, link bool) error {
	if !owner.known {
		return nil
	}
	uid, gid := owner.uid, owner.gid
	if os.Geteuid() != 0 {
		info, err := os.Lstat(path)
		if !link {
			info, err = os.Stat(path)
		}
		if err != nil {
			return err
		}
		if current := ownershipOf(info); current.gid == gid || current.uid != os.Geteuid() {
			return nil
		}
		uid = -1
	}
	if link {
		return os.Lchown(path, uid, gid)
	}
	return os.Chown(path, uid, gid)
}
