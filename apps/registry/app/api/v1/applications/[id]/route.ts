import { NextRequest, NextResponse } from "next/server";
import { createApiError } from "@/lib/logger";
import { prisma } from "@/lib/prisma";
import { withErrorHandling, safeDatabase } from "@/lib/error-handler";
import { invalidateOnDataChange, withCacheInvalidation } from "@/lib/cache-invalidation";
import { createOptimizedResponse, ResponseType } from "@/lib/response-optimization";
import { Prisma } from "@prisma/client";

// GET /api/v1/applications/[id] - Get a specific application by name
async function handleGetApplication(
	request: NextRequest,
	{ params }: { params: { id: string } }
): Promise<NextResponse> {
	const { id: applicationName } = params;

	const application = await safeDatabase(
		() => prisma.application.findUnique({
			where: { name: applicationName },
		}),
		{
			operation: "fetch-application",
			resource: "application",
			metadata: { name: applicationName }
		}
	);

	if (!application) {
		return createApiError("Application not found", 404);
	}

	// Transform the JSON platforms field to the expected format
	const platforms = application.platforms as any;
	const response = {
		...application,
		platforms: {
			linux: platforms?.linux || null,
			macos: platforms?.macos || null,
			windows: platforms?.windows || null,
		}
	};

	return NextResponse.json(response);
}

// PUT /api/v1/applications/[id] - Update an application by name
async function handleUpdateApplication(
	request: NextRequest,
	{ params }: { params: { id: string } }
): Promise<NextResponse> {
	const { id: applicationName } = params;
	const body = await request.json();

	// Validate and sanitize input
	const updateData: Prisma.ApplicationUpdateInput = {
		...(body.description && { description: body.description }),
		...(body.category && { category: body.category }),
		...(body.official !== undefined && { official: body.official }),
		...(body.default !== undefined && { default: body.default }),
		...(body.tags && { tags: body.tags }),
		...(body.desktopEnvironments && { desktopEnvironments: body.desktopEnvironments }),
		...(body.platforms && { platforms: body.platforms }),
	};

	const application = await safeDatabase(
		async () => {
			// Update the application by name
			const updated = await prisma.application.update({
				where: { name: applicationName },
				data: updateData,
			});

			// Invalidate caches after successful update
			await invalidateOnDataChange("update", "application", updated.id);

			return updated;
		},
		{
			operation: "update-application",
			resource: "application",
			metadata: { name: applicationName }
		}
	);

	if (!application) {
		return createApiError("Application not found", 404);
	}

	return NextResponse.json(application);
}

// DELETE /api/v1/applications/[id] - Delete an application by name
async function handleDeleteApplication(
	request: NextRequest,
	{ params }: { params: { id: string } }
): Promise<NextResponse> {
	const { id: applicationName } = params;

	const result = await safeDatabase(
		async () => {
			// Delete the application by name
			const deleted = await prisma.application.delete({
				where: { name: applicationName },
			});

			// Invalidate caches after successful deletion
			await invalidateOnDataChange("delete", "application", deleted.id);

			return deleted;
		},
		{
			operation: "delete-application",
			resource: "application",
			metadata: { name: applicationName }
		}
	);

	if (!result) {
		return createApiError("Application not found", 404);
	}

	return NextResponse.json({ success: true, message: "Application deleted successfully" });
}

// Export wrapped handlers with error handling
export const GET = withErrorHandling(handleGetApplication, "fetch-application");
export const PUT = withErrorHandling(handleUpdateApplication, "update-application");
export const DELETE = withErrorHandling(handleDeleteApplication, "delete-application");