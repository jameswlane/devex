package main_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	main "github.com/jameswlane/devex/packages/tool-stackdetector"
)

var _ = Describe("Machine-readable stack report", func() {
	capture := func(run func() error) (string, error) {
		reader, writer, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		stdout := os.Stdout
		os.Stdout = writer
		runErr := run()
		os.Stdout = stdout
		Expect(writer.Close()).To(Succeed())
		output, err := io.ReadAll(reader)
		Expect(err).NotTo(HaveOccurred())
		return string(output), runErr
	}

	It("writes only the JSON report to stdout, with lowercase keys", func() {
		project := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(project, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600)).To(Succeed())
		plugin := &main.StackDetectorPlugin{BasePlugin: sdk.NewBasePlugin(sdk.PluginInfo{Name: "tool-stackdetector", Version: "test"})}

		output, err := capture(func() error { return plugin.Execute("report", []string{"--format", "json", "--path", project}) })

		Expect(err).NotTo(HaveOccurred())
		var report map[string]any
		Expect(json.Unmarshal([]byte(output), &report)).To(Succeed(), output)
		Expect(report).To(HaveKey("technologies"))
		Expect(report).To(HaveKey("project_size"))
		technologies, ok := report["technologies"].([]any)
		Expect(ok).To(BeTrue())
		Expect(technologies).NotTo(BeEmpty())
		first, ok := technologies[0].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(first).To(HaveKey("name"))
		Expect(first).To(HaveKey("confidence"))
		names := []any{}
		for _, technology := range technologies {
			names = append(names, technology.(map[string]any)["name"])
		}
		Expect(names).To(ContainElement("Go"))
	})
})
