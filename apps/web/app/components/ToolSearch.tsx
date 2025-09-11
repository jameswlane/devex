"use client";

import { useMemo, useState } from "react";
import { categories, stats, tools } from "../generated/tools";
import type { Tool } from "../generated/types";

type Platform = "linux" | "macos" | "windows";
type FilterType = "all" | "application" | "plugin";

export function ToolSearch() {
	const [searchTerm, setSearchTerm] = useState("");
	const [selectedCategory, setSelectedCategory] = useState("all");
	const [selectedPlatform, setSelectedPlatform] = useState<Platform | "all">(
		"all",
	);
	const [filterType, setFilterType] = useState<FilterType>("all");

	const filteredTools = useMemo(() => {
		return tools.filter((tool) => {
			// Search term filter
			const matchesSearch =
				searchTerm === "" ||
				tool.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
				tool.description.toLowerCase().includes(searchTerm.toLowerCase()) ||
				tool.tags.some((tag) =>
					tag.toLowerCase().includes(searchTerm.toLowerCase()),
				);

			// Category filter
			const matchesCategory =
				selectedCategory === "all" || tool.category === selectedCategory;

			// Platform filter
			const matchesPlatform =
				selectedPlatform === "all" ||
				(selectedPlatform in tool.platforms &&
					tool.platforms[selectedPlatform] !== null);

			// Type filter
			const matchesType = filterType === "all" || tool.type === filterType;

			return matchesSearch && matchesCategory && matchesPlatform && matchesType;
		});
	}, [searchTerm, selectedCategory, selectedPlatform, filterType]);

	const getPlatformBadges = (tool: Tool) => {
		const platforms: Platform[] = [];
		if (tool.platforms.linux) platforms.push("linux");
		if (tool.platforms.macos) platforms.push("macos");
		if (tool.platforms.windows) platforms.push("windows");
		return platforms;
	};

	const getPlatformColor = (platform: Platform) => {
		switch (platform) {
			case "linux":
				return "bg-yellow-100 text-yellow-800";
			case "macos":
				return "bg-gray-100 text-gray-800";
			case "windows":
				return "bg-blue-100 text-blue-800";
		}
	};

	const getTypeColor = (type: string) => {
		return type === "plugin"
			? "bg-purple-100 text-purple-800"
			: "bg-green-100 text-green-800";
	};

	return (
		<section id="tools" className="py-12">
			<div className="flex justify-between items-center mb-6">
				<h2 className="text-3xl font-bold text-gray-800">Supported Tools</h2>
				<div className="text-sm text-gray-600">
					{filteredTools.length} of {stats.total} tools •{stats.applications}{" "}
					apps • {stats.plugins} plugins
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
						<label className="block text-sm font-medium text-gray-700 mb-1">
							Type
						</label>
						<select
							value={filterType}
							onChange={(e) => setFilterType(e.target.value as FilterType)}
							className="border border-gray-300 rounded-md px-3 py-1 text-sm"
						>
							<option value="all">All ({stats.total})</option>
							<option value="application">
								Applications ({stats.applications})
							</option>
							<option value="plugin">Plugins ({stats.plugins})</option>
						</select>
					</div>

					{/* Category Filter */}
					<div>
						<label className="block text-sm font-medium text-gray-700 mb-1">
							Category
						</label>
						<select
							value={selectedCategory}
							onChange={(e) => setSelectedCategory(e.target.value)}
							className="border border-gray-300 rounded-md px-3 py-1 text-sm"
						>
							<option value="all">All Categories</option>
							{categories.map((category) => (
								<option key={category} value={category}>
									{category}
								</option>
							))}
						</select>
					</div>

					{/* Platform Filter */}
					<div>
						<label className="block text-sm font-medium text-gray-700 mb-1">
							Platform
						</label>
						<select
							value={selectedPlatform}
							onChange={(e) =>
								setSelectedPlatform(e.target.value as Platform | "all")
							}
							className="border border-gray-300 rounded-md px-3 py-1 text-sm"
						>
							<option value="all">All Platforms</option>
							<option value="linux">Linux ({stats.platforms.linux})</option>
							<option value="macos">macOS ({stats.platforms.macos})</option>
							<option value="windows">
								Windows ({stats.platforms.windows})
							</option>
						</select>
					</div>
				</div>
			</div>

			{/* Tools Grid */}
			<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
				{filteredTools.map((tool) => {
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
				})}
			</div>

			{filteredTools.length === 0 && (
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
