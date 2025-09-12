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
	return (
		<div className="min-h-64 flex items-center justify-center">
			<div className="text-center p-8 bg-red-50 border border-red-200 rounded-lg max-w-md">
				<div className="text-red-600 text-6xl mb-4">⚠️</div>
				<h3 className="text-lg font-medium text-red-800 mb-2">
					Something went wrong
				</h3>
				<p className="text-red-600 text-sm mb-4">
					{error?.message ||
						"An unexpected error occurred while loading this component."}
				</p>
				<button
					onClick={retry}
					className="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 transition-colors text-sm"
					type="button"
				>
					Try Again
				</button>
				{process.env.NODE_ENV === "development" && error?.stack && (
					<details className="mt-4 text-left">
						<summary className="cursor-pointer text-red-700 font-medium">
							Technical Details
						</summary>
						<pre className="mt-2 text-xs bg-red-100 p-2 rounded overflow-auto max-h-40">
							{error.stack}
						</pre>
					</details>
				)}
			</div>
		</div>
	);
}
