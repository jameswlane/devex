package main_test

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentskills"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent skill startup", func() {
	It("installs the embedded bundle without runtime dependencies or a usable datastore", func(ctx SpecContext) {
		if runtime.GOOS == "windows" {
			Skip("Unix dependency-free startup")
		}
		boundedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		home := GinkgoT().TempDir()
		goBin, buildEnv := isolatedGoRunEnv(home)
		binary := filepath.Join(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(boundedCtx, goBin, "build", "-o", binary, "main.go")
		build.Env = buildEnv
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))

		state := filepath.Join(home, "state")
		Expect(os.MkdirAll(state, 0o700)).To(Succeed())
		datastore := filepath.Join(state, "datastore.db")
		Expect(os.WriteFile(datastore, []byte("not a sqlite database"), 0o600)).To(Succeed())
		runEnv := []string{
			"HOME=" + home,
			"PATH=",
			"DEVEX_CONFIG_DIR=" + filepath.Join(home, "config"),
			"DEVEX_STATE_DIR=" + state,
			"DEVEX_CACHE_DIR=" + filepath.Join(home, "cache"),
			"DEVEX_DATA_DIR=" + filepath.Join(home, "data"),
			"DEVEX_DISABLE_UPDATE_CHECK=true",
			"DEVEX_OFFLINE=true",
		}
		args := []string{"agent", "install-skill", "--agent", "claude", "--scope", "user"}
		before := startupSnapshot(home)
		dryRun := exec.CommandContext(boundedCtx, binary, "agent", "install-skill", "--agent", "claude", "--scope", "user", "--dry-run")
		dryRun.Env = runEnv
		output, err = dryRun.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("Would install"))
		Expect(startupSnapshot(home)).To(Equal(before))

		install := exec.CommandContext(boundedCtx, binary, args...)
		install.Env = runEnv
		output, err = install.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("Installed: "))
		expectInstalledAgentSkillBundle(filepath.Join(home, ".claude", "skills", "devex"))
		Expect(os.ReadFile(datastore)).To(Equal([]byte("not a sqlite database")))
	})
})

func expectInstalledAgentSkillBundle(destination string) {
	bundle, err := agentskills.Bundle()
	Expect(err).NotTo(HaveOccurred())
	expected := map[string][]byte{}
	Expect(fs.WalkDir(bundle, ".", func(path string, entry fs.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(bundle, path)
		Expect(err).NotTo(HaveOccurred())
		expected[path] = data
		return nil
	})).To(Succeed())
	actual := map[string][]byte{}
	Expect(filepath.WalkDir(destination, func(path string, entry fs.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(destination, path)
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		actual[filepath.ToSlash(relative)] = data
		return nil
	})).To(Succeed())
	Expect(actual).To(Equal(expected))
}
