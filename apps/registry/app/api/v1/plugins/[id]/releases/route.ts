import { NextResponse } from "next/server";
import {
	AuditEventType,
	extractAuditContext,
	initializeAuditLogger,
} from "@/lib/audit-log";
import { invalidateCdnTags, pluginCacheTags } from "@/lib/cdn-cache";
import { logger } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { releasePayloadSchema } from "@/lib/releases/contract";
import { recordPluginRelease } from "@/lib/releases/record";
import { verifyReleaseSignature } from "@/lib/releases/verify-signature";
import type { PrismaClient } from "../../../../../../generated/prisma/client";

export const runtime = "nodejs";
const MAX_BODY_BYTES = 1024 * 1024;
const headers = { "Cache-Control": "no-store" };

function errorResponse(
	status: number,
	code: string,
	message: string,
	issues?: unknown,
): NextResponse {
	return NextResponse.json(
		{
			success: false,
			error: { code, message, ...(issues !== undefined ? { issues } : {}) },
		},
		{ status, headers },
	);
}

/** Read the body as text, or return null once it exceeds maxBytes. */
async function readBodyWithLimit(
	request: Request,
	maxBytes: number,
): Promise<string | null> {
	if (!request.body) return "";
	const reader = request.body.getReader();
	const chunks: Uint8Array[] = [];
	let total = 0;
	for (;;) {
		const { done, value } = await reader.read();
		if (done) break;
		total += value.byteLength;
		if (total > maxBytes) {
			await reader.cancel();
			return null;
		}
		chunks.push(value);
	}
	return Buffer.concat(chunks).toString("utf8");
}

export async function POST(
	request: Request,
	{ params }: { params: Promise<{ id: string }> },
): Promise<NextResponse> {
	try {
		const contentLength = request.headers.get("content-length");
		if (contentLength && Number(contentLength) > MAX_BODY_BYTES) {
			return errorResponse(
				413,
				"PAYLOAD_TOO_LARGE",
				"Release body exceeds 1 MB",
			);
		}
		// content-length can be missing or wrong, so also count the bytes read.
		const rawBody = await readBodyWithLimit(request, MAX_BODY_BYTES);
		if (rawBody === null) {
			return errorResponse(
				413,
				"PAYLOAD_TOO_LARGE",
				"Release body exceeds 1 MB",
			);
		}
		const verified = verifyReleaseSignature(request.headers, rawBody);
		if (!verified.ok) {
			return errorResponse(verified.status, verified.code, verified.message);
		}
		let parsed: unknown;
		try {
			parsed = JSON.parse(rawBody);
		} catch {
			return errorResponse(400, "VALIDATION_ERROR", "Invalid JSON body");
		}
		const validated = releasePayloadSchema.safeParse(parsed);
		if (!validated.success) {
			return errorResponse(
				400,
				"VALIDATION_ERROR",
				"Invalid release payload",
				validated.error.issues,
			);
		}
		const { id } = await params;
		const payload = validated.data;
		if (payload.plugin !== id) {
			return errorResponse(
				400,
				"VALIDATION_ERROR",
				"Payload plugin must match the URL plugin",
			);
		}
		const result = await recordPluginRelease(
			prisma as unknown as PrismaClient,
			payload,
			"webhook",
		);
		await invalidateCdnTags(pluginCacheTags(payload.plugin));
		const status = result.versionCreated ? 201 : 200;
		try {
			await initializeAuditLogger(prisma as unknown as PrismaClient).log(
				AuditEventType.ADMIN_ACTION,
				"record-plugin-release",
				{ ...extractAuditContext(request), clientId: "release-webhook" },
				{
					resourceType: "plugin",
					resourceId: result.pluginId,
					result: "success",
					details: {
						method: request.method,
						path: new URL(request.url).pathname,
						statusCode: status,
						plugin: payload.plugin,
						version: payload.version,
						latestChanged: result.latestChanged,
					},
				},
			);
		} catch (error) {
			logger.warn("Failed to record release webhook audit entry", {
				error: error instanceof Error ? error.message : String(error),
			});
		}
		return NextResponse.json(
			{
				success: true,
				data: {
					plugin: payload.plugin,
					version: payload.version,
					latestChanged: result.latestChanged,
				},
			},
			{ status, headers },
		);
	} catch (error) {
		logger.error("Failed to record plugin release", {
			error: error instanceof Error ? error.message : String(error),
		});
		return errorResponse(
			500,
			"INTERNAL_ERROR",
			"Failed to record plugin release",
		);
	}
}
