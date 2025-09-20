"use client";

import React from "react";

interface ApiErrorFallbackProps {
	error?: Error;
	retry: () => void;
	endpoint?: string;
}

export function ApiErrorFallback({
	error,
	retry,
	endpoint,
}: ApiErrorFallbackProps) {
	const [isRetrying, setIsRetrying] = React.useState(false);
	const [retryCount, setRetryCount] = React.useState(0);

	const handleRetry = async () => {
		setIsRetrying(true);
		setRetryCount((prev) => prev + 1);

		// Progressive delay: 1s, 3s, 5s, etc.
		const delay = Math.min(1000 + retryCount * 2000, 10000);
		await new Promise((resolve) => setTimeout(resolve, delay));

		retry();
		setIsRetrying(false);
	};

	const isNetworkError =
		error?.message?.includes("fetch") || error?.message?.includes("network");
	const isTimeoutError = error?.message?.includes("timeout");
	const isServerError =
		error?.message?.includes("500") || error?.message?.includes("503");

	const getErrorTitle = () => {
		if (isNetworkError) return "Connection Problem";
		if (isTimeoutError) return "Request Timed Out";
		if (isServerError) return "Server Error";
		return "API Error";
	};

	const getErrorDescription = () => {
		if (isNetworkError) {
			return "We're having trouble connecting to our servers. Please check your internet connection and try again.";
		}
		if (isTimeoutError) {
			return "The request took too long to complete. This might be due to a slow connection or high server load.";
		}
		if (isServerError) {
			return "Our servers are experiencing issues. We're working to fix this as quickly as possible.";
		}
		return (
			error?.message || "An unexpected error occurred while fetching data."
		);
	};

	const getErrorIcon = () => {
		if (isNetworkError) return "📡";
		if (isTimeoutError) return "⏱️";
		if (isServerError) return "🔧";
		return "⚠️";
	};

	return (
		<div className="min-h-48 flex items-center justify-center p-4">
			<div className="text-center p-6 bg-gradient-to-br from-yellow-50 to-orange-50 border border-yellow-200 rounded-xl max-w-md shadow-sm">
				<div className="text-yellow-600 text-4xl mb-3">{getErrorIcon()}</div>

				<h3 className="text-lg font-semibold text-yellow-900 mb-2">
					{getErrorTitle()}
				</h3>

				<p className="text-yellow-800 text-sm mb-4 leading-relaxed">
					{getErrorDescription()}
				</p>

				{endpoint && (
					<p className="text-xs text-yellow-600 mb-4 font-mono bg-yellow-100 px-2 py-1 rounded">
						Endpoint: {endpoint}
					</p>
				)}

				<div className="flex flex-col gap-3">
					<button
						onClick={handleRetry}
						disabled={isRetrying || retryCount >= 5}
						className="px-6 py-2 bg-yellow-600 text-white rounded-lg hover:bg-yellow-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 text-sm font-medium"
						type="button"
					>
						{isRetrying
							? `Retrying... (${Math.ceil((1000 + retryCount * 2000) / 1000)}s)`
							: retryCount >= 5
								? "Max retries reached"
								: `🔄 Retry${retryCount > 0 ? ` (${retryCount}/5)` : ""}`}
					</button>

					{retryCount >= 3 && (
						<button
							onClick={() => window.location.reload()}
							className="px-4 py-1.5 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 transition-colors text-xs"
							type="button"
						>
							🔄 Refresh Page
						</button>
					)}
				</div>

				{retryCount >= 2 && (
					<div className="mt-4 p-3 bg-blue-50 border border-blue-200 rounded-lg text-xs text-blue-800">
						<p className="font-medium mb-1">💡 Troubleshooting Tips:</p>
						<ul className="text-left space-y-1 text-xs">
							<li>• Check your internet connection</li>
							<li>• Try refreshing the page</li>
							<li>• Clear your browser cache</li>
							{isServerError && <li>• Try again in a few minutes</li>}
						</ul>
					</div>
				)}

				<p className="text-xs text-gray-500 mt-3">
					Retry {retryCount + 1} at {new Date().toLocaleTimeString()}
				</p>
			</div>
		</div>
	);
}
