package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: go run main.go [task]\nAvailable tasks:\n  mise-registry")
	}

	switch os.Args[1] {
	case "mise-registry":
		if err := GenerateMiseRegistryYAML(); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown task: %q", os.Args[1]) // #nosec G706 -- dev-only tool; argument is quoted
	}
}
