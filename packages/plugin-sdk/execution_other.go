//go:build !unix

package sdk

import "os/exec"

func configureCancellation(cmd *exec.Cmd) {}
