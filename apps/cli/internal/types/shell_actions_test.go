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

var _ = Describe("Consent validation for user configuration", func() {
	DescribeTable("rejects raw access to protected configuration", func(raw string) {
		Expect((types.InstallCommand{Shell: raw}).Validate()).To(HaveOccurred())
		Expect((types.InstallCommand{Command: raw}).Validate()).To(HaveOccurred())
	},
		Entry("overwrite rc", "echo init > ~/.zshrc"),
		Entry("tee rc", "echo init | tee ~/.bashrc"),
		Entry("edit another app", `sed -i s/a/b/ "$HOME/.config/other/config"`),
		Entry("quoted home", `cp example "${HOME}"/.config/other/config`),
		Entry("xdg config", `mkdir -p "$XDG_CONFIG_HOME/other"`),
		Entry("absolute home", "rm /home/example/.gitconfig"),
		Entry("windows config", `echo init >> %USERPROFILE%\.gitconfig`),
	)
	It("checks structured copy destinations and nested alternatives", func() {
		command := types.InstallCommand{Copy: &types.CopyCommand{Source: "example", Destination: "~/.config/other/config"}}
		Expect(command.Validate()).To(HaveOccurred())
		Expect((types.OSConfig{Alternatives: []types.OSConfig{{Alternatives: []types.OSConfig{{PostInstall: []types.InstallCommand{command}}}}}}).ValidateShellActions()).To(HaveOccurred())
	})
})
