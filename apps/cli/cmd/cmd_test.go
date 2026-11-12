package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
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
		"HOME":                       home,
		"DEVEX_OFFLINE":              "true",
		"DEVEX_DISABLE_UPDATE_CHECK": "true",
		"GOPATH":                     values[1],
		"GOCACHE":                    values[2],
		"GOMODCACHE":                 values[3],
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

var _ = Describe("Runtime registry catalog", func() {
	It("uses the registry for filtered JSON listings and explicit install resolution", func(ctx SpecContext) {
		// A registry-only application in the registry's snake_case schema; the
		// bundled applications stay in the catalog alongside it.
		fixture := []byte(`{"items":[{"name":"registry-only","description":"Only in the registry","category":"Dependency","default":false,"desktopEnvironments":[],"platforms":{"linux":{"install_method":"apt","install_command":"registry-only","uninstall_command":"registry-only"},"macos":null,"windows":null}}],"pagination":{"total":1,"count":1,"limit":100,"offset":0,"page":1,"totalPages":1,"hasNext":false,"hasPrevious":false}}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(fixture) }))
		defer server.Close()
		home := GinkgoT().TempDir()
		goBin, env := isolatedGoRunEnv(home)
		// This test opts into its local httptest registry explicitly.
		for i, entry := range env {
			if strings.HasPrefix(entry, "DEVEX_OFFLINE=") {
				env[i] = "DEVEX_OFFLINE=false"
			}
		}
		env = append(env, "DEVEX_PLUGIN_REGISTRY_URL="+server.URL, "DEVEX_CACHE_DIR="+filepath.Join(home, "cache"), "DEVEX_DATA_DIR="+filepath.Join(home, "data"), "DEVEX_CONFIG_DIR="+filepath.Join(home, "config"))
		binary := filepath.Join(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(ctx, goBin, "build", "-o", binary, "main.go")
		build.Env = env
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		list := exec.CommandContext(ctx, binary, "list", "available", "--search", "registry-only", "--category", "Dependency", "--format", "json", "--skip-plugin-download")
		list.Env = env
		var stderr bytes.Buffer
		list.Stderr = &stderr
		output, err = list.Output()
		Expect(err).NotTo(HaveOccurred(), stderr.String())
		Expect(json.Valid(output)).To(BeTrue(), string(output))
		Expect(string(output)).To(ContainSubstring("registry-only"))
		Expect(stderr.String()).NotTo(ContainSubstring("Application catalog:"))
		verboseList := exec.CommandContext(ctx, binary, "list", "available", "--format", "json", "--skip-plugin-download", "--verbose")
		verboseList.Env = env
		stderr.Reset()
		verboseList.Stderr = &stderr
		output, err = verboseList.Output()
		Expect(err).NotTo(HaveOccurred(), stderr.String())
		Expect(json.Valid(output)).To(BeTrue())
		Expect(stderr.String()).To(ContainSubstring("Application catalog: cache"))
		Expect(stderr.String()).To(ContainSubstring(server.URL))
		Expect(stderr.String()).To(ContainSubstring("cache age:"))
		install := exec.CommandContext(ctx, binary, "install", "registry-only", "--dry-run", "--offline")
		install.Env = env
		output, err = install.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("registry-only"))
		Expect(string(output)).NotTo(ContainSubstring("Application catalog:"))
		// Force a stale snapshot and verify its offline fallback reason.
		entries, err := os.ReadDir(filepath.Join(home, "cache"))
		Expect(err).NotTo(HaveOccurred())
		for _, entry := range entries {
			path := filepath.Join(home, "cache", entry.Name())
			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			var snapshot map[string]any
			Expect(json.Unmarshal(data, &snapshot)).To(Succeed())
			snapshot["Checked"] = time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano)
			data, err = json.Marshal(snapshot)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(path, data, 0600)).To(Succeed())
		}
		stale := exec.CommandContext(ctx, binary, "list", "available", "--offline", "--skip-plugin-download")
		stale.Env = env
		output, err = stale.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("Application catalog: cache (stale)"))
		Expect(string(output)).To(ContainSubstring("offline mode enabled"))
		Expect(os.RemoveAll(filepath.Join(home, "cache"))).To(Succeed())
		fallback := exec.CommandContext(ctx, binary, "list", "available", "--offline", "--skip-plugin-download")
		fallback.Env = env
		output, err = fallback.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("Application catalog: bundled"))
		Expect(string(output)).To(ContainSubstring("offline mode enabled"))
	}, SpecTimeout(30*time.Second))
	It("does not request the catalog for installed listings, status, configuration, version, or help", func(ctx SpecContext) {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(503) }))
		defer server.Close()
		home := GinkgoT().TempDir()
		goBin, env := isolatedGoRunEnv(home)
		for i, entry := range env {
			if strings.HasPrefix(entry, "DEVEX_OFFLINE=") {
				env[i] = "DEVEX_OFFLINE=false"
			}
		}
		env = append(env, "DEVEX_PLUGIN_REGISTRY_URL="+server.URL, "DEVEX_CACHE_DIR="+filepath.Join(home, "cache"), "DEVEX_DATA_DIR="+filepath.Join(home, "data"), "DEVEX_CONFIG_DIR="+filepath.Join(home, "config"))
		binary := filepath.Join(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(ctx, goBin, "build", "-o", binary, "main.go")
		build.Env = env
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		for _, args := range [][]string{
			{"list", "installed", "--skip-plugin-download"},
			{"status", "--skip-plugin-download"},
			{"config", "show", "--skip-plugin-download"},
			{"--version"}, {"--help"}, {"help", "install"}, {"install", "--help"}, {"setup", "--help"},
		} {
			command := exec.CommandContext(ctx, binary, args...)
			command.Env = env
			output, err = command.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "args=%v output=%s", args, output)
			Expect(hits.Load()).To(BeZero(), "args=%v", args)
			Expect(string(output)).NotTo(ContainSubstring("Application catalog:"))
		}
	}, SpecTimeout(30*time.Second))

})

var _ = Describe("Runtime logging CLI output", func() {
	DescribeTable("preserves JSON stdout with logging flags", func(flags []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		goBin, goEnv := isolatedGoRunEnv(GinkgoT().TempDir())
		args := append([]string{"run", ".", "platform", "--format", "json"}, flags...)
		cmd := exec.CommandContext(ctx, goBin, args...)
		cmd.Env = goEnv
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		Expect(cmd.Run()).To(Succeed(), stderr.String())
		Expect(json.Valid(stdout.Bytes())).To(BeTrue(), stdout.String())
	},
		Entry("debug", []string{"--log-level", "debug"}),
		Entry("quiet overrides verbose", []string{"--quiet", "--verbose"}),
	)
	It("keeps invalid log-level errors actionable in quiet mode", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		goBin, goEnv := isolatedGoRunEnv(GinkgoT().TempDir())
		cmd := exec.CommandContext(ctx, goBin, "run", ".", "paths", "--quiet", "--log-level", "invalid")
		cmd.Env = goEnv
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		Expect(cmd.Run()).NotTo(Succeed())
		Expect(stdout.String()).To(BeEmpty())
		Expect(stderr.String()).To(ContainSubstring("invalid log-level"))
	})
})
