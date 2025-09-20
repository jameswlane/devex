# Frequently Asked Questions

Common questions and answers about DevEx.

## General Questions

### What is DevEx?

**DevEx** is a command-line tool that streamlines development environment setup and management. It automates the installation of applications, programming languages, and system configurations across different platforms (Linux, macOS, Windows).

### How is DevEx different from other tools?

DevEx is unique because it:
- **Cross-platform**: Works consistently on Linux, macOS, and Windows
- **Comprehensive**: Handles applications, languages, system configs, and desktop settings
- **Team-focused**: Built for sharing configurations across teams
- **Modern UX**: Beautiful terminal interface with progress tracking
- **Intelligent**: Automatically selects the best package manager for your system

### Is DevEx free to use?

Yes, DevEx is completely free and open-source under the MIT license. You can use it for personal projects, commercial work, and contribute to its development.

## Installation & Setup

### How do I install DevEx?

The easiest way is the one-line installer:
```bash
wget -qO- https://devex.sh/install | bash
```

For other installation methods, see the [Installation Guide](installation).

### Can I install DevEx without sudo access?

Yes! Use the `--user` flag during installation or install to `~/.local/bin`:
```bash
mkdir -p ~/.local/bin
wget -O ~/.local/bin/devex https://github.com/jameswlane/devex/releases/latest/download/devex-linux-amd64
chmod +x ~/.local/bin/devex
```

### What systems does DevEx support?

- **Linux**: Ubuntu, Debian, Fedora, CentOS, Arch Linux, openSUSE
- **macOS**: 10.15 (Catalina) and later
- **Windows**: Windows 10 and 11 (experimental)

### Do I need to install package managers manually?

No! DevEx automatically detects available package managers and can help install missing ones. It supports:
- **Linux**: apt, dnf, pacman, zypper, flatpak, snap
- **macOS**: brew, mas
- **Windows**: winget, chocolatey, scoop
- **Universal**: mise, docker, pip, npm, cargo

## Configuration

### Where are DevEx configurations stored?

DevEx stores all configurations in `~/.devex/`:
```
~/.devex/
├── applications.yaml    # Application definitions
├── environment.yaml     # Languages and tools
├── system.yaml         # Git, SSH, terminal settings
├── desktop.yaml        # Desktop customizations (optional)
├── cache/              # Installation cache
├── backups/            # Configuration backups
└── templates/          # Custom templates
```

### Can I use DevEx with existing configurations?

Yes! DevEx can:
- Import existing configurations from files or URLs
- Merge with your current setup
- Detect already installed applications
- Work alongside other tools

### How do I share configurations with my team?

```bash
# Export configuration as bundle
devex config export --format bundle --output team-config.zip

# Team members import it
devex config import team-config.zip --merge
```

You can also use templates or store configurations in version control.

### Can I customize which applications get installed?

Absolutely! You can:
- Use `devex add` to browse and select applications
- Edit configuration files directly
- Create custom templates
- Use categories to group applications
- Apply templates selectively

## Usage

### Do I need to run DevEx as root/administrator?

Generally no, but it depends on what you're installing:
- **User installations**: `devex install --user` (no sudo required)
- **System packages**: May require sudo for system package managers
- **Flatpak/Snap**: Usually no sudo needed
- **Language tools**: Usually installed to user directories

### Can I run DevEx in automated scripts?

Yes! DevEx supports non-interactive mode:
```bash
# Non-interactive installation
devex install --all --no-tui

# Dry-run for testing
devex install --all --dry-run

# JSON output for parsing
devex status --all --format json
```

### How do I update applications installed by DevEx?

```bash
# Update all applications
devex install --all --update

# Update specific applications
devex install docker code --update

# Update DevEx itself
devex self-update
```

### Can I undo changes made by DevEx?

Yes! DevEx includes comprehensive undo capabilities:
```bash
# Create backup before changes
devex config backup create "Before experiment"

# Restore from backup
devex config backup restore backup-id

# Uninstall applications
devex uninstall app-name
```

## Templates

### What templates are available?

DevEx includes templates for:
- **Web Development**: React, Vue, Next.js, full-stack
- **Backend**: APIs, microservices, databases
- **Mobile**: React Native, Flutter, native development
- **Data Science**: Python, R, Jupyter, ML tools
- **DevOps**: Kubernetes, Terraform, monitoring

See all templates: `devex template list`

### Can I create custom templates?

Yes! You can:
```bash
# Create from current configuration
devex template create my-stack

# Create manually
# Edit ~/.devex/templates/my-stack.yaml

# Share with team
devex template export my-stack --output my-template.yaml
```

### How do I update templates?

```bash
# Update all built-in templates
devex template update

# Update specific template
devex template update web-fullstack

# Update remote templates
devex template update --remote
```

## Troubleshooting

### DevEx command not found after installation

Add DevEx to your PATH:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Package installation fails

Try these solutions:
```bash
# Update package lists
sudo apt update  # or equivalent for your system

# Use different installer
devex install app --installer flatpak

# Install with verbose output
devex install app --verbose

# Check system compatibility
devex system doctor
```

### Configuration validation errors

```bash
# Get detailed error information
devex config validate --verbose

# Auto-fix common issues
devex config validate --fix

# Reset to defaults if corrupted
devex config reset --backup
```

### TUI looks broken or garbled

```bash
# Use non-TUI mode
devex install --no-tui --all

# Check terminal capabilities
echo $TERM
tput colors

# Use modern terminal (recommended)
# kitty, alacritty, or latest gnome-terminal
```

## Performance

### How can I speed up installations?

```bash
# Increase parallel installations (use carefully)
devex install --all --parallel 5

# Use cached packages
devex cache stats

# Clean old cache files
devex cache cleanup --max-age 30d
```

### DevEx is using too much disk space

```bash
# Check cache usage
devex cache stats

# Clean cache by size or age
devex cache cleanup --max-size 500MB --max-age 14d

# Remove unused applications
devex uninstall unused-app
```

## Security

### Is DevEx safe to use?

DevEx follows security best practices:
- **No sudo required** for most operations
- **Source verification** for downloads
- **Sandboxed installations** where possible (Flatpak, Snap)
- **Open source** - you can review all code
- **No data collection** - everything stays on your machine

### Does DevEx collect any data?

No. DevEx doesn't collect or transmit any personal data or usage statistics. Everything runs locally on your machine.

### Can I verify DevEx downloads?

Yes, all releases are:
- Signed with GPG keys
- Available with checksums
- Built from source on GitHub Actions
- Reproducible builds when possible

## Advanced Usage

### Can I use DevEx in Docker containers?

Yes! DevEx works great in containers:
```dockerfile
# Install DevEx in container
RUN wget -qO- https://devex.sh/install | bash

# Use in container setup
RUN devex init --template backend-api --non-interactive
RUN devex install --all --user
```

### Can I integrate DevEx with CI/CD?

Absolutely:
```yaml
# GitHub Actions example
- name: Setup development environment
  run: |
    wget -qO- https://devex.sh/install | bash
    devex config import .devex-config.yaml
    devex install --all --no-tui
```

### Can I extend DevEx with plugins?

Not yet, but plugin architecture is planned. Currently you can:
- Create custom installers
- Use template hooks
- Write wrapper scripts
- Contribute to the main project

### How do I contribute to DevEx?

- **Report bugs**: GitHub Issues
- **Suggest features**: GitHub Discussions
- **Contribute code**: Pull Requests welcome
- **Improve docs**: Documentation contributions
- **Share templates**: Community template sharing

## Comparison with Other Tools

### DevEx vs Homebrew

| Feature | DevEx | Homebrew |
|---------|-------|----------|
| Platforms | Linux, macOS, Windows | macOS, Linux |
| Package Managers | Multiple (apt, snap, etc.) | Homebrew only |
| Configuration | YAML files | Brewfile |
| Templates | Built-in + custom | Manual |
| Team Sharing | Built-in | Manual scripts |
| TUI | Yes | No |

### DevEx vs Ansible

| Feature | DevEx | Ansible |
|---------|-------|---------|
| Focus | Development environments | General automation |
| Complexity | Simple YAML | Complex playbooks |
| Learning Curve | Low | High |
| Desktop Apps | Excellent | Limited |
| Team Onboarding | Built-in | Custom playbooks |

### DevEx vs dotfiles

| Feature | DevEx | dotfiles |
|---------|-------|----------|
| Applications | Automated installation | Manual management |
| Cross-platform | Built-in | Complex scripting |
| Team Sharing | Templates + bundles | Git repositories |
| Maintenance | Automated updates | Manual scripts |
| New Machine Setup | Single command | Complex bootstrap |

## Getting More Help

### Where can I find more information?

- **Documentation**: https://docs.devex.sh
- **Command help**: `devex help` or `devex <command> --help`
- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Community Q&A
- **Examples**: https://github.com/jameswlane/devex/examples

### How do I report bugs?

1. Check existing issues: https://github.com/jameswlane/devex/issues
2. Gather system information: `devex system info`
3. Include steps to reproduce
4. Add relevant configuration files
5. Mention your DevEx version: `devex --version`

### How do I request features?

Start a discussion: https://github.com/jameswlane/devex/discussions
- Describe the use case
- Explain current limitations
- Suggest possible solutions
- Consider contributing if you can!

---

*Don't see your question here? Check the [full documentation](overview) or ask in our [community discussions](https://github.com/jameswlane/devex/discussions).*
