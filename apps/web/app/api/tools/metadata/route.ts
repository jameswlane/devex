import { NextResponse } from "next/server";
import toolsData from "../../../generated/tools.json";
import {
	AppError,
	formatErrorMessage,
	logError,
} from "../../../utils/error-handling";

export async function GET() {
	try {
		return NextResponse.json({
			categories: toolsData.categories,
			stats: toolsData.stats,
			generated: toolsData.generated,
		});
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
