#!/usr/bin/env bash
# Install one DevEx template on a clean QEMU/KVM virtual machine.
#
# Boots a distribution cloud image on a throwaway qcow2 overlay, runs
# `devex install --template` as a normal user with only the documented
# prerequisites installed, then checks from a new login that the template's
# tools, services and containers work. Plugins come from the plugin registry, so
# the run exercises what users receive. Logs land in
# ./artifacts/vm-templates/<distro>-<template>; the VM and its disk are destroyed
# on exit.
#
# Usage: TEMPLATE=<id> [DISTRO=<distro>] scripts/e2e-template-vm.sh
#        scripts/e2e-template-vm.sh --list   print every built-in template ID
#
# Environment: TEMPLATE, plus the DISTRO and VM_* settings in lib/vm.sh.
# VM_KEEP=1 leaves the VM running after the checks for debugging.
#
# Requires qemu-system-x86_64 with KVM access (/dev/kvm), qemu-img, python3,
# ssh, scp and ssh-keygen.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/vm.sh
source "$SCRIPT_DIR/lib/vm.sh"

# A step that outlives this has hung; the guest is killed with the run.
STEP_TIMEOUT=3600

# The IDs come from the CLI, so a new template gets a job without a list to
# maintain, and fails there until template-check.sh describes it.
list_templates() {
	WORK="$(mktemp -d "${TMPDIR:-/tmp}/devex-vm.XXXXXX")"
	trap 'rm -rf "$WORK"' EXIT
	build_devex
	mkdir "$WORK/home"
	env -u XDG_CONFIG_HOME -u XDG_DATA_HOME -u XDG_STATE_HOME -u XDG_CACHE_HOME HOME="$WORK/home" \
		"$WORK/devex" template list --format json |
		python3 -c 'import json, sys; print("\n".join(sorted(json.load(sys.stdin))))'
}

collect() {
	vm_running || return 0
	log "collecting logs"
	guest "sudo journalctl -b --no-pager" >"$ARTIFACTS/journal.log" 2>&1 || true
	guest "tar -C ~ -czf - --ignore-failed-read .local/share/devex .local/state/devex .cache/devex/logs 2>/dev/null" \
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
		log "template $TEMPLATE passed on $DISTRO (artifacts: $ARTIFACTS)"
	else
		log "template $TEMPLATE failed on $DISTRO (exit $status); see $ARTIFACTS"
	fi
	exit "$status"
}

main() {
	if [[ "${1:-}" == "--list" ]]; then
		list_templates
		return
	fi
	TEMPLATE="${TEMPLATE:-}"
	[[ "$TEMPLATE" =~ ^[a-z0-9][a-z0-9-]*$ ]] || die "set TEMPLATE to a template ID (see --list), got '$TEMPLATE'"
	ARTIFACTS="$CLI_DIR/artifacts/vm-templates/$DISTRO-$TEMPLATE"
	VM_NAME="devex-template-$DISTRO-$TEMPLATE"

	select_image
	check_host
	rm -rf "$ARTIFACTS"
	mkdir -p "$ARTIFACTS"
	WORK="$(mktemp -d "${TMPDIR:-/tmp}/devex-vm.XXXXXX")"
	trap cleanup EXIT
	trap 'exit 130' INT TERM

	boot_vm
	upload "$WORK/devex" "$GUEST_DIR/template-check.sh"
	guest "cloud-init status --wait >/dev/null; sudo ln -sf /var/tmp/devex-vm-bin/devex /usr/local/bin/devex"

	guest "bash /var/tmp/devex-vm-bin/template-check.sh prepare $TEMPLATE" 2>&1 | tee "$ARTIFACTS/prepare.log" >&2

	log "installing template $TEMPLATE"
	guest "timeout $STEP_TIMEOUT bash /var/tmp/devex-vm-bin/template-check.sh install $TEMPLATE" 2>&1 |
		tee "$ARTIFACTS/install.log" >&2
	# Each SSH command is a new login, so this sees what a user sees after
	# logging back in, including new group membership and shell setup.
	log "verifying template $TEMPLATE from a new login"
	guest "timeout $STEP_TIMEOUT bash /var/tmp/devex-vm-bin/template-check.sh verify $TEMPLATE" 2>&1 |
		tee "$ARTIFACTS/verify.log" >&2
}

main "$@"
