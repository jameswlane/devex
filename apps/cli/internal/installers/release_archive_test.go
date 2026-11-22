package installers

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	appconfig "github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Release archive installs", func() {
	var (
		fixtureDir  string
		destination string
		archive     string
		systemCalls [][]string
	)

	// buildArchive writes files (relative path -> content) into a tarball.
	buildArchive := func(files map[string]string) string {
		root := filepath.Join(fixtureDir, "root")
		Expect(os.MkdirAll(root, 0o755)).To(Succeed())
		names := make([]string, 0, len(files))
		for name, content := range files {
			full := filepath.Join(root, name)
			Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
			Expect(os.WriteFile(full, []byte(content), 0o755)).To(Succeed())
			names = append(names, name)
		}
		out := filepath.Join(fixtureDir, "release.tar.gz")
		args := append([]string{"-czf", out, "-C", root}, names...)
		Expect(exec.Command("tar", args...).Run()).To(Succeed())
		return out
	}

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("release archives install with POSIX tar and install")
		}
		fixtureDir = GinkgoT().TempDir()
		destination = filepath.Join(GinkgoT().TempDir(), "dest")
		systemCalls = nil
		viper.Set("no-cache", true)
		originalArchive, originalSystem := runArchiveCommand, runSystemCommand
		DeferCleanup(func() {
			runArchiveCommand, runSystemCommand = originalArchive, originalSystem
			viper.Set("no-cache", false)
		})
		runArchiveCommand = func(ctx context.Context, name string, args ...string) (string, error) {
			if name == "curl" {
				// curl -fsSL -o <file> -- <url>: serve the local fixture.
				data, err := os.ReadFile(archive)
				if err != nil {
					return "", err
				}
				return "", os.WriteFile(args[2], data, 0o600)
			}
			output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
			return string(output), err
		}
		runSystemCommand = func(ctx context.Context, name string, args ...string) (string, error) {
			systemCalls = append(systemCalls, append([]string{name}, args...))
			output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
			return string(output), err
		}
	})

	// app pins the checksum of the current fixture archive.
	app := func(url, binary string) types.AppConfig {
		data, err := os.ReadFile(archive)
		Expect(err).NotTo(HaveOccurred())
		config := types.AppConfig{
			BaseConfig:      types.BaseConfig{Name: "Tool"},
			InstallMethod:   "curlpipe",
			DownloadURL:     url,
			InstallDir:      destination,
			InstalledBinary: binary,
		}
		config.Verification.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		return config
	}

	It("recognises release tarballs but not install scripts", func() {
		Expect(IsReleaseArchive("https://github.com/neovim/neovim/releases/latest/download/nvim-linux-x86_64.tar.gz")).To(BeTrue())
		Expect(IsReleaseArchive("https://example.com/tool.tgz?download=1")).To(BeTrue())
		Expect(IsReleaseArchive("https://mise.run")).To(BeFalse())
		Expect(IsReleaseArchive("https://example.com/install.sh")).To(BeFalse())
	})

	It("installs only the named binary when it sits at the archive top level", func() {
		archive = buildArchive(map[string]string{"lazygit": "#!/bin/sh\necho lazygit\n", "LICENSE": "MIT"})
		config := app("https://github.com/jesseduffield/lazygit/releases/download/v1/lazygit_1_linux_x86_64.tar.gz", "lazygit")
		Expect(InstallReleaseArchive(context.Background(), config)).To(Succeed())

		info, err := os.Stat(filepath.Join(destination, "lazygit"))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o755)))
		Expect(filepath.Join(destination, "LICENSE")).NotTo(BeAnExistingFile())
		Expect(systemCalls).To(HaveLen(2))
		Expect(systemCalls[1][0]).To(Equal("install"))
	})

	It("extracts the whole archive when the binary lives in a subdirectory", func() {
		archive = buildArchive(map[string]string{
			"nvim-linux-x86_64/bin/nvim":                 "binary",
			"nvim-linux-x86_64/share/nvim/runtime/a.vim": "runtime",
		})
		Expect(InstallReleaseArchive(context.Background(), app("https://github.com/neovim/neovim/releases/latest/download/nvim-linux-x86_64.tar.gz", "nvim"))).To(Succeed())

		Expect(filepath.Join(destination, "nvim-linux-x86_64", "bin", "nvim")).To(BeARegularFile())
		Expect(filepath.Join(destination, "nvim-linux-x86_64", "share", "nvim", "runtime", "a.vim")).To(BeARegularFile())
	})

	It("installs a pinned archive from the installation cache", func() {
		archive = buildArchive(map[string]string{"lazygit": "binary"})
		config := app("https://github.com/example/tool/releases/download/v1/lazygit_1_linux_x86_64.tar.gz", "lazygit")
		home, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())
		manager, err := cache.NewCacheManager(appconfig.CrossPlatformSettings{HomeDir: home})
		Expect(err).NotTo(HaveOccurred())
		_, err = manager.SetCacheEntry(cache.ArtifactKey(config.DownloadURL, config.Verification.SHA256), cache.CacheTypeDownload, archive, nil, 0)
		Expect(err).NotTo(HaveOccurred())
		viper.Set("no-cache", false)
		viper.Set("offline", true)
		DeferCleanup(func() { viper.Set("offline", false) })
		runArchiveCommand = func(ctx context.Context, name string, args ...string) (string, error) {
			Expect(name).NotTo(Equal("curl"), "a cached artifact must not be downloaded again")
			output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
			return string(output), err
		}

		Expect(InstallReleaseArchive(context.Background(), config)).To(Succeed())
		Expect(filepath.Join(destination, "lazygit")).To(BeARegularFile())
	})

	It("refuses an archive from an untrusted host before downloading it", func() {
		archive = buildArchive(map[string]string{"lazygit": "binary"})
		downloaded := false
		runArchiveCommand = func(context.Context, string, ...string) (string, error) { downloaded = true; return "", nil }

		err := InstallReleaseArchive(context.Background(), app("https://example.com/lazygit.tar.gz", "lazygit"))
		Expect(err).To(MatchError(ContainSubstring("is not trusted")))
		Expect(downloaded).To(BeFalse())
	})

	It("refuses an unpinned archive before downloading it", func() {
		archive = buildArchive(map[string]string{"lazygit": "binary"})
		config := app("https://github.com/example/tool/releases/download/v1/lazygit.tar.gz", "lazygit")
		config.Verification.SHA256 = ""
		downloaded := false
		runArchiveCommand = func(context.Context, string, ...string) (string, error) { downloaded = true; return "", nil }

		err := InstallReleaseArchive(context.Background(), config)
		Expect(err).To(MatchError(ContainSubstring("needs verification.sha256")))
		Expect(downloaded).To(BeFalse())
		Expect(systemCalls).To(BeEmpty())
	})

	It("rejects an archive whose checksum does not match", func() {
		archive = buildArchive(map[string]string{"lazygit": "binary"})
		config := app("https://github.com/example/tool/releases/download/v1/lazygit.tar.gz", "lazygit")
		config.Verification.SHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte("something else")))

		err := InstallReleaseArchive(context.Background(), config)
		Expect(err).To(MatchError(ContainSubstring("integrity verification failed")))
		Expect(destination).NotTo(BeAnExistingFile())
		Expect(systemCalls).To(BeEmpty())
	})

	It("rejects archives with paths that escape the destination", func() {
		archive = filepath.Join(fixtureDir, "evil.tar")
		file, err := os.Create(archive)
		Expect(err).NotTo(HaveOccurred())
		writer := tar.NewWriter(file)
		Expect(writer.WriteHeader(&tar.Header{Name: "../evil", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})).To(Succeed())
		_, err = writer.Write([]byte("x"))
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.Close()).To(Succeed())
		Expect(file.Close()).To(Succeed())

		err = InstallReleaseArchive(context.Background(), app("https://github.com/example/tool/releases/download/v1/evil.tar", "tool"))
		Expect(err).To(MatchError(ContainSubstring("unsafe path")))
		Expect(systemCalls).To(BeEmpty())
	})

	It("rejects archives with link members", func() {
		root := filepath.Join(fixtureDir, "links")
		Expect(os.MkdirAll(root, 0o755)).To(Succeed())
		Expect(os.Symlink("../outside", filepath.Join(root, "escape"))).To(Succeed())
		archive = filepath.Join(fixtureDir, "links.tar.gz")
		Expect(exec.Command("tar", "-czf", archive, "-C", root, "escape").Run()).To(Succeed())

		err := InstallReleaseArchive(context.Background(), app("https://github.com/example/tool/releases/download/v1/links.tar.gz", "tool"))
		Expect(err).To(MatchError(ContainSubstring("link member")))
		Expect(systemCalls).To(BeEmpty())
	})

	It("routes a curlpipe release archive to the archive installer instead of running it as a script", func() {
		archive = buildArchive(map[string]string{"lazygit": "binary"})
		config := app("https://github.com/example/tool/releases/download/v1/lazygit.tar.gz", "lazygit")

		Expect(executeInstallCommand(context.Background(), config, nil)).To(Succeed())
		Expect(filepath.Join(destination, "lazygit")).To(BeARegularFile())
	})
})
