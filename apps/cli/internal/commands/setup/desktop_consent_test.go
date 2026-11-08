package setup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type consentExecutor struct {
	calls  [][]string
	output string
	fail   error
}

func (e *consentExecutor) ExecutePluginWithIO(_ context.Context, _ string, args []string, streams sdk.OperationIO) error {
	e.calls = append(e.calls, args)
	if streams.Stdout != nil {
		_, _ = io.WriteString(streams.Stdout, e.output)
	}
	return e.fail
}

var _ = Describe("Desktop settings consent", func() {
	It("returns from automated dry-run before installing or finalizing", func() {
		original := viper.GetBool("dry-run")
		viper.Set("dry-run", true)
		DeferCleanup(func() { viper.Set("dry-run", original) })
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			// A nil repository would fail if installation/finalization were attempted.
			Expect(RunAutomatedSetup(context.Background(), nil, config.CrossPlatformSettings{})).To(Succeed())
		})
		Expect(output).To(ContainSubstring("Desktop settings"))
		Expect(output).To(ContainSubstring("Setup will make these changes"))
	})
	const preview = `[["gsettings","set","org.gnome.desktop.interface","color-scheme","prefer-dark"]]`
	It("requires a visible preview and explicit opt-in", func() {
		m := &SetupModel{step: StepConfirmation}
		executor := &consentExecutor{}
		Expect(m.applyDesktopPreview(context.Background(), executor)).To(Succeed())
		Expect(executor.calls).To(BeEmpty())
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		Expect(m.selections.desktopSettings).To(BeFalse())
		m.selections.desktopPreview = preview
		m.selections.desktopPlugin = "desktop-gnome"
		Expect(m.renderDesktopPreview()).To(ContainSubstring("[ ] disabled"))
		Expect(m.renderDesktopPreview()).To(ContainSubstring("color-scheme"))
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		Expect(m.renderDesktopPreview()).To(ContainSubstring("[x] enabled"))
		Expect(m.applyDesktopPreview(context.Background(), executor)).To(Succeed())
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(preview)))
		Expect(executor.calls).To(Equal([][]string{{"configure", "--expected-plan", hash}}))
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		Expect(m.applyDesktopPreview(context.Background(), executor)).To(Succeed())
		Expect(executor.calls).To(HaveLen(1))
	})
	It("validates previews and propagates plugin failures", func() {
		for _, output := range []string{"", "[]", "null", "[[]]", "invalid"} {
			_, err := describeDesktop(context.Background(), &consentExecutor{output: output}, "desktop-gnome")
			Expect(err).To(HaveOccurred())
		}
		actual, err := describeDesktop(context.Background(), &consentExecutor{output: preview + "\n"}, "desktop-gnome")
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).To(Equal(preview))
		_, err = describeDesktop(context.Background(), &consentExecutor{fail: fmt.Errorf("unavailable")}, "desktop-gnome")
		Expect(err).To(MatchError("unavailable"))
	})
})
