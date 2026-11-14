#!/usr/bin/env bash
# In-session desktop checks for scripts/e2e-vm.sh; runs as the test user.
#
# Joins the running graphical session (its D-Bus, display and desktop
# variables come from the shell process), then:
#   apply   installs the desktop plugin, applies the previewed settings plan,
#           a theme, fonts and (GNOME) an extension, and verifies them
#   verify  re-checks the recorded expectations after a fresh login
#
# Usage: desktop-check.sh apply|verify gnome|kde
#
# Results are written to /var/tmp/devex-vm for the host to collect.

set -euo pipefail

MODE="$1"
DESKTOP="$2"
DEVEX=/var/tmp/devex-vm-bin/devex
STATE=/var/tmp/devex-vm
EXTENSION=caffeine@patapon.info
MONO_FONT="JetBrains Mono"
FAILURES=0

sudo install -d -o "$(id -un)" -m 0755 "$STATE"

log() { printf '[check] %s\n' "$*"; }
pass() { printf '[pass] %s\n' "$*" | tee -a "$STATE/results-$MODE.txt"; }
fail() {
	printf '[FAIL] %s\n' "$*" | tee -a "$STATE/results-$MODE.txt"
	FAILURES=$((FAILURES + 1))
}
expect_eq() { # description expected actual
	if [[ "$2" == "$3" ]]; then pass "$1 = $3"; else fail "$1: expected '$2', got '$3'"; fi
}
: >"$STATE/results-$MODE.txt"

# Adopt the environment of the session's shell, so every command below talks
# to the same user bus and display that the logged-in desktop uses.
join_session() {
	local process=gnome-shell pid
	[[ "$DESKTOP" == kde ]] && process=plasmashell
	pid="$(pgrep -u "$(id -u)" -o -x "$process")" || {
		fail "no $process process for $(id -un); no graphical session"
		exit 1
	}
	local name value
	while IFS='=' read -r -d '' name value; do
		case "$name" in
		DISPLAY | WAYLAND_DISPLAY | XAUTHORITY | DBUS_SESSION_BUS_ADDRESS | XDG_RUNTIME_DIR | \
			XDG_CURRENT_DESKTOP | XDG_SESSION_TYPE | XDG_SESSION_DESKTOP | DESKTOP_SESSION | XDG_DATA_DIRS | XDG_CONFIG_DIRS)
			export "$name=$value"
			;;
		esac
	done <"/proc/$pid/environ"
	# A Wayland compositor's own environment lacks WAYLAND_DISPLAY; the session
	# exports its display variables to the systemd user manager instead.
	while IFS='=' read -r name value; do
		case "$name" in
		DISPLAY | WAYLAND_DISPLAY | XAUTHORITY | DBUS_SESSION_BUS_ADDRESS)
			[[ -n "$value" ]] && export "$name=$value"
			;;
		esac
	done < <(systemctl --user show-environment 2>/dev/null || true)
	log "joined $process (pid $pid): XDG_CURRENT_DESKTOP=${XDG_CURRENT_DESKTOP:-} XDG_SESSION_TYPE=${XDG_SESSION_TYPE:-} DISPLAY=${DISPLAY:-} WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-}"
	[[ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ]] || fail "session has no DBUS_SESSION_BUS_ADDRESS"
	[[ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]] || fail "session has neither DISPLAY nor WAYLAND_DISPLAY"
	local session
	session="$(loginctl show-user "$(id -un)" -p Display --value 2>/dev/null || true)"
	if [[ -n "$session" ]]; then
		pass "logind graphical session $session ($(loginctl show-session "$session" -p Type --value 2>/dev/null))"
	else
		fail "logind reports no graphical session for $(id -un)"
	fi
}

# SSH without a TTY keeps DevEx non-interactive. The global --non-interactive
# flag is not passed because `plugin run` would read it as the plugin name.
devex() {
	"$DEVEX" "$@" </dev/null
}

normalize_gsetting() { # strip GVariant string quoting
	local value="$1"
	value="${value#\'}"
	printf '%s' "${value%\'}"
}

# Every command in the previewed plan must be visible through the session.
verify_plan() {
	# Each plan command becomes five NUL-separated fields: tool, schema or
	# file, group, key, value.
	local tool target group key value fields="$STATE/plan-fields-$MODE"
	# Parse first, so a missing or invalid plan fails instead of checking nothing.
	if ! python3 -c '
import json, sys
plan = json.load(open(sys.argv[1]))
assert isinstance(plan, list) and plan, "empty plan"
for c in plan:
    if c[:2] == ["gsettings", "set"] and len(c) == 5:
        fields = ["gsettings", c[2], "", c[3], c[4]]
    elif c[0] == "kwriteconfig5" and len(c) == 8 and c[1:7:2] == ["--file", "--group", "--key"]:
        fields = ["kwriteconfig5", c[2], c[4], c[6], c[7]]
    else:
        fields = ["unknown", " ".join(c), "", "", ""]
    sys.stdout.write("".join(f + "\0" for f in fields))
' "$STATE/plan.json" >"$fields"; then
		fail "could not read the settings plan $STATE/plan.json"
		return
	fi
	while IFS= read -r -d '' tool && IFS= read -r -d '' target && IFS= read -r -d '' group &&
		IFS= read -r -d '' key && IFS= read -r -d '' value; do
		case "$tool" in
		gsettings)
			expect_eq "gsettings $target $key" "$value" \
				"$(normalize_gsetting "$(gsettings get "$target" "$key" 2>&1)")"
			;;
		kwriteconfig5)
			expect_eq "kreadconfig5 $target [$group] $key" "$value" \
				"$(kreadconfig5 --file "$target" --group "$group" --key "$key" 2>&1)"
			;;
		*) fail "unexpected plan command: $tool $target $group $key $value" ;;
		esac
	done <"$fields"
}

verify_desktop() {
	verify_plan
	# grep -q on a pipe would SIGPIPE fc-list and fail the pipeline.
	if grep -qi "^$MONO_FONT" <<<"$(fc-list : family)"; then
		pass "font $MONO_FONT is installed"
	else
		fail "font $MONO_FONT is not in fc-list"
	fi
	case "$DESKTOP" in
	gnome)
		expect_eq "gtk-theme" Adwaita-dark "$(normalize_gsetting "$(gsettings get org.gnome.desktop.interface gtk-theme)")"
		expect_eq "monospace-font-name" "$MONO_FONT 11" "$(normalize_gsetting "$(gsettings get org.gnome.desktop.interface monospace-font-name)")"
		if [[ "$MODE" == apply && -e "$STATE/extensions-pending" ]]; then
			if [[ -d "$HOME/.local/share/gnome-shell/extensions/$EXTENSION" ]]; then
				pass "extension $EXTENSION is installed and waits for the next login"
			else
				fail "extension $EXTENSION is not installed"
			fi
		elif grep -qx "$EXTENSION" <<<"$(gnome-extensions list --enabled)"; then
			pass "extension $EXTENSION is enabled"
		else
			fail "extension $EXTENSION is not enabled"
		fi
		if [[ "$MODE" == verify ]]; then
			# A newly installed extension loads on the next login on Wayland.
			local state
			state="$(gnome-extensions info "$EXTENSION" | sed -n 's/^ *State: *//p')"
			if [[ "$state" == ACTIVE || "$state" == ENABLED ]]; then
				pass "extension $EXTENSION is running in the new session (state $state)"
			else
				fail "extension $EXTENSION state after login: ${state:-unknown}"
			fi
		fi
		gsettings list-recursively >"$STATE/gsettings-$MODE.txt" 2>&1 || true
		;;
	kde)
		expect_eq "kdeglobals [General] ColorScheme" BreezeDark "$(kreadconfig5 --file kdeglobals --group General --key ColorScheme)"
		local fixed
		fixed="$(kreadconfig5 --file kdeglobals --group General --key fixed)"
		expect_eq "kdeglobals [General] fixed family" "$MONO_FONT" "${fixed%%,*}"
		;;
	esac
}

# GNOME Shell on Wayland discovers a newly installed extension only at the
# next login, so the plugin stops before enabling it and asks the user to log
# out and back in and retry the accepted plan. The suite follows that flow:
# apply records the pending plan and verify retries it in the new session.
install_extensions() {
	local output status=0
	output="$(devex plugin run desktop-gnome install-extensions --extensions "$STATE/extensions.json" \
		--expected-plan "$(plan_hash "$STATE/extensions-plan.json")" 2>&1)" || status=$?
	printf '%s\n' "$output"
	if ((status == 0)); then
		rm -f "$STATE/extensions-pending"
		pass "install-extensions completed"
	elif [[ "$MODE" == apply && "$output" == *"log out and back in"* ]]; then
		touch "$STATE/extensions-pending"
		pass "install-extensions asks for a new login before enabling $EXTENSION"
	else
		fail "install-extensions exited $status"
	fi
}

# configure and configure-fonts succeed when only some writes fail, warning
# about each failed one, so any such warning fails the check.
run_settings() {
	local plugin="$1" output status=0
	shift
	output="$(devex plugin run "$plugin" "$@" 2>&1)" || status=$?
	printf '%s\n' "$output"
	if ((status != 0)); then
		fail "$plugin $1 exited $status; see its output above"
	elif grep -q "Warning: Failed" <<<"$output"; then
		fail "$plugin $1 could not apply every setting"
	else
		pass "$plugin $1 applied every setting"
	fi
}

# Plugins accept the SHA-256 of the previewed JSON line, without its newline.
plan_hash() {
	printf '%s' "$(<"$1")" | sha256sum | cut -d' ' -f1
}

apply_desktop() {
	local plugin="desktop-$DESKTOP"
	log "installing plugin $plugin"
	devex plugin install "$plugin"
	devex plugin run "$plugin" describe >"$STATE/plan.json"
	python3 -c 'import json, sys; plan = json.load(open(sys.argv[1])); assert plan and all(plan), "empty plan"' "$STATE/plan.json" ||
		{
			fail "describe did not return a settings plan: $(cat "$STATE/plan.json")"
			exit 1
		}
	local hash
	hash="$(plan_hash "$STATE/plan.json")"
	log "applying settings plan $hash"
	run_settings "$plugin" configure --expected-plan "$hash"
	log "installing and configuring fonts"
	devex plugin run "$plugin" install-fonts jetbrainsmono
	case "$DESKTOP" in
	gnome)
		run_settings "$plugin" configure-fonts "$MONO_FONT 11"
		devex plugin run "$plugin" apply-theme Adwaita-dark
		local shell
		shell="$(gnome-shell --version | awk '{print $3}')"
		log "installing extension $EXTENSION for GNOME Shell $shell"
		printf '{"shell_version":"%s","extensions":[{"uuid":"%s"}]}\n' "$shell" "$EXTENSION" >"$STATE/extensions.json"
		devex plugin run "$plugin" describe install-extensions --extensions "$STATE/extensions.json" >"$STATE/extensions-plan.json"
		install_extensions
		;;
	kde)
		run_settings "$plugin" configure-fonts "$MONO_FONT"
		devex plugin run "$plugin" apply-theme BreezeDark
		;;
	esac
}

join_session
case "$MODE" in
apply)
	apply_desktop
	verify_desktop
	;;
verify)
	if [[ "$DESKTOP" == gnome && -e "$STATE/extensions-pending" ]]; then
		log "retrying the accepted extension plan after logging in again"
		install_extensions
	fi
	verify_desktop
	;;
*)
	fail "unknown mode $MODE"
	exit 2
	;;
esac

if ((FAILURES > 0)); then
	log "$FAILURES check(s) failed"
	exit 1
fi
log "all checks passed"
