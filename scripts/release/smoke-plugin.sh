#!/usr/bin/env bash
# Post-release smoke test for a plugin version.
#
# Polls the registry until it serves the version record (up to ~5 minutes),
# downloads the linux-amd64 binary from the recorded URL, verifies its
# SHA-256 and size against the record, extracts it when archived, and checks
# that `<binary> --plugin-info` reports the released name and version.
#
# Usage: smoke-plugin.sh <plugin> <version> [registry_url]
# Needs: bash, curl, node, sha256sum, tar (and unzip for zip-format plugins).
set -euo pipefail

if [ "$#" -lt 2 ]; then
	echo "::error::usage: smoke-plugin.sh <plugin> <version> [registry_url]"
	exit 1
fi

plugin="$1"
version="${2#v}" # accept both 1.2.3 and v1.2.3
registry="${3:-https://registry.devex.sh}"
registry="${registry%/}"
platform="linux-amd64"
timeout_seconds="${SMOKE_TIMEOUT_SECONDS:-300}"
poll_interval="${SMOKE_POLL_INTERVAL:-15}"

endpoint="${registry}/api/v1/plugins/${plugin}/versions/${version}"
response="$(mktemp)"
workdir="$(mktemp -d)"
trap 'rm -rf "$response" "$workdir"' EXIT

echo "::group::Wait for ${endpoint}"
deadline=$((SECONDS + timeout_seconds))
while :; do
	code=$(curl -sS -o "$response" -w "%{http_code}" "$endpoint" || true)
	if [ "$code" = "200" ]; then
		echo "registry returned 200"
		break
	fi
	if [ "$SECONDS" -ge "$deadline" ]; then
		echo "::error::Timed out after ${timeout_seconds}s waiting for ${endpoint} (last HTTP status: ${code:-none})"
		exit 1
	fi
	echo "HTTP ${code:-error}; retrying in ${poll_interval}s"
	sleep "$poll_interval"
done
echo "::endgroup::"

# The endpoint may wrap the record as {data: ...} or return it directly.
mapfile -t fields < <(node -e '
	let raw = "";
	process.stdin.on("data", (chunk) => (raw += chunk)).on("end", () => {
		let parsed;
		try {
			parsed = JSON.parse(raw);
		} catch {
			console.error("response is not valid JSON");
			process.exit(2);
		}
		const record =
			parsed.data && typeof parsed.data === "object" && !Array.isArray(parsed.data)
				? parsed.data
				: parsed;
		const binary = (record.binaries || {})[process.argv[1]];
		if (!binary || typeof binary !== "object") {
			console.error(`record has no binaries["${process.argv[1]}"] entry`);
			process.exit(2);
		}
		for (const key of ["url", "checksum", "format", "archiveEntry", "size"]) {
			console.log(binary[key] ?? "");
		}
	});
' "$platform" <"$response") || {
	echo "::error::Could not parse a binaries[\"$platform\"] entry from ${endpoint}"
	exit 1
}
url="${fields[0]}"
checksum="${fields[1]}"
format="${fields[2]}"
archive_entry="${fields[3]}"
size="${fields[4]}"

if [ -z "$url" ] || [ -z "$checksum" ] || [ -z "$format" ]; then
	echo "::error::Incomplete binary record from ${endpoint} (url/checksum/format missing): ${fields[*]}"
	exit 1
fi
if ! [[ "$checksum" =~ ^[a-f0-9]{64}$ ]]; then
	echo "::error::Recorded checksum is not a hex SHA-256: $checksum"
	exit 1
fi
echo "binary: $format $url (sha256 $checksum)"

echo "::group::Download and verify"
download="$workdir/download"
if ! curl -fsSL --retry 3 -o "$download" "$url"; then
	echo "::error::Failed to download $url"
	exit 1
fi
actual_checksum=$(sha256sum "$download" | awk '{print $1}')
if [ "$actual_checksum" != "$checksum" ]; then
	echo "::error::Checksum mismatch for $url: expected $checksum, got $actual_checksum"
	exit 1
fi
echo "checksum ok"
if [ -n "$size" ]; then
	actual_size=$(stat -c %s "$download")
	if [ "$actual_size" != "$size" ]; then
		echo "::error::Size mismatch for $url: expected $size bytes, got $actual_size"
		exit 1
	fi
	echo "size ok ($size bytes)"
fi
echo "::endgroup::"

echo "::group::Extract"
binary="$workdir/plugin"
case "$format" in
binary)
	mv "$download" "$binary"
	;;
tar.gz | zip)
	if [ -z "$archive_entry" ]; then
		echo "::error::Record for a $format binary must set archiveEntry"
		exit 1
	fi
	if [ "$format" = "tar.gz" ]; then
		tar -xzf "$download" -C "$workdir"
	else
		unzip -q "$download" -d "$workdir"
	fi
	binary="$workdir/$archive_entry"
	;;
*)
	echo "::error::Unknown binary format '$format' (expected binary, tar.gz or zip)"
	exit 1
	;;
esac
if [ ! -f "$binary" ]; then
	echo "::error::Binary not found at '$archive_entry' after extracting $url"
	exit 1
fi
chmod +x "$binary"
echo "::endgroup::"

echo "::group::Run $plugin --plugin-info"
# --plugin-info is the SDK protocol the CLI uses to identify a plugin.
if ! info=$("$binary" --plugin-info 2>&1); then
	echo "::error::'--plugin-info' exited non-zero:"
	echo "$info"
	exit 1
fi
echo "$info"
if ! node -e '
	const [want_name, want_version] = process.argv.slice(1);
	let info;
	try {
		info = JSON.parse(require("node:fs").readFileSync(0, "utf8"));
	} catch {
		console.error("--plugin-info output is not JSON");
		process.exit(1);
	}
	if (info.name !== want_name || info.version !== want_version) {
		console.error(`expected ${want_name} ${want_version}, got ${info.name} ${info.version}`);
		process.exit(1);
	}
' "$plugin" "$version" <<<"$info"; then
	echo "::error::--plugin-info does not report $plugin $version"
	exit 1
fi
echo "::endgroup::"

echo "Smoke test passed: $plugin $version ($platform)"
