import crypto from "node:crypto";
import type { PrismaClient } from "@prisma/client";
import { NextResponse } from "next/server";
import {
	AuditEventType,
	extractAuditContext,
	initializeAuditLogger,
} from "@/lib/audit-log";
import { logger } from "@/lib/logger";
import { prisma } from "@/lib/prisma";

/**
 * Shared bearer-token guard for registry write and admin endpoints.
 *
 * - REGISTRY_ADMIN_API_KEY protects data writes (applications, plugins).
 * - REVALIDATE_API_KEY protects cache administration (revalidate, cache, health refresh).
 *
 * The guard fails closed: if the secret is not configured the endpoint
 * answers 503 instead of silently accepting or rejecting every request.
 */
export const ADMIN_API_KEY_ENV = "REGISTRY_ADMIN_API_KEY";
export const REVALIDATE_API_KEY_ENV = "REVALIDATE_API_KEY";

export type ApiKeyEnvVar =
	| typeof ADMIN_API_KEY_ENV
	| typeof REVALIDATE_API_KEY_ENV;

export interface AuditOptions {
	action: string;
	resourceType: string;
}

function errorResponse(
	status: number,
	code: string,
	message: string,
): NextResponse {
	const headers: Record<string, string> = { "Cache-Control": "no-store" };
	if (status === 401) {
		headers["WWW-Authenticate"] = 'Bearer realm="devex-registry"';
	}

	return NextResponse.json(
		{
			success: false,
			error: {
				code,
				message,
				timestamp: new Date().toISOString(),
				requestId: crypto.randomUUID(),
			},
		},
		{ status, headers },
	);
}

/**
 * Extract the token from an `Authorization: Bearer <token>` header.
 */
export function extractBearerToken(header: string | null): string | null {
	if (!header) return null;
	const match = /^Bearer\s+(\S+)\s*$/i.exec(header.trim());
	return match ? match[1] : null;
}

/**
 * Constant-time comparison that does not leak the secret's length:
 * both values are hashed to a fixed-size digest before timingSafeEqual.
 */
export function tokensMatch(provided: string, expected: string): boolean {
	const providedDigest = crypto.createHash("sha256").update(provided).digest();
	const expectedDigest = crypto.createHash("sha256").update(expected).digest();
	return crypto.timingSafeEqual(providedDigest, expectedDigest);
}

/**
 * Validate the request's bearer token against the secret in `envVar`.
 * Returns an error response when access is denied, or null when allowed.
 */
export function checkApiKey(
	request: Request,
	envVar: ApiKeyEnvVar,
): NextResponse | null {
	const secret = process.env[envVar];
	if (!secret) {
		logger.error("API key guard misconfigured: secret is not set", { envVar });
		return errorResponse(
			503,
			"CONFIGURATION_ERROR",
			"This endpoint is disabled until the server API key is configured",
		);
	}

	const token = extractBearerToken(request.headers.get("authorization"));
	if (!token || !tokensMatch(token, secret)) {
		logger.warn("Unauthorized request to protected endpoint", {
			envVar,
			method: request.method,
			url: request.url,
			ip: request.headers.get("x-forwarded-for") || "unknown",
			tokenProvided: token !== null,
		});
		return errorResponse(
			401,
			"UNAUTHORIZED",
			"Unauthorized - a valid bearer token is required",
		);
	}

	return null;
}

async function recordAuditEntry(
	request: Request,
	envVar: ApiKeyEnvVar,
	audit: AuditOptions,
	response: Response,
	routeContext: unknown,
): Promise<void> {
	try {
		const params = await (
			routeContext as { params?: Promise<Record<string, string>> } | undefined
		)?.params;
		const auditLogger = initializeAuditLogger(
			prisma as unknown as PrismaClient,
		);
		await auditLogger.log(
			AuditEventType.ADMIN_ACTION,
			audit.action,
			{ ...extractAuditContext(request), clientId: `api-key:${envVar}` },
			{
				resourceType: audit.resourceType,
				resourceId: params?.id,
				result: "success",
				details: {
					method: request.method,
					path: new URL(request.url).pathname,
					statusCode: response.status,
				},
			},
		);
	} catch (error) {
		// Auditing must never turn a successful write into a failure
		logger.warn("Failed to record audit log entry", {
			action: audit.action,
			error: error instanceof Error ? error.message : String(error),
		});
	}
}

/**
 * Wrap a route handler so it only runs for requests carrying the right bearer token.
 * When `audit` is given, successful (2xx) responses are recorded in the audit log.
 */
export function withApiKey<
	Req extends Request,
	Args extends unknown[],
	Res extends Response,
>(
	envVar: ApiKeyEnvVar,
	handler: (request: Req, ...args: Args) => Promise<Res>,
	audit?: AuditOptions,
): (request: Req, ...args: Args) => Promise<Res | NextResponse> {
	return async function protectedHandler(
		request: Req,
		...args: Args
	): Promise<Res | NextResponse> {
		const denied = checkApiKey(request, envVar);
		if (denied) return denied;

		const response = await handler(request, ...args);
		if (audit && response.ok) {
			await recordAuditEntry(request, envVar, audit, response, args[0]);
		}
		return response;
	};
}
