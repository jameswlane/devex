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
CLI_DIR="$(dirname "$SCRIPT_DIR")"
GUEST_DIR="$CLI_DIR/test/vm/guest"

DISTRO="${DISTRO:-ubuntu-24.04}"
DESKTOP="${DESKTOP:-gnome}"
CACHE_DIR="${VM_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/devex-vm}"
MEMORY="${VM_MEMORY:-6144}"
CPUS="${VM_CPUS:-4}"
DISK_SIZE="${VM_DISK_SIZE:-24G}"
ARTIFACTS="$CLI_DIR/artifacts/vm/$DISTRO-$DESKTOP"
GUEST_USER=tester

log() { printf '==> %s\n' "$*" >&2; }
die() {
	printf 'e2e-vm: %s\n' "$*" >&2
	exit 1
}

case "$DESKTOP" in gnome | kde) ;; *) die "DESKTOP must be gnome or kde, got '$DESKTOP'" ;; esac

# Base images and the checksum file that covers each one.
case "$DISTRO" in
ubuntu-24.04)
	IMAGE_URL=https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
	SUMS_URL=https://cloud-images.ubuntu.com/releases/24.04/release/SHA256SUMS
	SUM_TOOL=sha256sum
	;;
debian-12)
	# The generic image, unlike genericcloud, ships the DRM drivers a display needs.
	IMAGE_URL=https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2
	SUMS_URL=https://cloud.debian.org/images/cloud/bookworm/latest/SHA512SUMS
	SUM_TOOL=sha512sum
	;;
*) die "unsupported DISTRO '$DISTRO' (available: ubuntu-24.04 debian-12)" ;;
esac

check_host() {
	local tool
	for tool in qemu-system-x86_64 qemu-img python3 ssh scp ssh-keygen curl; do
		command -v "$tool" >/dev/null 2>&1 || die "'$tool' is not on PATH"
	done
	[[ -r /dev/kvm && -w /dev/kvm ]] || die "/dev/kvm is missing or not accessible to $(id -un); add the user to the kvm group"
}

# Download the base image once and verify it against the published checksums
# on every run, so a corrupt or replaced cache entry is never booted.
fetch_image() {
	local name="${IMAGE_URL##*/}" dir="$CACHE_DIR/$DISTRO"
	mkdir -p "$dir"
	BASE_IMAGE="$dir/$name"
	curl -fsSL --retry 3 -o "$dir/SUMS" "$SUMS_URL"
	if [[ -f "$BASE_IMAGE" ]] && (cd "$dir" && grep -E "[ *]$name\$" SUMS | sed 's/ \*/  /' | "$SUM_TOOL" --check --strict --status -); then
		log "using cached $BASE_IMAGE"
		return
	fi
	log "downloading $IMAGE_URL"
	curl -fL --retry 3 -o "$BASE_IMAGE.part" "$IMAGE_URL"
	mv "$BASE_IMAGE.part" "$BASE_IMAGE"
	(cd "$dir" && grep -E "[ *]$name\$" SUMS | sed 's/ \*/  /' | "$SUM_TOOL" --check --strict -) ||
		die "checksum mismatch for $BASE_IMAGE"
}

build_devex() {
	if [[ -n "${E2E_DEVEX_BINARY:-}" ]]; then
		[[ -f "$E2E_DEVEX_BINARY" ]] || die "E2E_DEVEX_BINARY=$E2E_DEVEX_BINARY is not a file"
		cp "$E2E_DEVEX_BINARY" "$WORK/devex"
		return
	fi
	local version
	version="$(git -C "$CLI_DIR" describe --tags --match 'v[0-9]*' --dirty 2>/dev/null || echo dev)"
	log "building devex ${version#v} (linux/amd64)"
	(cd "$CLI_DIR" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
		-ldflags "-s -w -X main.version=${version#v}" -o "$WORK/devex" ./cmd/main.go)
}

free_port() {
	python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])'
}

# cloud-init NoCloud seed served over QEMU's user network (10.0.2.2 is the host).
write_seed() {
	mkdir -p "$WORK/seed"
	ssh-keygen -q -t ed25519 -N '' -C devex-vm -f "$WORK/id_ed25519"
	cat >"$WORK/seed/user-data" <<EOF
#cloud-config
users:
  - name: $GUEST_USER
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - $(cat "$WORK/id_ed25519.pub")
ssh_pwauth: false
package_update: false
EOF
	printf 'instance-id: devex-vm-%s\nlocal-hostname: devex-vm\n' "$$" >"$WORK/seed/meta-data"
	: >"$WORK/seed/vendor-data"
	SEED_PORT="$(free_port)"
	python3 -m http.server --bind 127.0.0.1 --directory "$WORK/seed" "$SEED_PORT" >"$WORK/seed.log" 2>&1 &
	SEED_PID=$!
}

start_vm() {
	qemu-img create -q -f qcow2 -F qcow2 -b "$BASE_IMAGE" "$WORK/disk.qcow2" "$DISK_SIZE"
	SSH_PORT="$(free_port)"
	log "booting $DISTRO ($CPUS CPUs, ${MEMORY} MiB, ssh on 127.0.0.1:$SSH_PORT)"
	qemu-system-x86_64 \
		-name "devex-vm-$DISTRO-$DESKTOP" \
		-machine q35,accel=kvm -cpu host -smp "$CPUS" -m "$MEMORY" \
		-drive "file=$WORK/disk.qcow2,if=virtio,format=qcow2" \
		-netdev "user,id=net0,hostfwd=tcp:127.0.0.1:$SSH_PORT-:22" -device virtio-net-pci,netdev=net0 \
		-smbios "type=1,serial=ds=nocloud-net;s=http://10.0.2.2:$SEED_PORT/" \
		-vga none -device virtio-vga,xres=1280,yres=800 -display none \
		-qmp "unix:$WORK/qmp.sock,server=on,wait=off" \
		-serial "file:$ARTIFACTS/serial.log" \
		-pidfile "$WORK/qemu.pid" -daemonize
}

SSH_OPTS=()
guest() {
	ssh "${SSH_OPTS[@]}" -p "$SSH_PORT" "$GUEST_USER@127.0.0.1" "$@"
}

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

wait_for_ssh() {
	local deadline=$((SECONDS + ${1:-600}))
	until guest true 2>/dev/null; do
		vm_running || die "the VM exited; see $ARTIFACTS/serial.log"
		((SECONDS < deadline)) || die "timed out waiting for SSH; see $ARTIFACTS/serial.log"
		sleep 5
	done
}

vm_running() {
	[[ -f "$WORK/qemu.pid" ]] && kill -0 "$(cat "$WORK/qemu.pid")" 2>/dev/null
}

# QMP commands: screendump writes a PNG of the VM's display, quit stops it.
qmp() {
	python3 - "$WORK/qmp.sock" "$@" <<'PY'
import json, socket, sys
sock = socket.socket(socket.AF_UNIX)
sock.settimeout(30)
sock.connect(sys.argv[1])
stream = sock.makefile("rw")
def call(command, **arguments):
    stream.write(json.dumps({"execute": command, "arguments": arguments}) + "\n")
    stream.flush()
    while True:
        reply = json.loads(stream.readline())
        if "return" in reply:
            return reply["return"]
        if "error" in reply:
            sys.exit(f"qmp {command}: {reply['error']['desc']}")
json.loads(stream.readline())  # greeting
call("qmp_capabilities")
if sys.argv[2] == "screendump":
    call("screendump", filename=sys.argv[3], format="png")
else:
    call(sys.argv[2])
PY
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
	if vm_running; then
		qmp quit 2>/dev/null || kill "$(cat "$WORK/qemu.pid")" 2>/dev/null
		sleep 1
	fi
	[[ -n "${SEED_PID:-}" ]] && kill "$SEED_PID" 2>/dev/null
	rm -rf "$WORK"
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

	fetch_image
	build_devex
	write_seed
	start_vm
	SSH_OPTS=(-i "$WORK/id_ed25519" -o BatchMode=yes -o ConnectTimeout=5
		-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ServerAliveInterval=15)

	wait_for_ssh 600
	log "guest is up: $(guest 'cat /etc/os-release | grep ^PRETTY_NAME=')"
	guest "mkdir -p /var/tmp/devex-vm-bin"
	scp -q -P "$SSH_PORT" "${SSH_OPTS[@]}" \
		"$WORK/devex" "$GUEST_DIR/provision.sh" "$GUEST_DIR/desktop-check.sh" "$GUEST_USER@127.0.0.1:/var/tmp/devex-vm-bin/"

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
	guest "$registry bash /var/tmp/devex-vm-bin/desktop-check.sh apply $DESKTOP" 2>&1 |
		tee "$ARTIFACTS/apply.log" >&2
	screenshot 02-session-after-setup

	log "rebooting to verify the settings survive a fresh login"
	reboot_guest
	wait_for_session
	guest "bash /var/tmp/devex-vm-bin/desktop-check.sh verify $DESKTOP" 2>&1 | tee "$ARTIFACTS/verify.log" >&2
	screenshot 03-session-after-relogin
}

main "$@"
