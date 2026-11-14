#!/usr/bin/env bash
# Guest provisioning for scripts/e2e-vm.sh; runs as root inside the VM.
#
# Installs a GNOME or KDE Plasma desktop on a cloud image and configures the
# display manager to log USER in automatically on the next boot, so the
# suite gets a real graphical session with a user D-Bus.
#
# Usage: provision.sh gnome|kde USER

set -euo pipefail

DESKTOP="$1"
USER_NAME="$2"
export DEBIAN_FRONTEND=noninteractive
# needrestart would restart ssh.service mid-install and drop this connection.
export NEEDRESTART_SUSPEND=1 NEEDRESTART_MODE=l

log() { printf '[provision] %s\n' "$*"; }

# shellcheck source=/dev/null
. /etc/os-release
log "provisioning $DESKTOP on $PRETTY_NAME"

cloud-init status --wait >/dev/null 2>&1 || true
# Background upgrades would compete for the dpkg lock and change the image.
systemctl disable --now unattended-upgrades.service apt-daily.timer apt-daily-upgrade.timer >/dev/null 2>&1 || true
printf '%s\n' 'Acquire::Retries "5";' 'DPkg::Lock::Timeout "600";' >/etc/apt/apt.conf.d/80-devex-vm

# curl, git and unzip are the CLI's startup dependencies; fontconfig verifies fonts.
packages=(curl git unzip fontconfig python3)
case "$DESKTOP:$ID" in
gnome:ubuntu) packages+=(ubuntu-desktop-minimal) ;;
gnome:debian) packages+=(gnome-core) ;;
kde:*) packages+=(kde-plasma-desktop sddm xorg libkf5config-bin) ;;
*) log "unsupported desktop $DESKTOP on $ID" && exit 2 ;;
esac

apt-get update
apt-get install -y "${packages[@]}"

home="$(getent passwd "$USER_NAME" | cut -d: -f6)"
case "$DESKTOP" in
gnome)
	# Ubuntu reads custom.conf, Debian daemon.conf.
	for conf in /etc/gdm3/custom.conf /etc/gdm3/daemon.conf; do
		[[ -e "$conf" ]] || continue
		python3 - "$conf" "$USER_NAME" <<'PY'
import configparser, sys
path, user = sys.argv[1], sys.argv[2]
config = configparser.ConfigParser(strict=False, interpolation=None)
config.optionxform = str
config.read(path)
if not config.has_section("daemon"):
    config.add_section("daemon")
config["daemon"]["AutomaticLoginEnable"] = "true"
config["daemon"]["AutomaticLogin"] = user
with open(path, "w") as handle:
    config.write(handle)
PY
	done
	# Skip the first-login welcome wizard so the session reaches the desktop.
	install -d -o "$USER_NAME" -g "$USER_NAME" "$home/.config"
	echo yes | install -o "$USER_NAME" -g "$USER_NAME" -m 0644 /dev/stdin "$home/.config/gnome-initial-setup-done"
	# Keep the screen on and unlocked for screenshots. These are system
	# defaults the DevEx plan does not touch, so user values still come from it.
	install -d /etc/dconf/profile /etc/dconf/db/local.d
	printf 'user-db:user\nsystem-db:local\n' >/etc/dconf/profile/user
	cat >/etc/dconf/db/local.d/00-devex-vm <<'EOF'
[org/gnome/desktop/session]
idle-delay=uint32 0

[org/gnome/desktop/screensaver]
lock-enabled=false
EOF
	dconf update
	dm=gdm
	;;
kde)
	install -d /etc/sddm.conf.d
	printf '[Autologin]\nUser=%s\nSession=plasma\n' "$USER_NAME" >/etc/sddm.conf.d/devex-vm.conf
	install -d /etc/xdg
	printf '[Daemon]\nAutolock=false\nLockOnResume=false\nTimeout=0\n' >/etc/xdg/kscreenlockerrc
	dm=sddm
	;;
esac

# A server image boots to multi-user.target; switch to the desktop.
systemctl set-default graphical.target
systemctl enable "$dm" >/dev/null 2>&1 || systemctl enable "${dm}3" >/dev/null 2>&1 || true
log "desktop installed; $USER_NAME logs in automatically after reboot"
