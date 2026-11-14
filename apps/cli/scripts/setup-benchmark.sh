#!/usr/bin/env bash

set -euo pipefail

usage() {
	printf 'Usage: %s run BINARY VARIANT RUN REF REVISION OUTPUT_DIR\n' "$0" >&2
	printf '       %s summarize RESULTS_DIR EXPECTED_RUNS\n' "$0" >&2
}

run_benchmark() {
	if [[ $# -ne 6 ]]; then
		usage
		return 2
	fi

	local binary=$1 variant=$2 run=$3 ref=$4 revision=$5 output_dir=$6
	if [[ ! -x "$binary" ]]; then
		printf 'Benchmark binary is not executable: %s\n' "$binary" >&2
		return 2
	fi
	if [[ "$variant" != baseline && "$variant" != candidate ]]; then
		printf 'Unknown benchmark variant: %s\n' "$variant" >&2
		return 2
	fi
	if [[ ! "$run" =~ ^[1-9][0-9]*$ ]]; then
		printf 'Invalid benchmark run index: %s\n' "$run" >&2
		return 2
	fi
	for tool in date python3 script sudo; do
		if ! command -v "$tool" >/dev/null 2>&1; then
			printf 'Required benchmark tool is unavailable: %s\n' "$tool" >&2
			return 2
		fi
	done

	mkdir -p "$output_dir"
	local wrapper_dir="$output_dir/wrappers" runner="$output_dir/run-install.sh"
	local apt_log="$output_dir/apt-invocations.log" console_log="$output_dir/console.log"
	local marker="$output_dir/install-started.marker"
	mkdir -p "$wrapper_dir"
	: > "$apt_log"

	cat > "$wrapper_dir/sudo" <<'SH'
#!/bin/sh
previous=
for argument in "$@"; do
	if [ "$previous" = apt-get ]; then
		case "$argument" in
		update | install) printf '%s\n' "$argument" >> "$DEVEX_BENCHMARK_APT_LOG" ;;
		esac
	fi
	previous=$argument
done
exec /usr/bin/sudo "$@"
SH
	cat > "$runner" <<'SH'
#!/usr/bin/env bash
exec env -u CI -u GITHUB_ACTIONS -u GITLAB_CI -u DEVEX_NONINTERACTIVE -u DEVEX_NON_INTERACTIVE \
	TERM=xterm-256color DEBIAN_FRONTEND=noninteractive "$DEVEX_BENCHMARK_BINARY" install
SH
	chmod 0755 "$wrapper_dir/sudo" "$runner"
	touch "$marker"

	export DEVEX_BENCHMARK_APT_LOG="$apt_log"
	export DEVEX_BENCHMARK_BINARY="$binary"
	local started_at finished_at start_ns end_ns exit_code
	started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
	start_ns=$(date +%s%N)
	local runner_command
	printf -v runner_command '%q' "$runner"
	if PATH="$wrapper_dir:$PATH" script --quiet --return --command "$runner_command" "$console_log"; then
		exit_code=0
	else
		exit_code=$?
	fi
	end_ns=$(date +%s%N)
	finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

	local log_dir="$HOME/.local/state/devex/logs" devex_log="" candidate
	if [[ -d "$log_dir" ]]; then
		for candidate in "$log_dir"/devex-*.log; do
			if [[ -f "$candidate" && "$candidate" -nt "$marker" ]]; then
				devex_log=$candidate
			fi
		done
	fi
	local log_name=
	if [[ -n "$devex_log" ]]; then
		cp "$devex_log" "$output_dir/devex.log"
		log_name=devex.log
	fi

	python3 - "$output_dir/result.json" "$variant" "$run" "$ref" "$revision" "$started_at" "$finished_at" "$start_ns" "$end_ns" "$exit_code" "$apt_log" "$log_name" <<'PY'
import json
import os
from pathlib import Path
import sys

(output, variant, run, ref, revision, started, finished, start_ns, end_ns,
 exit_code, apt_log, log_name) = sys.argv[1:]
operations = Path(apt_log).read_text().splitlines()

os_release = {}
for line in Path('/etc/os-release').read_text().splitlines():
    if '=' in line:
        key, value = line.split('=', 1)
        os_release[key] = value.strip().strip('"')

memory_kib = None
for line in Path('/proc/meminfo').read_text().splitlines():
    if line.startswith('MemTotal:'):
        memory_kib = int(line.split()[1])
        break

cpu_model = None
for line in Path('/proc/cpuinfo').read_text().splitlines():
    if line.lower().startswith('model name'):
        cpu_model = line.split(':', 1)[1].strip()
        break

mirrors = set()
source_files = [Path('/etc/apt/sources.list')]
for pattern in ('*.list', '*.sources'):
    source_files.extend(Path('/etc/apt/sources.list.d').glob(pattern))
for source in source_files:
    if not source.is_file():
        continue
    for raw_line in source.read_text(errors='replace').splitlines():
        line = raw_line.strip()
        if not line or line.startswith('#'):
            continue
        if source.suffix == '.sources' and line.startswith('URIs:'):
            mirrors.update(line.split(':', 1)[1].split())
        elif source.suffix == '.list' and line.split()[0] in ('deb', 'deb-src'):
            mirrors.update(token for token in line.split()[1:] if token.startswith(('http://', 'https://')))

result = {
    'schema_version': 1,
    'variant': variant,
    'run': int(run),
    'ref': ref,
    'revision': revision,
    'command': 'devex install (default catalog, util-linux script pseudo-terminal, CI variables unset)',
    'default_app_source': 'the selected ref CLI default catalog; no positional application arguments',
    'started_at_utc': started,
    'finished_at_utc': finished,
    'wall_clock_seconds': round((int(end_ns) - int(start_ns)) / 1_000_000_000, 3),
    'exit_code': int(exit_code),
    'apt_get_update_count': operations.count('update'),
    'apt_get_install_count': operations.count('install'),
    'apt_invocation_source': 'PATH sudo wrapper observing apt-get argv',
    'ubuntu': {
        'pretty_name': os_release.get('PRETTY_NAME'),
        'version_id': os_release.get('VERSION_ID'),
        'architecture': os.uname().machine,
    },
    'cpu': {'logical_count': os.cpu_count(), 'model': cpu_model},
    'memory_total_kib': memory_kib,
    'apt_mirror_urls': sorted(mirrors),
    'devex_log': log_name or None,
}
Path(output).write_text(json.dumps(result, indent=2) + '\n')
PY

	if [[ -z "$log_name" ]]; then
		printf 'No new DevEx log found in %s\n' "$log_dir" >&2
		return 1
	fi
	return "$exit_code"
}

summarize() {
	if [[ $# -ne 2 || ! "$2" =~ ^[1-5]$ ]]; then
		usage
		return 2
	fi
	python3 - "$1" "$2" <<'PY'
import json
from pathlib import Path
import statistics
import sys

root = Path(sys.argv[1])
expected_runs = int(sys.argv[2])
expected = {(variant, run) for variant in ('baseline', 'candidate') for run in range(1, expected_runs + 1)}
results = {}
for path in sorted(root.rglob('result.json')):
    try:
        result = json.loads(path.read_text())
        key = (result['variant'], result['run'])
        if key in results:
            raise ValueError(f'duplicate result for {key}')
        seconds = result['wall_clock_seconds']
        if not isinstance(seconds, (int, float)) or isinstance(seconds, bool) or seconds < 0:
            raise ValueError(f'invalid wall time in {path}')
        for field in ('apt_get_update_count', 'apt_get_install_count', 'exit_code'):
            if not isinstance(result[field], int) or isinstance(result[field], bool) or result[field] < 0:
                raise ValueError(f'invalid {field} in {path}')
        results[key] = result
    except (KeyError, TypeError, ValueError, json.JSONDecodeError) as error:
        raise SystemExit(f'Invalid benchmark result {path}: {error}')
missing = expected - set(results)
extra = set(results) - expected
if missing or extra:
    raise SystemExit(f'Benchmark results do not match the matrix; missing={sorted(missing)}, extra={sorted(extra)}')

print('# Setup benchmark')
print()
print('Build time is excluded. Times include CLI startup, plugin/catalog loading, installation, and command finalization.')
print()
print('| Variant | Run | Wall time (s) | apt-get update | apt-get install | Exit code |')
print('| --- | ---: | ---: | ---: | ---: | ---: |')
for key in sorted(expected):
    result = results[key]
    print(f"| {key[0]} | {key[1]} | {result['wall_clock_seconds']:.3f} | {result['apt_get_update_count']} | {result['apt_get_install_count']} | {result['exit_code']} |")

print()
print('| Variant | Runs | Median wall time (s) | Under 5 minutes |')
print('| --- | ---: | ---: | --- |')
medians = {}
for variant in ('baseline', 'candidate'):
    medians[variant] = statistics.median(results[(variant, run)]['wall_clock_seconds'] for run in range(1, expected_runs + 1))
    target = 'PASS' if variant == 'candidate' and medians[variant] < 300 else 'FAIL' if variant == 'candidate' else 'n/a'
    print(f'| {variant} | {expected_runs} | {medians[variant]:.3f} | {target} |')
print()
print(f"Candidate median target result: **{'PASS' if medians['candidate'] < 300 else 'FAIL'}** (target is strictly under 300 seconds; report only).")
print()
print('Per-run revisions, OS/CPU/RAM, apt mirror URLs, and the measured command are in each artifact\'s `result.json`. The DevEx and console logs accompany it.')

failed = [key for key in sorted(expected) if results[key]['exit_code'] != 0]
if failed:
    print(f'Benchmark installation failed for: {failed}', file=sys.stderr)
    raise SystemExit(1)
PY
}

case "${1:-}" in
run)
	shift
	run_benchmark "$@"
	;;
summarize)
	shift
	summarize "$@"
	;;
*)
	usage
	exit 2
	;;
esac
