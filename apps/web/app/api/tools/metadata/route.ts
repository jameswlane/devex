import { NextResponse } from "next/server";
import toolsData from "../../../generated/tools.json";
import {
	AppError,
	formatErrorMessage,
	logError,
} from "../../../utils/error-handling";

export async function GET() {
	try {
		return NextResponse.json(
			{
				categories: toolsData.categories,
				stats: toolsData.stats,
				generated: toolsData.generated,
			},
			{
				headers: {
					"Cache-Control": "public, max-age=3600, s-maxage=86400",
					"CDN-Cache-Control": "public, max-age=86400",
					"Vercel-CDN-Cache-Control": "public, max-age=86400",
				},
			},
		);
	} catch (error) {
		logError(error, { endpoint: "/api/tools/metadata" });

		if (error instanceof AppError) {
			return NextResponse.json(
				{ error: formatErrorMessage(error), code: error.code },
				{ status: error.statusCode },
			);
		}

		return NextResponse.json(
			{ error: "Internal server error", code: "INTERNAL_ERROR" },
			{ status: 500 },
		);
	}
}
