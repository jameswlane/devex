// Package migrations contains the CLI's versioned SQLite schema resources.
package migrations

import "embed"

// Files is embedded in the binary so startup never depends on the working directory.
//
//go:embed *_up.sql *_down.sql
var Files embed.FS
