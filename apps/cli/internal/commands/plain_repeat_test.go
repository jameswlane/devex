package commands

import (
	"bytes"
	"context"
	"errors"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Plain repeat installation", func() {
	DescribeTable("checks native state before skipping a tracked application", func(present, queryFails bool) {
		oldPrepare, oldInstall, oldQuery := prepareApplicationInstallers, installApplication, nativeApplicationInstalled
		DeferCleanup(func() {
			prepareApplicationInstallers, installApplication, nativeApplicationInstalled = oldPrepare, oldInstall, oldQuery
		})
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		calls := 0
		installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			calls++
			return nil
		}
		nativeApplicationInstalled = func(context.Context, types.CrossPlatformApp) (bool, error) {
			if queryFails {
				return false, errors.New("native query failed")
			}
			return present, nil
		}
		repo := mocks.NewMockRepository()
		Expect(repo.AddApp("jq")).To(Succeed())
		var output bytes.Buffer
		err := runPlainInstall(context.Background(), &output, []types.CrossPlatformApp{{Name: "jq", AllPlatforms: types.OSConfig{InstallMethod: "dnf", InstallCommand: "jq"}}}, repo, config.CrossPlatformSettings{}, nil)
		if queryFails {
			Expect(err).To(HaveOccurred())
			Expect(output.String()).To(ContainSubstring("native query failed"))
			Expect(calls).To(BeZero())
		} else if present {
			Expect(err).NotTo(HaveOccurred())
			Expect(output.String()).To(ContainSubstring("skipped jq (already installed)"))
			Expect(calls).To(BeZero())
		} else {
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal(1))
			Expect(output.String()).To(ContainSubstring("installed jq"))
		}
	}, Entry("installed", true, false), Entry("stale tracking", false, false), Entry("query failure", false, true))

	DescribeTable("asks the native manager about tracked applications", func(method string, cfg types.OSConfig, query string) {
		oldInstaller := nativeInstaller
		DeferCleanup(func() { nativeInstaller = oldInstaller })
		probe := &queryRecorder{}
		nativeInstaller = func(_ context.Context, requested string) types.BaseInstaller {
			Expect(requested).To(Equal(method))
			return probe
		}
		cfg.InstallMethod = method
		present, err := nativeApplicationInstalled(context.Background(), types.CrossPlatformApp{Name: "jq", AllPlatforms: cfg})
		Expect(err).NotTo(HaveOccurred())
		Expect(present).To(BeTrue())
		Expect(probe.queries).To(Equal([]string{query}))
	},
		Entry("apt", "apt", types.OSConfig{InstallCommand: "jq"}, "jq"),
		Entry("dnf", "dnf", types.OSConfig{InstallCommand: "jq"}, "jq"),
		Entry("pacman", "pacman", types.OSConfig{InstallCommand: "jq"}, "jq"),
		Entry("pacman with structured packages", "pacman", types.OSConfig{Packages: []string{"jq", "yq"}}, "pacman install 'jq' 'yq'"),
	)
})

type queryRecorder struct{ queries []string }

func (r *queryRecorder) Install(string, types.Repository) error   { return nil }
func (r *queryRecorder) Uninstall(string, types.Repository) error { return nil }
func (r *queryRecorder) IsInstalled(command string) (bool, error) {
	r.queries = append(r.queries, command)
	return true, nil
}
