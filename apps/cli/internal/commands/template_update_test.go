package commands_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

var _ = Describe("template update", func() {
	var configDir string

	BeforeEach(func() {
		configDir = GinkgoT().TempDir()
		GinkgoT().Setenv("DEVEX_CONFIG_DIR", configDir)
		DeferCleanup(os.Chdir, mustWorkingDir())
		Expect(os.Chdir(GinkgoT().TempDir())).To(Succeed())
	})

	run := func(args ...string) (string, error) {
		cmd := commands.NewTemplateCmd(nil, config.CrossPlatformSettings{}, nil)
		cmd.SetArgs(append([]string{"update"}, args...))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		reader, writer, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		stdout := os.Stdout
		os.Stdout = writer
		runErr := cmd.Execute()
		os.Stdout = stdout
		Expect(writer.Close()).To(Succeed())
		output, err := io.ReadAll(reader)
		Expect(err).NotTo(HaveOccurred())
		return string(output), runErr
	}

	It("verifies and records the built-in templates compiled into the binary", func() {
		output, err := run("--all", "--force", "--format", "json")

		Expect(err).NotTo(HaveOccurred())
		var results []templates.TemplateUpdateResult
		Expect(json.Unmarshal([]byte(output), &results)).To(Succeed())
		byID := map[string]templates.TemplateUpdateResult{}
		for _, result := range results {
			byID[result.TemplateID] = result
		}
		Expect(byID).To(HaveKey("backend"))
		Expect(byID["backend"].Success).To(BeTrue())
		Expect(byID["backend"].FilesVerified).To(ContainElement("applications.yaml"))
		Expect(byID["backend"].Message).To(ContainSubstring("no template files were changed"))
		Expect(filepath.Join(configDir, "config", templates.TemplateVersionFile)).To(BeARegularFile())
	})

	It("reports an up-to-date template without claiming a download", func() {
		_, err := run("--all", "--force")
		Expect(err).NotTo(HaveOccurred())

		output, err := run("backend")

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("devex self-update"))
		Expect(output).NotTo(ContainSubstring("Download"))
	})

	It("fails for a template that is not registered", func() {
		_, err := run("does-not-exist")

		Expect(err).To(MatchError(ContainSubstring("template not found: does-not-exist")))
	})
})

func mustWorkingDir() string {
	wd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	return wd
}
