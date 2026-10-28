#!/usr/bin/env node
// Sends the release webhook for a plugin release to the registry.
//
// Reads GoReleaser's dist/metadata.json and dist/artifacts.json plus the plugin's
// packages/<plugin>/metadata.yaml, builds the payload described by
// apps/registry/lib/releases/contract.ts, signs it and POSTs it to
//   <registry-url>/api/v1/plugins/<plugin>/releases
//
// Usage:
//   node scripts/release/notify-registry.mjs --tag packages/package-manager-apt/v1.2.3
//   node scripts/release/notify-registry.mjs --tag ... --dry-run   # print the payload only
//
// Options:
//   --dist <dir>          GoReleaser dist directory (default: dist)
//   --tag <tag>           released tag, packages/<plugin>/v<version> (default: env TAG)
//   --registry-url <url>  registry base URL (default: env REGISTRY_URL or https://registry.devex.sh)
//   --repo <owner/name>   GitHub repository (default: env GITHUB_REPOSITORY or jameswlane/devex)
//   --metadata <file>     plugin metadata.yaml (default: packages/<plugin>/metadata.yaml)
//   --dry-run             print the payload and do not send it
//
// The signing secret comes from REGISTRY_RELEASE_WEBHOOK_SECRET and is never printed.
import crypto from "node:crypto";
import { existsSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";
import * as yaml from "js-yaml";

const repoRoot = path.resolve(
	path.dirname(fileURLToPath(import.meta.url)),
	"../..",
);

// Keep in step with apps/registry/lib/releases/contract.ts.
export const SECRET_ENV = "REGISTRY_RELEASE_WEBHOOK_SECRET";
export const SIGNATURE_HEADER = "x-devex-signature";
export const TIMESTAMP_HEADER = "x-devex-timestamp";
const DEFAULT_REGISTRY_URL = "https://registry.devex.sh";
const DEFAULT_REPO = "jameswlane/devex";
const NON_PLUGINS = new Set(["plugin-sdk"]);
const SUPPORTED_OS = ["linux", "darwin", "windows"];
const SUPPORTED_ARCH = ["amd64", "arm64"];
// metadata.yaml uses GOOS names; the registry uses "macos" for darwin.
const PLATFORM_NAMES = { linux: "linux", darwin: "macos", windows: "windows" };
const ARCHIVE_PREFERENCE = ["tar.gz", "zip"];
const SEMVER = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;
const HEX_SHA256 = /^[a-f0-9]{64}$/;
const PLUGIN_TAG = /^packages\/([a-z0-9]+(?:-[a-z0-9]+)*)\/v(.+)$/;

export function computeSignature(secret, timestamp, rawBody) {
	const digest = crypto
		.createHmac("sha256", secret)
		.update(`${timestamp}.${rawBody}`)
		.digest("hex");
	return `sha256=${digest}`;
}

export function parseTag(tag) {
	const match = PLUGIN_TAG.exec(tag ?? "");
	if (!match) {
		throw new Error(
			`tag "${tag}" is not a plugin tag; expected packages/<plugin>/v<version>`,
		);
	}
	const [, plugin, version] = match;
	if (NON_PLUGINS.has(plugin)) {
		throw new Error(`${plugin} is not a registry plugin`);
	}
	if (!SEMVER.test(version)) {
		throw new Error(`tag "${tag}" does not carry a semantic version`);
	}
	return { plugin, version };
}

export function isPrerelease(version) {
	// Strip build metadata first so "1.0.0+build-1" is not mistaken for a pre-release.
	return version.split("+")[0].includes("-");
}

function readJson(file) {
	try {
		return JSON.parse(readFileSync(file, "utf8"));
	} catch (err) {
		throw new Error(`cannot read ${file}: ${err.message}`);
	}
}

function parseChecksums(file) {
	const sums = new Map();
	if (!existsSync(file)) return sums;
	for (const line of readFileSync(file, "utf8").split("\n")) {
		const match = /^([a-f0-9]{64})\s+\*?(.+)$/.exec(line.trim());
		if (match) sums.set(match[2], match[1]);
	}
	return sums;
}

function releaseDownloadUrl(repo, tag, asset) {
	return `https://github.com/${repo}/releases/download/${encodeURIComponent(tag)}/${encodeURIComponent(asset)}`;
}

function shortNameFor(name, type) {
	return type && name.startsWith(`${type}-`)
		? name.slice(type.length + 1)
		: undefined;
}

export function metadataFromYaml(plugin, doc, repo) {
	if (!doc || typeof doc !== "object") {
		throw new Error(`${plugin}: metadata.yaml is empty or not a mapping`);
	}
	if (doc.name && doc.name !== plugin) {
		throw new Error(
			`metadata.yaml name "${doc.name}" does not match plugin "${plugin}"`,
		);
	}
	const platforms = [
		...new Set(
			Object.keys(doc.platforms ?? {})
				.filter((os) => SUPPORTED_OS.includes(os))
				.map((os) => PLATFORM_NAMES[os]),
		),
	];
	const strings = (list) =>
		Array.isArray(list) ? list.filter((v) => typeof v === "string") : [];
	const supports =
		doc.supports && typeof doc.supports === "object"
			? Object.fromEntries(
					Object.entries(doc.supports).filter(
						([, v]) => typeof v === "boolean",
					),
				)
			: undefined;

	const metadata = {
		description: doc.description,
		type: doc.type,
		shortName: shortNameFor(plugin, doc.type),
		author: doc.author,
		license: doc.license,
		homepage:
			doc.homepage ?? `https://github.com/${repo}/tree/main/packages/${plugin}`,
		repository: doc.repository ?? `https://github.com/${repo}`,
		platforms,
		priority: Number.isInteger(doc.priority) ? doc.priority : undefined,
		supports,
		sdkVersion:
			doc.sdk_version === undefined ? undefined : String(doc.sdk_version),
		apiVersion:
			doc.api_version === undefined ? undefined : String(doc.api_version),
		dependencies: strings(doc.dependencies),
		conflicts: strings(doc.conflicts),
	};
	for (const key of Object.keys(metadata)) {
		if (metadata[key] === undefined) delete metadata[key];
	}
	if (!metadata.description || !metadata.type) {
		throw new Error(`${plugin}: metadata.yaml needs description and type`);
	}
	if (platforms.length === 0) {
		throw new Error(`${plugin}: metadata.yaml declares no supported platforms`);
	}
	return metadata;
}

function artifactPlatform(artifact) {
	if (
		!SUPPORTED_OS.includes(artifact.goos) ||
		!SUPPORTED_ARCH.includes(artifact.goarch)
	)
		return null;
	return `${artifact.goos}-${artifact.goarch}`;
}

function isRawBinary(artifact) {
	return artifact.type === "Binary" && artifact.extra?.Format === "binary";
}

function isArchive(artifact) {
	return (
		artifact.type === "Archive" &&
		ARCHIVE_PREFERENCE.includes(artifact.extra?.Format)
	);
}

function archiveEntry(artifact, plugin) {
	const name = artifact.extra?.Binaries?.[0] ?? `devex-plugin-${plugin}`;
	const exe =
		artifact.goos === "windows" && !name.endsWith(".exe")
			? `${name}.exe`
			: name;
	const wrapped = artifact.extra?.WrappedIn;
	return wrapped ? `${wrapped}/${exe}` : exe;
}

/**
 * Picks one asset per "<os>-<arch>": the raw binary when there is one (older CLIs
 * cannot extract archives), otherwise a tar.gz, otherwise a zip.
 */
export function selectAssets(artifacts) {
	const chosen = new Map();
	const rank = (a) =>
		isRawBinary(a) ? 0 : 1 + ARCHIVE_PREFERENCE.indexOf(a.extra.Format);
	for (const artifact of artifacts) {
		if (!isRawBinary(artifact) && !isArchive(artifact)) continue;
		const key = artifactPlatform(artifact);
		if (!key) continue;
		const current = chosen.get(key);
		if (!current || rank(artifact) < rank(current)) chosen.set(key, artifact);
	}
	return chosen;
}

/** Builds the release payload from a GoReleaser dist directory. */
export function buildPayload({
	distDir = "dist",
	tag,
	repo = DEFAULT_REPO,
	metadataPath,
	commitSha,
	now = new Date(),
}) {
	const { plugin, version } = parseTag(tag);
	const dist = path.resolve(distDir);
	// artifacts.json paths are relative to the directory GoReleaser ran in, i.e. dist's parent.
	const base = path.dirname(dist);
	const releaseMeta = readJson(path.join(dist, "metadata.json"));
	const artifacts = readJson(path.join(dist, "artifacts.json"));
	if (!Array.isArray(artifacts)) {
		throw new Error(`${path.join(dist, "artifacts.json")} is not an array`);
	}
	if (releaseMeta.version && releaseMeta.version !== version) {
		throw new Error(
			`dist was built for version ${releaseMeta.version}, but tag ${tag} is ${version}`,
		);
	}

	const checksumArtifact = artifacts.find((a) => a.type === "Checksum");
	const checksumsName = checksumArtifact?.name ?? "checksums.txt";
	const checksums = parseChecksums(path.join(dist, checksumsName));

	const binaries = {};
	for (const [key, artifact] of [...selectAssets(artifacts)].sort(([a], [b]) =>
		a.localeCompare(b),
	)) {
		const fromExtra = /^sha256:([a-f0-9]{64})$/.exec(
			artifact.extra?.Checksum ?? "",
		)?.[1];
		const checksum = fromExtra ?? checksums.get(artifact.name);
		if (!checksum || !HEX_SHA256.test(checksum)) {
			throw new Error(`no sha256 checksum for ${artifact.name}`);
		}
		const file = path.resolve(base, artifact.path);
		let size;
		try {
			size = statSync(file).size;
		} catch (err) {
			throw new Error(
				`cannot stat ${artifact.name} at ${file}: ${err.message}`,
			);
		}
		const binary = {
			url: releaseDownloadUrl(repo, tag, artifact.name),
			checksum,
			size,
			format: isRawBinary(artifact) ? "binary" : artifact.extra.Format,
		};
		if (binary.format !== "binary")
			binary.archiveEntry = archiveEntry(artifact, plugin);
		binaries[key] = binary;
	}
	if (Object.keys(binaries).length === 0) {
		throw new Error(
			`no binaries or archives for ${plugin} in ${path.join(dist, "artifacts.json")}`,
		);
	}

	// metadata.json holds the tagged commit; GITHUB_SHA is only a fallback, since
	// re-runs and workflow_call runs report the branch head instead.
	const sha = releaseMeta.commit ?? commitSha;
	const doc = yaml.load(
		readFileSync(
			metadataPath ?? path.join(repoRoot, "packages", plugin, "metadata.yaml"),
			"utf8",
		),
	);

	const payload = {
		plugin,
		version,
		tag,
		prerelease: isPrerelease(version),
		releaseUrl: `https://github.com/${repo}/releases/tag/${tag}`,
		publishedAt: now.toISOString(),
		metadata: metadataFromYaml(plugin, doc, repo),
		binaries,
	};
	if (sha && /^[a-f0-9]{7,64}$/.test(sha)) payload.commitSha = sha;
	if (checksums.size > 0 || checksumArtifact) {
		payload.checksumsUrl = releaseDownloadUrl(repo, tag, checksumsName);
	}
	return payload;
}

class RetryableError extends Error {}

/**
 * POSTs the signed payload. Network errors and 5xx answers are retried with
 * exponential backoff; 4xx answers fail at once since resending will not help.
 */
export async function sendRelease({
	url,
	body,
	secret,
	fetchImpl = fetch,
	sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
	attempts = 3,
	backoffMs = 2000,
	clock = () => Date.now(),
	log = console.error,
}) {
	let lastError;
	for (let attempt = 1; attempt <= attempts; attempt++) {
		// Re-sign each attempt so a slow retry stays inside the receiver's clock tolerance.
		const timestamp = String(Math.floor(clock() / 1000));
		try {
			let response;
			try {
				response = await fetchImpl(url, {
					method: "POST",
					headers: {
						"content-type": "application/json",
						"user-agent": "devex-release-notifier",
						[SIGNATURE_HEADER]: computeSignature(secret, timestamp, body),
						[TIMESTAMP_HEADER]: timestamp,
					},
					body,
					signal: AbortSignal.timeout(30_000),
				});
			} catch (err) {
				throw new RetryableError(`request failed: ${err.message}`);
			}
			const text = (await response.text().catch(() => "")).slice(0, 500);
			if (response.ok) return { status: response.status, body: text };
			const message = `registry answered ${response.status}${text ? `: ${text}` : ""}`;
			if (response.status >= 500) throw new RetryableError(message);
			throw new Error(message);
		} catch (err) {
			if (!(err instanceof RetryableError)) throw err;
			lastError = err;
			if (attempt < attempts) {
				const delay = backoffMs * 2 ** (attempt - 1);
				log(
					`attempt ${attempt}/${attempts} failed (${err.message}); retrying in ${delay / 1000}s`,
				);
				await sleep(delay);
			}
		}
	}
	throw new Error(`giving up after ${attempts} attempts: ${lastError.message}`);
}

export async function main(argv = process.argv.slice(2), env = process.env) {
	const { values } = parseArgs({
		args: argv,
		options: {
			dist: { type: "string", default: "dist" },
			tag: { type: "string", default: env.TAG },
			"registry-url": {
				type: "string",
				default: env.REGISTRY_URL || DEFAULT_REGISTRY_URL,
			},
			repo: { type: "string", default: env.GITHUB_REPOSITORY || DEFAULT_REPO },
			metadata: { type: "string" },
			"dry-run": { type: "boolean", default: false },
		},
	});
	if (!values.tag) throw new Error("--tag is required (or set TAG)");

	const payload = buildPayload({
		distDir: values.dist,
		tag: values.tag,
		repo: values.repo,
		metadataPath: values.metadata,
		commitSha: env.GITHUB_SHA,
	});
	const body = JSON.stringify(payload);
	const url = `${values["registry-url"].replace(/\/+$/, "")}/api/v1/plugins/${payload.plugin}/releases`;

	if (values["dry-run"]) {
		console.log(`POST ${url}`);
		console.log(JSON.stringify(payload, null, 2));
		return;
	}
	const secret = env[SECRET_ENV];
	if (!secret) {
		throw new Error(
			`${SECRET_ENV} is not set; cannot sign the release webhook (use --dry-run to preview)`,
		);
	}
	const result = await sendRelease({ url, body, secret });
	console.log(
		`Registry accepted ${payload.plugin} ${payload.version} (${result.status})${result.body ? `: ${result.body}` : ""}`,
	);
}

if (
	process.argv[1] &&
	path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
	main().catch((err) => {
		console.error(`notify-registry: ${err.message}`);
		process.exitCode = 1;
	});
}
