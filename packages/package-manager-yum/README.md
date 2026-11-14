# DevEx YUM Package Manager Plugin

YUM package manager plugin for DevEx. It serves apps whose `install_method` is
`yum`, and is the required package manager plugin on CentOS.

## Supported systems

| System | Backend used |
| --- | --- |
| RHEL / CentOS 7 | native YUM 3 (`yum`) |
| RHEL 8+, CentOS Stream, Rocky Linux, AlmaLinux, Oracle Linux | the `yum` alias that DNF ships (`/usr/bin/yum -> dnf`) |
| Any of the above without a `yum` executable but with `dnf` | `dnf`, which accepts the same commands |

The backend is resolved on every run: `yum` if it is on `PATH`, otherwise
`dnf`. If neither exists the plugin stops with an error naming the supported
systems and runs nothing. Other distributions are not supported; use
`package-manager-dnf` on Fedora.

## Commands

| Command | Native command | Elevated |
| --- | --- | --- |
| `install <pkg>...` | `<backend> install --assumeyes <pkg>...` | yes |
| `remove <pkg>...` | `rpm -q` / `rpm -q --whatprovides` check per package, then `<backend> remove --assumeyes <pkg>...` | removal only |
| `update` | `<backend> makecache` | yes |
| `upgrade [pkg]...` | `<backend> upgrade --assumeyes [pkg]...` | yes |
| `search <term>` | `<backend> search <term>` | no |
| `list` | `<backend> list installed` on YUM 3, `<backend> list --installed` on DNF | no |
| `is-installed <pkg>...` | `rpm -q -- <pkg>` per package | no |
| `add-repository <https://...repo>` | `yum-config-manager --add-repo <url>` | yes |

- `--assumeyes` keeps every transaction noninteractive. With
  `--non-interactive` (or `CI=true`), elevation uses `sudo -n` through the
  plugin SDK, so a missing sudo credential fails instead of prompting.
- Native stderr is passed through and the native exit status is returned
  unchanged.
- `remove` fails before starting a transaction when a package matches
  nothing installed, because YUM can exit 0 in that case.
- `add-repository` needs `yum-config-manager` from `yum-utils` (RHEL/CentOS 7)
  or `dnf-plugins-core` (RHEL 8+); the plugin says so when it is missing.

## Disposable smoke procedure

Run these in throwaway containers; nothing touches the host.

```bash
# Build the plugin for Linux from the repository root
(cd packages/package-manager-yum && CGO_ENABLED=0 go build -o /tmp/yum-plugin .)

# RHEL 8+ family: the yum alias backed by DNF
docker run --rm -v /tmp/yum-plugin:/usr/local/bin/devex-plugin-package-manager-yum:ro \
  rockylinux:9 sh -c '
    devex-plugin-package-manager-yum update &&
    devex-plugin-package-manager-yum install tree &&
    devex-plugin-package-manager-yum is-installed tree &&
    devex-plugin-package-manager-yum list | grep -q "^tree" &&
    devex-plugin-package-manager-yum remove tree &&
    ! devex-plugin-package-manager-yum is-installed tree'

# Legacy YUM 3: CentOS 7 is end of life, so point it at the vault mirror first
docker run --rm -v /tmp/yum-plugin:/usr/local/bin/devex-plugin-package-manager-yum:ro \
  centos:7 sh -c '
    sed -i -e "s/^mirrorlist/#mirrorlist/" \
      -e "s|^#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|" \
      /etc/yum.repos.d/CentOS-*.repo &&
    devex-plugin-package-manager-yum install tree &&
    devex-plugin-package-manager-yum list | grep -q "^tree" &&
    devex-plugin-package-manager-yum remove tree'
```

The containers run as root, so no `sudo` is involved.
