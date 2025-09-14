export function logDatabaseError(error: any, context: string) {
	const errorInfo = {
		context,
		message: error.message || "Unknown error",
		code: error.code || "UNKNOWN",
		timestamp: new Date().toISOString(),
		stack: process.env.NODE_ENV === "development" ? error.stack : undefined,
	};

	console.error("Database error:", errorInfo);
}

export function createApiError(message: string, status: number = 500) {
	return new Response(
		JSON.stringify({
			error: message,
			timestamp: new Date().toISOString(),
		}),
		{
			status,
			headers: { "Content-Type": "application/json" },
		},
	);
}
