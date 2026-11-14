package security

import (
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Organization policy", func() {
	var home, teamDir string

	writePolicy := func(path, contents string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0o750)).To(Succeed())
		Expect(os.WriteFile(path, []byte(contents), 0o600)).To(Succeed())
	}

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		teamDir = filepath.Join(home, "team")
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		SystemOrgPolicyPath = filepath.Join(home, "system", "policy.yaml")
		DeferCleanup(func() { SystemOrgPolicyPath = "/etc/devex/policy.yaml" })
	})

	It("returns no policy when neither location has a file", func() {
		policy, err := LoadOrgPolicy(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(policy).To(BeNil())
	})

	It("loads the team policy and records its path and content digest", func() {
		path := filepath.Join(teamDir, "policy.yaml")
		writePolicy(path, "version: '2026.11.1'\nname: acme\n")
		policy, err := LoadOrgPolicy(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(policy.Name).To(Equal("acme"))
		Expect(policy.Version).To(Equal("2026.11.1"))
		Expect(policy.Source).To(Equal(path))
		Expect(policy.SHA256).To(MatchRegexp(`^[0-9a-f]{64}$`))
		Expect(policy.Privilege.MaxScope).To(Equal("system"))
		Expect(policy.Security.AllowUserOverrides).To(BeTrue())
	})

	It("prefers the system policy over the team policy", func() {
		writePolicy(filepath.Join(teamDir, "policy.yaml"), "version: team\nname: team\n")
		writePolicy(SystemOrgPolicyPath, "version: system\nname: system\n")
		policy, err := LoadOrgPolicy(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(policy.Name).To(Equal("system"))
		Expect(policy.Source).To(Equal(SystemOrgPolicyPath))
	})

	DescribeTable("fails closed for an invalid policy",
		func(contents string) {
			writePolicy(SystemOrgPolicyPath, contents)
			policy, err := LoadOrgPolicy(home)
			Expect(policy).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring(SystemOrgPolicyPath)))
		},
		Entry("invalid YAML", "version: [\n"),
		Entry("unknown fields", "version: '1'\nunknown: true\n"),
		Entry("missing version", "name: acme\n"),
		Entry("invalid deny command regex", "version: '1'\nsecurity:\n  deny_commands: ['[']\n"),
		Entry("invalid privilege scope", "version: '1'\nprivilege:\n  max_scope: root\n"),
		Entry("invalid security level", "version: '1'\nsecurity:\n  minimum_level: unsafe\n"),
	)

	It("fails closed when the selected policy cannot be read", func() {
		Expect(os.MkdirAll(SystemOrgPolicyPath, 0o750)).To(Succeed())
		policy, err := LoadOrgPolicy(home)
		Expect(policy).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring(SystemOrgPolicyPath)))
	})

	It("uses the default team directory beneath the DevEx config directory", func() {
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		path := filepath.Join(home, ".config", "devex", "team", "policy.yaml")
		writePolicy(path, "version: '1'\n")
		policy, err := LoadOrgPolicy(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(policy.Source).To(Equal(path))
	})

	Describe("rule precedence", func() {
		It("applies deny before allow and treats an empty allow list as allowing all", func() {
			policy := &OrgPolicy{
				Version: "1", Name: "acme",
				Applications:   PolicyRules{Allow: []string{"*"}, Deny: []string{"blocked-*"}},
				InstallMethods: PolicyRules{Deny: []string{"curlpipe"}},
				Plugins:        PolicyRules{Allow: []string{"package-manager-*"}},
			}
			Expect(policy.CheckApp(types.CrossPlatformApp{Name: "allowed", AllPlatforms: types.OSConfig{InstallMethod: "apt"}})).To(Succeed())
			Expect(policy.CheckApp(types.CrossPlatformApp{Name: "blocked-app", AllPlatforms: types.OSConfig{InstallMethod: "apt"}})).To(MatchError(And(ContainSubstring("acme 1"), ContainSubstring("applications.deny"), ContainSubstring("devex policy show"))))
			Expect(policy.CheckApp(types.CrossPlatformApp{Name: "allowed", AllPlatforms: types.OSConfig{InstallMethod: "curlpipe"}})).To(MatchError(ContainSubstring("install_methods.deny")))
			Expect(policy.CheckPlugin("package-manager-apt")).To(Succeed())
			Expect(policy.CheckPlugin("unrelated")).To(MatchError(ContainSubstring("plugins.allow")))
		})

		It("matches names case-insensitively, like application lookup", func() {
			policy := &OrgPolicy{Version: "1", Applications: PolicyRules{Deny: []string{"lazygit"}}}
			Expect(policy.CheckApp(types.CrossPlatformApp{Name: "LazyGit", AllPlatforms: types.OSConfig{InstallMethod: "apt"}})).To(MatchError(ContainSubstring("applications.deny")))
		})

		It("checks URL prefixes and hook scope and command rules", func() {
			policy := &OrgPolicy{
				Version: "1", Name: "acme",
				Sources:   PolicyRules{Allow: []string{"https://packages.example/"}, Deny: []string{"https://packages.example/blocked/"}},
				Privilege: OrgPrivilegePolicy{MaxScope: "user"},
				Security:  OrgSecurityPolicy{DenyCommands: []string{`forbidden`}},
			}
			app := types.CrossPlatformApp{Name: "app", AllPlatforms: types.OSConfig{
				InstallMethod: "curlpipe", DownloadURL: "https://packages.example/app/install.sh",
				PreInstall: []types.InstallCommand{{Shell: "echo allowed"}},
			}}
			Expect(policy.CheckApp(app)).To(Succeed())
			app.AllPlatforms.DownloadURL = "https://packages.example/blocked/install.sh"
			Expect(policy.CheckApp(app)).To(MatchError(ContainSubstring("sources.deny")))
			app.AllPlatforms.DownloadURL = "https://evil.example/install.sh"
			Expect(policy.CheckApp(app)).To(MatchError(ContainSubstring("sources.allow")))
			Expect(policy.CheckHook("echo forbidden", sdk.ScopeUser)).To(MatchError(ContainSubstring("security.deny_commands")))
			Expect(policy.CheckHook("echo ok", sdk.ScopeSystem)).To(MatchError(ContainSubstring("privilege.max_scope")))
		})
	})
})
