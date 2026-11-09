// configcheck is the read-only CI entry point for the shipped configuration tree.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configcatalog"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	diagnostics, err := configcatalog.Validate(context.Background(), root)
	failed := err != nil
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != config.SeverityError {
			continue
		}
		failed = true
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", diagnostic.File, diagnostic.Field, diagnostic.Message)
	}
	if failed {
		os.Exit(1)
	}
	fmt.Println("Shipped configuration and asset validation passed")
}
