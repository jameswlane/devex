import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "@jest/globals";
import { buildReleasePayload } from "../../../lib/releases/github-release-payload";

const fixture = path.resolve(
	__dirname,
	"../../../../../scripts/release/__fixtures__/package-manager-apt-snapshot",
);
const tag = "packages/package-manager-apt/v0.0.2-next";
const version = "0.0.2-next";
const plugin = "package-manager-apt";
const assetUrl = (name: string) =>
	`https://github.com/jameswlane/devex/releases/download/${tag}/${name}`;
const asset = (name: string, size: number) => ({
	name,
	size,
	browser_download_url: assetUrl(name),
});

describe("webhook and nightly sync payload parity", () => {
	it("builds the sender's expected payload from the same GoReleaser release", () => {
		const assets = [
			asset(`devex-plugin-${plugin}_${version}_linux_amd64`, 100),
			asset(`devex-plugin-${plugin}_${version}_linux_amd64.tar.gz`, 200),
			asset(`devex-plugin-${plugin}_${version}_linux_arm64`, 100),
			asset(`devex-plugin-${plugin}_${version}_linux_arm64.tar.gz`, 200),
			asset("checksums.txt", 416),
		];
		const payload = buildReleasePayload({
			tag: {
				name: tag,
				commit: { sha: "e4b080e81501ec61c8400df004bc55cb10eee34d" },
			},
			release: {
				html_url: `https://github.com/jameswlane/devex/releases/tag/${tag}`,
				prerelease: true,
				published_at: "2026-10-27T12:00:00.000Z",
				assets,
			},
			metadataYaml: readFileSync(path.join(fixture, "metadata.yaml"), "utf8"),
			checksumsContent: readFileSync(
				path.join(fixture, "checksums.txt"),
				"utf8",
			),
		});
		const download =
			"https://github.com/jameswlane/devex/releases/download/packages%2Fpackage-manager-apt%2Fv0.0.2-next";

		expect(payload).toEqual({
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
});
