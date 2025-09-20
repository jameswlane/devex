"use client";

import React from "react";
import {
	type ErrorBoundaryState,
	handleComponentError,
} from "../utils/error-handling";

interface ErrorBoundaryProps {
	children: React.ReactNode;
	fallback?: React.ComponentType<{ error?: Error; retry: () => void }>;
}

export class ErrorBoundary extends React.Component<
	ErrorBoundaryProps,
	ErrorBoundaryState
> {
	constructor(props: ErrorBoundaryProps) {
		super(props);
		this.state = { hasError: false };
	}

	static getDerivedStateFromError(error: Error): ErrorBoundaryState {
		return {
			hasError: true,
			error,
		};
	}

	componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
		const errorState = handleComponentError(error, errorInfo);
		this.setState(errorState);

		// Report error to monitoring service in production
		if (process.env.NODE_ENV === "production") {
			this.reportError(error, errorInfo);
		}
	}

	private reportError(error: Error, errorInfo: React.ErrorInfo) {
		// In production, this could be sent to monitoring services like Sentry
		const errorReport = {
			message: error.message,
			stack: error.stack,
			timestamp: new Date().toISOString(),
			userAgent: navigator.userAgent,
			url: window.location.href,
			componentStack: errorInfo.componentStack,
		};

		// For now, log structured error data
		console.error("ErrorBoundary: Component error reported", errorReport);

		// TODO: Integrate with monitoring service like Sentry, LogRocket, etc.
		// Example: Sentry.captureException(error, { extra: errorReport });
	}

	render() {
		if (this.state.hasError) {
			const FallbackComponent = this.props.fallback || DefaultErrorFallback;
			return (
				<FallbackComponent
					error={this.state.error}
					retry={() =>
						this.setState({
							hasError: false,
							error: undefined,
							errorInfo: undefined,
						})
					}
				/>
			);
		}

		return this.props.children;
	}
}

interface DefaultErrorFallbackProps {
	error?: Error;
	retry: () => void;
}

function DefaultErrorFallback({ error, retry }: DefaultErrorFallbackProps) {
	const [isRetrying, setIsRetrying] = React.useState(false);

	const handleRetry = async () => {
		setIsRetrying(true);
		// Add a small delay to prevent rapid retries
		await new Promise((resolve) => setTimeout(resolve, 500));
		retry();
		setIsRetrying(false);
	};

	const handleReportProblem = () => {
		// Open GitHub issues page for bug reports
		const issueTitle = encodeURIComponent(
			`Bug Report: ${error?.message || "Component Error"}`,
		);
		const issueBody = encodeURIComponent(
			`
**Error Description:**
${error?.message || "An unexpected error occurred"}

**User Agent:** ${navigator.userAgent}
**URL:** ${window.location.href}
**Timestamp:** ${new Date().toISOString()}

**Steps to Reproduce:**
1. [Please describe the steps that led to this error]

**Additional Context:**
[Please add any additional context about the problem]
		`.trim(),
		);

		const issueUrl = `https://github.com/jameswlane/devex/issues/new?title=${issueTitle}&body=${issueBody}&labels=bug`;
		window.open(issueUrl, "_blank");
	};

	return (
		<div className="min-h-64 flex items-center justify-center p-4">
			<div className="text-center p-8 bg-gradient-to-br from-red-50 to-red-100 border border-red-200 rounded-xl max-w-lg shadow-lg">
				<div className="text-red-600 text-6xl mb-4 animate-pulse">🚨</div>
				<h3 className="text-xl font-semibold text-red-900 mb-3">
					Oops! Something went wrong
				</h3>
				<p className="text-red-700 text-sm mb-6 leading-relaxed">
					{error?.message ||
						"We encountered an unexpected error while loading this component. Don't worry, this doesn't affect other parts of the application."}
				</p>

				<div className="flex flex-col sm:flex-row gap-3 justify-center">
					<button
						onClick={handleRetry}
						disabled={isRetrying}
						className="px-6 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 text-sm font-medium"
						type="button"
					>
						{isRetrying ? "Retrying..." : "🔄 Try Again"}
					</button>

					<button
						onClick={handleReportProblem}
						className="px-6 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 transition-all duration-200 text-sm font-medium"
						type="button"
					>
						📝 Report Problem
					</button>

					<button
						onClick={() => window.location.reload()}
						className="px-6 py-2 bg-blue-100 text-blue-700 rounded-lg hover:bg-blue-200 transition-all duration-200 text-sm font-medium"
						type="button"
					>
						🔄 Refresh Page
					</button>
				</div>

				{process.env.NODE_ENV === "development" && error?.stack && (
					<details className="mt-6 text-left">
						<summary className="cursor-pointer text-red-800 font-semibold text-sm mb-2 hover:text-red-900 transition-colors">
							🔧 Technical Details (Development)
						</summary>
						<pre className="text-xs bg-red-200 p-3 rounded-lg overflow-auto max-h-48 border border-red-300">
							{error.stack}
						</pre>
					</details>
				)}

				<p className="text-xs text-gray-500 mt-4">
					Error ID: {Date.now().toString(36).toUpperCase()}
				</p>
			</div>
		</div>
	);
}
