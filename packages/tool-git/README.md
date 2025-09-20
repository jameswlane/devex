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
devex tool git ssh-key --generate --add-to-agent

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
