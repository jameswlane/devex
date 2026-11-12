package backup

import "os"

// FailIndexWrites makes index writes stop halfway with err, as on a full
// disk, and returns a restore function.
func FailIndexWrites(err error) (restore func()) {
	previous := writeIndexData
	writeIndexData = func(file *os.File, data []byte) error {
		_, _ = file.Write(data[:len(data)/2])
		return err
	}
	return func() { writeIndexData = previous }
}

// BeforeFreshRestore runs hook once a restore has found no configuration to
// snapshot, and returns a restore function.
func BeforeFreshRestore(hook func(targetDir string)) (restore func()) {
	previous := beforeFreshRestore
	beforeFreshRestore = hook
	return func() { beforeFreshRestore = previous }
}
