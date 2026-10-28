import { describe, expect, it, jest } from "@jest/globals";
import type { NextRequest } from "next/server";

import {
	DELETE,
	GET,
	HEAD,
	OPTIONS,
	PATCH,
	POST,
	PUT,
} from "@/app/api/[...path]/route";

jest.spyOn(console, "error").mockImplementation(() => {});

const createMockRequest = (url: string, method = "GET") =>
	({
		url,
		method,
		headers: { get: jest.fn(() => null) },
	}) as unknown as NextRequest;

const createContext = (path: string[]) => ({
	params: Promise.resolve({ path }),
});

const handlers = { GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS } as const;

describe("catch-all /api/[...path]", () => {
	it.each(Object.keys(handlers))(
		"returns a JSON 404 for %s",
		async (method) => {
			const handler = handlers[method as keyof typeof handlers];
			const response = await handler(
				createMockRequest("http://localhost/api/v1/does-not-exist", method),
				createContext(["v1", "does-not-exist"]),
			);

			expect(response.status).toBe(404);
			expect(response.headers.get("Content-Type")).toContain(
				"application/json",
			);
			expect(response.headers.get("Cache-Control")).toBe("no-store");

			const body = await response.json();
			expect(body.success).toBe(false);
			expect(body.error.code).toBe("NOT_FOUND");
			expect(body.error.message).toContain("/api/v1/does-not-exist");
			expect(body.error.path).toBe("/api/v1/does-not-exist");
			expect(typeof body.error.requestId).toBe("string");
			expect(typeof body.error.timestamp).toBe("string");
		},
	);

	it.each([
		"/api/does-not-exist",
		"/api/v1/nope",
		"/api/v1/plugins/does-not-exist/unknown/leaf",
		"/api/wp-json/wp/v2/users",
	])("handles nested unknown path %s", async (pathname) => {
		const path = pathname.replace(/^\/api\//, "").split("/");
		const response = await GET(
			createMockRequest(`http://localhost${pathname}`),
			createContext(path),
		);

		expect(response.status).toBe(404);
		expect(response.headers.get("Content-Type")).toContain("application/json");

		const body = await response.json();
		expect(body.error.code).toBe("NOT_FOUND");
		expect(body.error.path).toBe(pathname);
	});

	it("echoes a malicious path as an escaped JSON string, not reflected content", async () => {
		const pathname = "/api/%3Cscript%3Ealert(1)%3C/script%3E";
		const response = await GET(
			createMockRequest(`http://localhost${pathname}`),
			createContext(["<script>alert(1)</script>"]),
		);

		expect(response.headers.get("Content-Type")).toContain("application/json");
		expect(response.status).toBe(404);
		const body = await response.json();
		expect(body.error.message).toContain(pathname);
	});
});
