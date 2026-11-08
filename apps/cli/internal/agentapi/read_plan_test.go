package agentapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type requestKey struct{}

type readSpy struct {
	types.Repository
	reads  int
	writes int
}

func (s *readSpy) ListApps() ([]types.AppConfig, error) { s.reads++; return s.Repository.ListApps() }
func (s *readSpy) SaveApp(_ types.AppConfig) error      { s.writes++; return errors.New("write forbidden") }
func (s *readSpy) Set(_, _ string) error                { s.writes++; return errors.New("write forbidden") }
func (s *readSpy) AddApp(_ string) error                { s.writes++; return errors.New("write forbidden") }
func (s *readSpy) DeleteApp(_ string) error             { s.writes++; return errors.New("write forbidden") }

var _ = Describe("Read and plan contracts", func() {
	var settings config.CrossPlatformSettings
	var repo *readSpy
	BeforeEach(func() {
		viper.Reset()
		settings = config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir(), ApplicationOverrides: []types.CrossPlatformApp{
			{Name: "base", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "base", UninstallCommand: "base"}},
			{Name: "app", Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "app", UninstallCommand: "app", Dependencies: []string{"base"}}},
		}}
		repo = &readSpy{Repository: mocks.NewMockRepository()}
	})
	AfterEach(func() { viper.Reset() })
	It("orders dependencies before dependents and states unknown plugin effects", func() {
		plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"app"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.DependencyOrder).To(Equal([]string{"base", "app"}))
		Expect(plan.Apps).To(HaveLen(2))
		Expect(plan.Apps[1].UnknownEffects).To(BeTrue())
		Expect(plan.Apps[1].Plugin).To(Equal("package-manager-apt"))
		Expect(plan.Apps[1].Privileges).To(Equal("unknown"))
		data, err := json.Marshal(plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Valid(data)).To(BeTrue())
		fixture, err := os.ReadFile("testdata/plan.json")
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(MatchJSON(fixture))
	})
	It("uses the execution selection policy for explicit and inline overrides", func() {
		viper.Set("applications", []any{map[string]any{"name": "inline", "linux": map[string]any{"install_method": "apt", "install_command": "inline", "uninstall_command": "inline"}}})
		apps, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveLen(1))
		Expect(apps[0].Name).To(Equal("inline"))
		plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.DependencyOrder).To(Equal([]string{"inline"}))
	})
	It("encodes an explicitly empty plan and reports unknown selection", func() {
		viper.Set("applications", []any{})
		plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{})
		Expect(err).NotTo(HaveOccurred())
		data, _ := json.Marshal(plan)
		Expect(data).To(MatchJSON(`{"apps":[],"dependency_order":[],"warnings":[]}`))
		_, err = agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"missing"}})
		Expect(err).To(HaveOccurred())
		Expect(agentapi.APIError(err).Category).To(Equal("invalid_request"))
	})
	It("redacts hook bodies, download credentials and environment settings without effects", func() {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
		defer server.Close()
		app := &settings.ApplicationOverrides[1]
		app.Linux.DownloadURL = server.URL + "/SECRET"
		app.Linux.PreInstall = []types.InstallCommand{{Command: "echo SECRET"}}
		viper.Set("api_token", "SECRET")
		before := snapshot(settings.HomeDir)
		effective, err := agentapi.Effective(context.Background(), settings)
		Expect(err).NotTo(HaveOccurred())
		data, _ := json.Marshal(effective)
		Expect(json.Valid(data)).To(BeTrue())
		Expect(string(data)).NotTo(ContainSubstring("SECRET"))
		Expect(string(data)).NotTo(ContainSubstring(server.URL))
		plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{})
		Expect(err).NotTo(HaveOccurred())
		data, _ = json.Marshal(plan)
		Expect(string(data)).NotTo(ContainSubstring("SECRET"))
		Expect(plan.Apps[1].Effects).To(ContainElement("pre_install_hooks"))
		_, err = agentapi.List(context.Background(), repo, settings, agentapi.ListRequest{Kind: "available"})
		Expect(err).NotTo(HaveOccurred())
		_, err = agentapi.Status(context.Background(), settings, repo, agentapi.StatusRequest{}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.writes).To(BeZero())
		Expect(calls.Load()).To(BeZero())
		Expect(snapshot(settings.HomeDir)).To(Equal(before))
	})
	It("returns empty discovery arrays with search misses", func() {
		result, err := agentapi.List(context.Background(), repo, settings, agentapi.ListRequest{Kind: "available", Options: agentapi.ListCommandOptions{Search: "missing"}})
		Expect(err).NotTo(HaveOccurred())
		data, _ := json.Marshal(result.Available)
		Expect(data).To(MatchJSON(`[]`))
		result, err = agentapi.List(context.Background(), repo, settings, agentapi.ListRequest{Kind: "installed"})
		Expect(err).NotTo(HaveOccurred())
		data, _ = json.Marshal(result.Installed)
		Expect(data).To(MatchJSON(`[]`))
	})
	It("retains partial status results without leaking plugin error details", func() {
		calls := 0
		ctx := context.WithValue(context.Background(), requestKey{}, "deadline")
		results, err := agentapi.Status(ctx, settings, repo, agentapi.StatusRequest{Names: []string{"base", "app"}}, func(ctx context.Context, app types.AppConfig) (bool, error) {
			Expect(ctx.Value(requestKey{})).To(Equal("deadline"))
			calls++
			if app.Name == "app" {
				return false, errors.New("SECRET credential failure")
			}
			return true, nil
		})
		// The same request context is explicitly forwarded to probes.
		Expect(calls).To(Equal(2))
		Expect(agentapi.APIError(err).Category).To(Equal("partial_failure"))
		Expect(results).To(HaveLen(2))
		Expect(results[0].Installed).To(BeTrue())
		Expect(results[1].Status).To(Equal("error"))
		data, _ := json.Marshal(results)
		Expect(json.Valid(data)).To(BeTrue())
		Expect(string(data)).NotTo(ContainSubstring("SECRET"))
	})
	It("stops all services before accessing dependencies on cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := agentapi.PlanInstall(ctx, settings, agentapi.PlanRequest{})
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		_, err = agentapi.Effective(ctx, settings)
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		_, err = agentapi.List(ctx, repo, settings, agentapi.ListRequest{Kind: "available"})
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		_, err = agentapi.Status(ctx, settings, repo, agentapi.StatusRequest{}, nil)
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		_, err = agentapi.DetectPlatform(ctx)
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(repo.reads).To(BeZero())
	})
})
