import type { PrismaClient } from "@prisma/client";
import { githubApi } from "../../lib/github-api";
import { recordPluginRelease } from "../../lib/releases/record";
import { syncPlugins } from "../../scripts/sync-github-plugins";

jest.mock("../../lib/github-api", () => ({
	githubApi: {
		listTags: jest.fn(),
		getReleaseByTag: jest.fn(),
		getFileContents: jest.fn(),
	},
}));

jest.mock("../../lib/releases/record", () => ({
	recordPluginRelease: jest.fn(),
}));

describe("sync-github-plugins", () => {
	const validChecksum = "c".repeat(64);

	const mockPrisma = {
		plugin: {
			count: jest.fn().mockResolvedValue(5),
		},
		application: {
			count: jest.fn().mockResolvedValue(2),
		},
		config: {
			count: jest.fn().mockResolvedValue(1),
		},
		stack: {
			count: jest.fn().mockResolvedValue(1),
		},
		registryStats: {
			upsert: jest.fn().mockResolvedValue({}),
		},
		syncLog: {
			create: jest.fn().mockResolvedValue({}),
		},
		$disconnect: jest.fn().mockResolvedValue(undefined),
	} as unknown as PrismaClient;

	beforeEach(() => {
		jest.clearAllMocks();
		delete process.env.SYNC_TAG;
		global.fetch = jest.fn();
	});

	function createMockTag(
		name: string,
		sha = "1234567890abcdef1234567890abcdef12345678",
	) {
		return {
			name,
			commit: { sha },
		};
	}

	function createMockRelease(
		tagName: string,
		version: string,
		options: {
			prerelease?: boolean;
			isArchive?: boolean;
			isWindows?: boolean;
		} = {},
	) {
		const isArchive = options.isArchive ?? true;
		const isWindows = options.isWindows ?? false;
		const ext = isWindows ? ".zip" : isArchive ? ".tar.gz" : "";
		const os = isWindows ? "windows" : "linux";
		const binaryName = `devex-plugin-tool-git_${version}_${os}_amd64${ext}`;

		return {
			html_url: `https://github.com/jameswlane/devex/releases/tag/${tagName}`,
			prerelease: options.prerelease ?? false,
			published_at: "2026-10-27T12:00:00Z",
			assets: [
				{
					name: binaryName,
					size: 1024,
					browser_download_url: `https://github.com/download/${binaryName}`,
				},
				{
					name: "checksums.txt",
					size: 128,
					browser_download_url: "https://github.com/download/checksums.txt",
				},
			],
		};
	}

	it("handles pagination over 2+ pages and processes all tags in ascending semver order", () => {
		const page1 = Array.from({ length: 100 }, (_, i) =>
			createMockTag(`packages/dummy-tool/v0.0.${i + 1}`),
		);
		const page2 = [
			createMockTag("packages/tool-git/v1.0.0"),
			createMockTag("packages/tool-git/v0.1.0"),
		];

		(githubApi.listTags as jest.Mock)
			.mockResolvedValueOnce(page1)
			.mockResolvedValueOnce(page2);

		(githubApi.getReleaseByTag as jest.Mock).mockImplementation(
			(_owner, _repo, tag) => {
				const version = tag.slice(tag.lastIndexOf("/v") + 2);
				return Promise.resolve(createMockRelease(tag, version));
			},
		);

		(global.fetch as jest.Mock).mockImplementation((url: string) => {
			if (url.includes("checksums.txt")) {
				return Promise.resolve({
					ok: true,
					text: () =>
						Promise.resolve(
							`${validChecksum}  devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz\n` +
								`${validChecksum}  devex-plugin-tool-git_0.1.0_linux_amd64.tar.gz\n` +
								`${validChecksum}  devex-plugin-dummy-tool_0.0.1_linux_amd64.tar.gz\n`,
						),
				});
			}
			return Promise.resolve({ ok: false });
		});

		(recordPluginRelease as jest.Mock).mockResolvedValue({
			pluginId: "plugin-1",
			versionCreated: true,
			latestChanged: true,
		});

		return syncPlugins({
			tag: "packages/tool-git/v1.0.0",
			prisma: mockPrisma,
		}).then((res) => {
			expect(githubApi.listTags).toHaveBeenCalledTimes(2);
			expect(res.succeeded).toBe(1);
			expect(recordPluginRelease).toHaveBeenCalledWith(
				mockPrisma,
				expect.objectContaining({
					plugin: "tool-git",
					version: "1.0.0",
				}),
				"sync",
			);
		});
	});

	it("sorts tags in semver order before invoking recordPluginRelease", async () => {
		const tags = [
			createMockTag("packages/tool-git/v1.0.0"),
			createMockTag("packages/tool-git/v0.1.0"),
			createMockTag("packages/tool-git/v0.2.0"),
		];

		(githubApi.listTags as jest.Mock).mockResolvedValueOnce(tags);

		(githubApi.getReleaseByTag as jest.Mock).mockImplementation(
			(_owner, _repo, tag) => {
				const version = tag.slice(tag.lastIndexOf("/v") + 2);
				return Promise.resolve(createMockRelease(tag, version));
			},
		);

		(global.fetch as jest.Mock).mockImplementation((url: string) => {
			if (url.includes("checksums.txt")) {
				return Promise.resolve({
					ok: true,
					text: () =>
						Promise.resolve(
							`${validChecksum}  devex-plugin-tool-git_0.1.0_linux_amd64.tar.gz\n` +
								`${validChecksum}  devex-plugin-tool-git_0.2.0_linux_amd64.tar.gz\n` +
								`${validChecksum}  devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz\n`,
						),
				});
			}
			return Promise.resolve({ ok: false });
		});

		(recordPluginRelease as jest.Mock).mockResolvedValue({
			pluginId: "plugin-1",
			versionCreated: true,
			latestChanged: true,
		});

		await syncPlugins({ prisma: mockPrisma });

		expect(recordPluginRelease).toHaveBeenCalledTimes(3);
		const recordedVersions = (recordPluginRelease as jest.Mock).mock.calls.map(
			(call) => call[1].version,
		);
		expect(recordedVersions).toEqual(["0.1.0", "0.2.0", "1.0.0"]);
	});

	it("skips plugin-sdk tags, which are a library rather than a plugin", async () => {
		const tags = [
			createMockTag("packages/plugin-sdk/v0.3.0"),
			createMockTag("packages/tool-git/v1.0.0"),
		];
		(githubApi.listTags as jest.Mock).mockResolvedValueOnce(tags);
		(githubApi.getReleaseByTag as jest.Mock).mockImplementation(
			(_owner, _repo, tag) =>
				Promise.resolve(
					createMockRelease(tag, tag.slice(tag.lastIndexOf("/v") + 2)),
				),
		);
		(global.fetch as jest.Mock).mockImplementation((url: string) =>
			Promise.resolve(
				url.includes("checksums.txt")
					? {
							ok: true,
							text: () =>
								Promise.resolve(
									`${validChecksum}  devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz\n`,
								),
						}
					: { ok: false },
			),
		);
		(recordPluginRelease as jest.Mock).mockResolvedValue({
			pluginId: "plugin-1",
			versionCreated: true,
			latestChanged: true,
		});

		const result = await syncPlugins({ prisma: mockPrisma });

		expect(githubApi.getReleaseByTag).not.toHaveBeenCalledWith(
			expect.anything(),
			expect.anything(),
			"packages/plugin-sdk/v0.3.0",
		);
		expect(recordPluginRelease).toHaveBeenCalledTimes(1);
		expect(result.failed).toEqual([]);
	});

	it("skips invalid releases and continues processing remaining tags", async () => {
		const tags = [
			createMockTag("packages/tool-git/v0.1.0"),
			createMockTag("packages/tool-git/v0.2.0"), // Missing release -> will fail/skip
			createMockTag("packages/tool-git/v1.0.0"),
		];

		(githubApi.listTags as jest.Mock).mockResolvedValueOnce(tags);

		(githubApi.getReleaseByTag as jest.Mock).mockImplementation(
			(_owner, _repo, tag) => {
				if (tag === "packages/tool-git/v0.2.0") {
					return Promise.resolve(null);
				}
				const version = tag.slice(tag.lastIndexOf("/v") + 2);
				return Promise.resolve(createMockRelease(tag, version));
			},
		);

		(global.fetch as jest.Mock).mockImplementation((url: string) => {
			if (url.includes("checksums.txt")) {
				return Promise.resolve({
					ok: true,
					text: () =>
						Promise.resolve(
							`${validChecksum}  devex-plugin-tool-git_0.1.0_linux_amd64.tar.gz\n` +
								`${validChecksum}  devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz\n`,
						),
				});
			}
			return Promise.resolve({ ok: false });
		});

		(recordPluginRelease as jest.Mock).mockResolvedValue({
			pluginId: "plugin-1",
			versionCreated: true,
			latestChanged: true,
		});

		const result = await syncPlugins({ prisma: mockPrisma });

		expect(result.succeeded).toBe(2);
		expect(result.failed).toEqual(["packages/tool-git/v0.2.0"]);
		expect(recordPluginRelease).toHaveBeenCalledTimes(2);
	});

	it("sets archiveEntry for archive binaries (.tar.gz and .zip on windows)", async () => {
		const tags = [createMockTag("packages/tool-git/v1.0.0")];
		(githubApi.listTags as jest.Mock).mockResolvedValueOnce(tags);

		(githubApi.getReleaseByTag as jest.Mock).mockResolvedValueOnce({
			html_url:
				"https://github.com/jameswlane/devex/releases/tag/packages/tool-git/v1.0.0",
			prerelease: false,
			published_at: "2026-10-27T12:00:00Z",
			assets: [
				{
					name: "devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz",
					size: 1024,
					browser_download_url: "https://github.com/download/linux.tar.gz",
				},
				{
					name: "devex-plugin-tool-git_1.0.0_windows_amd64.zip",
					size: 2048,
					browser_download_url: "https://github.com/download/windows.zip",
				},
				{
					name: "checksums.txt",
					size: 128,
					browser_download_url: "https://github.com/download/checksums.txt",
				},
			],
		});

		(global.fetch as jest.Mock).mockImplementation((url: string) => {
			if (url.includes("checksums.txt")) {
				return Promise.resolve({
					ok: true,
					text: () =>
						Promise.resolve(
							`${validChecksum}  devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz\n` +
								`${validChecksum}  devex-plugin-tool-git_1.0.0_windows_amd64.zip\n`,
						),
				});
			}
			return Promise.resolve({ ok: false });
		});

		(recordPluginRelease as jest.Mock).mockResolvedValue({
			pluginId: "plugin-1",
			versionCreated: true,
			latestChanged: true,
		});

		await syncPlugins({ prisma: mockPrisma });

		expect(recordPluginRelease).toHaveBeenCalledWith(
			mockPrisma,
			expect.objectContaining({
				binaries: expect.objectContaining({
					"linux-amd64": expect.objectContaining({
						format: "tar.gz",
						archiveEntry: "devex-plugin-tool-git",
					}),
					"windows-amd64": expect.objectContaining({
						format: "zip",
						archiveEntry: "devex-plugin-tool-git.exe",
					}),
				}),
			}),
			"sync",
		);
	});

	it("detects prerelease from version string and release flag", async () => {
		const tags = [
			createMockTag("packages/tool-git/v1.0.0-rc.1"),
			createMockTag("packages/tool-git/v1.0.0"),
		];
		(githubApi.listTags as jest.Mock).mockResolvedValueOnce(tags);

		(githubApi.getReleaseByTag as jest.Mock).mockImplementation(
			(_owner, _repo, tag) => {
				const version = tag.slice(tag.lastIndexOf("/v") + 2);
				const isPrerelease = tag.includes("-rc") || tag.includes("1.0.0-");
				return Promise.resolve(
					createMockRelease(tag, version, { prerelease: isPrerelease }),
				);
			},
		);

		(global.fetch as jest.Mock).mockImplementation((url: string) => {
			if (url.includes("checksums.txt")) {
				return Promise.resolve({
					ok: true,
					text: () =>
						Promise.resolve(
							`${validChecksum}  devex-plugin-tool-git_1.0.0-rc.1_linux_amd64.tar.gz\n` +
								`${validChecksum}  devex-plugin-tool-git_1.0.0_linux_amd64.tar.gz\n`,
						),
				});
			}
			return Promise.resolve({ ok: false });
		});

		(recordPluginRelease as jest.Mock).mockResolvedValue({
			pluginId: "plugin-1",
			versionCreated: true,
			latestChanged: false,
		});

		await syncPlugins({ prisma: mockPrisma });

		const payloads = (recordPluginRelease as jest.Mock).mock.calls.map(
			(c) => c[1],
		);
		expect(payloads[0].prerelease).toBe(true);
		expect(payloads[1].prerelease).toBe(false);
	});

	it("throws if all tags fail to sync", async () => {
		const tags = [createMockTag("packages/tool-git/v1.0.0")];
		(githubApi.listTags as jest.Mock).mockResolvedValueOnce(tags);
		(githubApi.getReleaseByTag as jest.Mock).mockResolvedValueOnce(null);

		await expect(syncPlugins({ prisma: mockPrisma })).rejects.toThrow(
			"Failed to sync all 1 tag(s)",
		);
	});

	it("throws if single-tag requested is not found", async () => {
		const tags = [createMockTag("packages/tool-git/v1.0.0")];
		(githubApi.listTags as jest.Mock).mockResolvedValueOnce(tags);

		await expect(
			syncPlugins({
				tag: "packages/non-existent/v1.0.0",
				prisma: mockPrisma,
			}),
		).rejects.toThrow("Tag packages/non-existent/v1.0.0 was not found");
	});
});
