package agentskills_test

import (
	"io"
	"os"

	. "github.com/onsi/gomega"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(fn func()) string {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())
	os.Stdout = writer
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()
	defer func() { os.Stdout = original }()
	fn()
	Expect(writer.Close()).To(Succeed())
	return <-done
}
