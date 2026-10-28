import {
	afterEach,
	beforeEach,
	describe,
	expect,
	it,
	jest,
} from "@jest/globals";
import {
	PrismaClientInitializationError,
	PrismaClientKnownRequestError,
	PrismaClientRustPanicError,
	PrismaClientUnknownRequestError,
	PrismaClientValidationError,
} from "@prisma/client/runtime/library";
import { NextRequest, type NextResponse } from "next/server";
import {
	classifyError,
	isRetryableDatabaseError,
	RegistryErrorHandler,
	RequestValidationError,
	resolveRequestId,
	safeDatabase,
	withErrorHandling,
} from "@/lib/error-handler";
import { createApiError, logger } from "@/lib/logger";

const CLIENT_VERSION = "6.16.2";

// The message Prisma produced in production when plugin_versions was missing.
const MISSING_TABLE_MESSAGE =
	"\nInvalid `prisma.pluginVersion.findUnique()` invocation:\n\n\nThe table `public.plugin_versions` does not exist in the current database.";

function knownError(code: string, message = `Prisma failure ${code}`) {
	return new PrismaClientKnownRequestError(message, {
		code,
		clientVersion: CLIENT_VERSION,
		meta: { modelName: "PluginVersion", table: "public.plugin_versions" },
	});
}

/**
 * Stand-in for a Prisma error thrown by a different copy of the runtime
 * (edge/wasm bundle): same name and shape, but not instanceof the class
 * imported from runtime/library.
 */
class ForeignPrismaClientKnownRequestError extends Error {
	readonly code: string;
	readonly clientVersion = CLIENT_VERSION;
	constructor(message: string, code: string) {
		super(message);
		this.name = "PrismaClientKnownRequestError";
		this.code = code;
	}
}

interface ErrorBody {
	success: boolean;
	error: {
		code: string;
		message: string;
		details?: Record<string, unknown>;
		timestamp: string;
		requestId: string;
		path?: string;
	};
	meta: { version: string; environment: string };
}

async function readBody(response: NextResponse): Promise<ErrorBody> {
	return (await response.json()) as ErrorBody;
}

// Strings that must never reach a production response body.
const INTERNAL_MARKERS = [
	"prisma",
	"Prisma",
	"plugin_versions",
	"findUnique",
	"invocation",
	"stack",
	" at ",
	"PrismaClient",
];

function setNodeEnv(value: string) {
	Object.defineProperty(process.env, "NODE_ENV", {
		value,
		configurable: true,
		writable: true,
		enumerable: true,
	});
}

describe("error classification and responses", () => {
	const originalNodeEnv = process.env.NODE_ENV;
	let errorSpy: jest.SpiedFunction<typeof logger.error>;

	beforeEach(() => {
		errorSpy = jest.spyOn(logger, "error").mockImplementation(() => {});
		jest.spyOn(logger, "warn").mockImplementation(() => {});
		jest.spyOn(logger, "info").mockImplementation(() => {});
	});

	afterEach(() => {
		jest.restoreAllMocks();
		setNodeEnv(originalNodeEnv ?? "test");
	});

	describe("classifyError", () => {
		it.each([
			["P2025", 404, "NOT_FOUND"],
			["P2001", 404, "NOT_FOUND"],
			["P2002", 409, "CONFLICT"],
			["P2003", 409, "CONFLICT"],
			["P2000", 422, "VALIDATION_ERROR"],
			["P2011", 422, "VALIDATION_ERROR"],
			["P2021", 503, "SERVICE_UNAVAILABLE"],
			["P2022", 503, "SERVICE_UNAVAILABLE"],
			["P2024", 503, "SERVICE_UNAVAILABLE"],
			["P1001", 503, "SERVICE_UNAVAILABLE"],
			["P1002", 503, "SERVICE_UNAVAILABLE"],
			["P1008", 503, "SERVICE_UNAVAILABLE"],
			["P1017", 503, "SERVICE_UNAVAILABLE"],
			["P6004", 503, "SERVICE_UNAVAILABLE"],
			["P2010", 500, "INTERNAL_ERROR"],
			["P2023", 500, "INTERNAL_ERROR"],
		])("maps known request error %s to %i %s", (code, status, errorCode) => {
			const result = classifyError(knownError(code));
			expect(result.status).toBe(status);
			expect(result.code).toBe(errorCode);
			expect(result.prismaCode).toBe(code);
			expect(result.clientError).toBe(status < 500);
		});

		it.each([
			[
				"PrismaClientInitializationError",
				new PrismaClientInitializationError(
					"Can't reach database server at db.internal:5432",
					CLIENT_VERSION,
					"P1001",
				),
				503,
				"SERVICE_UNAVAILABLE",
			],
			[
				"PrismaClientUnknownRequestError",
				new PrismaClientUnknownRequestError("driver exploded", {
					clientVersion: CLIENT_VERSION,
				}),
				503,
				"SERVICE_UNAVAILABLE",
			],
			[
				"PrismaClientRustPanicError",
				new PrismaClientRustPanicError("engine panicked", CLIENT_VERSION),
				500,
				"INTERNAL_ERROR",
			],
			[
				"PrismaClientValidationError",
				new PrismaClientValidationError(
					"Invalid `prisma.plugin.findMany()` invocation: Unknown argument `bogus`",
					{ clientVersion: CLIENT_VERSION },
				),
				500,
				"INTERNAL_ERROR",
			],
		])("maps %s to %i %s", (_name, error, status, errorCode) => {
			const result = classifyError(error);
			expect(result.status).toBe(status);
			expect(result.code).toBe(errorCode);
			expect(result.clientError).toBe(false);
		});

		it("classifies Prisma errors from another runtime copy by name", () => {
			const foreign = new ForeignPrismaClientKnownRequestError(
				MISSING_TABLE_MESSAGE,
				"P2021",
			);
			expect(foreign).not.toBeInstanceOf(PrismaClientKnownRequestError);
			expect(classifyError(foreign)).toMatchObject({
				status: 503,
				code: "SERVICE_UNAVAILABLE",
				prismaCode: "P2021",
			});
		});

		it("does not turn an error whose message says 'invalid' into a 4xx", () => {
			const result = classifyError(new Error("Invalid state in cache layer"));
			expect(result.status).toBe(500);
			expect(result.code).toBe("INTERNAL_ERROR");
		});

		it("keeps explicit request validation errors as 422 with their message", () => {
			const result = classifyError(
				new RequestValidationError("version must be semver"),
			);
			expect(result).toMatchObject({
				status: 422,
				code: "VALIDATION_ERROR",
				message: "version must be semver",
				clientError: true,
			});
		});

		it("uses the resource name in client-facing messages", () => {
			expect(
				classifyError(knownError("P2025"), { resource: "Plugin" }),
			).toMatchObject({ message: "Plugin not found" });
			expect(
				classifyError(knownError("P2002"), { resource: "Plugin" }),
			).toMatchObject({ message: "Plugin already exists" });
		});
	});

	describe("isRetryableDatabaseError", () => {
		it.each(["P1001", "P1002", "P1008", "P1017", "P2024", "P2034"])(
			"retries transient code %s",
			(code) => {
				expect(isRetryableDatabaseError(knownError(code))).toBe(true);
			},
		);

		it.each(["P2002", "P2003", "P2025", "P2021", "P2022", "P2010"])(
			"does not retry %s",
			(code) => {
				expect(isRetryableDatabaseError(knownError(code))).toBe(false);
			},
		);

		it("does not retry query validation errors or panics", () => {
			expect(
				isRetryableDatabaseError(
					new PrismaClientValidationError("bad query", {
						clientVersion: CLIENT_VERSION,
					}),
				),
			).toBe(false);
			expect(
				isRetryableDatabaseError(
					new PrismaClientRustPanicError("panic", CLIENT_VERSION),
				),
			).toBe(false);
		});
	});

	describe("safeDatabase", () => {
		it("fails fast on a missing table instead of retrying", async () => {
			const error = knownError("P2021", MISSING_TABLE_MESSAGE);
			const operation = jest
				.fn<() => Promise<never>>()
				.mockRejectedValue(error);

			await expect(
				safeDatabase(operation, { operation: "fetch-plugin-version" }),
			).rejects.toBe(error);
			expect(operation).toHaveBeenCalledTimes(1);
		});
	});

	describe("production responses", () => {
		beforeEach(() => setNodeEnv("production"));

		it("answers the missing-table case with a generic 503", async () => {
			const error = new ForeignPrismaClientKnownRequestError(
				MISSING_TABLE_MESSAGE,
				"P2021",
			);
			const response = RegistryErrorHandler.handleError(error, {
				operation: "fetch-plugin-version",
				requestId: "req-prod-1",
			});
			const body = await readBody(response);

			expect(response.status).toBe(503);
			expect(response.headers.get("X-Error-Code")).toBe("SERVICE_UNAVAILABLE");
			expect(response.headers.get("X-Request-ID")).toBe("req-prod-1");
			expect(body).toMatchObject({
				success: false,
				error: {
					code: "SERVICE_UNAVAILABLE",
					message: "Service temporarily unavailable, please retry later",
					requestId: "req-prod-1",
					path: "fetch-plugin-version",
					timestamp: expect.any(String),
				},
				meta: { environment: "production" },
			});
			expect(body.error.details).toBeUndefined();
		});

		it.each([
			knownError("P2021", MISSING_TABLE_MESSAGE),
			knownError("P2022", "The column `plugins.sdk_version` does not exist"),
			knownError("P2010", "Raw query failed: SELECT * FROM plugins"),
			new PrismaClientInitializationError(
				"Can't reach database server at db.internal:5432",
				CLIENT_VERSION,
				"P1001",
			),
			new PrismaClientUnknownRequestError("driver exploded", {
				clientVersion: CLIENT_VERSION,
			}),
			new PrismaClientRustPanicError("engine panicked", CLIENT_VERSION),
			new PrismaClientValidationError(
				"Invalid `prisma.plugin.findMany()` invocation",
				{ clientVersion: CLIENT_VERSION },
			),
			new Error("Validation failed: invalid input at prisma.plugin"),
			new TypeError("Cannot read properties of undefined (reading 'stack')"),
		])("does not leak internals for %p", async (error) => {
			const response = RegistryErrorHandler.handleError(error, {
				operation: "fetch-plugin-version",
			});
			const raw = JSON.stringify(await readBody(response));

			expect(response.status).toBeGreaterThanOrEqual(500);
			for (const marker of INTERNAL_MARKERS) {
				expect(raw).not.toContain(marker);
			}
			expect(raw).not.toContain(error.message);
		});

		it("keeps client-error responses free of Prisma details", async () => {
			const response = RegistryErrorHandler.handleError(
				knownError("P2025", "No PluginVersion found in table plugin_versions"),
				{ operation: "fetch-plugin-version", resource: "Plugin version" },
			);
			const body = await readBody(response);

			expect(response.status).toBe(404);
			expect(body.error.message).toBe("Plugin version not found");
			expect(JSON.stringify(body)).not.toContain("plugin_versions");
			expect(body.error.details).toBeUndefined();
		});

		it("strips caller-supplied details from 5xx responses", async () => {
			const response = createApiError(
				"Database operation failed",
				500,
				undefined,
				{ query: "SELECT * FROM plugins" },
				"test",
			);
			const body = await readBody(response);
			expect(body.error.details).toBeUndefined();
		});

		it("keeps details on 4xx responses", async () => {
			const response = createApiError(
				"Invalid platform format",
				400,
				undefined,
				{ field: "platform" },
			);
			const body = await readBody(response);
			expect(body.error.details).toEqual({ field: "platform" });
		});
	});

	describe("development responses", () => {
		beforeEach(() => setNodeEnv("development"));

		it("includes the underlying error to help debugging", async () => {
			const response = RegistryErrorHandler.handleError(
				knownError("P2021", MISSING_TABLE_MESSAGE),
				{ operation: "fetch-plugin-version" },
			);
			const body = await readBody(response);

			expect(response.status).toBe(503);
			expect(body.error.message).toBe(
				"Service temporarily unavailable, please retry later",
			);
			expect(body.error.details).toMatchObject({
				errorName: "PrismaClientKnownRequestError",
				errorMessage: MISSING_TABLE_MESSAGE,
				prismaCode: "P2021",
			});
		});
	});

	describe("request ids", () => {
		it("logs the internal error under the same requestId the client sees", async () => {
			setNodeEnv("production");
			const response = RegistryErrorHandler.handleError(
				knownError("P2021", MISSING_TABLE_MESSAGE),
				{ operation: "fetch-plugin-version" },
			);
			const body = await readBody(response);
			const requestId = body.error.requestId;

			expect(requestId).toMatch(/^[0-9a-f-]{36}$/);
			expect(response.headers.get("X-Request-ID")).toBe(requestId);
			expect(errorSpy).toHaveBeenCalledWith(
				"Prisma database error",
				expect.objectContaining({
					requestId,
					errorCode: "P2021",
					errorMessage: MISSING_TABLE_MESSAGE,
					operation: "fetch-plugin-version",
				}),
				expect.any(PrismaClientKnownRequestError),
			);
		});

		it("propagates a well-formed x-request-id through withErrorHandling", async () => {
			const handler = withErrorHandling(
				async () => {
					throw knownError("P2021", MISSING_TABLE_MESSAGE);
				},
				"fetch-plugin-version",
				false,
			);
			const response = await handler(
				new NextRequest(
					"https://registry.devex.sh/api/v1/plugins/x/versions/latest",
					{ headers: { "x-request-id": "edge-abc123" } },
				),
			);
			const body = await readBody(response);

			expect(response.status).toBe(503);
			expect(body.error.requestId).toBe("edge-abc123");
			expect(errorSpy).toHaveBeenCalledWith(
				"Prisma database error",
				expect.objectContaining({ requestId: "edge-abc123" }),
				expect.any(Error),
			);
		});

		it("replaces malformed request ids", () => {
			expect(resolveRequestId("req-1")).toBe("req-1");
			expect(resolveRequestId("bad id\nwith newline")).toMatch(
				/^[0-9a-f-]{36}$/,
			);
			expect(resolveRequestId(null)).toMatch(/^[0-9a-f-]{36}$/);
			expect(resolveRequestId("x".repeat(200))).toMatch(/^[0-9a-f-]{36}$/);
		});
	});
});
