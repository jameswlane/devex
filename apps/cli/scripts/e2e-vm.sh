#!/usr/bin/env bash
# Run DevEx desktop setup in a disposable QEMU/KVM virtual machine.
#
# Boots a distribution cloud image on a throwaway qcow2 overlay, installs a
# GNOME or KDE Plasma desktop, logs the test user into a real graphical
# session, then runs the DevEx desktop plugin inside that session's D-Bus and
# display environment and checks settings, themes, fonts and (GNOME)
# extensions, including after a fresh login. Screenshots, logs and desktop
# state land in ./artifacts/vm/<distro>-<desktop>; the VM and its disk are
# destroyed on exit.
#
# Usage: scripts/e2e-vm.sh
#
# Environment:
#   DISTRO              ubuntu-24.04 (default) | debian-12
#   DESKTOP             gnome (default) | kde
#   VM_CACHE_DIR        downloaded base images (default: ${XDG_CACHE_HOME:-~/.cache}/devex-vm)
#   VM_MEMORY           guest memory in MiB (default: 6144)
#   VM_CPUS             guest CPUs (default: 4)
#   VM_DISK_SIZE        overlay disk size (default: 24G)
#   VM_KEEP=1           leave the VM running after the checks for debugging;
#                       the script prints how to reach it and waits for Ctrl-C
#   E2E_DEVEX_BINARY    prebuilt linux devex binary instead of building one
#   DEVEX_PLUGIN_REGISTRY_URL  plugin registry the guest uses (default: the CLI's)
#
# Requires qemu-system-x86_64 with KVM access (/dev/kvm), qemu-img, python3,
# ssh, scp and ssh-keygen. Desktop plugins come from the plugin registry, so
# the suite exercises the plugins users receive.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/vm.sh
source "$SCRIPT_DIR/lib/vm.sh"

DESKTOP="${DESKTOP:-gnome}"
ARTIFACTS="$CLI_DIR/artifacts/vm/$DISTRO-$DESKTOP"
VM_NAME="devex-vm-$DISTRO-$DESKTOP"

case "$DESKTOP" in gnome | kde) ;; *) die "DESKTOP must be gnome or kde, got '$DESKTOP'" ;; esac
select_image

# Reboot and wait for the new boot, so a stale SSH session is never mistaken
# for the rebooted guest.
reboot_guest() {
	local boot_id
	boot_id="$(guest cat /proc/sys/kernel/random/boot_id)"
	guest "sudo systemctl reboot" || true
	local deadline=$((SECONDS + 600))
	until [[ "$(guest cat /proc/sys/kernel/random/boot_id 2>/dev/null)" =~ ^[0-9a-f-]+$ && "$(guest cat /proc/sys/kernel/random/boot_id 2>/dev/null)" != "$boot_id" ]]; do
		vm_running || die "the VM exited during reboot; see $ARTIFACTS/serial.log"
		((SECONDS < deadline)) || die "timed out waiting for the guest to reboot"
		sleep 5
	done
}

screenshot() {
	if qmp screendump "$ARTIFACTS/$1.png"; then
		log "screenshot $ARTIFACTS/$1.png"
	else
		log "could not capture screenshot $1"
	fi
}

# A graphical session is ready once its shell process runs for the test user.
wait_for_session() {
	local process=gnome-shell deadline=$((SECONDS + 600))
	[[ "$DESKTOP" == kde ]] && process=plasmashell
	until guest "pgrep -u $GUEST_USER -x $process >/dev/null" 2>/dev/null; do
		vm_running || die "the VM exited; see $ARTIFACTS/serial.log"
		((SECONDS < deadline)) || {
			screenshot no-session
			die "timed out waiting for $process in a graphical session"
		}
		sleep 5
	done
	# Let the shell finish startup (extensions, panels) before acting on it.
	sleep 20
}

collect() {
	vm_running || return 0
	log "collecting logs and desktop state"
	guest "sudo journalctl -b --no-pager" >"$ARTIFACTS/journal.log" 2>&1 || true
	guest "loginctl list-sessions --no-legend; loginctl show-user $GUEST_USER" >"$ARTIFACTS/sessions.txt" 2>&1 || true
	guest "sudo tar -C /var/tmp -czf - devex-vm 2>/dev/null" >"$ARTIFACTS/guest-results.tar.gz" 2>/dev/null || true
	guest "tar -C ~ -czf - --ignore-failed-read .local/share/devex .local/state/devex .cache/devex/logs .config/dconf .config/kdeglobals .config/kwinrc .config/plasmarc 2>/dev/null" \
		>"$ARTIFACTS/guest-state.tar.gz" 2>/dev/null || true
}

cleanup() {
	local status=$?
	set +e
	if [[ "${VM_KEEP:-0}" == "1" ]] && vm_running; then
		log "VM_KEEP=1: VM left running. Connect with:"
		printf '  ssh -i %q -p %s -o StrictHostKeyChecking=no %s@127.0.0.1\n' "$WORK/id_ed25519" "$SSH_PORT" "$GUEST_USER" >&2
		log "press Ctrl-C to destroy it"
		# A trapped SIGINT interrupts wait, so cleanup continues below.
		trap : INT
		sleep infinity &
		wait $! 2>/dev/null
		kill $! 2>/dev/null
	fi
	collect
	stop_vm
	if ((status == 0)); then
		log "desktop VM suite passed on $DISTRO/$DESKTOP (artifacts: $ARTIFACTS)"
	else
		log "desktop VM suite failed on $DISTRO/$DESKTOP (exit $status); see $ARTIFACTS"
	fi
	exit "$status"
}

main() {
	check_host
	rm -rf "$ARTIFACTS"
	mkdir -p "$ARTIFACTS"
	WORK="$(mktemp -d "${TMPDIR:-/tmp}/devex-vm.XXXXXX")"
	trap cleanup EXIT
	trap 'exit 130' INT TERM

	boot_vm
	upload "$WORK/devex" "$GUEST_DIR/provision.sh" "$GUEST_DIR/desktop-check.sh"

	log "installing the $DESKTOP desktop (this takes several minutes)"
	guest "sudo bash /var/tmp/devex-vm-bin/provision.sh $DESKTOP $GUEST_USER" 2>&1 | tee "$ARTIFACTS/provision.log" >&2
	reboot_guest
	wait_for_session
	screenshot 01-session-before

	local registry=""
	if [[ -n "${DEVEX_PLUGIN_REGISTRY_URL:-}" ]]; then
		registry="DEVEX_PLUGIN_REGISTRY_URL=$(printf '%q' "$DEVEX_PLUGIN_REGISTRY_URL")"
	fi
	log "running DevEx desktop setup inside the $DESKTOP session"
	guest "DEVEX_REGISTRY_CI=1 $registry bash /var/tmp/devex-vm-bin/desktop-check.sh apply $DESKTOP" 2>&1 |
		tee "$ARTIFACTS/apply.log" >&2
	screenshot 02-session-after-setup

	log "rebooting to verify the settings survive a fresh login"
	reboot_guest
	wait_for_session
	guest "DEVEX_REGISTRY_CI=1 bash /var/tmp/devex-vm-bin/desktop-check.sh verify $DESKTOP" 2>&1 | tee "$ARTIFACTS/verify.log" >&2
	screenshot 03-session-after-relogin
}

main "$@"
