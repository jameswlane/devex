//go:build !windows

package platform

import "context"

func (d *Detector) detectWindowsVersionContext(parent context.Context, platform *Platform) error {
	if err := parent.Err(); err != nil {
		return err
	}
	return d.detectWindowsVersionViaShell(parent, platform)
}
