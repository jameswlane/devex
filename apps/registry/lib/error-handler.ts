import type { NextRequest, NextResponse } from "next/server";
import {
	createApiError,
	createValidationError,
	ERROR_CODES,
	isProductionEnvironment,
	logger,
} from "./logger";

// Standard error types for the registry
export enum RegistryErrorType {
	DATABASE = "DATABASE",
	VALIDATION = "VALIDATION",
	CACHE = "CACHE",
	RATE_LIMIT = "RATE_LIMIT",
	AUTHENTICATION = "AUTHENTICATION",
	AUTHORIZATION = "AUTHORIZATION",
	NOT_FOUND = "NOT_FOUND",
	TRANSFORMATION = "TRANSFORMATION",
	EXTERNAL_SERVICE = "EXTERNAL_SERVICE",
	CONFIGURATION = "CONFIGURATION",
}

// Error context for better debugging
export interface ErrorContext {
	operation: string;
	resource?: string;
	userId?: string;
	requestId?: string;
	metadata?: Record<string, any>;
}

// Standard error handling configuration
export interface ErrorHandlerConfig {
	logError?: boolean;
	includeStack?: boolean;
	customMessage?: string;
	statusCode?: number;
}

/**
 * Thrown by route code for input the client got wrong. Its message is written
 * by us, so it is safe to return to the caller as-is.
 */
export class RequestValidationError extends Error {
	constructor(
		message: string,
		public readonly details?: Record<string, any>,
	) {
		super(message);
		this.name = "RequestValidationError";
	}
}

// Public messages for server-side failures; never derived from the error itself.
export const SERVICE_UNAVAILABLE_MESSAGE =
	"Service temporarily unavailable, please retry later";
export const INTERNAL_ERROR_MESSAGE = "An unexpected error occurred";

/**
 * Prisma error classes, matched by name rather than instanceof: the bundled
 * route code and the generated client can load different copies of the Prisma
 * runtime (library, edge, wasm), so instanceof against one copy misses errors
 * thrown by another. That miss is what turned a missing table into a 422.
 */
const PRISMA_ERROR_NAMES = [
	"PrismaClientKnownRequestError",
	"PrismaClientUnknownRequestError",
	"PrismaClientInitializationError",
	"PrismaClientRustPanicError",
	"PrismaClientValidationError",
] as const;

export type PrismaErrorName = (typeof PRISMA_ERROR_NAMES)[number];

export interface PrismaLikeError extends Error {
	name: PrismaErrorName;
	code?: string;
	errorCode?: string;
	meta?: Record<string, unknown>;
}

export function isPrismaError(error: unknown): error is PrismaLikeError {
	return (
		error instanceof Error &&
		(PRISMA_ERROR_NAMES as readonly string[]).includes(error.name)
	);
}

/** Prisma error code from a known request error (code) or init error (errorCode). */
export function getPrismaErrorCode(error: unknown): string | undefined {
	if (!isPrismaError(error)) return undefined;
	const code = error.code ?? error.errorCode;
	return typeof code === "string" ? code : undefined;
}

// Codes that describe something wrong with the client's request.
const NOT_FOUND_CODES = new Set(["P2001", "P2015", "P2018", "P2025"]);
const CONFLICT_CODES = new Set(["P2002", "P2003", "P2014"]);
const INVALID_INPUT_CODES = new Set([
	"P2000", // value too long for column
	"P2005", // invalid value for field type
	"P2006", // invalid value
	"P2007", // data validation error
	"P2011", // null constraint violation
	"P2012", // missing required value
	"P2013", // missing required argument
	"P2019", // input error
	"P2020", // value out of range
]);

// Codes where the database is unreachable, overloaded, or not yet migrated.
// Retrying later (or after a deploy finishes migrating) can succeed.
const TRANSIENT_CODES = new Set([
	"P1001", // can't reach database server
	"P1002", // database server timed out
	"P1008", // operations timed out
	"P1017", // server closed the connection
	"P2021", // table does not exist
	"P2022", // column does not exist
	"P2024", // connection pool timeout
	"P2034", // transaction conflict / deadlock
]);

// Of the transient codes, the ones worth retrying within the same request.
const RETRYABLE_CODES = new Set([
	"P1001",
	"P1002",
	"P1008",
	"P1017",
	"P2024",
	"P2034",
]);

export interface ErrorClassification {
	status: number;
	code: string;
	/** Message that is safe to return to any client. */
	message: string;
	/** True when the client caused the error (4xx). */
	clientError: boolean;
	prismaCode?: string;
}

function isTransientPrismaCode(code: string): boolean {
	// P1xxx: connection/engine, P5xxx: data proxy, P6xxx: Accelerate.
	return TRANSIENT_CODES.has(code) || /^P[156]\d{3}$/.test(code);
}

/**
 * Map an error to an HTTP status, error code and a client-safe message.
 * Only errors that really are the client's fault come back as 4xx.
 */
export function classifyError(
	error: unknown,
	context: Pick<ErrorContext, "resource"> = {},
): ErrorClassification {
	const resource = context.resource || "Resource";

	if (error instanceof RequestValidationError) {
		return {
			status: 422,
			code: ERROR_CODES.VALIDATION_ERROR,
			message: error.message,
			clientError: true,
		};
	}

	if (isPrismaError(error)) {
		const prismaCode = getPrismaErrorCode(error);

		if (error.name === "PrismaClientKnownRequestError" && prismaCode) {
			if (NOT_FOUND_CODES.has(prismaCode)) {
				return {
					status: 404,
					code: ERROR_CODES.NOT_FOUND,
					message: `${resource} not found`,
					clientError: true,
					prismaCode,
				};
			}
			if (CONFLICT_CODES.has(prismaCode)) {
				return {
					status: 409,
					code: ERROR_CODES.CONFLICT,
					message:
						prismaCode === "P2002"
							? `${resource} already exists`
							: `${resource} conflicts with related records`,
					clientError: true,
					prismaCode,
				};
			}
			if (INVALID_INPUT_CODES.has(prismaCode)) {
				return {
					status: 422,
					code: ERROR_CODES.VALIDATION_ERROR,
					message: "Request contains invalid data",
					clientError: true,
					prismaCode,
				};
			}
		}

		// Initialization and unknown request errors come from the engine or
		// driver (connectivity, Accelerate, protocol) and are worth retrying.
		const transient =
			error.name === "PrismaClientInitializationError" ||
			error.name === "PrismaClientUnknownRequestError" ||
			(prismaCode !== undefined && isTransientPrismaCode(prismaCode));

		// Validation errors (malformed query), Rust panics and the remaining
		// known codes are bugs on our side, not the client's.
		return transient
			? {
					status: 503,
					code: ERROR_CODES.SERVICE_UNAVAILABLE,
					message: SERVICE_UNAVAILABLE_MESSAGE,
					clientError: false,
					prismaCode,
				}
			: {
					status: 500,
					code: ERROR_CODES.INTERNAL_ERROR,
					message: INTERNAL_ERROR_MESSAGE,
					clientError: false,
					prismaCode,
				};
	}

	return {
		status: 500,
		code: ERROR_CODES.INTERNAL_ERROR,
		message: INTERNAL_ERROR_MESSAGE,
		clientError: false,
	};
}

/** Whether safeDatabase should retry this error within the same request. */
export function isRetryableDatabaseError(error: unknown): boolean {
	if (isPrismaError(error)) {
		if (
			error.name === "PrismaClientInitializationError" ||
			error.name === "PrismaClientUnknownRequestError"
		) {
			return true;
		}
		const code = getPrismaErrorCode(error);
		return code !== undefined && RETRYABLE_CODES.has(code);
	}
	// Non-Prisma errors (network blips in the driver/adapter) keep the
	// historical retry behaviour.
	return true;
}

const REQUEST_ID_PATTERN = /^[A-Za-z0-9._:-]{1,128}$/;

/** Use the caller's x-request-id when it is well-formed, otherwise mint one. */
export function resolveRequestId(candidate?: string | null): string {
	return candidate && REQUEST_ID_PATTERN.test(candidate)
		? candidate
		: crypto.randomUUID();
}

function withRequestId(context: ErrorContext): ErrorContext & {
	requestId: string;
} {
	return { ...context, requestId: resolveRequestId(context.requestId) };
}

/** Debug details for non-production responses only. */
function debugDetails(error: unknown): Record<string, any> | undefined {
	if (isProductionEnvironment()) return undefined;
	if (!(error instanceof Error)) return { error: String(error) };
	return {
		errorName: error.name,
		errorMessage: error.message,
		prismaCode: getPrismaErrorCode(error),
	};
}

/**
 * Standardized error handler class for consistent error processing
 */
export class RegistryErrorHandler {
	/**
	 * Handle database errors with proper categorization
	 */
	static handleDatabaseError(
		error: unknown,
		context: ErrorContext,
		config: ErrorHandlerConfig = {},
	): NextResponse {
		const ctx = withRequestId(context);
		const { logError = true, customMessage } = config;
		const classification = classifyError(error, ctx);

		if (logError) {
			logger.error(
				isPrismaError(error)
					? "Prisma database error"
					: "Database operation failed",
				{
					...ctx,
					statusCode: classification.status,
					errorName: error instanceof Error ? error.name : typeof error,
					errorCode: classification.prismaCode,
					errorMeta: isPrismaError(error) ? error.meta : undefined,
					errorMessage: error instanceof Error ? error.message : String(error),
				},
				error instanceof Error ? error : undefined,
			);
		}

		const meta = isPrismaError(error) ? error.meta : undefined;
		const details: Record<string, any> = {
			...(classification.prismaCode === "P2002" && meta?.target !== undefined
				? { field: meta.target }
				: {}),
			...debugDetails(error),
		};

		return createApiError(
			customMessage || classification.message,
			classification.status,
			classification.code,
			Object.keys(details).length > 0 ? details : undefined,
			ctx.operation,
			{ requestId: ctx.requestId },
		);
	}

	/**
	 * Handle validation errors consistently
	 */
	static handleValidationError(
		message: string,
		validationErrors: Record<string, string[]>,
		context: ErrorContext,
	): NextResponse {
		const ctx = withRequestId(context);
		logger.warn("Validation error", {
			...ctx,
			validationErrors,
		});

		return createValidationError(
			message,
			{ fields: validationErrors },
			ctx.operation,
			{ requestId: ctx.requestId },
		);
	}

	/**
	 * Handle cache-related errors
	 */
	static handleCacheError(
		error: unknown,
		context: ErrorContext,
		fallbackValue?: any,
	): any {
		logger.warn("Cache operation failed", {
			...context,
			error: error instanceof Error ? error.message : String(error),
		});

		// For cache errors, we typically want to continue operation
		// and return fallback value rather than failing the request
		return fallbackValue;
	}

	/**
	 * Handle transformation/serialization errors
	 */
	static handleTransformationError(
		error: unknown,
		context: ErrorContext,
	): NextResponse {
		const ctx = withRequestId(context);
		logger.error(
			"Data transformation failed",
			{
				...ctx,
				error: error instanceof Error ? error.message : String(error),
			},
			error instanceof Error ? error : undefined,
		);

		return createApiError(
			"Failed to process data",
			500,
			ERROR_CODES.INTERNAL_ERROR,
			{ operation: ctx.operation, ...debugDetails(error) },
			ctx.operation,
			{ requestId: ctx.requestId },
		);
	}

	/**
	 * Handle external service errors (Redis, GitHub API, etc.)
	 */
	static handleExternalServiceError(
		serviceName: string,
		error: unknown,
		context: ErrorContext,
		config: ErrorHandlerConfig = {},
	): NextResponse {
		const ctx = withRequestId(context);
		const { logError = true, customMessage } = config;

		if (logError) {
			logger.error(
				`External service error: ${serviceName}`,
				{
					...ctx,
					service: serviceName,
					error: error instanceof Error ? error.message : String(error),
				},
				error instanceof Error ? error : undefined,
			);
		}

		return createApiError(
			customMessage || `External service temporarily unavailable`,
			503,
			ERROR_CODES.EXTERNAL_SERVICE_ERROR,
			{
				service: serviceName,
				operation: ctx.operation,
				...debugDetails(error),
			},
			ctx.operation,
			{ requestId: ctx.requestId },
		);
	}

	/**
	 * Generic error handler for unknown errors
	 */
	static handleUnknownError(
		error: unknown,
		context: ErrorContext,
	): NextResponse {
		const ctx = withRequestId(context);
		logger.error(
			"Unexpected error occurred",
			{
				...ctx,
				error: error instanceof Error ? error.message : String(error),
			},
			error instanceof Error ? error : undefined,
		);

		return createApiError(
			INTERNAL_ERROR_MESSAGE,
			500,
			ERROR_CODES.INTERNAL_ERROR,
			{ operation: ctx.operation, ...debugDetails(error) },
			ctx.operation,
			{ requestId: ctx.requestId },
		);
	}

	/**
	 * Smart error handler that categorizes errors automatically
	 */
	static handleError(
		error: unknown,
		context: ErrorContext,
		config: ErrorHandlerConfig = {},
	): NextResponse {
		const ctx = withRequestId(context);

		if (isPrismaError(error)) {
			return RegistryErrorHandler.handleDatabaseError(error, ctx, config);
		}

		if (error instanceof RequestValidationError) {
			logger.warn("Request validation failed", {
				...ctx,
				error: error.message,
			});
			return createValidationError(
				config.customMessage || error.message,
				error.details,
				ctx.operation,
				{ requestId: ctx.requestId },
			);
		}

		// Network/external service errors
		if (error instanceof Error) {
			const message = error.message.toLowerCase();

			if (message.includes("redis") || message.includes("cache")) {
				return RegistryErrorHandler.handleExternalServiceError(
					"Redis",
					error,
					ctx,
					config,
				);
			}

			if (
				message.includes("network") ||
				message.includes("fetch") ||
				message.includes("timeout") ||
				message.includes("connect")
			) {
				return RegistryErrorHandler.handleExternalServiceError(
					"External API",
					error,
					ctx,
					config,
				);
			}
		}

		// Anything else is our bug, not the client's: never guess a 4xx from
		// the wording of an error message.
		return RegistryErrorHandler.handleUnknownError(error, ctx);
	}
}

/**
 * Higher-order function to wrap API route handlers with standardized error handling and rate limiting
 */
export function withErrorHandling<T extends NextRequest>(
	handler: (request: T, ...args: any[]) => Promise<NextResponse>,
	operation: string,
	enableRateLimit: boolean = true,
) {
	return async function wrappedHandler(
		request: T,
		...args: any[]
	): Promise<NextResponse> {
		const startTime = Date.now();

		// Apply rate limiting if enabled
		if (enableRateLimit) {
			const { withRedisRateLimit, REDIS_RATE_LIMIT_CONFIGS } = await import(
				"./rate-limit-redis"
			);

			// Determine rate limit config based on operation
			let rateLimitConfig = REDIS_RATE_LIMIT_CONFIGS.default as any;
			if (operation.includes("sync")) {
				rateLimitConfig = REDIS_RATE_LIMIT_CONFIGS.sync;
			} else if (operation.includes("search")) {
				rateLimitConfig = REDIS_RATE_LIMIT_CONFIGS.search;
			} else if (operation.includes("registry")) {
				rateLimitConfig = REDIS_RATE_LIMIT_CONFIGS.registry;
			}

			// Apply rate limiting
			const rateLimitedHandler = withRedisRateLimit(
				async (req: NextRequest) => {
					try {
						const response = await handler(req as T, ...args);

						// Log successful requests
						const responseTime = Date.now() - startTime;
						logger.info("Request completed successfully", {
							operation,
							method: req.method,
							path: new URL(req.url).pathname,
							statusCode: response.status,
							responseTime,
						});

						return response;
					} catch (error) {
						const responseTime = Date.now() - startTime;
						const url = new URL(req.url);

						return RegistryErrorHandler.handleError(error, {
							operation,
							requestId: resolveRequestId(req.headers.get("x-request-id")),
							metadata: {
								method: req.method,
								path: url.pathname,
								responseTime,
							},
						});
					}
				},
				rateLimitConfig,
			);

			return rateLimitedHandler(request);
		}

		// No rate limiting - proceed with regular error handling
		try {
			const response = await handler(request, ...args);

			// Log successful requests
			const responseTime = Date.now() - startTime;
			logger.info("Request completed successfully", {
				operation,
				method: request.method,
				path: new URL(request.url).pathname,
				statusCode: response.status,
				responseTime,
			});

			return response;
		} catch (error) {
			const responseTime = Date.now() - startTime;
			const url = new URL(request.url);

			return RegistryErrorHandler.handleError(error, {
				operation,
				requestId: resolveRequestId(request.headers.get("x-request-id")),
				metadata: {
					method: request.method,
					path: url.pathname,
					responseTime,
				},
			});
		}
	};
}

/**
 * Async operation wrapper with error handling
 */
export async function safeAsync<T>(
	operation: () => Promise<T>,
	context: ErrorContext,
	config: ErrorHandlerConfig = {},
): Promise<
	{ success: true; data: T } | { success: false; error: NextResponse }
> {
	try {
		const data = await operation();
		return { success: true, data };
	} catch (error) {
		const errorResponse = RegistryErrorHandler.handleError(
			error,
			context,
			config,
		);
		return { success: false, error: errorResponse };
	}
}

/**
 * Cache operation wrapper that handles failures gracefully
 */
export async function safeCache<T>(
	operation: () => Promise<T>,
	fallback: T,
	context: ErrorContext,
): Promise<T> {
	try {
		return await operation();
	} catch (error) {
		RegistryErrorHandler.handleCacheError(error, context, fallback);
		return fallback;
	}
}

/**
 * Database operation wrapper with automatic retry logic
 */
export async function safeDatabase<T>(
	operation: () => Promise<T>,
	context: ErrorContext,
	maxRetries: number = 3,
): Promise<T> {
	let lastError: unknown;

	// Log the operation start
	logger.info("Database operation starting", {
		...context,
		timestamp: new Date().toISOString(),
	});

	for (let attempt = 1; attempt <= maxRetries; attempt++) {
		try {
			const result = await operation();

			// Log successful operation with result info
			logger.info("Database operation completed", {
				...context,
				attempt,
				resultType: result === null ? "null" : typeof result,
				resultIsArray: Array.isArray(result),
				resultCount: Array.isArray(result) ? result.length : undefined,
				hasResult: result !== null && result !== undefined,
			});

			return result;
		} catch (error) {
			lastError = error;

			// Log the error with details
			logger.error(
				"Database operation error",
				{
					...context,
					attempt,
					maxRetries,
					errorType:
						error instanceof Error ? error.constructor.name : typeof error,
					errorMessage: error instanceof Error ? error.message : String(error),
					errorCode: getPrismaErrorCode(error),
					errorMeta: isPrismaError(error) ? error.meta : undefined,
				},
				error instanceof Error ? error : undefined,
			);

			// Only retry errors a second attempt can fix (connectivity, pool
			// timeouts); constraint violations, missing tables and query bugs
			// fail the same way every time.
			if (!isRetryableDatabaseError(error)) {
				logger.warn("Non-retryable database error", {
					...context,
					errorCode: getPrismaErrorCode(error),
					errorMessage: error instanceof Error ? error.message : String(error),
				});
				throw error;
			}

			if (attempt === maxRetries) {
				logger.error("Database operation failed after all retries", {
					...context,
					maxRetries,
					finalError: error instanceof Error ? error.message : String(error),
				});
				throw error;
			}

			// Exponential backoff
			const delay = Math.min(1000 * 2 ** (attempt - 1), 5000);
			await new Promise((resolve) => setTimeout(resolve, delay));

			logger.warn("Database operation retry", {
				...context,
				attempt,
				maxRetries,
				nextRetryIn: delay,
			});
		}
	}

	throw lastError;
}
