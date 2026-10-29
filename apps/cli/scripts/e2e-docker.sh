#!/usr/bin/env bash
# Run the DevEx CLI end-to-end suite inside a throwaway container.
#
# Builds a static linux devex binary and the Ginkgo suite binary, stages them
# with testdata in a temporary build context, builds the sandbox image for
# $DISTRO and runs it with ./artifacts/e2e/<distro> mounted at /artifacts.
# Exits with the container's exit code.
#
# Usage: scripts/e2e-docker.sh [extra suite args, e.g. --ginkgo.focus=init]
#
# Environment:
#   DISTRO                  ubuntu-24.04 (default) | debian-12
#   CONTAINER_ENGINE        docker | podman (default: whichever is found first)
#   E2E_DRY_RUN=1           print the engine commands instead of running them
#   E2E_DEVEX_BINARY        prebuilt linux devex binary to test instead of building one
#   DEVEX_E2E_DESKTOP       passed through to the suite when set
#   DEVEX_E2E_DESTRUCTIVE   passed through to the suite when set
#   DEVEX_E2E_REGISTRY_URL  passed through to the suite when set

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="$(dirname "$SCRIPT_DIR")"
E2E_DIR="$CLI_DIR/test/e2e"
DOCKERFILE_DIR="$E2E_DIR/docker"

DISTRO="${DISTRO:-ubuntu-24.04}"
DRY_RUN="${E2E_DRY_RUN:-0}"
PASSTHROUGH_VARS=(DEVEX_E2E_DESKTOP DEVEX_E2E_DESTRUCTIVE DEVEX_E2E_REGISTRY_URL)

log() {
	printf '==> %s\n' "$*" >&2
}

die() {
	printf 'e2e-docker: %s\n' "$*" >&2
	exit 1
}

# Print a command in a copy-pasteable form.
print_cmd() {
	local out
	printf -v out '%q ' "$@"
	printf '+ %s\n' "${out% }" >&2
}

# Engine commands are only printed in dry-run mode.
engine() {
	print_cmd "$ENGINE" "$@"
	if [[ "$DRY_RUN" == "1" ]]; then
		return 0
	fi
	"$ENGINE" "$@"
}

detect_engine() {
	if [[ -n "${CONTAINER_ENGINE:-}" ]]; then
		case "$CONTAINER_ENGINE" in
		docker | podman) ;;
		*) die "CONTAINER_ENGINE must be docker or podman, got '$CONTAINER_ENGINE'" ;;
		esac
		if ! command -v "$CONTAINER_ENGINE" >/dev/null 2>&1 && [[ "$DRY_RUN" != "1" ]]; then
			die "CONTAINER_ENGINE=$CONTAINER_ENGINE but '$CONTAINER_ENGINE' is not on PATH"
		fi
		ENGINE="$CONTAINER_ENGINE"
		return
	fi

	local candidate
	for candidate in docker podman; do
		if command -v "$candidate" >/dev/null 2>&1; then
			ENGINE="$candidate"
			return
		fi
	done

	if [[ "$DRY_RUN" == "1" ]]; then
		log "no container engine found; dry run assumes docker"
		ENGINE=docker
		return
	fi
	die "neither docker nor podman found on PATH (set CONTAINER_ENGINE to choose one)"
}

check_inputs() {
	DOCKERFILE="$DOCKERFILE_DIR/Dockerfile.$DISTRO"
	if [[ ! -f "$DOCKERFILE" ]]; then
		local available="" file
		for file in "$DOCKERFILE_DIR"/Dockerfile.*; do
			[[ -e "$file" ]] && available+="${file##*/Dockerfile.} "
		done
		die "unsupported DISTRO '$DISTRO' (available: ${available% })"
	fi

	if ! compgen -G "$E2E_DIR/*_test.go" >/dev/null; then
		die "no e2e suite found at $E2E_DIR (expected *_test.go files with the e2e build tag); the suite has not been added yet"
	fi
}

build_binaries() {
	local context="$1"
	local version goarch
	# Only CLI tags (vX.Y.Z); plugin tags (packages/<name>/vX.Y.Z) share the repository.
	version="$(git -C "$CLI_DIR" describe --tags --match 'v[0-9]*' --dirty 2>/dev/null || echo dev)"
	version="${version#v}"
	goarch="$(cd "$CLI_DIR" && go env GOARCH)"

	if [[ -n "${E2E_DEVEX_BINARY:-}" ]]; then
		[[ -f "$E2E_DEVEX_BINARY" ]] || die "E2E_DEVEX_BINARY=$E2E_DEVEX_BINARY is not a file"
		log "using prebuilt devex $E2E_DEVEX_BINARY"
		cp "$E2E_DEVEX_BINARY" "$context/devex"
	else
		log "building devex $version (linux/$goarch)"
		(
			cd "$CLI_DIR"
			CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -trimpath \
				-ldflags "-s -w -X main.version=$version" \
				-o "$context/devex" ./cmd/main.go
		)
	fi

	log "building e2e suite binary (linux/$goarch)"
	(
		cd "$CLI_DIR"
		CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go test -c -tags e2e \
			-o "$context/e2e.test" ./test/e2e
	)

	chmod 0755 "$context/devex" "$context/e2e.test"
}

main() {
	detect_engine
	check_inputs

	local context
	context="$(mktemp -d "${TMPDIR:-/tmp}/devex-e2e-context.XXXXXX")"
	# shellcheck disable=SC2064 # expand now: context is local to main
	trap "rm -rf '$context'" EXIT

	build_binaries "$context"
	cp "$DOCKERFILE" "$context/Dockerfile"
	cp -R "$E2E_DIR/testdata" "$context/testdata"

	local image="devex-e2e:$DISTRO"
	local artifacts="$CLI_DIR/artifacts/e2e/$DISTRO"

	log "building image $image with $ENGINE"
	engine build --file "$context/Dockerfile" --tag "$image" "$context"

	# The container runs as tester (UID 1000); make the mount writable even
	# when the host user or a rootless user namespace maps to another UID.
	mkdir -p "$artifacts"
	chmod a+rwx "$artifacts"

	local run_args=(run --rm --name "devex-e2e-$DISTRO-$$" --volume "$artifacts:/artifacts:Z")
	local var
	for var in "${PASSTHROUGH_VARS[@]}"; do
		if [[ -n "${!var:-}" ]]; then
			run_args+=(--env "$var=${!var}")
		fi
	done

	log "running suite in $image (artifacts: $artifacts)"
	local status=0
	engine "${run_args[@]}" "$image" "$@" || status=$?

	# The suite writes owner-only files as tester; open them up so the host user
	# (for example the CI runner, UID 1001) can read and upload them.
	engine run --rm --user 0 --entrypoint chmod --volume "$artifacts:/artifacts:Z" "$image" -R a+rX /artifacts ||
		log "could not make $artifacts readable; artifact upload may fail"

	if [[ "$DRY_RUN" == "1" ]]; then
		log "dry run complete; no image was built or run"
	elif [[ "$status" -eq 0 ]]; then
		log "e2e suite passed on $DISTRO"
	else
		log "e2e suite failed on $DISTRO (exit $status); see $artifacts"
	fi
	exit "$status"
}

main "$@"
