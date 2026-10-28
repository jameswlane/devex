import type { NextRequest } from "next/server";
import { createApiError, ERROR_CODES } from "@/lib/logger";

type CatchAllContext = {
	params: Promise<{ path?: string[] }>;
};

/**
 * Catch-all for unknown /api/* routes.
 *
 * More specific routes under app/api always win over this segment, so this
 * handler only runs when no other API route matched. It answers every HTTP
 * method with the registry's standard JSON error envelope instead of the
 * default HTML 404 page, so JSON clients (the CLI, automation) always get a
 * parseable response.
 */
async function handleApiNotFound(
	request: NextRequest,
	context: CatchAllContext,
) {
	await context.params;

	// JSON.stringify in NextResponse.json escapes the pathname, so echoing it
	// back here cannot produce reflected content.
	const pathname = new URL(request.url).pathname;

	const response = createApiError(
		`API route not found: ${pathname}`,
		404,
		ERROR_CODES.NOT_FOUND,
		undefined,
		pathname,
	);
	response.headers.set("Cache-Control", "no-store");
	return response;
}

export const GET = handleApiNotFound;
export const HEAD = handleApiNotFound;
export const POST = handleApiNotFound;
export const PUT = handleApiNotFound;
export const PATCH = handleApiNotFound;
export const DELETE = handleApiNotFound;
export const OPTIONS = handleApiNotFound;
