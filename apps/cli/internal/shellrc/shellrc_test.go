package shellrc_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rc "github.com/jameswlane/devex/apps/cli/internal/shellrc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShellActions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Declarative shell actions")
}

var _ = Describe("Declarative shell actions", func() {
	var home string
	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("SHELL", "/bin/bash")
	})
	It("honors setup's selection and never creates another shell's rc", func() {
		init := map[string]string{"bash": `eval "$(zoxide init bash)"`, "zsh": `eval "$(zoxide init zsh)"`, "fish": "zoxide init fish | source"}
		ctx := rc.WithShell(context.Background(), "zsh")
		changed, err := rc.ApplyActions(ctx, init, []string{"/opt/nvim/bin"})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
		path := filepath.Join(home, ".zshrc")
		first, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(first)).To(ContainSubstring("zoxide init zsh"))
		Expect(string(first)).To(ContainSubstring(`case ":$PATH:"`))
		changed, err = rc.ApplyActions(ctx, init, []string{"/opt/nvim/bin"})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
		second, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(Equal(first))
		Expect(filepath.Join(home, ".bashrc")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(home, ".config/fish/config.fish")).NotTo(BeAnExistingFile())
	})
	It("derives the current shell and retains entries across different apps", func() {
		_, err := rc.ApplyActions(context.Background(), map[string]string{"bash": "source ~/.tools/init"}, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = rc.ApplyActions(context.Background(), nil, []string{"/opt/nvim/bin", "/opt/nvim/bin"})
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(filepath.Join(home, ".bashrc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("source ~/.tools/init"))
		Expect(strings.Count(string(data), "export PATH=")).To(Equal(1))
	})
	It("skips existing PATH additions using equivalent home syntax", func() {
		original := []byte("export PATH=\"$HOME/.local/bin:$PATH\"\n")
		path := filepath.Join(home, ".bashrc")
		Expect(os.WriteFile(path, original, 0600)).To(Succeed())
		changed, err := rc.ApplyActions(context.Background(), nil, []string{"~/.local/bin"})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(Equal(original))
	})
	It("does not create an rc file for a directory already on PATH", func() {
		GinkgoT().Setenv("PATH", "/usr/bin:/opt/nvim/bin")
		changed, err := rc.ApplyActions(context.Background(), nil, []string{"/opt/nvim/bin"})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
		Expect(filepath.Join(home, ".bashrc")).NotTo(BeAnExistingFile())
	})
	It("emits fish syntax for fish only", func() {
		_, err := rc.ApplyActions(rc.WithShell(context.Background(), "fish"), nil, []string{"/opt/nvim/bin"})
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(filepath.Join(home, ".config/fish/config.fish"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("if not contains --"))
		Expect(string(data)).NotTo(ContainSubstring("export PATH="))
		Expect(filepath.Join(home, ".bashrc")).NotTo(BeAnExistingFile())
	})
})
