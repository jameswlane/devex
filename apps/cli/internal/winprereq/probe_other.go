//go:build !windows

package winprereq

import "os/exec"

// SystemProbe has no registry outside Windows, so registry checks report unknown.
func SystemProbe() Probe {
	return Probe{LookPath: exec.LookPath}
}
