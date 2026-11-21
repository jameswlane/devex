package configfiles_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

// shippedThemes lists every Neovim theme asset with the colorscheme its
// LazyVim spec selects and the colors_name prefix the real plugin reports.
var shippedThemes = map[string]struct{ colorscheme, realPrefix string }{
	"catppuccin.lua":  {"catppuccin", "catppuccin"},
	"everforest.lua":  {"everforest", "everforest"},
	"gruvbox.lua":     {"gruvbox", "gruvbox"},
	"kanagawa.lua":    {"kanagawa", "kanagawa"},
	"nord.lua":        {"nord", "nord"},
	"rose-pine.lua":   {"rose-pine-dawn", "rose-pine"},
	"tokyo-night.lua": {"tokyonight", "tokyonight"},
}

// stubLazy is a test-only stand-in for lazy.nvim and LazyVim. It imports the
// user's lua/plugins specs the way lazy.nvim does, merges the LazyVim opts and
// applies the chosen colorscheme with an opaque background, so the test can
// see whether the transparency snippet runs after the theme.
const stubLazy = `
local M = {}
function M.setup(opts)
  local lazyvim = {}
  for _, entry in ipairs(opts.spec) do
    if entry.import == "plugins" then
      for _, file in ipairs(vim.api.nvim_get_runtime_file("lua/plugins/*.lua", true)) do
        for _, spec in ipairs(dofile(file)) do
          if spec[1] == "LazyVim/LazyVim" then
            lazyvim = vim.tbl_deep_extend("force", lazyvim, spec.opts or {})
          end
        end
      end
    end
  end
  assert(type(lazyvim.colorscheme) == "string", "no LazyVim colorscheme configured")
  vim.g.colors_name = lazyvim.colorscheme
  vim.api.nvim_set_hl(0, "Normal", { bg = 0x123456 })
end
return M
`

const defaultNetworkTimeout = 15 * time.Minute

const userInit = `require("config.lazy")` + "\n"

const lazyBootstrap = `require("lazy").setup({ spec = { { "LazyVim/LazyVim", import = "lazyvim.plugins" }, { import = "plugins" } } })
`

// failOnError exits with an error when startup reported one, so a theme or
// snippet that fails part way cannot pass on its earlier effects.
const failOnError = `lua if vim.v.errmsg ~= "" then io.stdout:write("\nERROR " .. vim.v.errmsg .. "\n"); vim.cmd("cquit 1") end`

const reportTheme = `lua io.stdout:write("\nTHEME " .. tostring(vim.g.colors_name) .. " " .. tostring(vim.api.nvim_get_hl(0, { name = "Normal" }).bg) .. "\n")`

func neovimMappings() ([]types.ConfigFile, []types.ConfigFile) {
	data, err := os.ReadFile("../../config/applications/development/neovim.yaml")
	Expect(err).NotTo(HaveOccurred())
	var app struct {
		Linux types.OSConfig `yaml:"linux"`
	}
	Expect(yaml.Unmarshal(data, &app)).To(Succeed())
	Expect(app.Linux.Themes).NotTo(BeEmpty())
	return app.Linux.ConfigFiles, app.Linux.Themes[0].Files
}

// installAssets copies the repository assets to where setup places them.
func installAssets(home string) {
	share := filepath.Join(home, ".local", "share", "devex")
	for from, to := range map[string]string{
		"../../assets/defaults/neovim": filepath.Join(share, "assets", "defaults", "neovim"),
		"../../assets/themes/neovim":   filepath.Join(share, "themes", "neovim"),
	} {
		Expect(os.CopyFS(to, os.DirFS(from))).To(Succeed())
	}
}

// neovimEnv confines Neovim to home, including the system-wide config and
// data directories, so host plugins cannot affect the result.
func neovimEnv(home string) []string {
	empty := filepath.Join(home, "empty")
	Expect(os.MkdirAll(empty, 0700)).To(Succeed())
	return []string{
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
		"XDG_CONFIG_DIRS=" + empty,
		"XDG_DATA_DIRS=" + empty,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME=" + filepath.Join(home, ".local", "state"),
		"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"),
	}
}

// loadedTheme starts Neovim with the user config under home and returns the
// active colorscheme and the Normal background ("nil" when transparent). In
// strict mode any startup error fails.
func loadedTheme(ctx context.Context, nvim, home string, strict bool, extra ...string) (string, string) {
	args := append([]string{"--headless", "-i", "NONE", "-n"}, extra...)
	if strict {
		args = append(args, "-c", failOnError)
	}
	args = append(args, "-c", reportTheme, "-c", "qa!")
	command := exec.CommandContext(ctx, nvim, args...)
	command.Dir = home
	command.Env = neovimEnv(home)
	output, err := command.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "%s", output)
	for line := range strings.SplitSeq(string(output), "\n") {
		if fields := strings.Fields(line); len(fields) == 3 && fields[0] == "THEME" {
			return fields[1], fields[2]
		}
	}
	Fail("Neovim printed no theme report: " + string(output))
	return "", ""
}

const userLazyVimJSON = `{ "extras": [], "version": 6, "user": true }` + "\n"

func backups(home string) []string {
	var found []string
	Expect(filepath.WalkDir(filepath.Join(home, ".config", "nvim"), func(path string, _ os.DirEntry, err error) error {
		if strings.HasSuffix(path, ".devex-bak") {
			found = append(found, path)
		}
		return err
	})).To(Succeed())
	return found
}

// applyNeovimConfig seeds a customized lazyvim.json, then applies the
// catalog's Neovim config files and theme files twice, as first and repeated
// setup do. The user's file must be backed up once and never again. It
// returns the catalog's theme mapping.
func applyNeovimConfig(ctx context.Context, home string) types.ConfigFile {
	configs, themeFiles := neovimMappings()
	Expect(themeFiles).To(HaveLen(1))
	// LazyVim reads stdpath("config")/lazyvim.json, so the catalog must map it there.
	lazyvimJSON := filepath.Join(home, ".config", "nvim", "lazyvim.json")
	var mapped string
	for _, m := range configs {
		if filepath.Base(m.Destination) == "lazyvim.json" {
			dest, err := configfiles.Expand(m.Destination)
			Expect(err).NotTo(HaveOccurred())
			mapped = dest
		}
	}
	Expect(mapped).To(Equal(lazyvimJSON))
	Expect(os.WriteFile(lazyvimJSON, []byte(userLazyVimJSON), 0600)).To(Succeed())

	var first []string
	for run := range 2 {
		result := configfiles.Apply(ctx, "Neovim", configs, false, nil)
		Expect(result.Warnings).To(BeEmpty())
		result = configfiles.Apply(ctx, "Neovim", themeFiles, false, nil)
		Expect(result.Warnings).To(BeEmpty())
		if run == 0 {
			for _, m := range append(configs, themeFiles...) {
				dest, err := configfiles.Expand(m.Destination)
				Expect(err).NotTo(HaveOccurred())
				Expect(dest).To(BeARegularFile(), "first setup skipped %s", m.Destination)
			}
			first = backups(home)
		}
	}
	Expect(first).To(HaveLen(1))
	Expect(first[0]).To(HavePrefix(lazyvimJSON + "."))
	data, err := os.ReadFile(first[0])
	Expect(err).NotTo(HaveOccurred())
	Expect(string(data)).To(Equal(userLazyVimJSON))
	Expect(backups(home)).To(Equal(first), "repeated setup backed up its own files")
	// Setup records config and theme files under the app's one record; the
	// theme copy must not drop the user's backup from it.
	record, err := configfiles.ReadResult("Neovim")
	Expect(err).NotTo(HaveOccurred())
	recorded := make([]string, 0, len(record.Files))
	for _, file := range record.Files {
		recorded = append(recorded, file.Backup)
	}
	Expect(recorded).To(ContainElement(first[0]))
	return themeFiles[0]
}

// useTheme applies the catalog theme mapping with its source swapped for the
// named theme asset; the catalog's own theme is applied unchanged.
func useTheme(ctx context.Context, mapping types.ConfigFile, theme string) {
	mapping.Source = filepath.Join(filepath.Dir(mapping.Source), theme)
	result := configfiles.Apply(ctx, "Neovim", []types.ConfigFile{mapping}, false, nil)
	Expect(result.Warnings).To(BeEmpty())
}

var _ = Describe("Shipped Neovim configuration in a LazyVim layout", func() {
	It("lists every shipped theme asset", func() {
		themes, err := filepath.Glob("../../assets/themes/neovim/*.lua")
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(themes))
		for _, theme := range themes {
			names = append(names, filepath.Base(theme))
		}
		Expect(names).To(ConsistOf(keys(shippedThemes)))
	})

	It("loads each theme and then the transparency snippet from the catalog destinations", func(ctx SpecContext) {
		nvim, err := exec.LookPath("nvim")
		if err != nil {
			Skip("nvim is not installed")
		}
		if !nvimHas(ctx, nvim, "0.9") {
			Skip("the theme check needs Neovim 0.9 or later")
		}
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		installAssets(home)
		config := filepath.Join(home, ".config", "nvim")
		stub := filepath.Join(home, "stub")
		Expect(os.MkdirAll(filepath.Join(config, "lua", "config"), 0700)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(stub, "lua", "lazy"), 0700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(stub, "lua", "lazy", "init.lua"), []byte(stubLazy), 0600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(config, "init.lua"), []byte(userInit), 0600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(config, "lua", "config", "lazy.lua"), []byte(lazyBootstrap), 0600)).To(Succeed())

		catalogTheme := applyNeovimConfig(ctx, home)

		for user, want := range map[string]string{"init.lua": userInit, filepath.Join("lua", "config", "lazy.lua"): lazyBootstrap} {
			data, err := os.ReadFile(filepath.Join(config, user))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(want), "setup changed the user's %s", user)
		}
		rtp := []string{"--cmd", "lua vim.opt.runtimepath:prepend(" + strconv.Quote(stub) + ")"}
		catalogName := filepath.Base(catalogTheme.Source)
		colors, background := loadedTheme(ctx, nvim, home, true, rtp...)
		Expect(colors).To(Equal(shippedThemes[catalogName].colorscheme), "catalog theme %s", catalogName)
		Expect(background).To(Equal("nil"))
		for theme, want := range shippedThemes {
			useTheme(ctx, catalogTheme, theme)
			colors, background := loadedTheme(ctx, nvim, home, true, rtp...)
			Expect(colors).To(Equal(want.colorscheme), theme)
			Expect(background).To(Equal("nil"), "%s: the transparency snippet did not run after the theme", theme)
		}
	})

	It("installs LazyVim and every theme's plugin from the network when enabled", Label("network"), func(ctx SpecContext) {
		if os.Getenv("DEVEX_NEOVIM_NETWORK_TEST") != "1" {
			Skip("run with DEVEX_NEOVIM_NETWORK_TEST=1 ginkgo --label-filter=network ./internal/configfiles to install LazyVim and theme plugins from GitHub")
		}
		nvim, err := exec.LookPath("nvim")
		Expect(err).NotTo(HaveOccurred())
		if !nvimHas(ctx, nvim, "0.11.2") {
			Skip("LazyVim requires Neovim 0.11.2 or later")
		}
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		installAssets(home)
		config := filepath.Join(home, ".config", "nvim")
		clone := exec.CommandContext(ctx, "git", "clone", "--quiet", "--depth", "1", "https://github.com/LazyVim/starter", config)
		output, err := clone.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "%s", output)
		Expect(os.RemoveAll(filepath.Join(config, ".git"))).To(Succeed())

		catalogTheme := applyNeovimConfig(ctx, home)

		for theme, want := range shippedThemes {
			useTheme(ctx, catalogTheme, theme)
			// LazyVim reports unrelated missing tools (such as the tree-sitter
			// CLI) as startup errors, so this run is not strict. A theme that
			// fails to load still fails: LazyVim then falls back to habamax.
			colors, background := loadedTheme(ctx, nvim, home, false, "+Lazy! install")
			Expect(colors).To(HavePrefix(want.realPrefix), theme)
			Expect(background).To(Equal("nil"), "%s: the transparency snippet did not run after the theme", theme)
		}
		Expect(filepath.Join(home, ".local", "share", "nvim", "lazy", "LazyVim")).To(BeADirectory())
	}, SpecTimeout(defaultNetworkTimeout))
})

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// nvimHas reports whether nvim is at least the given version.
func nvimHas(ctx context.Context, nvim, version string) bool {
	out, err := exec.CommandContext(ctx, nvim, "--headless", "-u", "NONE", "-i", "NONE", "-c", `lua io.stdout:write(vim.fn.has("nvim-`+version+`"))`, "-c", "qa!").Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(out)) == "1"
}
