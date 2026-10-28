import assert from "node:assert/strict";
import {
	copyFileSync,
	mkdirSync,
	mkdtempSync,
	readFileSync,
	rmSync,
	writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, before, describe, it } from "node:test";
import { fileURLToPath } from "node:url";
import {
	buildPayload,
	computeSignature,
	isPrerelease,
	main,
	parseTag,
	SECRET_ENV,
	SIGNATURE_HEADER,
	selectAssets,
	sendRelease,
	TIMESTAMP_HEADER,
} from "./notify-registry.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
// Real output of `goreleaser release --snapshot` for package-manager-apt (GoReleaser Pro v2.19.0).
const fixture = path.join(here, "__fixtures__", "package-manager-apt-snapshot");
const metadataPath = path.join(fixture, "metadata.yaml");
const tag = "packages/package-manager-apt/v0.0.2-next";
const now = new Date("2026-10-27T12:00:00.000Z");

// Copied from apps/registry/lib/releases/contract.ts; the receiver tests use the same vector.
const RELEASE_SIGNATURE_TEST_VECTOR = {
	secret: "test-secret",
	timestamp: "1700000000",
	rawBody: '{"plugin":"package-manager-apt","version":"1.2.3"}',
	signature:
		"sha256=5d2d7681c67849efe5ce3a19534cb693683b4b2c2ca63628d9e95244145dec00",
};

/** Lays out the fixture as GoReleaser leaves it: <root>/dist/*.json plus the files they point at. */
function stageDist(artifacts) {
	const root = mkdtempSync(path.join(tmpdir(), "notify-registry-"));
	const dist = path.join(root, "dist");
	mkdirSync(dist);
	for (const file of ["metadata.json", "checksums.txt"])
		copyFileSync(path.join(fixture, file), path.join(dist, file));
	const list =
		artifacts ??
		JSON.parse(readFileSync(path.join(fixture, "artifacts.json"), "utf8"));
	writeFileSync(path.join(dist, "artifacts.json"), JSON.stringify(list));
	for (const artifact of list) {
		const file = path.join(root, artifact.path);
		mkdirSync(path.dirname(file), { recursive: true });
		// Size encodes the asset kind so tests can tell which file was stat'ed.
		if (
			!["metadata.json", "checksums.txt", "artifacts.json"].includes(
				path.basename(file),
			)
		) {
			writeFileSync(
				file,
				Buffer.alloc(artifact.type === "Archive" ? 200 : 100),
			);
		}
	}
	return { root, dist };
}

function response(status, text = "") {
	return { ok: status >= 200 && status < 300, status, text: async () => text };
}

describe("signature", () => {
	it("matches the contract's known-answer vector", () => {
		const { secret, timestamp, rawBody, signature } =
			RELEASE_SIGNATURE_TEST_VECTOR;
		assert.equal(computeSignature(secret, timestamp, rawBody), signature);
	});
});

describe("tags and versions", () => {
	it("parses plugin tags", () => {
		assert.deepEqual(parseTag("packages/package-manager-apt/v1.2.3"), {
			plugin: "package-manager-apt",
			version: "1.2.3",
		});
	});

	it("rejects CLI and plugin-sdk tags", () => {
		assert.throws(() => parseTag("v1.2.3"), /not a plugin tag/);
		assert.throws(
			() => parseTag("packages/plugin-sdk/v0.1.0"),
			/not a registry plugin/,
		);
		assert.throws(() => parseTag("packages/foo/vlatest"), /semantic version/);
	});

	it("detects pre-releases from the version's pre-release part", () => {
		assert.equal(isPrerelease("1.2.3"), false);
		assert.equal(isPrerelease("1.2.3-rc.1"), true);
		assert.equal(isPrerelease("0.0.2-next"), true);
		assert.equal(isPrerelease("1.2.3+build-7"), false);
		assert.equal(isPrerelease("1.2.3-beta+build-7"), true);
	});
});

describe("buildPayload from the GoReleaser snapshot fixture", () => {
	let staged;
	before(() => {
		staged = stageDist();
	});
	after(() => rmSync(staged.root, { recursive: true, force: true }));

	it("builds the contract payload", () => {
		const payload = buildPayload({
			distDir: staged.dist,
			tag,
			metadataPath,
			now,
		});
		const download =
			"https://github.com/jameswlane/devex/releases/download/packages%2Fpackage-manager-apt%2Fv0.0.2-next";
		assert.deepEqual(payload, {
			plugin: "package-manager-apt",
			version: "0.0.2-next",
			tag,
			prerelease: true,
			commitSha: "e4b080e81501ec61c8400df004bc55cb10eee34d",
			releaseUrl: `https://github.com/jameswlane/devex/releases/tag/${tag}`,
			checksumsUrl: `${download}/checksums.txt`,
			publishedAt: "2026-10-27T12:00:00.000Z",
			metadata: {
				description: "APT package manager support for Debian/Ubuntu systems",
				type: "package-manager",
				shortName: "apt",
				author: "DevEx Team",
				license: "MIT",
				homepage:
					"https://github.com/jameswlane/devex/tree/main/packages/package-manager-apt",
				repository: "https://github.com/jameswlane/devex",
				platforms: ["linux"],
				priority: 10,
				supports: {
					install: true,
					uninstall: true,
					update: true,
					search: true,
					list: true,
					repository_management: true,
				},
				sdkVersion: "1.0.0",
				apiVersion: "v1",
				dependencies: [],
				conflicts: [],
			},
			binaries: {
				"linux-amd64": {
					url: `${download}/devex-plugin-package-manager-apt_0.0.2-next_linux_amd64`,
					checksum:
						"b0579891143fcb53837c56e6958432eb99f127ea827069fc6930b09427808ed0",
					size: 100,
					format: "binary",
				},
				"linux-arm64": {
					url: `${download}/devex-plugin-package-manager-apt_0.0.2-next_linux_arm64`,
					checksum:
						"98a9bed165757f29dac0c60dfca763c0e9b497c49c5ef356d11995161b0c6b47",
					size: 100,
					format: "binary",
				},
			},
		});
	});

	it("prefers the tagged commit in metadata.json over GITHUB_SHA", () => {
		const payload = buildPayload({
			distDir: staged.dist,
			tag,
			metadataPath,
			now,
			commitSha: "abcdef1",
		});
		assert.equal(payload.commitSha, "e4b080e81501ec61c8400df004bc55cb10eee34d");
	});

	it("falls back to GITHUB_SHA when metadata.json has no commit", () => {
		const local = stageDist();
		try {
			const metaFile = path.join(local.dist, "metadata.json");
			const meta = JSON.parse(readFileSync(metaFile, "utf8"));
			delete meta.commit;
			writeFileSync(metaFile, JSON.stringify(meta));
			const payload = buildPayload({
				distDir: local.dist,
				tag,
				metadataPath,
				now,
				commitSha: "abcdef1",
			});
			assert.equal(payload.commitSha, "abcdef1");
		} finally {
			rmSync(local.root, { recursive: true, force: true });
		}
	});

	it("refuses a dist built for another version", () => {
		assert.throws(
			() =>
				buildPayload({
					distDir: staged.dist,
					tag: "packages/package-manager-apt/v1.0.0",
					metadataPath,
					now,
				}),
			/built for version 0\.0\.2-next/,
		);
	});
});

describe("asset selection", () => {
	const fixtureArtifacts = JSON.parse(
		readFileSync(path.join(fixture, "artifacts.json"), "utf8"),
	);
	const archivesOnly = fixtureArtifacts.filter(
		(a) => a.type !== "Binary" || a.extra?.Format !== "binary",
	);

	it("prefers the raw binary over the archive for each platform", () => {
		const chosen = selectAssets(fixtureArtifacts);
		assert.deepEqual([...chosen.keys()].sort(), ["linux-amd64", "linux-arm64"]);
		for (const artifact of chosen.values())
			assert.equal(artifact.extra.Format, "binary");
	});

	it("falls back to the archive with archiveEntry and checksums.txt", () => {
		// Drop extra.Checksum so the checksum has to come from checksums.txt.
		const artifacts = archivesOnly.map((a) =>
			a.type === "Archive"
				? { ...a, extra: { ...a.extra, Checksum: undefined } }
				: a,
		);
		const staged = stageDist(artifacts);
		try {
			const { binaries } = buildPayload({
				distDir: staged.dist,
				tag,
				metadataPath,
				now,
			});
			assert.deepEqual(binaries["linux-amd64"], {
				url: "https://github.com/jameswlane/devex/releases/download/packages%2Fpackage-manager-apt%2Fv0.0.2-next/devex-plugin-package-manager-apt_0.0.2-next_linux_amd64.tar.gz",
				checksum:
					"0d004412e55c2319a0a0ed0c3731a101e76f2ec765edaad28522146f9f46a089",
				size: 200,
				format: "tar.gz",
				archiveEntry: "devex-plugin-package-manager-apt",
			});
		} finally {
			rmSync(staged.root, { recursive: true, force: true });
		}
	});

	it("keys darwin assets as darwin-<arch>, uses .exe inside windows zips, and maps darwin to macos", () => {
		const extraPlatforms = [
			{
				name: "devex-plugin-package-manager-apt_0.0.2-next_darwin_arm64.tar.gz",
				path: "dist/devex-plugin-package-manager-apt_0.0.2-next_darwin_arm64.tar.gz",
				goos: "darwin",
				goarch: "arm64",
				type: "Archive",
				extra: {
					Binaries: ["devex-plugin-package-manager-apt"],
					Checksum: `sha256:${"a".repeat(64)}`,
					Format: "tar.gz",
					ID: "archive",
					WrappedIn: "",
				},
			},
			{
				name: "devex-plugin-package-manager-apt_0.0.2-next_windows_amd64.zip",
				path: "dist/devex-plugin-package-manager-apt_0.0.2-next_windows_amd64.zip",
				goos: "windows",
				goarch: "amd64",
				type: "Archive",
				extra: {
					Binaries: ["devex-plugin-package-manager-apt"],
					Checksum: `sha256:${"b".repeat(64)}`,
					Format: "zip",
					ID: "archive",
					WrappedIn: "",
				},
			},
		];
		const staged = stageDist([...fixtureArtifacts, ...extraPlatforms]);
		const yamlPath = path.join(staged.root, "metadata.yaml");
		writeFileSync(
			yamlPath,
			`${readFileSync(metadataPath, "utf8")}\n`.replace(
				"platforms:\n",
				"platforms:\n  darwin:\n    architectures: [arm64]\n  windows:\n    architectures: [amd64]\n",
			),
		);
		try {
			const payload = buildPayload({
				distDir: staged.dist,
				tag,
				metadataPath: yamlPath,
				now,
			});
			assert.deepEqual(Object.keys(payload.binaries), [
				"darwin-arm64",
				"linux-amd64",
				"linux-arm64",
				"windows-amd64",
			]);
			assert.equal(payload.binaries["darwin-arm64"].format, "tar.gz");
			assert.equal(
				payload.binaries["windows-amd64"].archiveEntry,
				"devex-plugin-package-manager-apt.exe",
			);
			assert.deepEqual(payload.metadata.platforms.sort(), [
				"linux",
				"macos",
				"windows",
			]);
		} finally {
			rmSync(staged.root, { recursive: true, force: true });
		}
	});
});

describe("sendRelease", () => {
	const base = {
		url: "https://registry.example/api/v1/plugins/package-manager-apt/releases",
		body: RELEASE_SIGNATURE_TEST_VECTOR.rawBody,
		secret: RELEASE_SIGNATURE_TEST_VECTOR.secret,
		clock: () => Number(RELEASE_SIGNATURE_TEST_VECTOR.timestamp) * 1000,
		log: () => {},
	};

	function stub(results) {
		const calls = [];
		const sleeps = [];
		const fetchImpl = async (url, init) => {
			calls.push({ url, init });
			const next = results[calls.length - 1];
			if (next instanceof Error) throw next;
			return next;
		};
		return { calls, sleeps, fetchImpl, sleep: async (ms) => sleeps.push(ms) };
	}

	it("sends a signed POST", async () => {
		const s = stub([response(201, '{"versionCreated":true}')]);
		const result = await sendRelease({
			...base,
			fetchImpl: s.fetchImpl,
			sleep: s.sleep,
		});
		assert.deepEqual(result, { status: 201, body: '{"versionCreated":true}' });
		assert.equal(s.calls.length, 1);
		const { init } = s.calls[0];
		assert.equal(init.method, "POST");
		assert.equal(init.body, base.body);
		assert.equal(init.headers["content-type"], "application/json");
		assert.equal(
			init.headers[TIMESTAMP_HEADER],
			RELEASE_SIGNATURE_TEST_VECTOR.timestamp,
		);
		assert.equal(
			init.headers[SIGNATURE_HEADER],
			RELEASE_SIGNATURE_TEST_VECTOR.signature,
		);
	});

	it("retries network errors and 5xx with backoff", async () => {
		const s = stub([
			new TypeError("fetch failed"),
			response(502, "bad gateway"),
			response(200, "ok"),
		]);
		const result = await sendRelease({
			...base,
			fetchImpl: s.fetchImpl,
			sleep: s.sleep,
		});
		assert.equal(result.status, 200);
		assert.equal(s.calls.length, 3);
		assert.deepEqual(s.sleeps, [2000, 4000]);
	});

	it("gives up after three attempts", async () => {
		const s = stub([response(500), response(503), response(500, "still down")]);
		await assert.rejects(
			sendRelease({ ...base, fetchImpl: s.fetchImpl, sleep: s.sleep }),
			/giving up after 3 attempts: registry answered 500: still down/,
		);
		assert.equal(s.calls.length, 3);
	});

	it("does not retry 4xx", async () => {
		const s = stub([
			response(401, '{"error":"invalid signature"}'),
			response(200),
		]);
		await assert.rejects(
			sendRelease({ ...base, fetchImpl: s.fetchImpl, sleep: s.sleep }),
			/registry answered 401/,
		);
		assert.equal(s.calls.length, 1);
		assert.deepEqual(s.sleeps, []);
	});
});

describe("main", () => {
	it("requires the secret unless --dry-run", async () => {
		const staged = stageDist();
		try {
			const args = [
				"--dist",
				staged.dist,
				"--tag",
				tag,
				"--metadata",
				metadataPath,
			];
			await assert.rejects(
				main(args, {}),
				new RegExp(`${SECRET_ENV} is not set`),
			);
			const logged = [];
			const original = console.log;
			console.log = (line) => logged.push(line);
			try {
				await main(
					[...args, "--dry-run", "--registry-url", "https://registry.example/"],
					{},
				);
			} finally {
				console.log = original;
			}
			assert.equal(
				logged[0],
				"POST https://registry.example/api/v1/plugins/package-manager-apt/releases",
			);
			assert.equal(JSON.parse(logged[1]).plugin, "package-manager-apt");
		} finally {
			rmSync(staged.root, { recursive: true, force: true });
		}
	});
});
