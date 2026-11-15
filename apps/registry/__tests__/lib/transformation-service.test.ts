import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../lib/logger", () => ({
	logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));

function plugin(name: string, overrides: Record<string, unknown> = {}) {
	return {
		name,
		description: `${name} plugin`,
		type: "package-manager",
		priority: 10,
		status: "active",
		supports: {},
		platforms: ["linux"],
		version: "1.0.0",
		latestVersion: "1.0.0",
		binaries: {},
		author: null,
		license: null,
		homepage: null,
		repository: null,
		dependencies: [],
		conflicts: [],
		sdkVersion: null,
		apiVersion: null,
		githubUrl: null,
		githubPath: `packages/${name}`,
		downloadCount: 0,
		lastDownload: null,
		...overrides,
	};
}

describe("RegistryTransformationService cache keys", () => {
	beforeEach(() => {
		// Force the in-memory store so the real cache path runs.
		delete process.env.KV_REST_API_URL;
		delete process.env.KV_REST_API_TOKEN;
		delete process.env.UPSTASH_REDIS_REST_URL;
		delete process.env.UPSTASH_REDIS_REST_TOKEN;
		delete process.env.REDIS_URL;
		vi.resetModules();
	});

	it("returns different results for two different pages of the same length", async () => {
		const { RegistryTransformationService } = await import(
			"../../lib/transformation-service"
		);
		const service = new RegistryTransformationService();

		const page1 = await service.transformPlugins([
			plugin("package-manager-apt"),
			plugin("package-manager-dnf"),
		] as never);
		const page2 = await service.transformPlugins([
			plugin("tool-git"),
			plugin("tool-shell"),
		] as never);

		expect(page1.map((p) => p.name)).toEqual([
			"package-manager-apt",
			"package-manager-dnf",
		]);
		expect(page2.map((p) => p.name)).toEqual(["tool-git", "tool-shell"]);
	});

	it("returns fresh results after an item changes", async () => {
		const { RegistryTransformationService } = await import(
			"../../lib/transformation-service"
		);
		const service = new RegistryTransformationService();

		await service.transformPlugins([plugin("tool-git")] as never);
		const updated = await service.transformPlugins([
			plugin("tool-git", { version: "2.0.0" }),
		] as never);

		expect(updated[0].version).toBe("2.0.0");
	});

	it("serves a repeated identical input from the cache", async () => {
		const { RegistryTransformationService } = await import(
			"../../lib/transformation-service"
		);
		const service = new RegistryTransformationService();
		const input = [plugin("tool-git")] as never;

		const first = await service.transformPlugins(input);
		const second = await service.transformPlugins(input);

		expect(second).toEqual(first);
		const stats = await service.getCacheStats();
		expect(stats.hits).toBe(1);
	});
});
