#!/usr/bin/env bash
# Template checks for scripts/e2e-template-vm.sh; runs as the test user.
#
#   prepare  installs the prerequisites the installation guide lists
#   install  runs `devex install --template` the way a user would
#   verify   runs `devex status --template` and checks the expected results
#
# Usage: template-check.sh prepare|install|verify <template>
#
# Every built-in template needs a case below. Answers select every option a
# template offers on Linux, so the optional steps are covered too. Expectations:
#   pkg:<name>        the Debian package is installed
#   run:<command>     the command succeeds in a new interactive shell
#   svc:<unit>        the systemd unit is active
#   ctr:<name>        the Docker container is running, without sudo
#   flatpak:<app-id>  the Flatpak app is installed
#
# known_issue lists steps that fail because of an open bug. A failure with that
# bug's error message is reported as [known] without failing the run. Any other
# failure fails it, and so does the step passing, so the entry goes with the fix.

# Answers are comma-separated lists, not array separators.
# shellcheck disable=SC2054

set -euo pipefail

MODE="$1"
TEMPLATE="$2"
PROJECT=/var/tmp/devex-vm/project
KNOWN_INSTALL_FAILURE="$PROJECT/.known-install-failure"
FAILURES=0

ANSWERS=()
EXPECT=()

case "$TEMPLATE" in
minimal)
	EXPECT=(pkg:vim "run:vim --version")
	;;
web-development)
	EXPECT=("run:node --version")
	;;
backend)
	EXPECT=(pkg:git pkg:curl pkg:jq pkg:htop pkg:net-tools pkg:postgresql pkg:redis-server pkg:rabbitmq-server
		svc:docker svc:postgresql svc:redis-server svc:rabbitmq-server
		"run:docker ps" "run:kubectl version --client" "run:helm version")
	;;
full-stack)
	EXPECT=(pkg:git pkg:curl pkg:wget pkg:postgresql pkg:redis-server pkg:code pkg:google-chrome-stable pkg:firefox-devedition
		svc:docker svc:postgresql svc:redis-server
		"run:docker ps" "run:code --version" "run:google-chrome --version" "run:firefox-devedition --version")
	;;
databases)
	ANSWERS=(databases=postgres,redis,mysql mysql_password=devex-smoke-test)
	EXPECT=(svc:docker ctr:devex-postgres ctr:devex-redis ctr:devex-mysql)
	;;
server-stack)
	EXPECT=(svc:docker ctr:devex-server-app-1)
	;;
factorio-server)
	EXPECT=(svc:docker ctr:devex-factorio)
	;;
mobile)
	ANSWERS=(targets=react-native,flutter,android)
	EXPECT=("run:node --version" "run:java -version" "run:flutter --version" flatpak:com.google.AndroidStudio)
	;;
devops)
	ANSWERS=(tools=docker,kubectl,helm,kind,terraform,opentofu,awscli)
	EXPECT=(svc:docker "run:docker ps" "run:kubectl version --client" "run:helm version" "run:kind version"
		"run:terraform version" "run:tofu version" "run:aws --version")
	;;
data-science)
	ANSWERS=(components=jupyter,mlflow)
	EXPECT=(svc:docker ctr:devex-jupyter ctr:devex-mlflow)
	;;
game-development)
	ANSWERS=(engine=godot)
	EXPECT=(flatpak:org.godotengine.Godot)
	;;
*)
	printf '[FAIL] no smoke test case for template %s; add one to %s\n' "$TEMPLATE" "${BASH_SOURCE[0]##*/}"
	exit 1
	;;
esac

ANSWER_ARGS=()
for answer in "${ANSWERS[@]}"; do
	ANSWER_ARGS+=(--answer "$answer")
done

known_issue() { # step; sets ISSUE and SIGNATURE
	ISSUE="" SIGNATURE=""
	case "$TEMPLATE:$1" in
	mobile:install | game-development:install)
		ISSUE=569 SIGNATURE='Flatpak system operation ConfigureRemote not allowed'
		;;
	minimal:status | backend:status | full-stack:status)
		ISSUE=570 SIGNATURE='has no recorded resources for template'
		;;
	esac
}

# Whether output shows the bug known_issue selected.
is_known_failure() {
	[[ -n "$ISSUE" ]] && grep -Eq "$SIGNATURE" <<<"$1"
}

# After a known install failure, whether the failed step blocks this expectation.
# The mobile toolchains install before Flathub, so they are still checked.
blocked_by_install_issue() {
	case "$TEMPLATE:$1" in
	mobile:flatpak:*) return 0 ;;
	mobile:*) return 1 ;;
	*) return 0 ;;
	esac
}

pass() { printf '[pass] %s\n' "$*"; }
fail() {
	printf '[FAIL] %s\n' "$*"
	FAILURES=$((FAILURES + 1))
}
check() { # description command...
	local description="$1"
	shift
	local output
	if output="$("$@" 2>&1)"; then
		pass "$description"
	else
		fail "$description"
		[[ -z "$output" ]] || printf '%s\n' "$output" | tail -n 20 | sed 's/^/    /'
	fi
}

check_status() {
	local output
	known_issue status
	if output="$(devex status --template "$TEMPLATE" --non-interactive "${ANSWER_ARGS[@]}" 2>&1)"; then
		if [[ -n "$ISSUE" ]]; then
			fail "devex status passes; remove known issue #$ISSUE from ${BASH_SOURCE[0]##*/}"
		else
			pass "devex status --template $TEMPLATE"
		fi
	elif is_known_failure "$output"; then
		printf '[known] devex status --template %s fails: issue #%s\n' "$TEMPLATE" "$ISSUE"
	else
		fail "devex status --template $TEMPLATE"
		printf '%s\n' "$output" | tail -n 20 | sed 's/^/    /'
	fi
}

# dpkg-query also succeeds for removed or half-configured packages.
package_installed() {
	[[ "$(dpkg-query -W -f '${db:Status-Status}' "$1")" == installed ]]
}

# mise activates from the prompt hook, so feed the command to a real prompt.
in_new_shell() {
	printf '%s >/dev/null 2>&1; echo "devex-rc=$?"\nexit\n' "$1" | bash -i 2>/dev/null | grep -qx 'devex-rc=0'
}

# Containers can take a while to settle, and one that crash-loops is not running.
container_running() {
	local _ stable=0
	for _ in 1 2 3 4 5 6 7 8 9; do
		sleep 10
		if [[ "$(docker inspect -f '{{.State.Running}} {{.State.Restarting}}' "$1" 2>/dev/null)" == "true false" ]]; then
			stable=$((stable + 1))
		else
			stable=0
		fi
		((stable < 3)) || return 0
	done
	return 1
}

mkdir -p "$PROJECT"
cd "$PROJECT"

case "$MODE" in
prepare)
	# Background apt jobs on a new machine would otherwise fail runs at random.
	printf '%s\n' 'Acquire::Retries "5";' 'DPkg::Lock::Timeout "600";' |
		sudo tee /etc/apt/apt.conf.d/80-devex-vm >/dev/null
	sudo apt-get update -qq
	sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl git unzip ca-certificates >/dev/null
	;;
install)
	# server-stack starts the Compose project in the working directory.
	cat >docker-compose.yml <<'EOF'
services:
  app:
    image: busybox:1.37
    command: ["sleep", "infinity"]
EOF
	known_issue install
	rm -f "$KNOWN_INSTALL_FAILURE"
	status=0
	devex install --template "$TEMPLATE" --non-interactive "${ANSWER_ARGS[@]}" 2>&1 | tee install-output.log || status=$?
	if ((status == 0)) && [[ -n "$ISSUE" ]]; then
		printf '[FAIL] install passes; remove known issue #%s from %s\n' "$ISSUE" "${BASH_SOURCE[0]##*/}"
		exit 1
	fi
	if ((status != 0)); then
		is_known_failure "$(cat install-output.log)" || exit "$status"
		printf '[known] install of %s fails: issue #%s\n' "$TEMPLATE" "$ISSUE"
		printf '%s\n' "$ISSUE" >"$KNOWN_INSTALL_FAILURE"
	fi
	;;
verify)
	install_issue=""
	[[ ! -f "$KNOWN_INSTALL_FAILURE" ]] || install_issue="$(cat "$KNOWN_INSTALL_FAILURE")"
	if [[ -n "$install_issue" ]]; then
		printf '[known] skipping what install issue #%s blocks for %s\n' "$install_issue" "$TEMPLATE"
	else
		check_status
	fi
	for item in "${EXPECT[@]}"; do
		value="${item#*:}"
		if [[ -n "$install_issue" ]] && blocked_by_install_issue "$item"; then
			continue
		fi
		case "$item" in
		pkg:*) check "package $value" package_installed "$value" ;;
		run:*) check "command $value" in_new_shell "$value" ;;
		svc:*) check "service $value" systemctl is-active --quiet "$value" ;;
		ctr:*) check "container $value" container_running "$value" ;;
		flatpak:*) check "flatpak $value" flatpak info "$value" ;;
		esac
	done
	((FAILURES == 0)) || {
		printf '%d check(s) failed for %s\n' "$FAILURES" "$TEMPLATE"
		exit 1
	}
	;;
*)
	printf 'unknown mode %s\n' "$MODE" >&2
	exit 2
	;;
esac
