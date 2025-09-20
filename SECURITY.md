# Security Policy

## Overview

DevEx takes security seriously. As a development environment setup tool that can install software and modify system configurations, we implement security best practices to protect users and their systems.

## Supported Versions

We provide security updates for the following versions:

| Version | Supported          | Notes |
| ------- | ------------------ | ----- |
| 1.x.x   | :white_check_mark: | Current stable release |
| 0.x.x   | :white_check_mark: | Pre-release versions (limited support) |

## Security Features

### Built-in Security Measures

- **Checksum Verification**: All downloads are verified using SHA256 checksums
- **Dry-run Mode**: Preview all changes before execution with `--dry-run`
- **No Arbitrary Code Execution**: Controlled installation through predefined configurations
- **Source Verification**: All package sources use verified repositories and signing keys
- **Minimal Privileges**: Requests sudo only when necessary for system modifications

### Installation Security

- **Signed Releases**: All GitHub releases are signed and include checksums
- **Repository Verification**: APT sources include proper GPG key verification
- **HTTPS Only**: All downloads use HTTPS with certificate verification
- **Configuration Validation**: All YAML configurations are validated before execution

## Reporting a Vulnerability

If you discover a security vulnerability in DevEx, please report it responsibly:

### How to Report

**For critical vulnerabilities:**
- Email: security@devex.sh (or james.w.lane@mac.com if security email is unavailable)
- Subject: "[SECURITY] Critical vulnerability in DevEx"

**For non-critical security issues:**
- Create a private GitHub security advisory at: https://github.com/jameswlane/devex/security/advisories
- Or email: security@devex.sh

### What to Include

Please include the following information:
- **Description**: Clear description of the vulnerability
- **Steps to reproduce**: Detailed steps to reproduce the issue
- **Impact**: What systems/data could be affected
- **Environment**: OS, DevEx version, configuration details
- **Proof of concept**: If applicable (but please don't include actual exploits)

### Response Timeline

- **Initial response**: Within 48 hours
- **Confirmation**: Within 7 days
- **Fix timeline**: Depends on severity
  - Critical: 7-14 days
  - High: 14-30 days
  - Medium/Low: 30-90 days

### Security Advisory Process

1. **Report received**: We acknowledge receipt and begin investigation
2. **Assessment**: We assess the impact and severity
3. **Fix development**: We develop and test a fix
4. **Coordinated disclosure**: We coordinate release timing with the reporter
5. **Public disclosure**: We publish a security advisory and release the fix
6. **Credit**: We credit the reporter (unless they prefer to remain anonymous)

## Security Best Practices for Users

### Safe Usage

- **Review configurations**: Always review YAML configurations before installation
- **Use dry-run**: Test installations with `--dry-run` before applying changes
- **Backup systems**: Create system backups before running DevEx
- **Verify sources**: Only use trusted configuration sources
- **Keep updated**: Use the latest version of DevEx

### Configuration Security

- **No secrets in configs**: Never store passwords, API keys, or secrets in configuration files
- **Review dependencies**: Understand what applications you're installing
- **Validate URLs**: Ensure download URLs are from trusted sources
- **Check permissions**: Review file permissions and ownership changes

### Enterprise Considerations

- **Network policies**: Configure appropriate firewall rules for package downloads
- **Proxy support**: Use corporate proxies for internet access
- **Audit logging**: Enable system audit logging for installation tracking
- **Access control**: Limit who can modify DevEx configurations
- **Change management**: Follow change management processes for system modifications

## Known Security Considerations

### By Design Limitations

- **Requires sudo**: Some operations require administrative privileges
- **Package installation**: Installs packages from external repositories
- **System modification**: Modifies system configurations and user environments
- **External downloads**: Downloads software from internet sources

### Mitigation Strategies

- All external sources are verified through checksums and signatures
- Configurations are validated before execution
- Dry-run mode allows preview of all changes
- Comprehensive logging of all operations

## Security Updates

Security updates are released as soon as possible after discovery and verification. Users are notified through:

- **GitHub Security Advisories**: https://github.com/jameswlane/devex/security/advisories
- **Release Notes**: Detailed security fix information in release notes
- **Website**: Security notifications at https://devex.sh/security

## Contact

- **General Security**: security@devex.sh (or james.w.lane@mac.com)
- **Emergency**: For critical vulnerabilities requiring immediate attention
- **PGP Key**: Available upon request for encrypted communications

## External Dependencies

DevEx relies on external package managers and repositories. Users should also monitor security updates for:

- Operating system package managers (APT, DNF, etc.)
- Programming language managers (mise, npm, pip)
- Third-party repositories and sources

---

*This security policy is reviewed and updated regularly. Last updated: January 2025*