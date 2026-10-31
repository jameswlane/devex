package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Startup migration preview policy", func() {
	It("honors environment and explicit true/false flags, stopping at the argument boundary", func() {
		GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
		Expect(migrationDryRun(nil)).To(BeTrue())
		Expect(migrationDryRun([]string{"--dry-run=false", "paths"})).To(BeFalse())
		GinkgoT().Setenv("DEVEX_DRY_RUN", "")
		Expect(migrationDryRun([]string{"paths", "--dry-run"})).To(BeTrue())
		Expect(migrationDryRun([]string{"--", "--dry-run"})).To(BeFalse())
	})
})

var _ = Describe("Startup migration recovery", func() {
	It("recognizes inspection commands with global flag values and preserves normal setup migration", func() {
		Expect(informationalCommand([]string{"--log-level", "debug", "--config", "custom.yaml", "paths"})).To(BeTrue())
		Expect(informationalCommand(nil)).To(BeFalse())
		Expect(informationalCommand([]string{"install", "--app", "paths"})).To(BeFalse())
	})

	It("warns with repair guidance and continues after a refusal for every command", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		dirs := devexpaths.ForHome(home)
		Expect(os.MkdirAll(filepath.Join(home, ".devex"), 0700)).To(Succeed())
		Expect(os.Symlink("dotfiles.yaml", filepath.Join(home, ".devex", "config.yaml"))).To(Succeed())
		for _, args := range [][]string{{"--help"}, {"completion", "bash"}, {"paths"}, {"--version"}, {"config", "show"}, {"install", "editor"}} {
			var output bytes.Buffer
			Expect(migrateLegacy(ctx, dirs, false, args, &output)).To(Succeed())
			Expect(output.String()).To(And(ContainSubstring("Warning:"), ContainSubstring("devex paths"), ContainSubstring("regular files"), ContainSubstring("backing them up")))
		}
		Expect(dirs.Config).NotTo(BeADirectory())
	})

	It("allows inspection after I/O failures while operational commands return the error", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		// An overlong component produces an I/O error even when tests run as root.
		dirs := devexpaths.ForHome(filepath.Join(home, strings.Repeat("x", 300)))
		var output bytes.Buffer
		Expect(migrateLegacy(ctx, dirs, false, []string{"paths"}, &output)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Warning:"))
		Expect(migrateLegacy(ctx, dirs, false, []string{"install", "editor"}, &output)).To(HaveOccurred())
	})

	It("leaves safe legacy files in place during informational commands", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		legacy := filepath.Join(home, ".devex", "config.yaml")
		Expect(os.MkdirAll(filepath.Dir(legacy), 0700)).To(Succeed())
		Expect(os.WriteFile(legacy, []byte("config"), 0600)).To(Succeed())
		var output bytes.Buffer
		Expect(migrateLegacy(ctx, devexpaths.ForHome(home), false, []string{"paths"}, &output)).To(Succeed())
		Expect(legacy).To(BeAnExistingFile())
		Expect(devexpaths.ForHome(home).Config).NotTo(BeADirectory())
	})
})
