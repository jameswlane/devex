package agentapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Organization policy installation plans", func() {
	// The fixtures model Linux catalog entries; pin them so macOS hosts agree with CI.
	BeforeEach(func() { DeferCleanup(types.PinCatalogOSForTesting("linux")) })

	var home, teamDir, policyPath string
	var settings config.CrossPlatformSettings

	writePolicy := func(contents string) {
		Expect(os.MkdirAll(filepath.Dir(policyPath), 0o750)).To(Succeed())
		Expect(os.WriteFile(policyPath, []byte(contents), 0o600)).To(Succeed())
	}

	BeforeEach(func() {
		viper.Reset()
		home = GinkgoT().TempDir()
		teamDir = filepath.Join(home, "team")
		policyPath = filepath.Join(teamDir, "policy.yaml")
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		original := security.SystemOrgPolicyPath
		security.SystemOrgPolicyPath = filepath.Join(home, "system-policy.yaml")
		DeferCleanup(func() { security.SystemOrgPolicyPath = original; viper.Reset() })
		settings = config.CrossPlatformSettings{HomeDir: home, ApplicationOverrides: []types.CrossPlatformApp{
			{Name: "base", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "base", UninstallCommand: "base"}},
			{Name: "app", Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "app", UninstallCommand: "app", Dependencies: []string{"base"}}},
		}}
	})

	DescribeTable("rejects denied applications and dependencies with policy_denied",
		func(policy string, mutate func(*config.CrossPlatformSettings), subject string) {
			writePolicy(policy)
			if mutate != nil {
				mutate(&settings)
			}
			_, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"app"}})
			var apiErr *agentapi.Error
			Expect(errors.As(err, &apiErr)).To(BeTrue())
			Expect(apiErr.Category).To(Equal("policy_denied"))
			Expect(apiErr.Message).To(ContainSubstring(subject))
		},
		Entry("application", "version: '1'\napplications:\n  deny: [app]\n", nil, "app"),
		Entry("dependency", "version: '1'\napplications:\n  deny: [base]\n", nil, "base"),
		Entry("install method", "version: '1'\ninstall_methods:\n  deny: [apt]\n", nil, "apt"),
		Entry("download source", "version: '1'\nsources:\n  allow: ['https://allowed.example/']\n", func(s *config.CrossPlatformSettings) {
			s.ApplicationOverrides[1].Linux.DownloadURL = "https://denied.example/install.sh"
		}, "https://denied.example/install.sh"),
		Entry("system hook under user scope", "version: '1'\nprivilege:\n  max_scope: user\n", func(s *config.CrossPlatformSettings) {
			s.ApplicationOverrides[1].Linux.PreInstall = []types.InstallCommand{{Scope: "system", Shell: "echo ok"}}
		}, "privilege.max_scope"),
	)

	It("lists every denied app instead of stopping at the first", func() {
		writePolicy("version: '1'\napplications:\n  deny: [app, base]\n")
		_, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"app"}})
		Expect(err).To(MatchError(And(ContainSubstring(`"app"`), ContainSubstring(`"base"`))))
	})

	It("records an allowed policy reference in the plan", func() {
		writePolicy("version: '2026.11.1'\nname: acme\napplications:\n  allow: [app, base]\n")
		plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"app"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.OrganizationPolicy).NotTo(BeNil())
		Expect(plan.OrganizationPolicy.Name).To(Equal("acme"))
		Expect(plan.OrganizationPolicy.Version).To(Equal("2026.11.1"))
		Expect(plan.OrganizationPolicy.Source).To(Equal(policyPath))
		Expect(plan.OrganizationPolicy.SHA256).To(MatchRegexp(`^[0-9a-f]{64}$`))
		data, err := json.Marshal(plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Valid(data)).To(BeTrue())
		Expect(string(data)).To(ContainSubstring(`"organization_policy":{"name":"acme","version":"2026.11.1"`))
	})

	It("changes a confirmed digest when the policy version changes", func() {
		writePolicy("version: '1'\nname: acme\n")
		first, err := agentapi.ResolveExecution(context.Background(), settings, agentapi.PlanRequest{Names: []string{"app"}}, agentapi.ExecutionPolicy{}, agentapi.ExecutionTrust{})
		Expect(err).NotTo(HaveOccurred())
		writePolicy("version: '2'\nname: acme\n")
		second, err := agentapi.ResolveExecution(context.Background(), settings, agentapi.PlanRequest{Names: []string{"app"}}, agentapi.ExecutionPolicy{}, agentapi.ExecutionTrust{})
		Expect(err).NotTo(HaveOccurred())
		Expect(second.Digest).NotTo(Equal(first.Digest))
	})
})
