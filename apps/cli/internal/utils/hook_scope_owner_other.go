//go:build !unix

package utils

import "os"

type fileOwnership struct{}

type fileIdentity struct{}

func ownershipOf(os.FileInfo) fileOwnership { return fileOwnership{} }

func identityOf(os.FileInfo) fileIdentity { return fileIdentity{} }

func introducedHardLink(os.FileInfo, fileIdentity) bool { return false }

func restoreOwnership(string, fileOwnership, bool) error { return nil }
