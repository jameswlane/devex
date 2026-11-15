package security

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	"gopkg.in/yaml.v3"
)

// SystemOrgPolicyPath is the administrator-managed policy location.
var SystemOrgPolicyPath = "/etc/devex/policy.yaml"

var cachedOrgPolicy struct {
	sync.Mutex
	loaded  bool
	homeDir string
	policy  *OrgPolicy
	err     error
}

// PolicyRules contains glob or URL-prefix allow and deny rules.
type PolicyRules struct {
	Allow []string `yaml:"allow,omitempty" json:"allow"`
	Deny  []string `yaml:"deny,omitempty" json:"deny"`
}

// OrgPrivilegePolicy limits the privilege scope an installation may request.
type OrgPrivilegePolicy struct {
	MaxScope string `yaml:"max_scope,omitempty" json:"max_scope"`
}

// OrgSecurityPolicy sets the organization command-security floor.
type OrgSecurityPolicy struct {
	MinimumLevel       string   `yaml:"minimum_level,omitempty" json:"minimum_level"`
	AllowUserOverrides bool     `yaml:"allow_user_overrides" json:"allow_user_overrides"`
	DenyCommands       []string `yaml:"deny_commands,omitempty" json:"deny_commands"`
}

// OrgPolicy is the single organization policy applied to an operation.
type OrgPolicy struct {
	Version        string             `yaml:"version" json:"version"`
	Name           string             `yaml:"name,omitempty" json:"name,omitempty"`
	Applications   PolicyRules        `yaml:"applications,omitempty" json:"applications"`
	InstallMethods PolicyRules        `yaml:"install_methods,omitempty" json:"install_methods"`
	Plugins        PolicyRules        `yaml:"plugins,omitempty" json:"plugins"`
	Sources        PolicyRules        `yaml:"sources,omitempty" json:"sources"`
	Privilege      OrgPrivilegePolicy `yaml:"privilege,omitempty" json:"privilege"`
	Security       OrgSecurityPolicy  `yaml:"security,omitempty" json:"security"`
	Approval       OrgApprovalPolicy  `yaml:"approval,omitempty" json:"approval"`
	Source         string             `yaml:"-" json:"source"`
	SHA256         string             `yaml:"-" json:"sha256"`
}

type rawOrgPolicy struct {
	Version        string             `yaml:"version"`
	Name           string             `yaml:"name"`
	Applications   PolicyRules        `yaml:"applications"`
	InstallMethods PolicyRules        `yaml:"install_methods"`
	Plugins        PolicyRules        `yaml:"plugins"`
	Sources        PolicyRules        `yaml:"sources"`
	Privilege      OrgPrivilegePolicy `yaml:"privilege"`
	Approval       OrgApprovalPolicy  `yaml:"approval"`
	Security       struct {
		MinimumLevel       string   `yaml:"minimum_level"`
		AllowUserOverrides *bool    `yaml:"allow_user_overrides"`
		DenyCommands       []string `yaml:"deny_commands"`
	} `yaml:"security"`
}

// PolicyViolationError identifies the organization rule that rejected a subject.
type PolicyViolationError struct {
	PolicyName    string
	PolicyVersion string
	Rule          string
	Subject       string
}

func (e *PolicyViolationError) Error() string {
	name := e.PolicyName
	if name == "" {
		name = "organization"
	}
	return fmt.Sprintf("organization policy %s %s denied %q by %s; contact your administrator or run 'devex policy show'", name, e.PolicyVersion, e.Subject, e.Rule)
}

// DisplayName returns a useful name when the optional policy name is omitted.
func (p *OrgPolicy) DisplayName() string {
	if p.Name == "" {
		return "organization"
	}
	return p.Name
}

// TeamConfigDir returns the team configuration directory for a home directory.
func TeamConfigDir(homeDir string) string {
	if teamDir := os.Getenv("DEVEX_TEAM_CONFIG_DIR"); teamDir != "" {
		return teamDir
	}
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return filepath.Join(devexpaths.ForHome(homeDir).Config, "team")
}

// LoadOrgPolicy loads the system policy, or the team policy when no system file exists.
func LoadOrgPolicy(homeDir string) (*OrgPolicy, error) {
	candidates := []string{SystemOrgPolicyPath, filepath.Join(TeamConfigDir(homeDir), "policy.yaml")}
	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if errorsIsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("load organization policy %s: %w", candidate, err)
		}
		return loadOrgPolicyFile(candidate)
	}
	return nil, nil
}

// LoadOrgPolicyCached loads the process-wide plugin policy once. Tests must call
// ResetOrgPolicyCache before changing the policy file or its locations.
func LoadOrgPolicyCached(homeDir string) (*OrgPolicy, error) {
	cachedOrgPolicy.Lock()
	defer cachedOrgPolicy.Unlock()
	if cachedOrgPolicy.loaded && cachedOrgPolicy.homeDir == homeDir {
		return cachedOrgPolicy.policy, cachedOrgPolicy.err
	}
	cachedOrgPolicy.loaded = true
	cachedOrgPolicy.homeDir = homeDir
	cachedOrgPolicy.policy, cachedOrgPolicy.err = LoadOrgPolicy(homeDir)
	return cachedOrgPolicy.policy, cachedOrgPolicy.err
}

// PluginFilter returns a plugin manager filter that rejects plugins the
// organization policy denies, and every plugin when the policy cannot load.
func PluginFilter(homeDir string) func(pluginName string) error {
	return func(pluginName string) error {
		policy, err := LoadOrgPolicyCached(homeDir)
		if err != nil {
			return err
		}
		return policy.CheckPlugin(pluginName)
	}
}

// ResetOrgPolicyCache clears the process-wide plugin policy cache.
func ResetOrgPolicyCache() {
	cachedOrgPolicy.Lock()
	defer cachedOrgPolicy.Unlock()
	cachedOrgPolicy.loaded = false
	cachedOrgPolicy.homeDir = ""
	cachedOrgPolicy.policy = nil
	cachedOrgPolicy.err = nil
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func loadOrgPolicyFile(path string) (*OrgPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load organization policy %s: %w", path, err)
	}
	var raw rawOrgPolicy
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid organization policy %s: %w", path, err)
	}
	if err := ensureYAMLEnd(decoder); err != nil {
		return nil, fmt.Errorf("invalid organization policy %s: %w", path, err)
	}
	allowOverrides := true
	if raw.Security.AllowUserOverrides != nil {
		allowOverrides = *raw.Security.AllowUserOverrides
	}
	policy := &OrgPolicy{
		Version: raw.Version, Name: raw.Name,
		Applications: raw.Applications, InstallMethods: raw.InstallMethods,
		Plugins: raw.Plugins, Sources: raw.Sources, Privilege: raw.Privilege, Approval: raw.Approval,
		Security: OrgSecurityPolicy{MinimumLevel: raw.Security.MinimumLevel, AllowUserOverrides: allowOverrides, DenyCommands: raw.Security.DenyCommands},
		Source:   path,
	}
	sum := sha256.Sum256(data)
	policy.SHA256 = hex.EncodeToString(sum[:])
	if policy.Privilege.MaxScope == "" {
		policy.Privilege.MaxScope = "system"
	}
	if err := policy.validate(); err != nil {
		return nil, fmt.Errorf("invalid organization policy %s: %w", path, err)
	}
	return policy, nil
}

func ensureYAMLEnd(decoder *yaml.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple YAML documents are not supported")
		}
		return err
	}
	return nil
}

func (p *OrgPolicy) validate() error {
	if strings.TrimSpace(p.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if p.Privilege.MaxScope != "user" && p.Privilege.MaxScope != "system" {
		return fmt.Errorf("privilege.max_scope must be user or system")
	}
	if _, err := parseSecurityLevel(p.Security.MinimumLevel); err != nil {
		return fmt.Errorf("security.minimum_level: %w", err)
	}
	for _, pattern := range p.Security.DenyCommands {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("security.deny_commands pattern %q: %w", pattern, err)
		}
	}
	if err := p.Approval.validate(); err != nil {
		return err
	}
	for _, rules := range []PolicyRules{p.Applications, p.InstallMethods, p.Plugins} {
		for _, pattern := range append(append([]string{}, rules.Allow...), rules.Deny...) {
			if _, err := path.Match(pattern, ""); err != nil {
				return fmt.Errorf("invalid glob %q: %w", pattern, err)
			}
		}
	}
	return nil
}

func parseSecurityLevel(level string) (SecurityLevel, error) {
	switch level {
	case "strict":
		return SecurityLevelStrict, nil
	case "moderate", "":
		return SecurityLevelModerate, nil
	case "permissive":
		return SecurityLevelPermissive, nil
	case "enterprise":
		return SecurityLevelEnterprise, nil
	default:
		return SecurityLevelModerate, fmt.Errorf("must be strict, moderate, permissive, or enterprise")
	}
}

func (p *OrgPolicy) violation(rule, subject string) error {
	return &PolicyViolationError{PolicyName: p.Name, PolicyVersion: p.Version, Rule: rule, Subject: subject}
}

// matchName compares names case-insensitively, like application lookup does.
func matchName(pattern, subject string) bool {
	matched, _ := path.Match(strings.ToLower(strings.TrimSpace(pattern)), strings.ToLower(strings.TrimSpace(subject)))
	return matched
}

func (p *OrgPolicy) checkRules(rules PolicyRules, rule, subject string) error {
	for _, pattern := range rules.Deny {
		if matchName(pattern, subject) {
			return p.violation(rule+".deny", subject)
		}
	}
	if len(rules.Allow) == 0 {
		return nil
	}
	for _, pattern := range rules.Allow {
		if matchName(pattern, subject) {
			return nil
		}
	}
	return p.violation(rule+".allow", subject)
}

// CheckPlugin rejects a plugin denied by name.
func (p *OrgPolicy) CheckPlugin(name string) error {
	if p == nil {
		return nil
	}
	return p.checkRules(p.Plugins, "plugins", name)
}

// CheckHook rejects denied shell text and privilege exceeding max_scope.
func (p *OrgPolicy) CheckHook(shell string, scope sdk.OperationScope) error {
	if p == nil {
		return nil
	}
	for _, pattern := range p.Security.DenyCommands {
		if regexp.MustCompile(pattern).MatchString(shell) {
			return p.violation("security.deny_commands", shell)
		}
	}
	if scope == "" {
		scope = sdk.ScopeUser
	}
	if scope == sdk.ScopeSystem && p.Privilege.MaxScope == "user" {
		return p.violation("privilege.max_scope", string(scope))
	}
	return nil
}

// CheckApp rejects an application and its selected installation definition.
func (p *OrgPolicy) CheckApp(app types.CrossPlatformApp) error {
	if p == nil {
		return nil
	}
	if err := p.checkRules(p.Applications, "applications", app.Name); err != nil {
		return err
	}
	return p.checkOSConfig(app.GetOSConfig())
}

// CheckAppConfig applies application policy to the legacy installation shape.
func (p *OrgPolicy) CheckAppConfig(app types.AppConfig) error {
	if p == nil {
		return nil
	}
	return p.CheckApp(types.CrossPlatformApp{Name: app.Name, AllPlatforms: types.OSConfig{
		InstallMethod: app.InstallMethod, InstallCommand: app.InstallCommand,
		UninstallCommand: app.UninstallCommand, Packages: app.Packages, Options: app.Options,
		Dependencies: app.Dependencies, PreInstall: app.PreInstall, PostInstall: app.PostInstall,
		ConfigFiles: app.ConfigFiles, Themes: app.Themes, AptSources: app.AptSources, CleanupFiles: app.CleanupFiles,
		Conflicts: app.Conflicts, DownloadURL: app.DownloadURL, Verification: app.Verification,
		Destination: app.InstallDir,
	}})
}

// checkOSConfig checks the configuration GetOSConfig selected for this
// platform; unused alternatives never run, so they are not checked.
func (p *OrgPolicy) checkOSConfig(config types.OSConfig) error {
	if err := p.checkRules(p.InstallMethods, "install_methods", config.InstallMethod); err != nil {
		return err
	}
	if err := p.CheckPlugin("package-manager-" + config.InstallMethod); err != nil {
		return err
	}
	for _, source := range appSources(config) {
		if err := p.checkSource(source); err != nil {
			return err
		}
	}
	for _, hook := range append(append([]types.InstallCommand{}, config.PreInstall...), config.PostInstall...) {
		if err := p.CheckHook(hook.Shell, sdk.OperationScope(hook.Scope)); err != nil {
			return err
		}
	}
	return nil
}

func appSources(config types.OSConfig) []string {
	var sources []string
	// Fields may hold more than a URL, such as an APT source line
	// "deb [signed-by=...] https://repo suite main", so check every token.
	add := func(values ...string) {
		for _, value := range values {
			for _, token := range strings.Fields(value) {
				token = strings.Trim(token, `"'[]`)
				if strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://") {
					sources = append(sources, token)
				}
			}
		}
	}
	add(config.InstallCommand, config.DownloadURL, config.BrewTap, config.Verification.SignatureURL, config.Verification.PublicKeyPath)
	for _, source := range config.AptSources {
		add(source.KeySource, source.SourceRepo)
	}
	for _, file := range config.ConfigFiles {
		add(file.Source)
	}
	for _, theme := range config.Themes {
		for _, file := range theme.Files {
			add(file.Source)
		}
	}
	for _, hook := range append(append([]types.InstallCommand{}, config.PreInstall...), config.PostInstall...) {
		if hook.Copy != nil {
			add(hook.Copy.Source)
		}
	}
	return sources
}

func (p *OrgPolicy) checkSource(source string) error {
	for _, prefix := range p.Sources.Deny {
		if strings.HasPrefix(source, prefix) {
			return p.violation("sources.deny", source)
		}
	}
	if len(p.Sources.Allow) == 0 {
		return nil
	}
	for _, prefix := range p.Sources.Allow {
		if strings.HasPrefix(source, prefix) {
			return nil
		}
	}
	return p.violation("sources.allow", source)
}
