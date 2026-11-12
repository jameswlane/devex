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
