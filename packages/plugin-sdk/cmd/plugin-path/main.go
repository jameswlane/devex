// plugin-path lets plugin developer install tasks share the runtime path resolver
// without requiring an installed copy of the CLI.
package main

import (
	"fmt"
	"github.com/jameswlane/devex/packages/plugin-sdk/paths"
	"os"
)

func main() {
	dirs, err := paths.Resolve()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(dirs.Plugins())
}
