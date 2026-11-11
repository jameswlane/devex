//go:build linux

package setup

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
)

var _ = Describe("Setup undo record", func() {
	It("records shell rc and Git identity changes that devex undo reverses", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "git")
		recorder.Script("git", []string{"config", "--global", "--get", "user.name"}, sdktest.Response{Stdout: "Old Name\n"})
		recorder.Script("git", []string{"config", "--global", "--get", "user.email"}, sdktest.Response{ExitCode: 1})
		home := os.Getenv("HOME")
		bashrc := filepath.Join(home, ".bashrc")
		Expect(os.WriteFile(bashrc, []byte("export EDITOR=vim\n"), 0o600)).To(Succeed())

		model := &SetupModel{settings: config.CrossPlatformSettings{HomeDir: home}}
		model.beginSystemUndo()
		_, err := managed.Apply(bashrc, []string{"export CLICOLOR=1"})
		Expect(err).NotTo(HaveOccurred())
		model.systemUndo.changes = append(model.systemUndo.changes, model.captureGitIdentity(context.Background(), "New Name", "new@example.com")...)
		model.finishSystemUndo()

		var undoLine string
		for _, change := range model.installation.getChanges() {
			if strings.HasPrefix(change, "Undo the shell, Git and desktop changes with: devex undo setup-") {
				undoLine = change
			}
		}
		Expect(undoLine).NotTo(BeEmpty(), "changes: %v", model.installation.getChanges())

		// A DevEx configuration edit made after setup must survive the undo.
		configFile := filepath.Join(devexpaths.ForHome(home).Config, "config", "applications.yaml")
		Expect(os.WriteFile(configFile, []byte("later-user-edit\n"), 0o600)).To(Succeed())

		recorder.Reset()
		result, err := undo.NewUndoManager(devexpaths.ForHome(home).Config).UndoLast(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(result.Message).To(Equal("Restored 3 setup changes"))

		data, err := os.ReadFile(bashrc)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("export EDITOR=vim\n"))
		edited, err := os.ReadFile(configFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(edited)).To(Equal("later-user-edit\n"))
		calls := recorder.Calls("git")
		restores := make([][]string, 0, len(calls))
		for _, call := range calls {
			restores = append(restores, call.Args)
		}
		Expect(restores).To(ConsistOf(
			[]string{"config", "--global", "--", "user.name", "Old Name"},
			[]string{"config", "--global", "--unset", "user.email"},
		))
	})

	It("records nothing when setup changed no shell, Git or desktop settings", func() {
		sdktest.NewRecorder(GinkgoT(), "git")
		model := &SetupModel{settings: config.CrossPlatformSettings{HomeDir: os.Getenv("HOME")}}
		model.beginSystemUndo()
		model.finishSystemUndo()
		Expect(model.installation.getChanges()).To(BeEmpty())
	})
})
