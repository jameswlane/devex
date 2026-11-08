# Font package mappings

Both desktop plugins keep a small local catalog rather than introducing desktop
font policy into the generic plugin SDK. Keep their catalogs and contracts aligned.

Repository names checked on 2026-11-08. APT targets the common package names in
Ubuntu 24.04 (Noble) and Debian 13 (Trixie); Ubuntu universe and Debian non-free
may need enabling. Fedora mappings target Fedora repositories, not arbitrary RHEL
or EPEL installations. Pacman targets Arch Extra; zypper targets openSUSE Tumbleweed.

| Font alias | apt-get (Ubuntu / Debian) | dnf (Fedora) | pacman (Arch Extra) | zypper (Tumbleweed) |
| --- | --- | --- | --- | --- |
| firacode | `fonts-firacode` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-firacode), [Debian](https://packages.debian.org/trixie/fonts-firacode)) | [`fira-code-fonts`](https://packages.fedoraproject.org/pkgs/fira-code-fonts/fira-code-fonts/) | [`ttf-fira-code`](https://archlinux.org/packages/extra/any/ttf-fira-code/) | [`fira-code-fonts`](https://software.opensuse.org/package/fira-code-fonts) † |
| jetbrainsmono | `fonts-jetbrains-mono` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-jetbrains-mono), [Debian](https://packages.debian.org/trixie/fonts-jetbrains-mono)) | [`jetbrains-mono-fonts`](https://packages.fedoraproject.org/pkgs/jetbrains-mono-fonts/jetbrains-mono-fonts/) | [`ttf-jetbrains-mono`](https://archlinux.org/packages/extra/any/ttf-jetbrains-mono/) | [`jetbrains-mono-fonts`](https://software.opensuse.org/package/jetbrains-mono-fonts) † |
| hack | `fonts-hack` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-hack), [Debian](https://packages.debian.org/trixie/fonts-hack)) | [`source-foundry-hack-fonts`](https://packages.fedoraproject.org/pkgs/source-foundry-hack-fonts/source-foundry-hack-fonts/) | [`ttf-hack`](https://archlinux.org/packages/extra/any/ttf-hack/) | [`hack-fonts`](https://software.opensuse.org/package/hack-fonts) † |
| sourcecodepro | No standalone package verified | [`adobe-source-code-pro-fonts`](https://packages.fedoraproject.org/pkgs/adobe-source-code-pro-fonts/adobe-source-code-pro-fonts/) | [`adobe-source-code-pro-fonts`](https://archlinux.org/packages/extra/any/adobe-source-code-pro-fonts/) | [`adobe-sourcecodepro-fonts`](https://software.opensuse.org/package/adobe-sourcecodepro-fonts) † |
| inconsolata | `fonts-inconsolata` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-inconsolata), [Debian](https://packages.debian.org/trixie/fonts-inconsolata)) | [`levien-inconsolata-fonts`](https://packages.fedoraproject.org/pkgs/levien-inconsolata-fonts/levien-inconsolata-fonts/) | [`ttf-inconsolata`](https://archlinux.org/packages/extra/any/ttf-inconsolata/) | [`google-inconsolata-fonts`](https://software.opensuse.org/package/google-inconsolata-fonts) † |
| cascadia | `fonts-cascadia-code` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-cascadia-code), [Debian](https://packages.debian.org/trixie/fonts-cascadia-code)) | [`cascadia-code-fonts`](https://packages.fedoraproject.org/pkgs/cascadia-code-fonts/cascadia-code-fonts/) | [`ttf-cascadia-code`](https://archlinux.org/packages/extra/any/ttf-cascadia-code/) | [`saja-cascadia-code-fonts`](https://software.opensuse.org/package/saja-cascadia-code-fonts) † |
| ubuntu | `fonts-ubuntu` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-ubuntu), [Debian](https://packages.debian.org/trixie/fonts-ubuntu)) | No Fedora package verified | [`ttf-ubuntu-font-family`](https://archlinux.org/packages/extra/any/ttf-ubuntu-font-family/) | [`ubuntu-fonts`](https://software.opensuse.org/package/ubuntu-fonts) † |
| liberation | `fonts-liberation` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-liberation), [Debian](https://packages.debian.org/trixie/fonts-liberation)) | [`liberation-fonts-all`](https://packages.fedoraproject.org/pkgs/liberation-fonts/liberation-fonts-all/) | [`ttf-liberation`](https://archlinux.org/packages/extra/any/ttf-liberation/) | [`liberation-fonts`](https://software.opensuse.org/package/liberation-fonts) † |
| noto | `fonts-noto` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-noto), [Debian](https://packages.debian.org/trixie/fonts-noto)) | [`google-noto-sans-fonts`](https://packages.fedoraproject.org/pkgs/google-noto-fonts/google-noto-sans-fonts/) | [`noto-fonts`](https://archlinux.org/packages/extra/any/noto-fonts/) | [`google-noto-sans-fonts`](https://software.opensuse.org/package/google-noto-sans-fonts) † |
| roboto | `fonts-roboto` ([Ubuntu](https://packages.ubuntu.com/noble/fonts-roboto), [Debian](https://packages.debian.org/trixie/fonts-roboto)) | [`google-roboto-fonts`](https://packages.fedoraproject.org/pkgs/google-roboto-fonts/google-roboto-fonts/) | [`ttf-roboto`](https://archlinux.org/packages/extra/any/ttf-roboto/) | [`google-roboto-fonts`](https://software.opensuse.org/package/google-roboto-fonts) † |
| oxygen | No common package (absent in Debian Trixie) | [`oxygen-fonts`](https://packages.fedoraproject.org/pkgs/oxygen-fonts/oxygen-fonts/) | No Arch Extra package verified | [`kde-oxygen-fonts`](https://software.opensuse.org/package/kde-oxygen-fonts) † |

† software.opensuse.org returned HTTP 403 for package pages in this environment.
All listed zypper binary names were instead verified in the official
[Tumbleweed OSS repository metadata](https://download.opensuse.org/tumbleweed/repo/oss/repodata/repomd.xml)
(primary XML referenced by repomd.xml). The software.opensuse.org page contents
remain unverified. In particular, Inconsolata is `google-inconsolata-fonts`,
Cascadia is `saja-cascadia-code-fonts`, and Oxygen is `kde-oxygen-fonts`.

APT omits Source Code Pro and Oxygen from portable defaults. Oxygen is available
as [fonts-oxygen on Ubuntu](https://packages.ubuntu.com/noble/fonts-oxygen),
but [Debian Trixie has no fonts-oxygen package](https://packages.debian.org/trixie/fonts-oxygen).
Fedora has no verified Ubuntu font package; Arch Extra has no verified Oxygen
font package. No third-party repositories or AUR helpers are added automatically.

Unavailable defaults produce a warning on stderr and are skipped. Explicitly
requesting an unavailable alias fails before installing any packages. Unrecognized
names are treated as explicit native package operands; their availability is the
caller's responsibility. Aliases are case insensitive and accept spaces/hyphens,
including `Fira Code`, `JetBrains Mono`, `Source Code Pro`, `Cascadia Code`, and
`jetbrains`. Historical Debian `fonts-*` operands also resolve through the catalog.
Duplicate resolved packages are installed once, preserving request order.

## Exact font install preview

`describe` without arguments continues to return the configure settings plan used
by setup (#271). `describe install-fonts [font...]` returns a JSON array of the
exact ordered native commands for installing those fonts, including available
font-cache refresh tools. It runs without a desktop session or privilege elevation.
A supported package manager must be available to resolve distribution packages.

Hash the compact JSON array (excluding the output newline) with SHA-256 and pass
`install-fonts --expected-plan HASH [font...]` to require that exact plan before
any native command executes. Changes in manager, resolved package set, or available
cache tools reject the accepted plan. Package installation uses system scope;
cache refreshes use user scope and remain best effort. Warnings go to stderr.
