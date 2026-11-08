package main

import "fmt"

// settingsBatch tracks a configure operation's independent, best-effort writes.
// A failed write is warned about and the remaining writes still run; the
// operation fails only when every attempted write failed, because then nothing
// was configured. The error wraps the first native failure so its exit status
// is preserved.
type settingsBatch struct {
	applied int
	failed  int
	first   error
}

// record counts one write's result and returns it unchanged.
func (b *settingsBatch) record(err error) error {
	if err == nil {
		b.applied++
		return nil
	}
	b.failed++
	if b.first == nil {
		b.first = err
	}
	return err
}

// err reports the complete-failure case; partial success returns nil.
func (b *settingsBatch) err(operation string) error {
	if b.applied > 0 || b.failed == 0 {
		return nil
	}
	return fmt.Errorf("%s applied no settings: all %d writes failed: %w", operation, b.failed, b.first)
}
