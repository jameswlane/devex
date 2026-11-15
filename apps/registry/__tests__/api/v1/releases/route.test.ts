import { invalidateByTag } from "@vercel/functions";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { POST } from "@/app/api/v1/plugins/[id]/releases/route";
import {
	AuditEventType,
	type AuditLogger,
	initializeAuditLogger,
} from "@/lib/audit-log";
import { invalidateCdnTags, pluginCacheTags } from "@/lib/cdn-cache";
import { computeReleaseSignature } from "@/lib/releases/contract";
import { recordPluginRelease } from "@/lib/releases/record";
import { payload } from "../../../lib/releases/fixture";

vi.mock("@vercel/functions", () => ({
	invalidateByTag: vi.fn<typeof invalidateByTag>(),
}));
vi.mock("@/lib/cdn-cache", async () => ({
	...(await vi.importActual<typeof import("@/lib/cdn-cache")>(
		"@/lib/cdn-cache",
	)),
	invalidateCdnTags: vi.fn<typeof invalidateCdnTags>(),
}));
vi.mock("@/lib/releases/record", () => ({
	recordPluginRelease: vi.fn<typeof recordPluginRelease>(),
}));
vi.mock("@/lib/audit-log", () => ({
	AuditEventType: { ADMIN_ACTION: "admin_action" },
	extractAuditContext: vi.fn<() => object>().mockReturnValue({}),
	initializeAuditLogger: vi.fn<typeof initializeAuditLogger>(),
}));
vi.mock("@/lib/logger", () => ({
	logger: { warn: vi.fn(), error: vi.fn() },
}));
const auditLog = vi.fn<AuditLogger["log"]>();
const originalSecret = process.env.REGISTRY_RELEASE_WEBHOOK_SECRET;
const context = { params: Promise.resolve({ id: payload.plugin }) };
function request(
	body = JSON.stringify(payload),
	extraHeaders: Record<string, string> = {},
) {
	const timestamp = String(Math.floor(Date.now() / 1000));
	return new Request(
		`https://registry.devex.sh/api/v1/plugins/${payload.plugin}/releases`,
		{
			method: "POST",
			body,
			headers: {
				"content-type": "application/json",
				"x-devex-timestamp": timestamp,
				"x-devex-signature": computeReleaseSignature(
					"test-secret",
					timestamp,
					body,
				),
				...extraHeaders,
			},
		},
	);
}
beforeEach(() => {
	process.env.REGISTRY_RELEASE_WEBHOOK_SECRET = "test-secret";
	vi.mocked(invalidateCdnTags).mockResolvedValue(undefined);
	vi.mocked(recordPluginRelease).mockResolvedValue({
		pluginId: "plugin-id",
		versionCreated: true,
		latestChanged: true,
	});
	vi.mocked(initializeAuditLogger).mockReturnValue({
		log: auditLog,
	} as unknown as AuditLogger);
	auditLog.mockResolvedValue(undefined);
});
afterEach(() => {
	if (originalSecret === undefined)
		delete process.env.REGISTRY_RELEASE_WEBHOOK_SECRET;
	else process.env.REGISTRY_RELEASE_WEBHOOK_SECRET = originalSecret;
});

describe("POST plugin release", () => {
	it("returns 201 for a created version and audits the webhook identity", async () => {
		const response = await POST(request(), context);
		expect(response.status).toBe(201);
		expect(invalidateCdnTags).toHaveBeenCalledWith(
			pluginCacheTags(payload.plugin),
		);
		expect(response.headers.get("cache-control")).toBe("no-store");
		expect(await response.json()).toEqual({
			success: true,
			data: {
				plugin: payload.plugin,
				version: payload.version,
				latestChanged: true,
			},
		});
		expect(recordPluginRelease).toHaveBeenCalledWith(
			expect.anything(),
			payload,
			"webhook",
		);
		expect(auditLog).toHaveBeenCalledWith(
			AuditEventType.ADMIN_ACTION,
			"record-plugin-release",
			{ clientId: "release-webhook" },
			expect.objectContaining({ resourceId: "plugin-id", result: "success" }),
		);
	});
	it("returns 200 when refreshing a previously recorded version", async () => {
		vi.mocked(recordPluginRelease).mockResolvedValue({
			pluginId: "plugin-id",
			versionCreated: false,
			latestChanged: false,
		});
		const response = await POST(request(), context);
		expect(response.status).toBe(200);
		expect(await response.json()).toMatchObject({
			data: { latestChanged: false },
		});
	});
	it("does not fail the webhook when the CDN runtime purge fails", async () => {
		const { invalidateCdnTags: realPurge } =
			await vi.importActual<typeof import("@/lib/cdn-cache")>(
				"@/lib/cdn-cache",
			);
		vi.mocked(invalidateCdnTags).mockImplementation(realPurge);
		vi.mocked(invalidateByTag).mockRejectedValue(new Error("CDN unavailable"));
		const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
		expect((await POST(request(), context)).status).toBe(201);
		expect(warn).toHaveBeenCalledWith(
			expect.stringContaining("CDN purge failed"),
		);
		warn.mockRestore();
	});
	it("accepts signed JSON whitespace without reserializing it", async () => {
		expect(
			(await POST(request(JSON.stringify(payload, null, 2)), context)).status,
		).toBe(201);
	});
	it("does not fail successful writes when auditing fails", async () => {
		auditLog.mockRejectedValue(new Error("audit unavailable"));
		expect((await POST(request(), context)).status).toBe(201);
	});
	it.each([
		"{",
		JSON.stringify({ plugin: payload.plugin }),
		JSON.stringify({ ...payload, binaries: {} }),
	])("rejects invalid JSON or schema", async (body) => {
		const response = await POST(request(body), context);
		expect(response.status).toBe(400);
		expect(await response.json()).toMatchObject({
			error: { code: "VALIDATION_ERROR" },
		});
		expect(recordPluginRelease).not.toHaveBeenCalled();
		expect(invalidateCdnTags).not.toHaveBeenCalled();
	});
	it("includes Zod issues for schema failures", async () => {
		const response = await POST(request("{}"), context);
		expect(await response.json()).toMatchObject({
			error: {
				issues: expect.arrayContaining([
					expect.objectContaining({ path: ["version"] }),
				]),
			},
		});
	});
	it("requires plugin to match the URL", async () => {
		const response = await POST(request(), {
			params: Promise.resolve({ id: "other-plugin" }),
		});
		expect(response.status).toBe(400);
		expect(recordPluginRelease).not.toHaveBeenCalled();
		expect(invalidateCdnTags).not.toHaveBeenCalled();
	});
	it("verifies signatures before parsing JSON", async () => {
		const response = await POST(
			request("{", { "x-devex-signature": `sha256=${"0".repeat(64)}` }),
			context,
		);
		expect(response.status).toBe(401);
		expect(response.headers.get("cache-control")).toBe("no-store");
		expect(recordPluginRelease).not.toHaveBeenCalled();
		expect(invalidateCdnTags).not.toHaveBeenCalled();
	});
	it("fails closed if the secret is missing", async () => {
		delete process.env.REGISTRY_RELEASE_WEBHOOK_SECRET;
		const response = await POST(request(), context);
		expect(response.status).toBe(503);
		expect(await response.json()).toMatchObject({
			error: { code: "CONFIGURATION_ERROR" },
		});
		expect(recordPluginRelease).not.toHaveBeenCalled();
		expect(invalidateCdnTags).not.toHaveBeenCalled();
	});
	it("rejects bodies over 1 MiB without trusting content-length", async () => {
		const response = await POST(request("x".repeat(1024 * 1024 + 1)), context);
		expect(response.status).toBe(413);
		expect(recordPluginRelease).not.toHaveBeenCalled();
		expect(invalidateCdnTags).not.toHaveBeenCalled();
	});
	it("stops reading a streamed body once it passes 1 MiB", async () => {
		// No content-length: a chunked body is only bounded by counting bytes.
		let pulled = 0;
		const stream = new ReadableStream<Uint8Array>({
			pull(controller) {
				pulled++;
				controller.enqueue(new Uint8Array(256 * 1024));
				if (pulled > 64) controller.close();
			},
		});
		const streamed = new Request(
			`https://registry.devex.sh/api/v1/plugins/${payload.plugin}/releases`,
			{ method: "POST", body: stream, duplex: "half" } as RequestInit,
		);
		const response = await POST(streamed, context);
		expect(response.status).toBe(413);
		expect(pulled).toBeLessThan(10);
		expect(recordPluginRelease).not.toHaveBeenCalled();
		expect(invalidateCdnTags).not.toHaveBeenCalled();
	});
	it("measures UTF-8 bytes rather than characters", async () => {
		expect((await POST(request("é".repeat(524289)), context)).status).toBe(413);
	});
	it("rejects oversized declared content-length", async () => {
		expect(
			(await POST(request("{}", { "content-length": "1048577" }), context))
				.status,
		).toBe(413);
	});
	it("returns a no-store error for a failed write", async () => {
		vi.mocked(recordPluginRelease).mockRejectedValue(
			new Error("database unavailable"),
		);
		const response = await POST(request(), context);
		expect(response.status).toBe(500);
		expect(response.headers.get("cache-control")).toBe("no-store");
		expect(auditLog).not.toHaveBeenCalled();
		expect(invalidateCdnTags).not.toHaveBeenCalled();
	});
});
