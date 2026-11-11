package configfiles_test

import (
	"os/exec"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shipped Neovim Lua assets", func() {
	It("loads every Lua file in isolated headless Neovim without network access", func(ctx SpecContext) {
		nvim, err := exec.LookPath("nvim")
		if err != nil {
			Skip("nvim is not installed")
		}
		assets, err := filepath.Glob("../../assets/defaults/neovim/*.lua")
		Expect(err).NotTo(HaveOccurred())
		themes, err := filepath.Glob("../../assets/themes/neovim/*.lua")
		Expect(err).NotTo(HaveOccurred())
		assets = append(assets, themes...)
		Expect(assets).NotTo(BeEmpty())

		for _, asset := range assets {
			path, err := filepath.Abs(asset)
			Expect(err).NotTo(HaveOccurred())
			command := exec.CommandContext(ctx, nvim, "--headless", "-u", "NONE", "-i", "NONE", "-n", "-c", "lua dofile("+strconv.Quote(path)+")", "-c", "qa!")
			command.Env = []string{"HOME=" + GinkgoT().TempDir(), "XDG_CONFIG_HOME=" + GinkgoT().TempDir(), "XDG_DATA_HOME=" + GinkgoT().TempDir(), "XDG_STATE_HOME=" + GinkgoT().TempDir()}
			output, err := command.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "%s: %s", asset, output)
		}
	})
})
