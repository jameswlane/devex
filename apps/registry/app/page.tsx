import Link from "next/link";
import { ensurePrisma } from "@/lib/prisma-client";
import { initializeApplication } from "@/lib/startup";
import { logger } from "@/lib/logger";

interface RegistryStats {
	totals: {
		applications: number;
		plugins: number;
		configs: number;
		stacks: number;
		all: number;
	};
	platforms: {
		linux: number;
		macos: number;
		windows: number;
	};
	categories: {
		applications: Record<string, number>;
		plugins: Record<string, number>;
		configs: Record<string, number>;
	};
	lastUpdated: string;
}

async function getRegistryStats(): Promise<RegistryStats> {
	try {
		const prisma = ensurePrisma();
		
		// Use optimized single query instead of multiple separate queries
		const statsQuery = await prisma.$queryRaw<any[]>`
			SELECT 
				'totals' as category,
				(SELECT COUNT(*) FROM applications) as applications_count,
				(SELECT COUNT(*) FROM plugins) as plugins_count,
				(SELECT COUNT(*) FROM configs) as configs_count,
				(SELECT COUNT(*) FROM stacks) as stacks_count,
				(SELECT COUNT(*) FROM applications WHERE linux_support_id IS NOT NULL) as linux_apps,
				(SELECT COUNT(*) FROM applications WHERE macos_support_id IS NOT NULL) as macos_apps,
				(SELECT COUNT(*) FROM applications WHERE windows_support_id IS NOT NULL) as windows_apps
		`;

		// Get category breakdowns in separate optimized queries
		const [appCategories, pluginTypes, configCategories, recentStats] = await Promise.all([
			prisma.application.groupBy({
				by: ["category"],
				_count: { category: true },
			}),
			prisma.plugin.groupBy({
				by: ["type"],
				_count: { type: true },
			}),
			prisma.config.groupBy({
				by: ["category"],
				_count: { category: true },
			}),
			prisma.registryStats.findFirst({
				orderBy: { date: "desc" },
			}),
		]);

		const stats = statsQuery[0];
		const applicationsCount = Number(stats.applications_count);
		const pluginsCount = Number(stats.plugins_count);
		const configsCount = Number(stats.configs_count);
		const stacksCount = Number(stats.stacks_count);
		const linuxApps = Number(stats.linux_apps);
		const macosApps = Number(stats.macos_apps);
		const windowsApps = Number(stats.windows_apps);

		return {
			totals: {
				applications: applicationsCount,
				plugins: pluginsCount,
				configs: configsCount,
				stacks: stacksCount,
				all: applicationsCount + pluginsCount + configsCount + stacksCount,
			},
			platforms: {
				linux: linuxApps,
				macos: macosApps,
				windows: windowsApps,
			},
			categories: {
				applications: appCategories.reduce(
					(acc: Record<string, number>, cat: any) => {
						const category = cat.category;
						const count = cat._count.category;
						if (category && typeof category === 'string') {
							acc[category] = count;
						}
						return acc;
					},
					{},
				),
				plugins: pluginTypes.reduce(
					(acc: Record<string, number>, type: any) => {
						const pluginType = type.type;
						const count = type._count.type;
						if (pluginType && typeof pluginType === 'string') {
							acc[pluginType] = count;
						}
						return acc;
					},
					{},
				),
				configs: configCategories.reduce(
					(acc: Record<string, number>, cat: any) => {
						const category = cat.category;
						const count = cat._count.category;
						if (category && typeof category === 'string') {
							acc[category] = count;
						}
						return acc;
					},
					{},
				),
			},
			lastUpdated: recentStats?.date?.toISOString() || new Date().toISOString(),
		};
	} catch (error) {
		logger.error("Failed to fetch registry stats", { error: error instanceof Error ? error.message : String(error) }, error instanceof Error ? error : undefined);
		// Return fallback stats to prevent page from breaking
		return {
			totals: {
				applications: 0,
				plugins: 0,
				configs: 0,
				stacks: 0,
				all: 0,
			},
			platforms: {
				linux: 0,
				macos: 0,
				windows: 0,
			},
			categories: {
				applications: {},
				plugins: {},
				configs: {},
			},
			lastUpdated: new Date().toISOString(),
		};
	}
}

function StatCard({
	title,
	count,
	description,
	href,
}: {
	title: string;
	count: number;
	description: string;
	href?: string;
}) {
	const content = (
		<div className="bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow">
			<h3 className="text-lg font-semibold text-gray-800 mb-2">{title}</h3>
			<div className="text-3xl font-bold text-blue-600 mb-1">
				{count.toLocaleString()}
			</div>
			<p className="text-sm text-gray-600">{description}</p>
		</div>
	);

	return href ? (
		<Link href={href} className="block">
			{content}
		</Link>
	) : (
		content
	);
}

function CategoryBreakdown({
	title,
	categories,
}: {
	title: string;
	categories: Record<string, number>;
}) {
	const sortedCategories = Object.entries(categories)
		.sort(([, a], [, b]) => b - a)
		.slice(0, 5); // Top 5 categories

	return (
		<div className="bg-white rounded-lg shadow-md p-6">
			<h3 className="text-lg font-semibold text-gray-800 mb-4">{title}</h3>
			<div className="space-y-2">
				{sortedCategories.map(([category, count]) => (
					<div key={category} className="flex justify-between items-center">
						<span className="text-gray-700 capitalize">
							{category.replace("-", " ")}
						</span>
						<span className="text-blue-600 font-medium">{count}</span>
					</div>
				))}
			</div>
		</div>
	);
}

// Force dynamic rendering since we need database access
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

	const stats = await getRegistryStats();

	return (
		<div className="min-h-screen bg-gray-50">
			{/* Header */}
			<header className="bg-white shadow-sm">
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
								className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
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
								className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
							>
								API Access
							</Link>
						</div>
					</div>
				</div>
			</header>

			{/* Main Content */}
			<main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
				{/* Overview Stats */}
				<div className="mb-8">
					<h2 className="text-2xl font-bold text-gray-900 mb-6">
						Registry Overview
					</h2>
					<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
						<StatCard
							title="Applications"
							count={stats.totals.applications}
							description="Cross-platform applications"
							href="/api/v1/applications"
						/>
						<StatCard
							title="Plugins"
							count={stats.totals.plugins}
							description="DevEx CLI plugins"
							href="/api/v1/plugins"
						/>
						<StatCard
							title="Configs"
							count={stats.totals.configs}
							description="Configuration templates"
						/>
						<StatCard
							title="Stacks"
							count={stats.totals.stacks}
							description="Curated app collections"
						/>
					</div>

					<StatCard
						title="Total Registry Items"
						count={stats.totals.all}
						description="Everything available in the DevEx ecosystem"
					/>
				</div>

				{/* Platform Support */}
				<div className="mb-8">
					<h2 className="text-2xl font-bold text-gray-900 mb-6">
						Platform Support
					</h2>
					<div className="grid grid-cols-1 md:grid-cols-3 gap-6">
						<StatCard
							title="Linux"
							count={stats.platforms.linux}
							description="Applications with Linux support"
						/>
						<StatCard
							title="macOS"
							count={stats.platforms.macos}
							description="Applications with macOS support"
						/>
						<StatCard
							title="Windows"
							count={stats.platforms.windows}
							description="Applications with Windows support"
						/>
					</div>
				</div>

				{/* Category Breakdowns */}
				<div className="mb-8">
					<h2 className="text-2xl font-bold text-gray-900 mb-6">
						Popular Categories
					</h2>
					<div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
						<CategoryBreakdown
							title="Application Categories"
							categories={stats.categories.applications}
						/>
						<CategoryBreakdown
							title="Plugin Types"
							categories={stats.categories.plugins}
						/>
						{Object.keys(stats.categories.configs).length > 0 && (
							<CategoryBreakdown
								title="Config Categories"
								categories={stats.categories.configs}
							/>
						)}
					</div>
				</div>

				{/* API Documentation */}
				<div className="mb-8">
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
								<code className="bg-gray-100 px-3 py-1 rounded text-sm">
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
								<code className="bg-gray-100 px-3 py-1 rounded text-sm">
									GET /api/v1/applications?category=development&platform=linux
								</code>
							</div>
							<div className="border-b pb-4">
								<h3 className="text-lg font-semibold text-gray-800">Plugins</h3>
								<p className="text-gray-600 mb-2">
									Browse DevEx CLI plugins with filtering by type and search
								</p>
								<code className="bg-gray-100 px-3 py-1 rounded text-sm">
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
								<code className="bg-gray-100 px-3 py-1 rounded text-sm">
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
						<p className="text-gray-600">
							Last updated:{" "}
							{new Date(stats.lastUpdated).toLocaleDateString("en-US", {
								year: "numeric",
								month: "long",
								day: "numeric",
								hour: "2-digit",
								minute: "2-digit",
								timeZoneName: "short",
							})}
						</p>
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
