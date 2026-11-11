package inventory_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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
		installed, _, err := inventory.NativeQuery(context.Background(), "apt", "definitely-not-a-real-package-xyz")
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeFalse())
		_, _, err = inventory.NativeQuery(context.Background(), "apt", "-bad")
		Expect(err).To(HaveOccurred())
	})
})
