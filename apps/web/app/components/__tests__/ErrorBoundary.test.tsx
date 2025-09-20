/**
 * @jest-environment jsdom
 */

import { fireEvent, render, screen } from "@testing-library/react";
import type React from "react";
import "@testing-library/jest-dom";
import { ErrorBoundary } from "../ErrorBoundary";

// Mock the error handling module
jest.mock("../../utils/error-handling", () => ({
	handleComponentError: jest.fn((error, errorInfo) => ({
		hasError: true,
		error,
		errorInfo,
	})),
}));

import { handleComponentError } from "../../utils/error-handling";

// Component that throws an error
const ThrowingComponent: React.FC<{ shouldThrow?: boolean }> = ({
	shouldThrow = true,
}) => {
	if (shouldThrow) {
		throw new Error("Test error");
	}
	return <div>No error</div>;
};

// Custom fallback component for testing
const CustomFallback: React.FC<{ error?: Error; retry: () => void }> = ({
	error,
	retry,
}) => (
	<div>
		<h1>Custom Error Fallback</h1>
		<p>Error: {error?.message}</p>
		<button type="button" onClick={retry}>
			Custom Retry
		</button>
	</div>
);

// Mock console.error to avoid noise in tests
const consoleSpy = jest.spyOn(console, "error").mockImplementation(() => {});

afterEach(() => {
	consoleSpy.mockClear();
	jest.clearAllMocks();
});

afterAll(() => {
	consoleSpy.mockRestore();
});

describe("ErrorBoundary", () => {
	describe("when no error occurs", () => {
		it("should render children normally", () => {
			render(
				<ErrorBoundary>
					<div>Child component</div>
				</ErrorBoundary>,
			);

			expect(screen.getByText("Child component")).toBeInTheDocument();
		});
	});

	describe("when an error occurs", () => {
		it("should render default error fallback", () => {
			render(
				<ErrorBoundary>
					<ThrowingComponent />
				</ErrorBoundary>,
			);

			expect(screen.getByText("Something went wrong")).toBeInTheDocument();
			expect(screen.getByText("Test error")).toBeInTheDocument();
			expect(
				screen.getByRole("button", { name: "Try Again" }),
			).toBeInTheDocument();
		});

		it("should render custom fallback when provided", () => {
			render(
				<ErrorBoundary fallback={CustomFallback}>
					<ThrowingComponent />
				</ErrorBoundary>,
			);

			expect(screen.getByText("Custom Error Fallback")).toBeInTheDocument();
			expect(screen.getByText("Error: Test error")).toBeInTheDocument();
			expect(
				screen.getByRole("button", { name: "Custom Retry" }),
			).toBeInTheDocument();
		});

		it("should call handleComponentError when error occurs", () => {
			render(
				<ErrorBoundary>
					<ThrowingComponent />
				</ErrorBoundary>,
			);

			expect(handleComponentError).toHaveBeenCalledWith(
				expect.objectContaining({ message: "Test error" }),
				expect.objectContaining({ componentStack: expect.any(String) }),
			);
		});

		it("should show generic error message when no error message available", () => {
			// Mock handleComponentError to return error without message
			(handleComponentError as jest.Mock).mockReturnValue({
				hasError: true,
				error: null,
				errorInfo: {},
			});

			render(
				<ErrorBoundary>
					<ThrowingComponent />
				</ErrorBoundary>,
			);

			expect(
				screen.getByText(
					"An unexpected error occurred while loading this component.",
				),
			).toBeInTheDocument();
		});

		it("should allow retry functionality", () => {
			let shouldThrow = true;
			const TestComponent = () => (
				<ThrowingComponent shouldThrow={shouldThrow} />
			);

			const { rerender } = render(
				<ErrorBoundary>
					<TestComponent />
				</ErrorBoundary>,
			);

			// Error should be shown
			expect(screen.getByText("Something went wrong")).toBeInTheDocument();

			// Change the throwing condition
			shouldThrow = false;

			// Click retry button
			fireEvent.click(screen.getByRole("button", { name: "Try Again" }));

			// Rerender with new state
			rerender(
				<ErrorBoundary>
					<ThrowingComponent shouldThrow={false} />
				</ErrorBoundary>,
			);

			// Should show the non-error content
			expect(screen.getByText("No error")).toBeInTheDocument();
		});

		describe("in development mode", () => {
			const originalEnv = process.env.NODE_ENV;

			beforeAll(() => {
				process.env.NODE_ENV = "development";
			});

			afterAll(() => {
				process.env.NODE_ENV = originalEnv;
			});

			it("should show technical details in development", () => {
				const testError = new Error("Test error");
				testError.stack = "Error: Test error\\n    at test.js:1:1";

				// Mock handleComponentError to return error with stack
				(handleComponentError as jest.Mock).mockReturnValue({
					hasError: true,
					error: testError,
					errorInfo: {},
				});

				render(
					<ErrorBoundary>
						<ThrowingComponent />
					</ErrorBoundary>,
				);

				expect(screen.getByText("Technical Details")).toBeInTheDocument();

				// Click to expand details
				fireEvent.click(screen.getByText("Technical Details"));

				// Check that the error message is present (stack trace will vary)
				expect(screen.getByText(/Error: Test error/)).toBeInTheDocument();
			});
		});

		describe("in production mode", () => {
			const originalEnv = process.env.NODE_ENV;

			beforeAll(() => {
				process.env.NODE_ENV = "production";
			});

			afterAll(() => {
				process.env.NODE_ENV = originalEnv;
			});

			it("should not show technical details in production", () => {
				const testError = new Error("Test error");
				testError.stack = "Error: Test error\\n    at test.js:1:1";

				(handleComponentError as jest.Mock).mockReturnValue({
					hasError: true,
					error: testError,
					errorInfo: {},
				});

				render(
					<ErrorBoundary>
						<ThrowingComponent />
					</ErrorBoundary>,
				);

				expect(screen.queryByText("Technical Details")).not.toBeInTheDocument();
			});
		});

		it("should reset error state when retry is clicked", async () => {
			// Create a component that can toggle error state
			const ControllableComponent: React.FC<{ version: number }> = ({
				version,
			}) => {
				if (version === 1) {
					throw new Error("Version 1 error");
				}
				return <div>Version {version} loaded</div>;
			};

			const { rerender } = render(
				<ErrorBoundary>
					<ControllableComponent version={1} />
				</ErrorBoundary>,
			);

			// Error should be shown
			expect(screen.getByText("Something went wrong")).toBeInTheDocument();

			// Click retry - this resets the error boundary state
			fireEvent.click(screen.getByRole("button", { name: "Try Again" }));

			// Rerender with version that doesn't throw
			rerender(
				<ErrorBoundary>
					<ControllableComponent version={2} />
				</ErrorBoundary>,
			);

			// Should show successful content after state change
			await waitFor(() => {
				expect(screen.getByText("Version 2 loaded")).toBeInTheDocument();
			});
		});
	});
});
