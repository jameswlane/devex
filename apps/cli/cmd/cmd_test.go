package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	devex "github.com/jameswlane/devex/apps/cli/cmd"
	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/errors"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Main", func() {
	Context("handleError", func() {
		var originalExit func(int)
		var exitCalled bool

		BeforeEach(func() {
			originalExit = devex.Exit
			exitCalled = false
			devex.Exit = func(code int) {
				exitCalled = true
			}
		})

		AfterEach(func() {
			devex.Exit = originalExit // Restore the original Exit function
		})

		It("logs and exits on invalid input error", func() {
			Expect(func() {
				devex.HandleErrorForTest("test", errors.ErrInvalidInput)
			}).ToNot(Panic())
			Expect(exitCalled).To(BeTrue())
		})

		It("preserves a plugin process exit code at the CLI boundary", func() {
			if runtime.GOOS == "windows" {
				Skip("requires a POSIX shell")
			}
			childError := exec.CommandContext(context.Background(), "sh", "-c", "exit 23").Run()
			Expect(childError).To(HaveOccurred())
			code := 0
			devex.Exit = func(value int) { code = value }
			devex.HandleErrorForTest("plugin", childError)
			Expect(code).To(Equal(23))
		})

		It("logs and exits on unexpected error", func() {
			Expect(func() {
				devex.HandleErrorForTest("test", errors.New("unexpected error"))
			}).ToNot(Panic())
			Expect(exitCalled).To(BeTrue())
		})
	})

	Context("CLI error execution", func() {
		It("prints the error message exactly once on stderr and exits nonzero for unknown commands", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			goBin, goEnv := isolatedGoRunEnv(GinkgoT().TempDir())
			cmd := exec.CommandContext(ctx, goBin, "run", ".", "notacommand")
			cmd.Dir = "."
			cmd.Env = goEnv
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			Expect(err).To(HaveOccurred())

			stderrOutput := stderr.String()
			Expect(strings.Count(stderrOutput, "Error:")).To(Equal(1))
			Expect(stderrOutput).To(ContainSubstring(`unknown command "notacommand" for "devex"`))
			Expect(stderrOutput).To(ContainSubstring("Run 'devex --help' for usage."))
			Expect(stderrOutput).NotTo(ContainSubstring("Error executing root command"))
		})
	})
})

// isolatedGoRunEnv returns the parent's go binary and an environment for `go run`
// with a temporary HOME and no debug variables, keeping the parent's Go cache locations.
func isolatedGoRunEnv(home string) (string, []string) {
	out, err := exec.CommandContext(context.Background(), "go", "env", "GOROOT", "GOPATH", "GOCACHE", "GOMODCACHE").Output()
	Expect(err).ToNot(HaveOccurred(), "failed to read go env")
	values := strings.Split(strings.TrimSpace(string(out)), "\n")
	Expect(values).To(HaveLen(4))

	overrides := map[string]string{
		"HOME":       home,
		"GOPATH":     values[1],
		"GOCACHE":    values[2],
		"GOMODCACHE": values[3],
	}
	excluded := map[string]bool{"DEBUG": true, "DEVEX_DEBUG": true, "DEVEX_VERBOSE": true}

	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if _, overridden := overrides[key]; overridden || excluded[key] {
			continue
		}
		env = append(env, kv)
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return filepath.Join(values[0], "bin", "go"), env
}

var _ = Describe("CLI startup regression coverage", func() {
	It("keeps inspection commands usable with a linked legacy config and previews tracked applications", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		goBin, env := isolatedGoRunEnv(home)
		binary := filepath.Join(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(ctx, goBin, "build", "-o", binary, "main.go")
		build.Env = env
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		// Set a legacy link that migration refuses before touching any source.
		Expect(os.MkdirAll(filepath.Join(home, ".devex"), 0700)).To(Succeed())
		Expect(os.Symlink("dotfiles.yaml", filepath.Join(home, ".devex", "config.yaml"))).To(Succeed())
		for _, args := range [][]string{{"--help"}, {"completion", "bash"}, {"paths"}, {"--version"}, {"config", "show"}} {
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Env = append(env, "DEVEX_OFFLINE=true", "DEVEX_DISABLE_UPDATE_CHECK=true")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			output, err := cmd.Output()
			Expect(err).NotTo(HaveOccurred(), "%v: %s", args, stderr.String())
			Expect(output).NotTo(BeEmpty())
			Expect(stderr.String()).To(And(ContainSubstring("Warning: legacy migration skipped"), ContainSubstring("devex paths")))
		}
		state := filepath.Join(home, ".local", "state", "devex")
		Expect(os.MkdirAll(state, 0700)).To(Succeed())
		path := filepath.Join(state, "datastore.db")
		db, err := datastore.NewSQLiteContext(ctx, path)
		Expect(err).NotTo(HaveOccurred())
		repo := repository.NewRepository(db)
		Expect(repo.AddApp("tracked-editor")).To(Succeed())
		definition, err := json.Marshal(types.CrossPlatformApp{Name: "tracked-editor", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "tracked-package"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.Set("installed-app-definition:tracked-editor", string(definition))).To(Succeed())
		Expect(db.Close()).To(Succeed())
		before, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		for _, args := range [][]string{{"uninstall", "--all", "--dry-run"}, {"uninstall", "--app", "tracked-editor", "--dry-run"}} {
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Env = append(env, "DEVEX_OFFLINE=true", "DEVEX_DISABLE_UPDATE_CHECK=true")
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			Expect(string(output)).To(And(ContainSubstring("tracked-editor"), ContainSubstring("tracked-package")))
			Expect(string(output)).NotTo(ContainSubstring("No applications are currently installed"))
			Expect(os.ReadFile(path)).To(Equal(before))
		}
	})
})
