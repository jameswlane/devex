import {
	afterEach,
	beforeEach,
	describe,
	expect,
	it,
	jest,
} from "@jest/globals";
import { invalidateByTag } from "@vercel/functions";
import {
	invalidateCdnTags,
	pluginCacheTags,
	requestPluginCdnPurge,
} from "@/lib/cdn-cache";

jest.mock("@vercel/functions", () => ({
	invalidateByTag: jest.fn<typeof invalidateByTag>(),
}));
const originalKey = process.env.REVALIDATE_API_KEY;
const originalUrl = process.env.REGISTRY_URL;
const originalFetch = global.fetch;
const fetchMock = jest.fn<typeof fetch>();
beforeEach(() => {
	delete process.env.REVALIDATE_API_KEY;
	delete process.env.REGISTRY_URL;
	global.fetch = fetchMock;
	jest.mocked(invalidateByTag).mockResolvedValue(undefined);
	jest.spyOn(console, "warn").mockImplementation(() => {});
});
afterEach(() => {
	global.fetch = originalFetch;
	if (originalKey === undefined) delete process.env.REVALIDATE_API_KEY;
	else process.env.REVALIDATE_API_KEY = originalKey;
	if (originalUrl === undefined) delete process.env.REGISTRY_URL;
	else process.env.REGISTRY_URL = originalUrl;
	jest.restoreAllMocks();
});
describe("CDN cache tags", () => {
	it("includes collection tags to cover aliases and registry responses", () => {
		expect(pluginCacheTags("package-manager-apt")).toEqual([
			"plugin:package-manager-apt",
			"plugins",
			"registry",
		]);
	});
	it("uses the documented runtime API", async () => {
		await invalidateCdnTags(pluginCacheTags("package-manager-apt"));
		expect(invalidateByTag).toHaveBeenCalledWith([
			"plugin:package-manager-apt",
			"plugins",
			"registry",
		]);
	});
	it("warns on runtime failure without exposing the error", async () => {
		jest
			.mocked(invalidateByTag)
			.mockRejectedValue(new Error("sensitive request"));
		await expect(invalidateCdnTags(["plugins"])).resolves.toBeUndefined();
		expect(console.warn).toHaveBeenCalledWith(
			"CDN purge failed; cache TTL remains the freshness fallback",
		);
	});
	it.each(["REVALIDATE_API_KEY", "REGISTRY_URL"])(
		"warns without fetching if %s is missing",
		async (missing) => {
			process.env.REVALIDATE_API_KEY = "test-key";
			process.env.REGISTRY_URL = "https://registry.devex.sh";
			delete process.env[missing];
			await requestPluginCdnPurge();
			expect(fetchMock).not.toHaveBeenCalled();
			expect(console.warn).toHaveBeenCalledWith(
				expect.stringContaining("must be configured"),
			);
		},
	);
	it("authenticates a bounded request to the registry endpoint", async () => {
		process.env.REVALIDATE_API_KEY = "test-key";
		process.env.REGISTRY_URL = "https://registry.devex.sh";
		fetchMock.mockResolvedValue(new Response("{}"));
		await requestPluginCdnPurge();
		const [url, options] = fetchMock.mock.calls[0];
		expect(String(url)).toBe("https://registry.devex.sh/api/v1/revalidate");
		expect(options).toMatchObject({
			method: "POST",
			redirect: "error",
			headers: { Authorization: "Bearer test-key" },
			body: JSON.stringify({ type: "plugin-cdn" }),
			signal: expect.any(AbortSignal),
		});
	});
	it.each(["http://registry.devex.sh", "invalid URL"])(
		"refuses an insecure or invalid URL: %s",
		async (url) => {
			process.env.REVALIDATE_API_KEY = "test-key";
			process.env.REGISTRY_URL = url;
			await requestPluginCdnPurge();
			expect(fetchMock).not.toHaveBeenCalled();
			expect(console.warn).toHaveBeenCalled();
		},
	);
	it("warns on API failure without failing sync", async () => {
		process.env.REVALIDATE_API_KEY = "test-key";
		process.env.REGISTRY_URL = "https://registry.devex.sh";
		fetchMock.mockResolvedValue(
			new Response("sensitive response", { status: 403 }),
		);
		await expect(requestPluginCdnPurge()).resolves.toBeUndefined();
		expect(console.warn).toHaveBeenCalledWith("CDN purge failed with HTTP 403");
	});
	it("warns on network failure without failing sync", async () => {
		process.env.REVALIDATE_API_KEY = "test-key";
		process.env.REGISTRY_URL = "https://registry.devex.sh";
		fetchMock.mockRejectedValue(new Error("sensitive request"));
		await expect(requestPluginCdnPurge()).resolves.toBeUndefined();
		expect(console.warn).toHaveBeenCalledWith(
			"CDN purge request failed; cache TTL remains the freshness fallback",
		);
	});
});
