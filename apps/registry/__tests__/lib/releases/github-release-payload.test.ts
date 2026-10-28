import { describe, expect, it } from "@jest/globals";
import {
	buildReleasePayload,
	extractBinariesFromReleaseAssets,
	isPrerelease,
	parseChecksums,
	parseMetadataYaml,
	parsePluginTag,
	sortTagsByVersion,
} from "../../../lib/releases/github-release-payload";

describe("github-release-payload", () => {
	describe("parsePluginTag", () => {
		it("parses typed plugin tags correctly", () => {
			const apt = parsePluginTag("packages/package-manager-apt/v1.0.0");
			expect(apt).toEqual({
				type: "package-manager",
				name: "apt",
				fullName: "package-manager-apt",
				githubPath: "packages/package-manager-apt",
				version: "1.0.0",
			});

			const git = parsePluginTag("packages/tool-git/v0.2.1");
			expect(git).toEqual({
				type: "tool",
				name: "git",
				fullName: "tool-git",
				githubPath: "packages/tool-git",
				version: "0.2.1",
			});

			const gnome = parsePluginTag("packages/desktop-gnome/v3.0.0");
			expect(gnome).toEqual({
				type: "desktop",
				name: "gnome",
				fullName: "desktop-gnome",
				githubPath: "packages/desktop-gnome",
				version: "3.0.0",
			});

			const setup = parsePluginTag("packages/system-setup/v1.0.0");
			expect(setup).toEqual({
				type: "system",
				name: "setup",
				fullName: "system-setup",
				githubPath: "packages/system-setup",
				version: "1.0.0",
			});
		});

		it("parses generic plugin tags", () => {
			const custom = parsePluginTag("packages/custom-extension/v2.1.0");
			expect(custom).toEqual({
				type: "utility",
				name: "custom-extension",
				fullName: "custom-extension",
				githubPath: "packages/custom-extension",
				version: "2.1.0",
			});
		});

		it("handles prerelease and build metadata in version strings", () => {
			const pre = parsePluginTag(
				"packages/package-manager-apt/v1.0.0-beta.1+build2",
			);
			expect(pre).toEqual({
				type: "package-manager",
				name: "apt",
				fullName: "package-manager-apt",
				githubPath: "packages/package-manager-apt",
				version: "1.0.0-beta.1+build2",
			});
		});

		it("returns null for non-plugin tags", () => {
			expect(parsePluginTag("v1.0.0")).toBeNull();
			expect(parsePluginTag("releases/v1.0.0")).toBeNull();
			expect(parsePluginTag("packages/invalid")).toBeNull();
			expect(parsePluginTag("apps/registry/v1.0.0")).toBeNull();
		});
	});

	describe("sortTagsByVersion", () => {
		it("sorts tags in ascending semver order", () => {
			const tags = [
				{ name: "packages/tool-git/v1.0.0" },
				{ name: "packages/tool-git/v0.1.0" },
				{ name: "packages/tool-git/v0.2.0" },
				{ name: "packages/tool-git/v1.0.0-rc.1" },
				{ name: "packages/tool-git/v0.0.9" },
				{ name: "packages/tool-git/v1.0.1" },
			];

			const sorted = sortTagsByVersion(tags);
			expect(sorted.map((t) => t.name)).toEqual([
				"packages/tool-git/v0.0.9",
				"packages/tool-git/v0.1.0",
				"packages/tool-git/v0.2.0",
				"packages/tool-git/v1.0.0-rc.1",
				"packages/tool-git/v1.0.0",
				"packages/tool-git/v1.0.1",
			]);
		});
	});

	describe("parseChecksums", () => {
		it("parses valid checksums.txt lines", () => {
			const content = `
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  devex-plugin-package-manager-apt_0.0.1_linux_amd64.tar.gz
ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad *devex-plugin-package-manager-apt_0.0.1_windows_amd64.zip
88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589  devex-plugin-package-manager-apt_0.0.1_darwin_arm64
`;
			const map = parseChecksums(content);
			expect(map.size).toBe(3);
			expect(
				map.get("devex-plugin-package-manager-apt_0.0.1_linux_amd64.tar.gz"),
			).toBe(
				"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			);
			expect(
				map.get("devex-plugin-package-manager-apt_0.0.1_windows_amd64.zip"),
			).toBe(
				"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
			);
			expect(
				map.get("devex-plugin-package-manager-apt_0.0.1_darwin_arm64"),
			).toBe(
				"88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589",
			);
		});

		it("ignores invalid or empty lines", () => {
			const content = "invalid line\n\n123 not-a-hash filename.tar.gz";
			const map = parseChecksums(content);
			expect(map.size).toBe(0);
		});
	});

	describe("parseMetadataYaml", () => {
		const pluginInfo = {
			name: "apt",
			fullName: "package-manager-apt",
			type: "package-manager",
			version: "1.0.0",
			githubPath: "packages/package-manager-apt",
		};

		it("parses full metadata.yaml and normalizes darwin to macos", () => {
			const yaml = `
name: package-manager-apt
type: package-manager
description: APT package manager for Debian and Ubuntu
author: DevEx Team
license: Apache-2.0
homepage: https://devex.sh/plugins/apt
repository: https://github.com/jameswlane/devex
platforms:
  linux:
    distros: [debian, ubuntu]
  darwin:
    distros: []
  windows:
    distros: []
sdk_version: "1.2.0"
api_version: "v2"
priority: 15
supports:
  install: true
  uninstall: true
dependencies:
  - tool-git
conflicts:
  - package-manager-dnf
`;
			const meta = parseMetadataYaml(yaml, pluginInfo);
			expect(meta.description).toBe(
				"APT package manager for Debian and Ubuntu",
			);
			expect(meta.author).toBe("DevEx Team");
			expect(meta.license).toBe("Apache-2.0");
			expect(meta.homepage).toBe("https://devex.sh/plugins/apt");
			expect(meta.repository).toBe("https://github.com/jameswlane/devex");
			expect(meta.platforms).toEqual(
				expect.arrayContaining(["linux", "macos", "windows"]),
			);
			expect(meta.sdkVersion).toBe("1.2.0");
			expect(meta.apiVersion).toBe("v2");
			expect(meta.priority).toBe(15);
			expect(meta.supports).toEqual({ install: true, uninstall: true });
			expect(meta.dependencies).toEqual(["tool-git"]);
			expect(meta.conflicts).toEqual(["package-manager-dnf"]);
		});

		it("rejects empty metadata like the webhook sender", () => {
			expect(() => parseMetadataYaml(undefined, pluginInfo)).toThrow(
				"metadata.yaml is empty or not a mapping",
			);
		});

		it("uses sender defaults and derives shortName from the plugin type", () => {
			const meta = parseMetadataYaml(
				`
name: package-manager-apt
type: package-manager
description: APT support
shortName: ignored
platforms:
  darwin: {}
  linux: {}
sdkVersion: ignored
apiVersion: ignored
`,
				pluginInfo,
			);
			expect(meta).toEqual({
				description: "APT support",
				type: "package-manager",
				shortName: "apt",
				homepage:
					"https://github.com/jameswlane/devex/tree/main/packages/package-manager-apt",
				repository: "https://github.com/jameswlane/devex",
				platforms: ["macos", "linux"],
				dependencies: [],
				conflicts: [],
			});
		});
	});

	describe("extractBinariesFromReleaseAssets", () => {
		const checksum = "a".repeat(64);
		const repository = "jameswlane/devex";
		const tagName = "packages/package-manager-apt/v1.0.0";
		const download = `https://github.com/${repository}/releases/download/${encodeURIComponent(tagName)}`;
		const checksumMap = new Map([
			["devex-plugin-package-manager-apt_1.0.0_linux_amd64.tar.gz", checksum],
			["devex-plugin-package-manager-apt_1.0.0_linux_amd64", checksum],
			["devex-plugin-package-manager-apt_1.0.0_windows_amd64.zip", checksum],
			["devex-plugin-package-manager-apt_1.0.0_darwin_arm64", checksum],
		]);

		it("extracts archiveEntry for archives and omits it for raw binaries", () => {
			const assets = [
				{
					name: "devex-plugin-package-manager-apt_1.0.0_windows_amd64.zip",
					size: 1024,
					browser_download_url: "https://example.com/windows.zip",
				},
				{
					name: "devex-plugin-package-manager-apt_1.0.0_darwin_arm64",
					size: 2048,
					browser_download_url: "https://example.com/darwin-raw",
				},
			];

			const binaries = extractBinariesFromReleaseAssets(
				assets,
				"package-manager-apt",
				checksumMap,
				repository,
				tagName,
			);

			expect(binaries["windows-amd64"]).toEqual({
				url: `${download}/devex-plugin-package-manager-apt_1.0.0_windows_amd64.zip`,
				checksum,
				size: 1024,
				format: "zip",
				archiveEntry: "devex-plugin-package-manager-apt.exe",
			});

			expect(binaries["darwin-arm64"]).toEqual({
				url: `${download}/devex-plugin-package-manager-apt_1.0.0_darwin_arm64`,
				checksum,
				size: 2048,
				format: "binary",
			});
			expect(binaries["darwin-arm64"].archiveEntry).toBeUndefined();
		});

		it("prefers raw binaries over archives for the same platform", () => {
			const assets = [
				{
					name: "devex-plugin-package-manager-apt_1.0.0_linux_amd64.tar.gz",
					size: 1024,
					browser_download_url: "https://example.com/linux.tar.gz",
				},
				{
					name: "devex-plugin-package-manager-apt_1.0.0_linux_amd64",
					size: 2048,
					browser_download_url: "https://example.com/linux-raw",
				},
			];

			const binaries = extractBinariesFromReleaseAssets(
				assets,
				"package-manager-apt",
				checksumMap,
				repository,
				tagName,
			);

			expect(binaries["linux-amd64"]).toEqual({
				url: `${download}/devex-plugin-package-manager-apt_1.0.0_linux_amd64`,
				checksum,
				size: 2048,
				format: "binary",
			});
		});
	});

	describe("isPrerelease", () => {
		it("uses only the version's prerelease component", () => {
			expect(isPrerelease("1.0.0")).toBe(false);
			expect(isPrerelease("1.0.0-beta.1")).toBe(true);
			expect(isPrerelease("1.0.0-rc.1")).toBe(true);
			expect(isPrerelease("1.0.0+build-7")).toBe(false);
			expect(isPrerelease("1.0.0-beta+build-7")).toBe(true);
		});
	});

	describe("buildReleasePayload", () => {
		const validChecksum = "b".repeat(64);
		const tag = {
			name: "packages/tool-git/v1.0.0",
			commit: { sha: "1234567890abcdef1234567890abcdef12345678" },
		};
		const release = {
			html_url:
				"https://github.com/jameswlane/devex/releases/tag/packages/tool-git/v1.0.0",
			prerelease: false,
			published_at: "2026-10-27T12:00:00Z",
			assets: [
				{
					name: "devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz",
					size: 512,
					browser_download_url: "https://github.com/downloads/linux.tar.gz",
				},
				{
					name: "checksums.txt",
					size: 128,
					browser_download_url: "https://github.com/downloads/checksums.txt",
				},
			],
		};
		const checksumsContent = `${validChecksum}  devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz\n`;
		const metadataYaml = `
name: tool-git
type: tool
description: Git tooling
platforms:
  linux: {}
`;

		it("builds a validated ReleasePayload", () => {
			const payload = buildReleasePayload({
				tag,
				release,
				metadataYaml,
				checksumsContent,
			});

			expect(payload.plugin).toBe("tool-git");
			expect(payload.version).toBe("1.0.0");
			expect(payload.tag).toBe("packages/tool-git/v1.0.0");
			expect(payload.prerelease).toBe(false);
			expect(payload.commitSha).toBe(
				"1234567890abcdef1234567890abcdef12345678",
			);
			expect(payload.releaseUrl).toBe(
				"https://github.com/jameswlane/devex/releases/tag/packages/tool-git/v1.0.0",
			);
			const download =
				"https://github.com/jameswlane/devex/releases/download/packages%2Ftool-git%2Fv1.0.0";
			expect(payload.checksumsUrl).toBe(`${download}/checksums.txt`);
			expect(payload.publishedAt).toBe("2026-10-27T12:00:00.000Z");
			expect(payload.binaries["linux-amd64"]).toEqual({
				url: `${download}/devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz`,
				checksum: validChecksum,
				size: 512,
				format: "tar.gz",
				archiveEntry: "devex-plugin-tool-git",
			});
		});

		it("does not let GitHub's prerelease flag override a stable version", () => {
			const payload = buildReleasePayload({
				tag,
				release: { ...release, prerelease: true },
				metadataYaml,
				checksumsContent,
			});
			expect(payload.prerelease).toBe(false);
		});

		it("throws an error if no binaries match or checksums are missing", () => {
			expect(() => {
				buildReleasePayload({
					tag,
					release: { ...release, assets: [] },
					metadataYaml,
					checksumsContent: "",
				});
			}).toThrow();
		});
	});
});
