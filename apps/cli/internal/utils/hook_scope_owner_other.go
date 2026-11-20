//go:build !unix

package utils

import "os"

type fileOwnership struct{}

func ownershipOf(os.FileInfo) fileOwnership { return fileOwnership{} }

func restoreOwnership(string, fileOwnership, bool) error { return nil }
