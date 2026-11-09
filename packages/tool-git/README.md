# DevEx Git Tool Plugin

[![Plugin Version](https://img.shields.io/badge/Version-1.0.0-green)](../../CHANGELOG.md)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../../LICENSE)
[![Git](https://img.shields.io/badge/Git-Version%20Control-F05032?logo=git)](https://git-scm.com/)

Git version control system configuration plugin for DevEx. Provides comprehensive Git setup, configuration management, and credential handling for development workflows.

## 🚀 Features

- **⚙️ Configuration Management**: Complete Git configuration setup and management
- **🔐 Credential Handling**: SSH keys, GPG signing, and credential storage
- **🌿 Workflow Optimization**: Branch management, aliases, and hooks setup
- **🛡️ Security**: GPG commit signing and verification setup
- **🔄 Repository Management**: Clone, initialize, and configure repositories
- **📊 Performance**: Git performance optimization and large repository handling

## 📦 Supported Operations

### Configuration
- **User Setup**: Name, email, and global preferences
- **Aliases**: Useful Git command shortcuts and workflows
- **Editor Integration**: Configure default editor and merge tools
- **Credential Storage**: SSH keys, GPG keys, and credential managers
- **Hooks**: Pre-commit, post-commit, and other Git hooks

### Security Features  
- **SSH Key Management**: Generate, configure, and manage SSH keys
- **GPG Signing**: Set up commit and tag signing with GPG
- **Credential Storage**: Secure credential caching and storage
- **Repository Verification**: Signature verification and security checks

## 🚀 Quick Start

```bash
# Configure Git identity
devex tool git setup --name "Your Name" --email "your@email.com"

# Generate and configure SSH key
devex plugin run tool-git ssh --mode generate --path "$HOME/.ssh/id_devex"
# Review the preview, then add --apply to consent to an unencrypted key

# Set up GPG signing
devex tool git gpg --setup-signing --key-id "your-key-id"

# Install useful aliases
devex tool git aliases --install-defaults

# Configure development workflow
devex tool git workflow --enable-hooks --set-editor "code"
```

## 🔧 Configuration

### Git Configuration
```yaml
# ~/.devex/tool-git.yaml
git:
  user:
    name: "Your Name"
    email: "your@email.com"
    
  core:
    editor: "code --wait"
    autocrlf: false
    filemode: true
    
  aliases:
    st: "status -s"
    co: "checkout"
    br: "branch"
    ci: "commit"
    unstage: "reset HEAD --"
    
  signing:
    enabled: true
    key_id: "your-gpg-key-id"
    
  credential:
    helper: "manager-core"
    
  hooks:
    pre_commit: true
    commit_msg: true
    pre_push: true
```

### SSH Configuration
```yaml
ssh:
  key_type: "ed25519"
  key_file: "~/.ssh/id_ed25519"
  hosts:
    - hostname: "github.com"
      user: "git"
      identity_file: "~/.ssh/id_ed25519"
    - hostname: "gitlab.com"
      user: "git"
      identity_file: "~/.ssh/id_ed25519"
```

## 🧪 Testing

```bash
# Test Git configuration
go test -run TestGitConfig

# Test SSH key generation
go test -run TestSSHKeyManagement

# Test GPG setup
go test -run TestGPGSigning
```

## 🚀 Platform Support

### Operating Systems
- **Linux**: All distributions with Git support
- **macOS**: 10.15+, 11+, 12+, 13+, 14+
- **Windows**: Windows 10+, Windows 11 (via WSL or native)

### Git Versions
- **Git 2.25+**: Full feature support
- **Git 2.20+**: Core features supported
- **Older versions**: Limited compatibility

## 📄 License

Licensed under the [Apache-2.0 License](../../../LICENSE).

---

<div align="center">

**[DevEx CLI](../../cli)** • **[Plugin Registry](https://registry.devex.sh)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>

## Explicit SSH setup (Debian/Ubuntu)

SSH is optional and skipped by default. In `devex setup`, open Git
Configuration (`p` from Confirmation if an existing identity was detected),
cycle SSH to `existing` or `generate`, and enter an absolute private-key path.
The Confirmation screen previews the operation before installation. Existing
identity remains unchanged when only SSH was selected.

Create the parent directory yourself with mode 0700 (for example,
`mkdir -m 700 ~/.ssh` if it does not exist). Existing private keys must have
mode 0600 or stricter and a regular public `.pub` file alongside them. Symlink
paths are rejected. DevEx does not change existing files or permissions, so
there are no existing files to back up. Generation refuses either occupied
key path, stages files in a protected directory, and publishes without overwrite.

The standalone plugin command also supports preview and public-key display:

```sh
devex plugin run tool-git ssh --mode generate --path "$HOME/.ssh/id_devex"
# Review the preview before explicitly consenting:
devex plugin run tool-git ssh --mode generate --path "$HOME/.ssh/id_devex" --apply
# Display/export only the public key (never the private key):
devex plugin run tool-git ssh --mode existing --path "$HOME/.ssh/id_devex"
cat "$HOME/.ssh/id_devex.pub"  # copy this public key to your host
```

Generation creates an **unencrypted Ed25519 key** for unattended setup. For a
passphrase-protected key, generate it yourself with
`ssh-keygen -t ed25519 -f ~/.ssh/id_devex` and select `existing` in setup.
OpenSSH prompts for the passphrase; DevEx never collects it. You can also add a
passphrase afterward with `ssh-keygen -p -f ~/.ssh/id_devex`.

DevEx never starts an agent, loads a key, edits SSH/Git configuration, or
contacts a remote host for SSH setup. In your current shell, use an existing
agent or start one with `eval "$(ssh-agent -s)"`, then explicitly load your key
with `ssh-add -t 1h ~/.ssh/id_devex`. The key expires after an hour; the agent
lasts until stopped (`ssh-agent -k`) or its session ends. Agents started by a
subprocess cannot export their environment into your parent shell.

Register the displayed public key in your Git host's account SSH keys settings
(GitHub: Settings → SSH and GPG keys; GitLab: Preferences → SSH Keys).
Registration is manual: DevEx performs no upload. Setup captures public-key
output in its log; the `.pub` file is also always available for copying.
Verify the host fingerprint independently before accepting a first connection,
then test authentication (for example, `ssh -T git@github.com`). Use an SSH remote
URL. For a custom key name, load it with ssh-add or explicitly use
`git -c core.sshCommand='ssh -i ~/.ssh/id_devex -o IdentitiesOnly=yes' fetch`.
Windows/macOS integration and automatic agent/keychain configuration are deferred.
