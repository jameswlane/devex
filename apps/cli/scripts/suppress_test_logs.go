package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	testhelperImport = `"github.com/jameswlane/devex/pkg/testhelper"`
	logSuppression   = `
// Set up test logging suppression for all tests in this suite
var _ = testhelper.SetupTestLogging()`
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run suppress_test_logs.go <directory>")
		os.Exit(1)
	}

	dir := os.Args[1]
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process *_suite_test.go files
		if !strings.HasSuffix(path, "_suite_test.go") {
			return nil
		}

		// Skip the testhelper package itself
		if strings.Contains(path, "testhelper") {
			return nil
		}

		fmt.Printf("Processing: %s\n", path)
		if err := updateTestSuite(path); err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else {
			fmt.Println("  Updated successfully")
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}
}

func updateTestSuite(path string) error {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	fileContent := string(content)

	// Check if already updated
	if strings.Contains(fileContent, "testhelper.SetupTestLogging()") {
		fmt.Println("  Already updated, skipping")
		return nil
	}

	// Find the import block
	importStart := strings.Index(fileContent, "import (")
	if importStart == -1 {
		return fmt.Errorf("no import block found")
	}

	importEnd := strings.Index(fileContent[importStart:], ")")
	if importEnd == -1 {
		return fmt.Errorf("import block not properly closed")
	}
	importEnd += importStart

	// Add testhelper import if not present
	if !strings.Contains(fileContent, testhelperImport) {
		// Find where to insert the import (after other imports but before ginkgo)
		importBlock := fileContent[importStart:importEnd]
		lines := strings.Split(importBlock, "\n")

		// Find the line with ginkgo import
		insertIndex := -1
		for i, line := range lines {
			if strings.Contains(line, `"github.com/onsi/ginkgo/v2"`) {
				insertIndex = i
				break
			}
		}

		if insertIndex == -1 {
			return fmt.Errorf("ginkgo import not found")
		}

		// Insert testhelper import before ginkgo
		lines = append(lines[:insertIndex], append([]string{"\t" + testhelperImport}, lines[insertIndex:]...)...)

		// Reconstruct the file
		newImportBlock := strings.Join(lines, "\n")
		fileContent = fileContent[:importStart] + newImportBlock + fileContent[importEnd:]
	}

	// Add log suppression at the end of the file
	fileContent = strings.TrimRight(fileContent, "\n") + logSuppression + "\n"

	// Write the updated content
	return ioutil.WriteFile(path, []byte(fileContent), 0644)
}
