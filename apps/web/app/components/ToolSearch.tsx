"use client";

import { useCallback, useEffect, useId, useMemo, useState } from "react";
import type { Tool } from "../services/tools-api";
import {
	formatErrorMessage,
	logError,
	NetworkError,
	withRetry,
} from "../utils/error-handling";

type Platform = "linux" | "macos" | "windows";
type FilterType = "all" | "application" | "plugin";

interface ApiResponse {
	tools: Tool[];
	pagination: {
		page: number;
		limit: number;
		total: number;
		totalPages: number;
		hasNext: boolean;
		hasPrev: boolean;
	};
	stats: {
		total: number;
		filtered: number;
		applications: number;
		plugins: number;
	};
}

interface MetadataResponse {
	categories: string[];
	stats: {
		total: {
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
	};
}

// Custom debounce hook
function useDebouncedValue<T>(value: T, delay: number): T {
	const [debouncedValue, setDebouncedValue] = useState<T>(value);

	useEffect(() => {
		const handler = setTimeout(() => {
			setDebouncedValue(value);
		}, delay);

		return () => {
			clearTimeout(handler);
		};
	}, [value, delay]);

	return debouncedValue;
}

export function ToolSearch() {
	const [searchTerm, setSearchTerm] = useState("");
	const debouncedSearchTerm = useDebouncedValue(searchTerm, 300);
	const [selectedCategory, setSelectedCategory] = useState("all");
	const [selectedPlatform, setSelectedPlatform] = useState<Platform | "all">(
		"all",
	);
	const [filterType, setFilterType] = useState<FilterType>("all");
	const [currentPage, setCurrentPage] = useState(1);

	// Generate unique IDs for form controls
	const sectionId = useId();
	const typeFilterId = useId();
	const categoryFilterId = useId();
	const platformFilterId = useId();
	const [tools, setTools] = useState<Tool[]>([]);
	const [metadata, setMetadata] = useState<MetadataResponse | null>(null);
	const [pagination, setPagination] = useState<
		ApiResponse["pagination"] | null
	>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Fetch metadata on mount
	useEffect(() => {
		const fetchMetadata = async () => {
			try {
				await withRetry(
					async () => {
						const response = await fetch("/api/tools/metadata");
						if (!response.ok) {
							throw new NetworkError(
								`Failed to fetch metadata: ${response.status} ${response.statusText}`,
							);
						}
						const data: MetadataResponse = await response.json();
						setMetadata(data);
					},
					{ maxAttempts: 3, initialDelay: 1000 },
					{ operation: "fetch_metadata" },
				);
			} catch (err) {
				const errorMessage = formatErrorMessage(err);
				logError(err, { operation: "fetch_metadata" });
				setError(errorMessage);
			}
		};
		fetchMetadata();
	}, []);

	// Fetch tools when filters change
	const fetchTools = useCallback(async () => {
		setLoading(true);
		setError(null);

		try {
			await withRetry(
				async () => {
					const params = new URLSearchParams({
						page: currentPage.toString(),
						limit: "24",
					});

					if (debouncedSearchTerm) params.set("search", debouncedSearchTerm);
					if (selectedCategory !== "all")
						params.set("category", selectedCategory);
					if (selectedPlatform !== "all")
						params.set("platform", selectedPlatform);
					if (filterType !== "all") params.set("type", filterType);

					const response = await fetch(`/api/tools?${params}`);
					if (!response.ok) {
						throw new NetworkError(
							`Failed to fetch tools: ${response.status} ${response.statusText}`,
						);
					}

					const data: ApiResponse = await response.json();
					setTools(data.tools);
					setPagination(data.pagination);
				},
				{ maxAttempts: 3, initialDelay: 1000 },
				{
					operation: "fetch_tools",
					filters: {
						searchTerm: debouncedSearchTerm,
						selectedCategory,
						selectedPlatform,
						filterType,
						currentPage,
					},
				},
			);
		} catch (err) {
			const errorMessage = formatErrorMessage(err);
			logError(err, {
				operation: "fetch_tools",
				filters: {
					searchTerm: debouncedSearchTerm,
					selectedCategory,
					selectedPlatform,
					filterType,
					currentPage,
				},
			});
			setError(errorMessage);
			setTools([]);
			setPagination(null);
		} finally {
			setLoading(false);
		}
	}, [
		debouncedSearchTerm,
		selectedCategory,
		selectedPlatform,
		filterType,
		currentPage,
	]);

	useEffect(() => {
		fetchTools();
	}, [fetchTools]);

	// Reset page when filters change
	const resetPage = useCallback(() => {
		setCurrentPage(1);
	}, []);

	// biome-ignore lint/correctness/useExhaustiveDependencies: We want to reset page when filters change
	useEffect(() => {
		resetPage();
	}, [
		debouncedSearchTerm,
		selectedCategory,
		selectedPlatform,
		filterType,
		resetPage,
	]);

	// Memoize filtered categories to avoid recalculation
	const filteredCategories = useMemo(
		() => metadata?.categories.filter((category) => category !== "all") || [],
		[metadata?.categories],
	);

	// Memoize platform statistics for display optimization
	const platformStats = useMemo(
		() => ({
			linux: metadata?.stats.platforms.linux || 0,
			macos: metadata?.stats.platforms.macos || 0,
			windows: metadata?.stats.platforms.windows || 0,
		}),
		[metadata?.stats.platforms],
	);

	const getPlatformBadges = useMemo(
		() => (tool: Tool) => {
			const platforms: Platform[] = [];
			if (tool.platforms.linux) platforms.push("linux");
			if (tool.platforms.macos) platforms.push("macos");
			if (tool.platforms.windows) platforms.push("windows");
			return platforms;
		},
		[],
	);

	const getPlatformColor = useMemo(
		() => (platform: Platform) => {
			switch (platform) {
				case "linux":
					return "bg-yellow-100 text-yellow-800";
				case "macos":
					return "bg-gray-100 text-gray-800";
				case "windows":
					return "bg-blue-100 text-blue-800";
			}
		},
		[],
	);

	const getTypeColor = useMemo(
		() => (type: string) =>
			type === "plugin"
				? "bg-purple-100 text-purple-800"
				: "bg-green-100 text-green-800",
		[],
	);

	// Memoize individual tool cards to prevent unnecessary re-renders
	const ToolCard = useMemo(
		() =>
			({ tool }: { tool: Tool }) => {
				const platforms = getPlatformBadges(tool);
				return (
					<div
						key={tool.name}
						className="bg-white p-6 rounded-lg shadow-md hover:shadow-lg transition-shadow"
					>
						{/* Header */}
						<div className="flex justify-between items-start mb-2">
							<h3 className="text-xl font-semibold">{tool.name}</h3>
							<div className="flex space-x-1">
								{tool.official && (
									<span className="text-xs bg-blue-100 text-blue-800 px-2 py-1 rounded-full">
										Official
									</span>
								)}
								<span
									className={`text-xs px-2 py-1 rounded-full ${getTypeColor(tool.type)}`}
								>
									{tool.type}
								</span>
							</div>
						</div>

						{/* Description */}
						<p className="text-gray-600 mb-4 text-sm leading-relaxed">
							{tool.description}
						</p>

						{/* Category */}
						<div className="mb-3">
							<span className="inline-block bg-gray-200 rounded-full px-3 py-1 text-xs font-medium text-gray-700">
								{tool.category}
							</span>
						</div>

						{/* Platform Support */}
						<div className="mb-4">
							<div className="text-xs text-gray-500 mb-1">Platforms:</div>
							<div className="flex flex-wrap gap-1">
								{platforms.map((platform) => (
									<span
										key={platform}
										className={`text-xs px-2 py-1 rounded ${getPlatformColor(platform)}`}
									>
										{platform === "macos"
											? "macOS"
											: platform.charAt(0).toUpperCase() + platform.slice(1)}
									</span>
								))}
							</div>
						</div>

						{/* Installation Preview */}
						{platforms.length > 0 && (
							<div className="mt-4 pt-4 border-t border-gray-100">
								<div className="text-xs text-gray-500 mb-2">Install via:</div>
								<div className="text-xs font-mono bg-gray-50 p-2 rounded">
									{tool.type === "plugin" ? (
										<span className="text-gray-600">
											devex plugin install {tool.name}
										</span>
									) : (
										<span className="text-gray-600">
											{tool.platforms[platforms[0]]?.installMethod ||
												"auto-detect"}
										</span>
									)}
								</div>
							</div>
						)}
					</div>
				);
			},
		[getPlatformBadges, getPlatformColor, getTypeColor],
	);

	if (error) {
		return (
			<section id={sectionId} className="py-12">
				<div className="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
					<h3 className="text-lg font-medium text-red-800 mb-2">
						Error Loading Tools
					</h3>
					<p className="text-red-600">{error}</p>
					<button
						type="button"
						onClick={fetchTools}
						className="mt-4 px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 transition-colors"
					>
						Try Again
					</button>
				</div>
			</section>
		);
	}

	return (
		<section id={sectionId} className="py-12">
			<div className="flex justify-between items-center mb-6">
				<h2 className="text-3xl font-bold text-gray-800">Supported Tools</h2>
				<div className="text-sm text-gray-600">
					{loading ? (
						"Loading..."
					) : (
						<>
							{pagination?.total || 0} of {metadata?.stats.total?.all || 0}{" "}
							tools • {metadata?.stats.total?.applications || 0} apps •{" "}
							{metadata?.stats.total?.plugins || 0} plugins
						</>
					)}
				</div>
			</div>

			{/* Filters */}
			<div className="bg-gray-50 p-4 rounded-lg mb-6 space-y-4">
				{/* Search */}
				<input
					type="text"
					placeholder="Search tools, descriptions, or tags..."
					className="w-full p-3 border border-gray-300 rounded-md"
					value={searchTerm}
					onChange={(e) => setSearchTerm(e.target.value)}
				/>

				{/* Filter Row */}
				<div className="flex flex-wrap gap-4">
					{/* Type Filter */}
					<div>
						<label
							htmlFor={typeFilterId}
							className="block text-sm font-medium text-gray-700 mb-1"
						>
							Type
						</label>
						<select
							id={typeFilterId}
							value={filterType}
							onChange={(e) => setFilterType(e.target.value as FilterType)}
							className="border border-gray-300 rounded-md px-3 py-1 text-sm"
							disabled={loading}
						>
							<option value="all">
								All ({metadata?.stats.total?.all || 0})
							</option>
							<option value="application">
								Applications ({metadata?.stats.total?.applications || 0})
							</option>
							<option value="plugin">
								Plugins ({metadata?.stats.total?.plugins || 0})
							</option>
						</select>
					</div>

					{/* Category Filter */}
					<div>
						<label
							htmlFor={categoryFilterId}
							className="block text-sm font-medium text-gray-700 mb-1"
						>
							Category
						</label>
						<select
							id={categoryFilterId}
							value={selectedCategory}
							onChange={(e) => setSelectedCategory(e.target.value)}
							className="border border-gray-300 rounded-md px-3 py-1 text-sm"
							disabled={loading}
						>
							<option value="all">All Categories</option>
							{filteredCategories.map((category) => (
								<option key={category} value={category}>
									{category}
								</option>
							))}
						</select>
					</div>

					{/* Platform Filter */}
					<div>
						<label
							htmlFor={platformFilterId}
							className="block text-sm font-medium text-gray-700 mb-1"
						>
							Platform
						</label>
						<select
							id={platformFilterId}
							value={selectedPlatform}
							onChange={(e) =>
								setSelectedPlatform(e.target.value as Platform | "all")
							}
							className="border border-gray-300 rounded-md px-3 py-1 text-sm"
							disabled={loading}
						>
							<option value="all">All Platforms</option>
							<option value="linux">Linux ({platformStats.linux})</option>
							<option value="macos">macOS ({platformStats.macos})</option>
							<option value="windows">Windows ({platformStats.windows})</option>
						</select>
					</div>
				</div>
			</div>

			{/* Loading State */}
			{loading && (
				<div className="flex justify-center items-center py-12">
					<div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
					<span className="ml-2 text-gray-600">Loading tools...</span>
				</div>
			)}

			{/* Tools Grid */}
			{!loading && (
				<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
					{tools.map((tool) => (
						<ToolCard key={tool.name} tool={tool} />
					))}
				</div>
			)}

			{/* Pagination */}
			{!loading && pagination && pagination.totalPages > 1 && (
				<div className="flex justify-center items-center space-x-4 mt-8">
					<button
						type="button"
						onClick={() => setCurrentPage(currentPage - 1)}
						disabled={!pagination.hasPrev}
						className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Previous
					</button>

					<span className="text-sm text-gray-600">
						Page {pagination.page} of {pagination.totalPages} (
						{pagination.total} total)
					</span>

					<button
						type="button"
						onClick={() => setCurrentPage(currentPage + 1)}
						disabled={!pagination.hasNext}
						className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Next
					</button>
				</div>
			)}

			{!loading && tools.length === 0 && (
				<div className="text-center py-12 text-gray-500">
					<p className="text-lg">No tools found matching your filters.</p>
					<p className="text-sm mt-2">
						Try adjusting your search or filter criteria.
					</p>
				</div>
			)}
		</section>
	);
}
