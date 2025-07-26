package asciiart_test

import (
	"bytes"
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/asciiart"
	"github.com/jameswlane/devex/pkg/log"
)

var _ = Describe("Asciiart", func() {
	BeforeEach(func() {
		// Force ANSI rendering by setting TERM
		err := os.Setenv("TERM", "xterm-256color")
		if err != nil {
			return
		}
	})

	Context("RenderArt", func() {
		It("renders the ASCII art with styled lines", func() {
			// Capture the output of RenderArt
			r, w, _ := os.Pipe()
			oldStdout := os.Stdout
			os.Stdout = w

			// Run RenderArt
			asciiart.RenderArt()
			err := w.Close()
			if err != nil {
				return
			}

			// Read captured output
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, r); err != nil {
				log.Fatal("", err)
			}
			os.Stdout = oldStdout

			output := buf.String()

			// Ensure the output contains expected ASCII characters
			Expect(output).To(ContainSubstring("DDDDDDDDDDDDD"))
			Expect(output).To(ContainSubstring("EEEEEEEEEEEEEEEEEEEE"))
		})
	})
})
