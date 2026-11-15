package main_test

import (
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/tool-stackdetector"
)

var _ = Describe("Command flags", func() {
	var (
		plugin  *main.StackDetectorPlugin
		project string
	)

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

	BeforeEach(func() {
		project = GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o600)).To(Succeed())
		plugin = main.NewStackDetectorPlugin()
	})

	DescribeTable("detect analyzes the requested directory",
		func(args func() []string) {
			output, err := capture(func() error { return plugin.Execute("detect", args()) })
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("Detecting development stack in: " + project))
		},
		Entry("with --path <dir>", func() []string { return []string{"--path", project} }),
		Entry("with --path=<dir>", func() []string { return []string{"--path=" + project} }),
		Entry("with a positional directory", func() []string { return []string{project} }),
	)

	It("rejects --path without a directory", func() {
		_, err := capture(func() error { return plugin.Execute("detect", []string{"--path"}) })
		Expect(err).To(MatchError(ContainSubstring("--path requires a directory")))
	})

	It("rejects unknown detect flags instead of treating them as a directory", func() {
		_, err := capture(func() error { return plugin.Execute("detect", []string{"--bogus"}) })
		Expect(err).To(MatchError(ContainSubstring(`unknown flag "--bogus"`)))
	})

	DescribeTable("--help prints usage",
		func(command, flag string) {
			output, err := capture(func() error { return plugin.Execute(command, []string{"--help"}) })
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("Usage: tool-stackdetector " + command))
			Expect(output).To(ContainSubstring("--" + flag))
		},
		Entry("detect", "detect", "path"),
		Entry("analyze", "analyze", "verbose"),
		Entry("report", "report", "format"),
	)

	It("accepts -h as help", func() {
		output, err := capture(func() error { return plugin.Execute("detect", []string{"-h"}) })
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("Usage: tool-stackdetector detect"))
	})
})
