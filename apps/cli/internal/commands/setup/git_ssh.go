package setup

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func (m *SetupModel) renderSSHPreview() string {
	switch m.git.sshMode {
	case "generate":
		return fmt.Sprintf("  • SSH: create UNENCRYPTED Ed25519 key %s (0600) and %s.pub (0644); parent directory must exist with mode 0700; never overwrite files.\n  • No SSH config changes or agent startup; register the public key with your Git host, then ssh-add the private key yourself.\n", m.git.sshPath, m.git.sshPath)
	case "existing":
		return fmt.Sprintf("  • SSH: select existing key %s; require safe permissions and display %s.pub in the setup log; no files changed. Register it with your Git host and run ssh-add yourself.\n", m.git.sshPath, m.git.sshPath)
	default:
		return "  • SSH: skipped (optional; press p to configure a key).\n"
	}
}

func (m *SetupModel) applyGitConfiguration(ctx context.Context, manager configurationPluginExecutor, name, email string) error {
	if m.git.sshMode != "" {
		if m.system.detectedPlatform.OS != "linux" {
			return fmt.Errorf("SSH setup currently supports Linux only")
		}
		if !filepath.IsAbs(m.git.sshPath) || strings.ContainsAny(m.git.sshPath, "\r\n\x00") {
			return fmt.Errorf("SSH key path must be absolute")
		}
		args := []string{"ssh", "--mode", m.git.sshMode, "--path", m.git.sshPath}
		if m.git.sshMode == "generate" {
			args = append(args, "--apply")
		}
		if err := runConfigurationPlugin(ctx, manager, "tool-git", args); err != nil {
			return err
		}
		if m.git.sshMode == "generate" {
			m.installation.addChange(fmt.Sprintf("SSH key created: %s and %s.pub", m.git.sshPath, m.git.sshPath))
		} else {
			m.installation.addChange(fmt.Sprintf("SSH key selected: %s (no files changed)", m.git.sshPath))
		}
	}
	if name == "" && email == "" {
		return nil
	}
	if m.gitIdentityUnchanged() {
		m.installation.addChange(fmt.Sprintf("Git identity kept: %s <%s>", name, email))
		return nil
	}
	// Record the previous identity even if the plugin fails partway; restoring
	// an unchanged key is harmless.
	m.systemUndo.changes = append(m.systemUndo.changes, m.captureGitIdentity(ctx, name, email)...)
	if err := runConfigurationPlugin(ctx, manager, "tool-git", gitConfigurationArgs(name, email)); err != nil {
		return err
	}
	m.installation.addChange(fmt.Sprintf("Git identity set: %s <%s>", name, email))
	return nil
}
