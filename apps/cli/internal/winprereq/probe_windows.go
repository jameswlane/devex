//go:build windows

package winprereq

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os/exec"

	"golang.org/x/sys/windows/registry"
)

// SystemProbe reads HKEY_LOCAL_MACHINE and PATH on this machine.
func SystemProbe() Probe {
	return Probe{ReadDWORD: readDWORD, WriteDWORD: writeDWORD, LookPath: exec.LookPath}
}

func readDWORD(key, value string) (uint32, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE)
	if err != nil {
		return 0, notExist(err)
	}
	defer func() { _ = k.Close() }()
	data, kind, err := k.GetIntegerValue(value)
	if err != nil {
		return 0, notExist(err)
	}
	if kind != registry.DWORD || data > math.MaxUint32 {
		return 0, fmt.Errorf("value has registry type %d, want REG_DWORD", kind)
	}
	return uint32(data), nil
}

func writeDWORD(key, value string, data uint32) error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, key, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = k.Close() }()
	return k.SetDWordValue(value, data)
}

// notExist marks a missing key or value so Check reports it as off.
func notExist(err error) error {
	if errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("%w: %w", fs.ErrNotExist, err)
	}
	return err
}
