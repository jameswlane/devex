package remote

// SetMaxEntryBytes lowers the per-file limit for a spec and returns a restore
// function, so limit boundaries are tested without 100 MiB fixtures.
func SetMaxEntryBytes(limit int64) (restore func()) {
	previous := maxEntryBytes
	maxEntryBytes = limit
	return func() { maxEntryBytes = previous }
}

// SetMaxEntries lowers the archive entry limit for a spec and returns a
// restore function.
func SetMaxEntries(limit int) (restore func()) {
	previous := maxEntries
	maxEntries = limit
	return func() { maxEntries = previous }
}
