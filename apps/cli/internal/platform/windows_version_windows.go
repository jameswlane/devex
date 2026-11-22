//go:build windows

package platform

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows"
)

func (d *Detector) detectWindowsVersionContext(parent context.Context, platform *Platform) error {
	if err := parent.Err(); err != nil {
		return err
	}
	if v := windows.RtlGetVersion(); v != nil {
		platform.Version = fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
		platform.Distribution = "windows"
		return nil
	}
	return d.detectWindowsVersionViaShell(parent, platform)
}
