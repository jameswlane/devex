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
 * Timeout configuration for async operations
 */
interface AsyncOperationConfig {
	timeout?: number;
	retries?: number;
	retryDelay?: number;
	circuitBreaker?: boolean;
	abortSignal?: AbortSignal;
}

/**
 * Circuit breaker state management for preventing cascade failures
 */
class CircuitBreaker {
	private failures = 0;
	private lastFailureTime = 0;
	private state: "CLOSED" | "OPEN" | "HALF_OPEN" = "CLOSED";

	constructor(
		private failureThreshold = 5,
		private recoveryTimeMs = 60000, // 1 minute
		private timeoutMs = 30000, // 30 seconds
	) {}

	async execute<T>(operation: () => Promise<T>): Promise<T> {
		if (this.state === "OPEN") {
			if (Date.now() - this.lastFailureTime > this.recoveryTimeMs) {
				this.state = "HALF_OPEN";
			} else {
				throw new AppError(
					"Circuit breaker is OPEN",
					"CIRCUIT_BREAKER_OPEN",
					503,
				);
			}
		}

		try {
			const result = await Promise.race([
				operation(),
				new Promise<never>((_, reject) =>
					setTimeout(
						() => reject(new AppError("Operation timeout", "TIMEOUT", 408)),
						this.timeoutMs,
					),
				),
			]);

			// Reset on success
			if (this.state === "HALF_OPEN") {
				this.state = "CLOSED";
				this.failures = 0;
			}

			return result;
		} catch (error) {
			this.failures++;
			this.lastFailureTime = Date.now();

			if (this.failures >= this.failureThreshold) {
				this.state = "OPEN";
			}

			throw error;
		}
	}

	getState() {
		return {
			state: this.state,
			failures: this.failures,
			lastFailure: this.lastFailureTime,
		};
	}
}

// Global circuit breaker instances for different operation types
const circuitBreakers = new Map<string, CircuitBreaker>();

function getCircuitBreaker(key: string): CircuitBreaker {
	if (!circuitBreakers.has(key)) {
		circuitBreakers.set(key, new CircuitBreaker());
	}
	const breaker = circuitBreakers.get(key);
	if (!breaker) {
		throw new Error(`Circuit breaker not found for key: ${key}`);
	}
	return breaker;
}

/**
 * Enhanced safe error handler for async operations with comprehensive error handling
 */
export async function safeAsync<T>(
	operation: () => Promise<T>,
	fallback?: T,
	context?: Record<string, unknown>,
	config?: AsyncOperationConfig,
): Promise<T | undefined> {
	const {
		timeout = 30000,
		retries = 0,
		retryDelay = 1000,
		circuitBreaker = false,
		abortSignal,
	} = config || {};

	const operationKey = (context?.operationKey as string) || "default";

	const executeWithConfig = async (): Promise<T> => {
		// Check for cancellation
		if (abortSignal?.aborted) {
			throw new AppError("Operation was cancelled", "OPERATION_CANCELLED", 499);
		}

		const promises: Promise<T>[] = [operation()];

		// Add timeout
		if (timeout > 0) {
			promises.push(
				new Promise<never>((_, reject) =>
					setTimeout(
						() => reject(new AppError("Operation timeout", "TIMEOUT", 408)),
						timeout,
					),
				),
			);
		}

		// Add abort signal handling
		if (abortSignal) {
			promises.push(
				new Promise<never>((_, reject) => {
					const onAbort = () =>
						reject(new AppError("Operation aborted", "ABORTED", 499));
					if (abortSignal.aborted) {
						onAbort();
					} else {
						abortSignal.addEventListener("abort", onAbort, { once: true });
					}
				}),
			);
		}

		return Promise.race(promises);
	};

	const executeOperation = circuitBreaker
		? () => getCircuitBreaker(operationKey).execute(executeWithConfig)
		: executeWithConfig;

	// Execute with retries
	let lastError: unknown;
	const maxAttempts = retries + 1;

	for (let attempt = 1; attempt <= maxAttempts; attempt++) {
		try {
			return await executeOperation();
		} catch (error) {
			lastError = error;

			// Don't retry on certain errors
			if (
				error instanceof AppError &&
				[
					"VALIDATION_ERROR",
					"NOT_FOUND_ERROR",
					"ABORTED",
					"OPERATION_CANCELLED",
				].includes(error.code)
			) {
				break;
			}

			// Don't retry on final attempt
			if (attempt === maxAttempts) {
				break;
			}

			// Wait before retry with exponential backoff
			const delay = retryDelay * 2 ** (attempt - 1);
			await new Promise((resolve) => setTimeout(resolve, delay));
		}
	}

	// Log the final error
	logError(lastError, {
		...context,
		attempts: maxAttempts,
		timeout,
		retries,
		circuitBreakerState: circuitBreaker
			? getCircuitBreaker(operationKey).getState()
			: undefined,
	});

	return fallback;
}

/**
 * Promise-based timeout wrapper
 */
export function withTimeout<T>(
	promise: Promise<T>,
	timeoutMs: number,
	errorMessage = "Operation timed out",
): Promise<T> {
	return Promise.race([
		promise,
		new Promise<never>((_, reject) =>
			setTimeout(
				() => reject(new AppError(errorMessage, "TIMEOUT", 408)),
				timeoutMs,
			),
		),
	]);
}

/**
 * Batch async operations with concurrency control
 */
export async function batchAsync<T, R>(
	items: T[],
	operation: (item: T, index: number) => Promise<R>,
	options: {
		concurrency?: number;
		failFast?: boolean;
		timeout?: number;
	} = {},
): Promise<(R | Error)[]> {
	const { concurrency = 5, failFast = false, timeout = 30000 } = options;
	const results: (R | Error)[] = new Array(items.length);

	const executeItem = async (item: T, index: number): Promise<void> => {
		try {
			const result =
				timeout > 0
					? await withTimeout(operation(item, index), timeout)
					: await operation(item, index);
			results[index] = result;
		} catch (error) {
			results[index] =
				error instanceof Error ? error : new Error(String(error));
			if (failFast) {
				throw error;
			}
		}
	};

	// Process items in batches with concurrency control
	for (let i = 0; i < items.length; i += concurrency) {
		const batch = items.slice(i, i + concurrency);
		const batchPromises = batch.map((item, batchIndex) =>
			executeItem(item, i + batchIndex),
		);

		if (failFast) {
			await Promise.all(batchPromises);
		} else {
			await Promise.allSettled(batchPromises);
		}
	}

	return results;
}

/**
 * Enhanced retry mechanism with exponential backoff and jitter
 */
export async function withRetry<T>(
	operation: () => Promise<T>,
	options: {
		maxAttempts?: number;
		initialDelay?: number;
		maxDelay?: number;
		backoffFactor?: number;
		jitter?: boolean;
		shouldRetry?: (error: unknown, attempt: number) => boolean;
		abortSignal?: AbortSignal;
	} = {},
	context?: Record<string, unknown>,
): Promise<T> {
	const {
		maxAttempts = 3,
		initialDelay = 1000,
		maxDelay = 30000,
		backoffFactor = 2,
		jitter = true,
		shouldRetry = (error, _attempt) => {
			// Don't retry validation errors, not found errors, or abort signals
			if (error instanceof AppError) {
				return !["VALIDATION_ERROR", "NOT_FOUND_ERROR", "ABORTED"].includes(
					error.code,
				);
			}
			return true;
		},
		abortSignal,
	} = options;

	let lastError: unknown;

	for (let attempt = 1; attempt <= maxAttempts; attempt++) {
		try {
			// Check for cancellation
			if (abortSignal?.aborted) {
				throw new AppError(
					"Operation was cancelled",
					"OPERATION_CANCELLED",
					499,
				);
			}

			return await operation();
		} catch (error) {
			lastError = error;

			// Check if we should retry this error
			if (!shouldRetry(error, attempt) || attempt === maxAttempts) {
				logError(error, {
					...context,
					attempt,
					maxAttempts,
					finalAttempt: true,
				});
				throw error;
			}

			// Calculate delay with exponential backoff
			let delay = Math.min(
				initialDelay * backoffFactor ** (attempt - 1),
				maxDelay,
			);

			// Add jitter to prevent thundering herd
			if (jitter) {
				delay = delay * (0.5 + Math.random() * 0.5);
			}

			logError(error, {
				...context,
				attempt,
				maxAttempts,
				retryDelay: delay,
				willRetry: true,
			});

			// Wait before retry with abort signal support
			await new Promise((resolve, reject) => {
				const timeoutId = setTimeout(resolve, delay);

				if (abortSignal) {
					const onAbort = () => {
						clearTimeout(timeoutId);
						reject(new AppError("Retry cancelled", "RETRY_CANCELLED", 499));
					};

					if (abortSignal.aborted) {
						onAbort();
					} else {
						abortSignal.addEventListener("abort", onAbort, { once: true });
					}
				}
			});
		}
	}

	throw lastError;
}

/**
 * Async operation queue with concurrency control and error handling
 */
export class AsyncQueue<T = unknown> {
	private queue: Array<{
		operation: () => Promise<T>;
		resolve: (value: T) => void;
		reject: (error: unknown) => void;
		priority: number;
		timeout?: number;
	}> = [];

	private running = 0;
	private paused = false;

	constructor(
		private concurrency = 1,
		private defaultTimeout = 30000,
	) {}

	async add<R = T>(
		operation: () => Promise<R>,
		options: {
			priority?: number;
			timeout?: number;
		} = {},
	): Promise<R> {
		return new Promise<R>((resolve, reject) => {
			this.queue.push({
				operation: operation as unknown as () => Promise<T>,
				resolve: resolve as unknown as (value: T) => void,
				reject,
				priority: options.priority || 0,
				timeout: options.timeout || this.defaultTimeout,
			});

			// Sort by priority (higher priority first)
			this.queue.sort((a, b) => b.priority - a.priority);

			this.processQueue();
		});
	}

	private async processQueue(): Promise<void> {
		if (
			this.paused ||
			this.running >= this.concurrency ||
			this.queue.length === 0
		) {
			return;
		}

		this.running++;
		const task = this.queue.shift();
		if (!task) {
			return;
		}

		try {
			const result = await withTimeout(
				task.operation(),
				task.timeout || this.defaultTimeout,
			);
			task.resolve(result);
		} catch (error) {
			task.reject(error);
		} finally {
			this.running--;
			this.processQueue();
		}
	}

	pause(): void {
		this.paused = true;
	}

	resume(): void {
		this.paused = false;
		this.processQueue();
	}

	clear(): void {
		for (const task of this.queue) {
			task.reject(new AppError("Queue cleared", "QUEUE_CLEARED", 499));
		}
		this.queue = [];
	}

	getStats() {
		return {
			queueLength: this.queue.length,
			running: this.running,
			paused: this.paused,
			concurrency: this.concurrency,
		};
	}
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
