/**
 * Centralized error handling utilities for consistent error management
 * across the application
 */

export class AppError extends Error {
	constructor(
		message: string,
		public code: string = "UNKNOWN_ERROR",
		public statusCode: number = 500,
		public context?: Record<string, unknown>,
	) {
		super(message);
		this.name = "AppError";
	}
}

export class NetworkError extends AppError {
	constructor(message: string, context?: Record<string, unknown>) {
		super(message, "NETWORK_ERROR", 503, context);
		this.name = "NetworkError";
	}
}

export class ValidationError extends AppError {
	constructor(message: string, context?: Record<string, unknown>) {
		super(message, "VALIDATION_ERROR", 400, context);
		this.name = "ValidationError";
	}
}

export class NotFoundError extends AppError {
	constructor(message: string, context?: Record<string, unknown>) {
		super(message, "NOT_FOUND_ERROR", 404, context);
		this.name = "NotFoundError";
	}
}

/**
 * Formats error messages for user display
 */
export function formatErrorMessage(error: unknown): string {
	if (error instanceof AppError) {
		return error.message;
	}

	if (error instanceof Error) {
		return error.message;
	}

	if (typeof error === "string") {
		return error;
	}

	return "An unexpected error occurred";
}

/**
 * Logs errors with context for debugging
 */
export function logError(
	error: unknown,
	context?: Record<string, unknown>,
): void {
	const errorInfo = {
		message: formatErrorMessage(error),
		stack: error instanceof Error ? error.stack : undefined,
		context,
		timestamp: new Date().toISOString(),
	};

	console.error("Application Error:", errorInfo);
}

/**
 * Safe error handler for async operations
 */
export async function safeAsync<T>(
	operation: () => Promise<T>,
	fallback?: T,
	context?: Record<string, unknown>,
): Promise<T | undefined> {
	try {
		return await operation();
	} catch (error) {
		logError(error, context);
		return fallback;
	}
}

/**
 * Retry mechanism for operations that might fail temporarily
 */
export async function withRetry<T>(
	operation: () => Promise<T>,
	maxAttempts: number = 3,
	delayMs: number = 1000,
	context?: Record<string, unknown>,
): Promise<T> {
	let lastError: unknown;

	for (let attempt = 1; attempt <= maxAttempts; attempt++) {
		try {
			return await operation();
		} catch (error) {
			lastError = error;

			if (attempt === maxAttempts) {
				logError(error, { ...context, attempt, maxAttempts });
				throw error;
			}

			// Wait before retry
			await new Promise((resolve) => setTimeout(resolve, delayMs * attempt));
		}
	}

	throw lastError;
}

/**
 * Error boundary helper for React components
 */
export interface ErrorBoundaryState {
	hasError: boolean;
	error?: Error;
	errorInfo?: React.ErrorInfo;
}

export function handleComponentError(
	error: Error,
	errorInfo: React.ErrorInfo,
): ErrorBoundaryState {
	logError(error, {
		componentStack: errorInfo.componentStack,
		errorBoundary: true,
	});

	return {
		hasError: true,
		error,
		errorInfo,
	};
}
