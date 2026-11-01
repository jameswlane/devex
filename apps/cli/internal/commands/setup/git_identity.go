package setup

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// gitIdentityTimeout bounds each `git config` lookup so a broken git install
// cannot stall setup.
const gitIdentityTimeout = 3 * time.Second

// readGlobalGitIdentity returns the global user.name and user.email, or empty
// strings when git is missing or a value is unset. Tests replace it.
var readGlobalGitIdentity = func(ctx context.Context) (name, email string) {
	get := func(key string) string {
		ctx, cancel := context.WithTimeout(ctx, gitIdentityTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "git", "config", "--global", "--get", key).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return get("user.name"), get("user.email")
}

// loadExistingGitIdentity reads the global git identity once and pre-fills the
// Git Configuration fields that the user has not typed into yet.
func (m *SetupModel) loadExistingGitIdentity() {
	if m.git.identityLoaded {
		return
	}
	m.git.identityLoaded = true
	m.git.existingName, m.git.existingEmail = readGlobalGitIdentity(context.Background())
	if m.git.gitFullName == "" {
		m.git.gitFullName = m.git.existingName
	}
	if m.git.gitEmail == "" {
		m.git.gitEmail = m.git.existingEmail
	}
}

// hasCompleteExistingIdentity reports whether git already has a usable global
// name and email, so setup can skip asking for them.
func (m *SetupModel) hasCompleteExistingIdentity() bool {
	return m.git.existingName != "" && isValidEmail(m.git.existingEmail)
}

// gitIdentityUnchanged reports whether the entered identity equals the
// existing global identity, in which case setup must not rewrite it.
func (m *SetupModel) gitIdentityUnchanged() bool {
	return m.hasCompleteExistingIdentity() &&
		strings.TrimSpace(m.git.gitFullName) == m.git.existingName &&
		strings.TrimSpace(m.git.gitEmail) == m.git.existingEmail
}
