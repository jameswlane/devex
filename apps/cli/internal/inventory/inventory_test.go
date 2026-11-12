package inventory_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type fakeRepo struct {
	apps        []string
	definitions map[string]types.CrossPlatformApp
}

func (f fakeRepo) ListApps() ([]types.AppConfig, error) {
	out := make([]types.AppConfig, 0, len(f.apps))
	for _, name := range f.apps {
		out = append(out, types.AppConfig{BaseConfig: types.BaseConfig{Name: name}})
	}
	return out, nil
}

func (f fakeRepo) Get(key string) (string, error) {
	name := strings.TrimPrefix(key, "installed-app-definition:")
	app, ok := f.definitions[name]
	if !ok {
		return "", errors.New("not found")
	}
	data, err := json.Marshal(app)
	return string(data), err
}

func aptApp(name string, packages ...string) types.CrossPlatformApp {
	return types.CrossPlatformApp{Name: name, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: strings.Join(packages, " ")}}
}

var _ = Describe("Inventory", func() {
	var (
		repo    fakeRepo
		options inventory.Options
		native  map[string]string // installed package -> version
	)

	BeforeEach(func() {
		native = map[string]string{"git": "1:2.43.0-1", "curl": "8.5.0-2"}
		repo = fakeRepo{apps: []string{"git", "curl-tools", "gone", "legacy"}, definitions: map[string]types.CrossPlatformApp{
			"git":        aptApp("git", "git"),
			"curl-tools": aptApp("curl-tools", "curl", "jq"),
			"gone":       aptApp("gone", "gone-pkg"),
		}}
		machineID := filepath.Join(GinkgoT().TempDir(), "machine-id")
		Expect(os.WriteFile(machineID, []byte("0123456789abcdef0123456789abcdef\n"), 0o600)).To(Succeed())
		options = inventory.Options{
			DevexVersion:  "1.2.3",
			MachineIDPath: machineID,
			Now:           func() time.Time { return time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) },
			Query: func(_ context.Context, manager, pkg string) (bool, string, error) {
				if manager != "apt" {
					return false, "", inventory.ErrUnsupported
				}
				version, ok := native[pkg]
				return ok, version, nil
			},
		}
	})

	It("reports installed versions, missing software and records without definitions", func() {
		native["jq"] = "1.7.1-3"
		result, err := inventory.Build(context.Background(), repo, options)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.SchemaVersion).To(Equal("devex.inventory/v1"))
		Expect(result.Items).To(HaveLen(4))
		byName := map[string]inventory.Item{}
		for _, item := range result.Items {
			byName[item.Name] = item
		}
		Expect(byName["git"]).To(SatisfyAll(HaveField("State", "installed"), HaveField("Version", "1:2.43.0-1"), HaveField("Manager", "apt"), HaveField("ManagedBy", "devex")))
		Expect(byName["curl-tools"].Version).To(Equal("8.5.0-2, 1.7.1-3"))
		Expect(byName["gone"].State).To(Equal("missing"))
		Expect(byName["legacy"]).To(SatisfyAll(HaveField("State", "unknown"), HaveField("Definition", false)))
		Expect(result.Drift).To(ConsistOf(
			HaveField("Kind", "missing"),
			HaveField("Kind", "no_definition"),
		))
	})

	It("reports changed software: a package removed since install makes the app missing", func() {
		delete(native, "git")
		result, err := inventory.Build(context.Background(), repo, options)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Drift).To(ContainElement(SatisfyAll(HaveField("App", "git"), HaveField("Kind", "missing"))))
	})

	It("checks records without a definition against the current catalog", func() {
		options.Resolve = func(name string) (types.CrossPlatformApp, bool) {
			if name == "legacy" {
				return aptApp("legacy", "git"), true
			}
			return types.CrossPlatformApp{}, false
		}
		result, err := inventory.Build(context.Background(), repo, options)
		Expect(err).NotTo(HaveOccurred())
		for _, item := range result.Items {
			if item.Name == "legacy" {
				Expect(item.State).To(Equal("installed"))
				Expect(item.Definition).To(BeFalse())
			}
		}
		Expect(result.Drift).To(ContainElement(HaveField("Detail", ContainSubstring("current catalog definition"))))
	})

	It("marks managers it cannot query as unknown, not missing", func() {
		repo.definitions["git"] = types.CrossPlatformApp{Name: "git", Linux: types.OSConfig{InstallMethod: "mise", InstallCommand: "go"}}
		result, err := inventory.Build(context.Background(), repo, options)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Items[1]).To(SatisfyAll(HaveField("Name", "git"), HaveField("State", "unknown"), HaveField("Detail", ContainSubstring("not queried"))))
	})

	It("identifies the machine by a salted hash and omits the hostname by default", func() {
		result, err := inventory.Build(context.Background(), repo, options)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Machine.ID).To(HaveLen(16))
		Expect(result.Machine.ID).NotTo(ContainSubstring("0123456789abcdef"))
		Expect(result.Machine.Hostname).To(BeEmpty())
		data, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("0123456789abcdef0123"))

		options.IncludeHostname = true
		result, err = inventory.Build(context.Background(), repo, options)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Machine.Hostname).NotTo(BeEmpty())
	})

	It("reports an empty inventory with a stable shape", func() {
		result, err := inventory.Build(context.Background(), fakeRepo{}, options)
		Expect(err).NotTo(HaveOccurred())
		data, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"items":[]`))
		Expect(string(data)).To(ContainSubstring(`"drift":[]`))
	})

	It("stops on cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := inventory.Build(ctx, repo, options)
		Expect(err).To(MatchError(context.Canceled))
	})

	It("queries dpkg for real apt packages when dpkg-query exists", func() {
		if _, err := os.Stat("/usr/bin/dpkg-query"); err != nil {
			Skip("dpkg-query is not available")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		installed, _, err := inventory.NativeQuery(ctx, "apt", "definitely-not-a-real-package-xyz")
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeFalse())
		_, _, err = inventory.NativeQuery(ctx, "apt", "-bad")
		Expect(err).To(HaveOccurred())
	})

	Context("when a query is canceled", func() {
		It("fails instead of exporting when the final query is canceled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			repo = fakeRepo{apps: []string{"git"}, definitions: map[string]types.CrossPlatformApp{"git": aptApp("git", "git")}}
			options.Query = func(ctx context.Context, _, _ string) (bool, string, error) {
				cancel()
				return false, "", ctx.Err()
			}
			_, err := inventory.Build(ctx, repo, options)
			Expect(err).To(MatchError(context.Canceled))
		})

		It("fails even when a canceled query reports the package as absent", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			repo = fakeRepo{apps: []string{"git"}, definitions: map[string]types.CrossPlatformApp{"git": aptApp("git", "git")}}
			options.Query = func(context.Context, string, string) (bool, string, error) {
				cancel()
				return false, "", nil
			}
			_, err := inventory.Build(ctx, repo, options)
			Expect(err).To(MatchError(context.Canceled))
		})
	})

	Context("with legacy install commands", func() {
		var queried []string

		build := func(app types.CrossPlatformApp) inventory.Item {
			queried = nil
			options.Query = func(_ context.Context, _, pkg string) (bool, string, error) {
				queried = append(queried, pkg)
				return true, "1.0", nil
			}
			repo = fakeRepo{apps: []string{app.Name}, definitions: map[string]types.CrossPlatformApp{app.Name: app}}
			result, err := inventory.Build(context.Background(), repo, options)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Items).To(HaveLen(1))
			return result.Items[0]
		}

		It("queries only the application ID of a full flatpak install command", func() {
			item := build(types.CrossPlatformApp{Name: "zen", Linux: types.OSConfig{InstallMethod: "flatpak", InstallCommand: "flatpak install flathub io.github.zen_browser.zen"}})
			Expect(queried).To(Equal([]string{"io.github.zen_browser.zen"}))
			Expect(item).To(SatisfyAll(HaveField("State", "installed"), HaveField("Packages", []string{"io.github.zen_browser.zen"})))
		})

		DescribeTable("drops snap options wherever they appear",
			func(command string) {
				item := build(types.CrossPlatformApp{Name: "neovim", Linux: types.OSConfig{InstallMethod: "snap", InstallCommand: command}})
				Expect(queried).To(Equal([]string{"nvim"}))
				Expect(item.State).To(Equal("installed"))
			},
			Entry("after the name", "nvim --classic"),
			Entry("before the name", "--classic nvim"),
			Entry("with a valued option", "sudo snap install --channel=stable nvim --classic"),
		)

		It("unquotes quoted package names", func() {
			build(types.CrossPlatformApp{Name: "tools", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: `"git" 'curl'`}})
			Expect(queried).To(Equal([]string{"git", "curl"}))
		})

		It("uses structured packages and never queries their options", func() {
			build(types.CrossPlatformApp{Name: "code", Linux: types.OSConfig{InstallMethod: "snap", Packages: []string{"code"}, Options: []string{"--classic"}}})
			Expect(queried).To(Equal([]string{"code"}))
		})

		It("reports a command it cannot read as unknown without querying fragments", func() {
			item := build(types.CrossPlatformApp{Name: "odd", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git && make install"}})
			Expect(queried).To(BeEmpty())
			Expect(item).To(SatisfyAll(HaveField("State", "unknown"), HaveField("Detail", ContainSubstring("cannot read package identifiers"))))
		})
	})

	Context("with deb artifacts", func() {
		var queried []string

		BeforeEach(func() {
			queried = nil
			// Everything is absent natively: a queried artifact would be missing.
			options.Query = func(_ context.Context, _, pkg string) (bool, string, error) {
				queried = append(queried, pkg)
				return false, "", nil
			}
		})

		DescribeTable("never reports a downloaded package file as missing",
			func(method, command string) {
				repo = fakeRepo{apps: []string{"artifact"}, definitions: map[string]types.CrossPlatformApp{
					"artifact": {Name: "artifact", Linux: types.OSConfig{InstallMethod: method, InstallCommand: command}},
				}}
				result, err := inventory.Build(context.Background(), repo, options)
				Expect(err).NotTo(HaveOccurred())
				Expect(queried).To(BeEmpty())
				Expect(result.Items[0]).To(SatisfyAll(HaveField("State", "unknown"), HaveField("Detail", ContainSubstring("not a dpkg package name"))))
				Expect(result.Drift).To(BeEmpty())
			},
			Entry("a deb URL", "deb", "https://proton.me/download/pass/linux/proton-pass_1.23.1_amd64.deb"),
			Entry("a deb file name", "deb", "ulauncher_5.15.7_all.deb"),
			Entry("a local deb installed with apt", "apt", "./tool_1.0_amd64.deb"),
		)

		It("queries the package name of a deb downloaded from download_url", func() {
			repo = fakeRepo{apps: []string{"fastfetch"}, definitions: map[string]types.CrossPlatformApp{
				"fastfetch": {Name: "fastfetch", Linux: types.OSConfig{InstallMethod: "deb", InstallCommand: "fastfetch", DownloadURL: "https://example.invalid/fastfetch.deb"}},
			}}
			result, err := inventory.Build(context.Background(), repo, options)
			Expect(err).NotTo(HaveOccurred())
			Expect(queried).To(Equal([]string{"fastfetch"}))
			Expect(result.Items[0].State).To(Equal("missing"))
		})
	})
})

var _ = Describe("NativeQuery", func() {
	var bin, sleep string

	// tool installs a fake package manager; PATH holds only the fakes.
	tool := func(name, script string) {
		Expect(os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755)).To(Succeed())
	}

	BeforeEach(func() {
		var err error
		sleep, err = exec.LookPath("sleep")
		Expect(err).NotTo(HaveOccurred())
		bin = GinkgoT().TempDir()
		GinkgoT().Setenv("PATH", bin)
	})

	query := func(manager, pkg string) (bool, string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return inventory.NativeQuery(ctx, manager, pkg)
	}

	DescribeTable("reports a package as absent only when the manager says so",
		func(manager, name, script string) {
			tool(name, script)
			installed, version, err := query(manager, "pkg")
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
			Expect(version).To(BeEmpty())
		},
		Entry("dpkg-query", "apt", "dpkg-query", `echo "dpkg-query: no packages found matching $4" >&2; exit 1`),
		Entry("dpkg-query for a removed package with config files", "deb", "dpkg-query", `printf 'deinstall ok config-files\t1.0'`),
		Entry("dpkg-query for a half-installed package", "apt", "dpkg-query", `printf 'install reinstreq half-installed\t1.0'`),
		Entry("snap", "snap", "snap", `echo "error: no matching snaps installed" >&2; exit 1`),
		Entry("flatpak", "flatpak", "flatpak", `echo "error: pkg/*unspecified*/*unspecified* not installed" >&2; exit 1`),
	)

	DescribeTable("returns operational failures instead of reporting absence",
		func(manager, name, script, detail string) {
			tool(name, script)
			installed, _, err := query(manager, "pkg")
			Expect(installed).To(BeFalse())
			Expect(err).To(MatchError(ContainSubstring(detail)))
		},
		Entry("snapd unreachable", "snap", "snap", `echo "error: cannot communicate with server: connection refused" >&2; exit 1`, "cannot communicate with server"),
		Entry("flatpak permission failure", "flatpak", "flatpak", `echo "error: Permission denied" >&2; exit 1`, "Permission denied"),
		Entry("dpkg database failure", "apt", "dpkg-query", `echo "dpkg-query: error: parsing file '/var/lib/dpkg/status'" >&2; exit 2`, "status 2"),
		Entry("snap killed without output", "snap", "snap", `exit 3`, "no error output"),
	)

	DescribeTable("returns an actionable error when the manager is unavailable",
		func(manager, name string) {
			_, _, err := query(manager, "pkg")
			Expect(err).To(MatchError(SatisfyAll(ContainSubstring(name), ContainSubstring("not on PATH"))))
		},
		Entry("dpkg-query", "apt", "dpkg-query"),
		Entry("snap", "snap", "snap"),
		Entry("flatpak", "flatpak", "flatpak"),
	)

	It("reports installed versions", func() {
		tool("dpkg-query", `printf 'install ok installed\t1:2.43.0-1'`)
		tool("snap", `printf 'Name  Version  Rev\nnvim  0.10.2   3780\n'`)
		tool("flatpak", `echo 1.2.3`)
		for manager, want := range map[string]string{"apt": "1:2.43.0-1", "snap": "0.10.2", "flatpak": "1.2.3"} {
			installed, version, err := query(manager, "pkg")
			Expect(err).NotTo(HaveOccurred(), manager)
			Expect(installed).To(BeTrue(), manager)
			Expect(version).To(Equal(want), manager)
		}
	})

	It("returns the context error when the query is canceled while it runs", func() {
		tool("snap", "exec "+sleep+" 30")
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		installed, _, err := inventory.NativeQuery(ctx, "snap", "pkg")
		Expect(installed).To(BeFalse())
		Expect(err).To(MatchError(context.DeadlineExceeded))
	})
})
