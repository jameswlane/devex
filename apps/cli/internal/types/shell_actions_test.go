package types_test

import (
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shell action validation", func() {
	DescribeTable("rejects raw rc appends", func(target string) {
		err := (types.InstallCommand{Shell: "echo init >> " + target}).Validate()
		Expect(err).To(MatchError(ContainSubstring("use declarative shell_init or path")))
	}, Entry("bash", "~/.bashrc"), Entry("zsh", "$HOME/.zshrc"), Entry("profile", `"$HOME/.profile"`), Entry("bash profile", "${HOME}/.bash_profile"), Entry("fish", "~/.config/fish/config.fish"), Entry("quoted home", `"$HOME"/.bashrc`))
	It("checks alternatives and permits unrelated file writes", func() {
		config := types.OSConfig{Alternatives: []types.OSConfig{{PostInstall: []types.InstallCommand{{Shell: "echo bad >> ~/.bashrc"}}}}}
		Expect(config.ValidateShellActions()).To(HaveOccurred())
		Expect((types.InstallCommand{Shell: "echo log >> ~/.local/share/app.log"}).Validate()).To(Succeed())
		Expect((types.InstallCommand{ShellInit: map[string]string{"bash": `eval "$(zoxide init bash)"`}, Path: []string{"/opt/nvim/bin"}}).Validate()).To(Succeed())
	})
})
