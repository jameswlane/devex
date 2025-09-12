/**
 * @jest-environment node
 */
import {
	AppError,
	formatErrorMessage,
	handleComponentError,
	logError,
	NetworkError,
	NotFoundError,
	safeAsync,
	ValidationError,
	withRetry,
} from "../error-handling";

// Mock console methods
const consoleSpy = {
	error: jest.spyOn(console, "error").mockImplementation(() => {}),
};

afterEach(() => {
	consoleSpy.error.mockClear();
});

afterAll(() => {
	consoleSpy.error.mockRestore();
});

describe("Error Classes", () => {
	describe("AppError", () => {
		it("should create error with default values", () => {
			const error = new AppError("Test message");

			expect(error.message).toBe("Test message");
			expect(error.code).toBe("UNKNOWN_ERROR");
			expect(error.statusCode).toBe(500);
			expect(error.context).toBeUndefined();
			expect(error.name).toBe("AppError");
		});

		it("should create error with custom values", () => {
			const context = { userId: "123", operation: "test" };
			const error = new AppError("Custom message", "CUSTOM_CODE", 400, context);

			expect(error.message).toBe("Custom message");
			expect(error.code).toBe("CUSTOM_CODE");
			expect(error.statusCode).toBe(400);
			expect(error.context).toBe(context);
		});
	});

	describe("NetworkError", () => {
		it("should create network error with correct defaults", () => {
			const error = new NetworkError("Network failed");

			expect(error.message).toBe("Network failed");
			expect(error.code).toBe("NETWORK_ERROR");
			expect(error.statusCode).toBe(503);
			expect(error.name).toBe("NetworkError");
		});
	});

	describe("ValidationError", () => {
		it("should create validation error with correct defaults", () => {
			const error = new ValidationError("Invalid input");

			expect(error.message).toBe("Invalid input");
			expect(error.code).toBe("VALIDATION_ERROR");
			expect(error.statusCode).toBe(400);
			expect(error.name).toBe("ValidationError");
		});
	});

	describe("NotFoundError", () => {
		it("should create not found error with correct defaults", () => {
			const error = new NotFoundError("Resource not found");

			expect(error.message).toBe("Resource not found");
			expect(error.code).toBe("NOT_FOUND_ERROR");
			expect(error.statusCode).toBe(404);
			expect(error.name).toBe("NotFoundError");
		});
	});
});

describe("formatErrorMessage", () => {
	it("should format AppError messages", () => {
		const error = new AppError("App error message");
		expect(formatErrorMessage(error)).toBe("App error message");
	});

	it("should format standard Error messages", () => {
		const error = new Error("Standard error");
		expect(formatErrorMessage(error)).toBe("Standard error");
	});

	it("should format string errors", () => {
		expect(formatErrorMessage("String error")).toBe("String error");
	});

	it("should handle unknown error types", () => {
		expect(formatErrorMessage(null)).toBe("An unexpected error occurred");
		expect(formatErrorMessage(undefined)).toBe("An unexpected error occurred");
		expect(formatErrorMessage(42)).toBe("An unexpected error occurred");
		expect(formatErrorMessage({})).toBe("An unexpected error occurred");
	});
});

describe("logError", () => {
	it("should log error with context", () => {
		const error = new Error("Test error");
		const context = { operation: "test", userId: "123" };

		logError(error, context);

		expect(consoleSpy.error).toHaveBeenCalledWith("Application Error:", {
			message: "Test error",
			stack: expect.any(String),
			context,
			timestamp: expect.any(String),
		});
	});

	it("should log error without context", () => {
		const error = new Error("Test error");

		logError(error);

		expect(consoleSpy.error).toHaveBeenCalledWith("Application Error:", {
			message: "Test error",
			stack: expect.any(String),
			context: undefined,
			timestamp: expect.any(String),
		});
	});

	it("should handle non-Error objects", () => {
		logError("String error", { test: true });

		expect(consoleSpy.error).toHaveBeenCalledWith("Application Error:", {
			message: "String error",
			stack: undefined,
			context: { test: true },
			timestamp: expect.any(String),
		});
	});
});

describe("safeAsync", () => {
	it("should return operation result on success", async () => {
		const operation = jest.fn().mockResolvedValue("success");

		const result = await safeAsync(operation);

		expect(result).toBe("success");
		expect(operation).toHaveBeenCalledTimes(1);
	});

	it("should return fallback on error", async () => {
		const operation = jest.fn().mockRejectedValue(new Error("Failed"));
		const fallback = "fallback";

		const result = await safeAsync(operation, fallback);

		expect(result).toBe("fallback");
		expect(consoleSpy.error).toHaveBeenCalled();
	});

	it("should return undefined if no fallback provided", async () => {
		const operation = jest.fn().mockRejectedValue(new Error("Failed"));

		const result = await safeAsync(operation);

		expect(result).toBeUndefined();
		expect(consoleSpy.error).toHaveBeenCalled();
	});

	it("should log error with context", async () => {
		const error = new Error("Operation failed");
		const operation = jest.fn().mockRejectedValue(error);
		const context = { userId: "123" };

		await safeAsync(operation, undefined, context);

		expect(consoleSpy.error).toHaveBeenCalledWith("Application Error:", {
			message: "Operation failed",
			stack: expect.any(String),
			context,
			timestamp: expect.any(String),
		});
	});
});

describe("withRetry", () => {
	beforeEach(() => {
		jest.useFakeTimers();
	});

	afterEach(() => {
		jest.useRealTimers();
	});

	it("should succeed on first attempt", async () => {
		const operation = jest.fn().mockResolvedValue("success");

		const result = await withRetry(operation, 3, 1000);

		expect(result).toBe("success");
		expect(operation).toHaveBeenCalledTimes(1);
	});

	it("should retry on failure and eventually succeed", async () => {
		const operation = jest
			.fn()
			.mockRejectedValueOnce(new Error("Fail 1"))
			.mockRejectedValueOnce(new Error("Fail 2"))
			.mockResolvedValue("success");

		const promise = withRetry(operation, 3, 1000);

		// Fast-forward through retries
		await jest.runAllTimersAsync();
		const result = await promise;

		expect(result).toBe("success");
		expect(operation).toHaveBeenCalledTimes(3);
	});

	it("should fail after max attempts", async () => {
		const error = new Error("Persistent failure");
		const operation = jest.fn().mockRejectedValue(error);

		const promise = withRetry(operation, 2, 100);
		await jest.runAllTimersAsync();

		await expect(promise).rejects.toThrow("Persistent failure");
		expect(operation).toHaveBeenCalledTimes(2);
		expect(consoleSpy.error).toHaveBeenCalled();
	});

	it("should use exponential backoff", async () => {
		const operation = jest
			.fn()
			.mockRejectedValueOnce(new Error("Fail 1"))
			.mockRejectedValueOnce(new Error("Fail 2"))
			.mockResolvedValue("success");

		const promise = withRetry(operation, 3, 1000);

		// Verify timing of retries
		expect(setTimeout).toHaveBeenCalledWith(expect.any(Function), 1000);
		jest.advanceTimersByTime(1000);

		expect(setTimeout).toHaveBeenCalledWith(expect.any(Function), 2000);
		jest.advanceTimersByTime(2000);

		const result = await promise;
		expect(result).toBe("success");
	});
});

describe("handleComponentError", () => {
	it("should create error boundary state", () => {
		const error = new Error("Component error");
		const errorInfo = {
			componentStack: "Component stack trace",
		} as React.ErrorInfo;

		const result = handleComponentError(error, errorInfo);

		expect(result).toEqual({
			hasError: true,
			error,
			errorInfo,
		});

		expect(consoleSpy.error).toHaveBeenCalledWith("Application Error:", {
			message: "Component error",
			stack: expect.any(String),
			context: {
				componentStack: "Component stack trace",
				errorBoundary: true,
			},
			timestamp: expect.any(String),
		});
	});
});
