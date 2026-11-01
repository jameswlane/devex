package bootstrap

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Unattended plugin bootstrap", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "DEVEX_UNATTENDED", "DEVEX_NONINTERACTIVE", "DEVEX_NON_INTERACTIVE"} {
			GinkgoT().Setenv(key, "")
		}
		GinkgoT().Setenv("TERM", "xterm")
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
	})
	DescribeTable("forwards interaction policy to every execution entry point", func(mode, entry string) {
		switch mode {
		case "flag":
			viper.Set("non-interactive", true)
		case "ci":
			GinkgoT().Setenv("CI", "true")
		}
		dir := GinkgoT().TempDir()
		script := `#!/bin/sh
if [ "${1-}" = --plugin-info ]; then
 echo '{"name":"example","version":"1.0.0"}'
 exit 0
fi
printf '%s\n' "${DEVEX_UNATTENDED:-0}"
if read -r line; then printf '%s\n' "$line"; fi
`
		Expect(os.WriteFile(filepath.Join(dir, "devex-plugin-example"), []byte(script), 0700)).To(Succeed())
		bootstrap := &PluginBootstrap{manager: sdk.NewExecutableManager(dir)}
		ctx := context.Background()
		if entry == "io" {
			var output bytes.Buffer
			Expect(bootstrap.ExecutePluginWithIO(ctx, "example", []string{"install"}, sdk.OperationIO{Stdin: strings.NewReader("caller input\n"), Stdout: &output})).To(Succeed())
			if mode == "interactive" {
				Expect(output.String()).To(Equal("0\ncaller input\n"))
			} else {
				Expect(output.String()).To(Equal("1\n"))
			}
		} else {
			// Use a pipe to capture the default streams for ExecutePlugin*.
			output, writer, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = output.Close() }()
			original := os.Stdout
			os.Stdout = writer
			defer func() { os.Stdout = original; _ = writer.Close() }()
			input, err := os.Open(os.DevNull)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = input.Close() }()
			originalInput := os.Stdin
			os.Stdin = input
			defer func() { os.Stdin = originalInput }()
			if entry == "context" {
				Expect(bootstrap.ExecutePluginWithContext(ctx, "example", []string{"install"})).To(Succeed())
			} else {
				Expect(bootstrap.ExecutePlugin("example", []string{"install"})).To(Succeed())
			}
			Expect(writer.Close()).To(Succeed())
			data, err := io.ReadAll(output)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("1\n"))
		}
	},
		Entry("non-interactive flag with streams", "flag", "io"),
		Entry("CI with streams", "ci", "io"),
		Entry("interactive streams", "interactive", "io"),
		Entry("context execution", "flag", "context"),
		Entry("default execution", "ci", "default"),
	)
})
