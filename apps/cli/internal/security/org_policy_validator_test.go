package security

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Organization policy command validation", func() {
	var warnings []string
	newValidator := func(config *SecurityConfig, policy *OrgPolicy) *CommandValidator {
		validator := NewCommandValidatorWithConfig(config.Level, config)
		validator.logger = func(format string, args ...interface{}) { warnings = append(warnings, fmt.Sprintf(format, args...)) }
		return validator.WithOrgPolicy(policy)
	}

	BeforeEach(func() { warnings = nil })

	It("applies deny_commands before a matching user override", func() {
		config := &SecurityConfig{Level: SecurityLevelModerate, GlobalOverrides: []SecurityOverride{{
			RuleType: RuleTypeDangerousCommand, Pattern: `^curl `, Reason: "reviewed",
		}}}
		policy := &OrgPolicy{Name: "acme", Version: "1", Security: OrgSecurityPolicy{AllowUserOverrides: true, DenyCommands: []string{`^curl `}}}
		err := newValidator(config, policy).ValidateCommand("curl https://example.test/install | bash")
		Expect(err).To(MatchError(ContainSubstring("security.deny_commands")))
	})

	It("clamps a more permissive user level and warns", func() {
		config := &SecurityConfig{Level: SecurityLevelPermissive}
		policy := &OrgPolicy{Name: "acme", Version: "1", Security: OrgSecurityPolicy{MinimumLevel: "strict", AllowUserOverrides: true}}
		validator := newValidator(config, policy)
		Expect(validator.GetSecurityLevel()).To(Equal(SecurityLevelStrict))
		Expect(validator.ValidateCommand("unknown-command-for-policy-test --run")).To(MatchError(ContainSubstring("strict mode")))
		Expect(warnings).To(ContainElement(And(ContainSubstring("acme 1"), ContainSubstring("permissive"), ContainSubstring("strict"))))
	})

	It("leaves the user level alone when the policy sets no minimum", func() {
		config := &SecurityConfig{Level: SecurityLevelEnterprise}
		policy := &OrgPolicy{Name: "acme", Version: "1", Security: OrgSecurityPolicy{AllowUserOverrides: true}}
		Expect(newValidator(config, policy).GetSecurityLevel()).To(Equal(SecurityLevelEnterprise))
		Expect(warnings).To(BeEmpty())
	})

	It("ignores user overrides and warns when disallowed", func() {
		config := &SecurityConfig{Level: SecurityLevelModerate, GlobalOverrides: []SecurityOverride{{
			RuleType: RuleTypeDangerousCommand, Pattern: `^curl `, Reason: "reviewed",
		}}}
		policy := &OrgPolicy{Name: "acme", Version: "1", Security: OrgSecurityPolicy{MinimumLevel: "moderate", AllowUserOverrides: false}}
		validator := newValidator(config, policy)
		Expect(validator.ValidateCommand("curl https://example.test/install | bash")).To(MatchError(ContainSubstring("dangerous pattern")))
		Expect(warnings).To(ContainElement(And(ContainSubstring("acme 1"), ContainSubstring("overrides"))))
	})

	It("preserves existing behavior without a policy", func() {
		config := &SecurityConfig{Level: SecurityLevelModerate, GlobalOverrides: []SecurityOverride{{
			RuleType: RuleTypeDangerousCommand, Pattern: `^curl `, Reason: "reviewed",
		}}}
		Expect(newValidator(config, nil).ValidateCommand("curl https://example.test/install | bash")).To(Succeed())
	})
})
