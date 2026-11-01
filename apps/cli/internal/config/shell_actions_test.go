package config_test

import (
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Catalog shell action validation", func() {
	It("rejects raw rc writes while loading the application catalog", func() {
		root := GinkgoT().TempDir()
		dir := filepath.Join(root, "applications")
		Expect(os.Mkdir(dir, 0755)).To(Succeed())
		path := filepath.Join(dir, "example.yaml")
		data := "name: example\ndescription: example\nlinux:\n  install_method: apt\n  install_command: example\n  alternatives:\n    - install_method: brew\n      post_install:\n        - shell: echo init >> ~/.zshrc\n"
		Expect(os.WriteFile(path, []byte(data), 0600)).To(Succeed())
		_, err := (config.FileApplicationLoader{}).LoadApplications([]string{root})
		Expect(err).To(MatchError(ContainSubstring("use declarative shell_init or path")))
		validator := config.NewConfigValidator(root, false)
		Expect(validator.ValidateDirectoryStructure(root)).To(HaveOccurred())
		Expect(validator.GetErrors()).NotTo(BeEmpty())
	})
	It("loads the migrated zoxide and neovim actions", func() {
		root, err := filepath.Abs("../../config")
		Expect(err).NotTo(HaveOccurred())
		apps, err := (config.FileApplicationLoader{}).LoadApplications([]string{root})
		Expect(err).NotTo(HaveOccurred())
		found := 0
		for _, app := range apps {
			if app.Name == "zoxide" {
				found++
				Expect(app.Linux.PostInstall).To(HaveLen(1))
				Expect(app.Linux.PostInstall[0].ShellInit).To(HaveKey("fish"))
			}
			if app.Name == "Neovim" {
				found++
				Expect(app.Linux.PostInstall[1].Path).To(Equal([]string{"/opt/nvim-linux-x86_64/bin"}))
			}
		}
		Expect(found).To(Equal(2))
	})
})
