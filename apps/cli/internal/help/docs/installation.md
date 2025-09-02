# Installation Guide

Get DevEx up and running on your system in just a few steps.

## Quick Installation

### One-Line Installer (Recommended)

The fastest way to install DevEx on Linux or macOS:

```bash
wget -qO- https://devex.sh/install | bash
```

Or with curl:

```bash
curl -fsSL https://devex.sh/install | bash
```

This installer will:
- Download the latest DevEx binary for your platform
- Install it to `/usr/local/bin/devex` (or `~/.local/bin/devex` if no sudo)
- Set up basic configuration
- Verify the installation

### Manual Installation

#### Linux

1. **Download the binary**:
   ```bash
   wget https://github.com/jameswlane/devex/releases/latest/download/devex-linux-amd64
   ```

2. **Make it executable**:
   ```bash
   chmod +x devex-linux-amd64
   ```

3. **Move to PATH**:
   ```bash
   sudo mv devex-linux-amd64 /usr/local/bin/devex
   ```

#### macOS

1. **Download the binary**:
   ```bash
   wget https://github.com/jameswlane/devex/releases/latest/download/devex-darwin-amd64
   ```

2. **Make it executable**:
   ```bash
   chmod +x devex-darwin-amd64
   ```

3. **Move to PATH**:
   ```bash
   sudo mv devex-darwin-amd64 /usr/local/bin/devex
   ```

#### Windows

1. Download `devex-windows-amd64.exe` from the [releases page](https://github.com/jameswlane/devex/releases/latest)
2. Rename it to `devex.exe`
3. Add it to your PATH or place it in a directory already in your PATH

## Package Manager Installation

### Homebrew (macOS/Linux)

```bash
brew install jameswlane/devex/devex
```

### APT (Debian/Ubuntu)

```bash
curl -fsSL https://devex.sh/gpg | sudo apt-key add -
echo "deb https://devex.sh/apt stable main" | sudo tee /etc/apt/sources.list.d/devex.list
sudo apt update
sudo apt install devex
```

### DNF (Fedora/RHEL)

```bash
sudo dnf config-manager --add-repo https://devex.sh/rpm/devex.repo
sudo dnf install devex
```

### Snap

```bash
sudo snap install devex
```

### Flatpak

```bash
flatpak install flathub sh.devex.DevEx
```

## Build from Source

### Prerequisites

- Go 1.21 or later
- Git

### Build Steps

1. **Clone the repository**:
   ```bash
   git clone https://github.com/jameswlane/devex.git
   cd devex/apps/cli
   ```

2. **Build the binary**:
   ```bash
   go build -o devex ./cmd/main.go
   ```

3. **Install globally**:
   ```bash
   sudo mv devex /usr/local/bin/
   ```

## Post-Installation Setup

### Verify Installation

```bash
devex --version
```

### Initialize DevEx

```bash
devex init
```

This will:
- Create the `~/.devex/` directory structure
- Set up default configuration files
- Initialize the template system
- Create the cache directory

### Test Installation

```bash
devex status --all
```

This should show the status of your development environment.

## System Requirements

### Minimum Requirements
- **OS**: Linux (Ubuntu 18.04+), macOS (10.15+), or Windows 10
- **RAM**: 512 MB
- **Disk**: 100 MB for DevEx + space for applications
- **Network**: Internet connection for package downloads

### Recommended Requirements
- **OS**: Latest stable Linux distribution or macOS
- **RAM**: 2 GB or more
- **Disk**: 10 GB free space for applications and cache
- **Terminal**: Modern terminal with Unicode support

### Supported Package Managers

DevEx automatically detects and uses available package managers:

- **Linux**: apt, dnf, pacman, flatpak, snap, AppImage
- **macOS**: brew, mas (Mac App Store)
- **Windows**: winget, chocolatey, scoop
- **Universal**: mise, docker, pip, npm, cargo

## Troubleshooting Installation

### Permission Issues

If you get permission errors:

```bash
# Install to user directory instead
mkdir -p ~/.local/bin
mv devex ~/.local/bin/
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Network Issues

If downloads fail:

```bash
# Use alternative download method
curl -L -o devex https://github.com/jameswlane/devex/releases/latest/download/devex-linux-amd64
```

### PATH Issues

If `devex` is not found:

```bash
# Check if it's in PATH
which devex

# Add to PATH if needed
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Platform Issues

For unsupported platforms, try building from source or use the universal binary approach.

## Updating DevEx

### Automatic Update

```bash
devex self-update
```

### Manual Update

Follow the same installation steps with the latest version.

## Uninstalling DevEx

### Remove Binary

```bash
sudo rm /usr/local/bin/devex
# or
rm ~/.local/bin/devex
```

### Remove Configuration

```bash
rm -rf ~/.devex/
```

### Remove Cache

```bash
rm -rf ~/.cache/devex/
```

## Next Steps

- Read the [Quick Start Guide](quick-start) to begin using DevEx
- Explore [Command Reference](commands) for all available commands
- Join our [community](https://github.com/jameswlane/devex/discussions) for support

---

*Need help? Check our [Troubleshooting Guide](troubleshooting) or open an issue on GitHub.*
