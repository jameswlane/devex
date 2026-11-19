# shellcheck shell=bash
# Disposable QEMU/KVM virtual machines for the VM test suites. Source this from
# a runner that sets ARTIFACTS, VM_NAME and WORK (see boot_vm).
#
# Environment:
#   DISTRO              ubuntu-24.04 (default) | debian-12
#   VM_CACHE_DIR        downloaded base images (default: ${XDG_CACHE_HOME:-~/.cache}/devex-vm)
#   VM_MEMORY           guest memory in MiB (default: 6144)
#   VM_CPUS             guest CPUs (default: 4)
#   VM_DISK_SIZE        overlay disk size (default: 24G)
#   E2E_DEVEX_BINARY    prebuilt linux devex binary instead of building one

VM_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="$(dirname "$(dirname "$VM_LIB_DIR")")"
# shellcheck disable=SC2034 # used by the runners that source this file
GUEST_DIR="$CLI_DIR/test/vm/guest"

DISTRO="${DISTRO:-ubuntu-24.04}"
CACHE_DIR="${VM_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/devex-vm}"
MEMORY="${VM_MEMORY:-6144}"
CPUS="${VM_CPUS:-4}"
DISK_SIZE="${VM_DISK_SIZE:-24G}"
GUEST_USER=tester
SSH_OPTS=()

log() { printf '==> %s\n' "$*" >&2; }
die() {
	printf 'e2e-vm: %s\n' "$*" >&2
	exit 1
}

# Base images and the checksum file that covers each one.
select_image() {
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
}

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
	# qemu-img resolves a relative backing file against the overlay's directory.
	dir="$(cd "$dir" && pwd)"
	BASE_IMAGE="$dir/$name"
	# Runs can share the cache, so each one downloads to its own temporary name.
	curl -fsSL --retry 3 -o "$dir/SUMS.$$" "$SUMS_URL"
	mv "$dir/SUMS.$$" "$dir/SUMS"
	if [[ -f "$BASE_IMAGE" ]] && (cd "$dir" && grep -E "[ *]$name\$" SUMS | sed 's/ \*/  /' | "$SUM_TOOL" --check --strict --status -); then
		log "using cached $BASE_IMAGE"
		return
	fi
	log "downloading $IMAGE_URL"
	curl -fL --retry 3 -o "$BASE_IMAGE.part.$$" "$IMAGE_URL"
	mv "$BASE_IMAGE.part.$$" "$BASE_IMAGE"
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
		-name "$VM_NAME" \
		-machine q35,accel=kvm -cpu host -smp "$CPUS" -m "$MEMORY" \
		-drive "file=$WORK/disk.qcow2,if=virtio,format=qcow2" \
		-netdev "user,id=net0,hostfwd=tcp:127.0.0.1:$SSH_PORT-:22" -device virtio-net-pci,netdev=net0 \
		-smbios "type=1,serial=ds=nocloud-net;s=http://10.0.2.2:$SEED_PORT/" \
		-vga none -device virtio-vga,xres=1280,yres=800 -display none \
		-qmp "unix:$WORK/qmp.sock,server=on,wait=off" \
		-serial "file:$ARTIFACTS/serial.log" \
		-pidfile "$WORK/qemu.pid" -daemonize
}

guest() {
	ssh "${SSH_OPTS[@]}" -p "$SSH_PORT" "$GUEST_USER@127.0.0.1" "$@"
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

stop_vm() {
	if vm_running; then
		qmp quit 2>/dev/null || kill "$(cat "$WORK/qemu.pid")" 2>/dev/null
		sleep 1
	fi
	[[ -n "${SEED_PID:-}" ]] && kill "$SEED_PID" 2>/dev/null
	rm -rf "$WORK"
}

# Boot a fresh VM from the verified base image and wait until SSH answers.
boot_vm() {
	fetch_image
	build_devex
	write_seed
	start_vm
	SSH_OPTS=(-i "$WORK/id_ed25519" -o BatchMode=yes -o ConnectTimeout=5
		-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ServerAliveInterval=15)
	wait_for_ssh 600
	log "guest is up: $(guest 'cat /etc/os-release | grep ^PRETTY_NAME=')"
}

# Copy files into /var/tmp/devex-vm-bin on the guest.
upload() {
	guest "mkdir -p /var/tmp/devex-vm-bin"
	scp -q -P "$SSH_PORT" "${SSH_OPTS[@]}" "$@" "$GUEST_USER@127.0.0.1:/var/tmp/devex-vm-bin/"
}
