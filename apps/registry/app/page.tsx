import Link from "next/link";
import { initializeApplication } from "@/lib/startup";
import { logger } from "@/lib/logger";
import StatsDashboard from "@/components/stats-dashboard";
import { StatsErrorBoundary } from "@/components/error-boundary";

// Force dynamic rendering since we have client-side data fetching
export const dynamic = "force-dynamic";

export default async function RegistryHomepage() {
	// Initialize application startup with proper error handling
	try {
		const startupResult = await initializeApplication({
			enableWarmup: process.env.NODE_ENV === "production",
			timeoutMs: 5000, // Shorter timeout for page loads
			retries: 1, // Single retry for page loads
		});
		
		if (!startupResult.success) {
			logger.warn("Application startup completed with warnings", {
				database: startupResult.database,
				redis: startupResult.redis,
			});
		}
	} catch (error) {
		logger.error("Application startup failed", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
		// Continue anyway - page should still render with cached/fallback data
	}

	return (
		<div className="min-h-screen bg-gray-50">
			{/* Header */}
			<header className="bg-white shadow-xs">
				<div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
					<div className="flex items-center justify-between">
						<div>
							<h1 className="text-3xl font-bold text-gray-900">
								DevEx Registry
							</h1>
							<p className="mt-2 text-gray-600">
								The official registry for DevEx CLI applications, plugins,
								configs, and stacks
							</p>
						</div>
						<div className="flex space-x-4">
							<Link
								href="https://github.com/jameswlane/devex"
								className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-xs text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
							>
								<svg
									className="w-5 h-5 mr-2"
									fill="currentColor"
									viewBox="0 0 20 20"
								>
									<path
										fillRule="evenodd"
										d="M10 0C4.477 0 0 4.484 0 10.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0110 4.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0020 10.017C20 4.484 15.522 0 10 0z"
										clipRule="evenodd"
									/>
								</svg>
								GitHub
							</Link>
							<Link
								href="/api/v1/registry"
								className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-xs text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
							>
								API Access
							</Link>
						</div>
					</div>
				</div>
			</header>

			{/* Main Content */}
			<main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
				{/* Statistics Dashboard with Loading States */}
				<StatsErrorBoundary>
					<StatsDashboard />
				</StatsErrorBoundary>

				{/* Browse Categories */}
				<div className="mb-8 mt-12">
					<h2 className="text-2xl font-bold text-gray-900 mb-6">
						Browse Registry
					</h2>
					<div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
						<Link
							href="/applications"
							className="block bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow"
						>
							<div className="flex items-center">
								<div className="flex-shrink-0">
									<svg className="w-8 h-8 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
									</svg>
								</div>
								<div className="ml-4">
									<h3 className="text-lg font-semibold text-gray-900">Applications</h3>
									<p className="text-gray-600">Browse cross-platform development applications with advanced filtering and search</p>
								</div>
							</div>
						</Link>
						
						<Link
							href="/plugins"
							className="block bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow"
						>
							<div className="flex items-center">
								<div className="flex-shrink-0">
									<svg className="w-8 h-8 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
									</svg>
								</div>
								<div className="ml-4">
									<h3 className="text-lg font-semibold text-gray-900">Plugins</h3>
									<p className="text-gray-600">Explore DevEx CLI plugins and extensions with type-based filtering</p>
								</div>
							</div>
						</Link>
					</div>
				</div>

				{/* API Documentation */}
				<div className="mb-8 mt-12">
					<h2 className="text-2xl font-bold text-gray-900 mb-6">
						API Endpoints
					</h2>
					<div className="bg-white rounded-lg shadow-md p-6">
						<div className="space-y-4">
							<div className="border-b pb-4">
								<h3 className="text-lg font-semibold text-gray-800">
									Full Registry
								</h3>
								<p className="text-gray-600 mb-2">
									Get complete registry data including all applications,
									plugins, configs, and stacks
								</p>
								<code className="bg-gray-100 px-3 py-1 rounded-sm text-sm">
									GET /api/v1/registry
								</code>
							</div>
							<div className="border-b pb-4">
								<h3 className="text-lg font-semibold text-gray-800">
									Applications
								</h3>
								<p className="text-gray-600 mb-2">
									Browse applications with filtering by category, platform, and
									search
								</p>
								<code className="bg-gray-100 px-3 py-1 rounded-sm text-sm">
									GET /api/v1/applications?category=development&platform=linux
								</code>
							</div>
							<div className="border-b pb-4">
								<h3 className="text-lg font-semibold text-gray-800">Plugins</h3>
								<p className="text-gray-600 mb-2">
									Browse DevEx CLI plugins with filtering by type and search
								</p>
								<code className="bg-gray-100 px-3 py-1 rounded-sm text-sm">
									GET /api/v1/plugins?type=package-manager
								</code>
							</div>
							<div>
								<h3 className="text-lg font-semibold text-gray-800">
									Statistics
								</h3>
								<p className="text-gray-600 mb-2">
									Get comprehensive registry statistics and metrics
								</p>
								<code className="bg-gray-100 px-3 py-1 rounded-sm text-sm">
									GET /api/v1/stats
								</code>
							</div>
						</div>
					</div>
				</div>
			</main>

			{/* Footer */}
			<footer className="bg-white border-t border-gray-200">
				<div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
					<div className="text-center">
						<p className="text-gray-500 mt-2">
							Powered by{" "}
							<Link
								href="https://devex.sh"
								className="text-blue-600 hover:text-blue-800"
							>
								DevEx CLI
							</Link>
						</p>
					</div>
				</div>
			</footer>
		</div>
	);
}
