# DevEx Yay Package Manager Plugin

Native package management for Arch Linux and compatible systems using yay.
Install yay from a trusted source before using this plugin. AUR installs and
upgrades also require `git` and the `base-devel` package; prepare these with
`sudo pacman -S --needed git base-devel` on an already synchronized system.
The plugin checks these prerequisites and reports setup guidance instead of
installing a helper or toolchain automatically.

## Operations

The standalone plugin executable accepts an operation followed by its operands:

| Operation | Native argv |
| --- | --- |
| `install <packages...>` | `yay -S --needed --noconfirm --answerdiff=None --answerclean=None <packages...>` |
| `remove <packages...>` | `yay -R --noconfirm <packages...>` |
| `update` | `yay -Syu --noconfirm` |
| `search <terms...>` | `yay -Ss <terms...>` |
| `list` | `yay -Q` |
| `is-installed <packages...>` | `yay -Q <packages...>` |

`update` synchronizes repositories and performs a **full system and AUR upgrade**.
It never runs `-Sy` alone, which would leave Arch exposed to partial upgrades.
`is-installed` preserves yay's native nonzero query status when a package is
absent. Native failures, cancellation, and authorization errors propagate to the
caller. Package arguments and search terms are passed as separate argv entries;
raw yay flags are not accepted as operands.

## User identity and authorization

All operations declare SDK **user scope**. The SDK resolves the invoking account,
including `SUDO_USER` in an elevated session, and switches to that account with
its account home before starting yay. A root session without an identifiable
non-root invoking account fails before executing any native command. A rejected
identity switch also fails; the plugin never retries yay as root.

Yay itself invokes sudo for the pacman installation/removal step. Authenticate
sudo as the regular user before unattended use, or configure an appropriate
sudo policy in your environment. `--noconfirm` does not supply a sudo password or
grant authorization. The plugin does not elevate the build helper or run
`makepkg` as root. Read-only operations and removal do not require build tools.

## AUR trust and confirmation choices

AUR PKGBUILDs and install scripts are community-supplied executable code. Review
the PKGBUILD, sources, checksums, dependencies, and install scripts yourself
before selecting a package for automation; DevEx does not perform that review.
Only automate packages and maintainers you trust, and review upstream changes
before upgrading them.

Install uses `--needed` to skip satisfied packages and `--noconfirm` to accept
default native transaction answers. `--answerdiff=None` skips the diff selection
menu, and `--answerclean=None` selects no packages for a clean build, retaining
existing build directories. These choices deliberately skip interactive review
and clean-build selection; they are not a security review or a sandbox. Remove
and update use `--noconfirm`; update otherwise retains yay's configured/default
AUR review and build behavior. Signing-key issues, conflicts, build failures,
and sudo authorization may still require manual resolution.

## Manual isolated smoke procedure

Use a disposable Arch VM with a snapshot and its own disk, package database,
network, and regular user's home. Do not mount the host home, package database,
or package cache. These commands perform real installations and full upgrades
**inside that VM**; automated tests instead use SDK recorders exclusively.

1. In the VM, fully upgrade Arch with `sudo pacman -Syu`, install `git`,
   `base-devel`, and `sudo`, and create a regular account with sudo access.
   Review and install yay from a trusted source as that account. Copy the DevEx
   checkout into the VM and build the standalone plugin there:

   ```sh
   cd packages/package-manager-yay
   mise exec -- go build -o /tmp/devex-package-manager-yay .
   ```

2. Log in as the regular user, authenticate with `sudo -v`, and choose a small
   AUR package after reviewing its PKGBUILD and scripts. Set `aur_package` to
   that reviewed package name, then exercise each operation:

   ```sh
   /tmp/devex-package-manager-yay search "$aur_package"
   /tmp/devex-package-manager-yay install "$aur_package"
   /tmp/devex-package-manager-yay is-installed "$aur_package"
   /tmp/devex-package-manager-yay list
   /tmp/devex-package-manager-yay update
   /tmp/devex-package-manager-yay remove "$aur_package"
   /tmp/devex-package-manager-yay is-installed "$aur_package" # expect nonzero
   ```

3. Repeat installation through an elevated entry point:
   `sudo /tmp/devex-package-manager-yay install "$aur_package"`. During a build,
   inspect `ps -eo user,pid,ppid,args` from a second VM terminal: yay and makepkg
   must belong to the invoking account, while pacman may run as root. Check that
   build/cache files under that user's home are user-owned, then remove the
   package again. Run `sudo env -u SUDO_USER /tmp/devex-package-manager-yay list`
   and confirm it refuses root execution.
4. In snapshot clones, remove yay, git, or base-devel in turn and verify the
   corresponding actionable setup error. Deny the regular user's sudo access
   and verify installation fails without running yay or makepkg as root.
   Restore/delete the VM and snapshots when finished.

## License

See the repository [license](../../LICENSE).
