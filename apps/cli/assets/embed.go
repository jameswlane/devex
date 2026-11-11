// Package assets holds the CLI assets that are compiled into the binary.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed templates
var templateFiles embed.FS

// Templates contains the built-in templates shipped with this DevEx release,
// rooted at the templates directory. Built-in templates change only when the
// binary changes (for example through devex self-update), so they always match the
// running CLI version.
var Templates fs.FS = mustSub(templateFiles, "templates")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err) // dir is a constant valid path; fs.Sub cannot fail for it.
	}
	return sub
}
