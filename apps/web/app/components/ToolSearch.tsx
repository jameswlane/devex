"use client";

import { useState } from "react";
import { FaGithub, FaGlobe } from "react-icons/fa";
import { tools } from "../tools";

export function ToolSearch() {
	const [searchTerm, setSearchTerm] = useState("");

	const filteredTools = tools.filter(
		(tool) =>
			tool.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
			tool.categories.some((category) =>
				category.toLowerCase().includes(searchTerm.toLowerCase()),
			) ||
			tool.description.toLowerCase().includes(searchTerm.toLowerCase()),
	);

	return (
		<section id="tools" className="py-12">
			<h2 className="text-3xl font-bold text-gray-800 mb-6">Supported Tools</h2>
			<input
				type="text"
				placeholder="Search tools, categories, or descriptions..."
				className="w-full p-2 border border-gray-300 rounded-md mb-4"
				value={searchTerm}
				onChange={(e) => setSearchTerm(e.target.value)}
			/>
			<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
				{filteredTools.map((tool) => (
					<div key={tool.name} className="bg-white p-6 rounded-lg shadow-md">
						<h3 className="text-xl font-semibold mb-2">{tool.name}</h3>
						<p className="text-gray-600 mb-4">{tool.description}</p>
						<div className="mb-4">
							{tool.categories.map((category) => (
								<span
									key={category}
									className="inline-block bg-gray-200 rounded-full px-3 py-1 text-sm font-semibold text-gray-700 mr-2 mb-2"
								>
									{category}
								</span>
							))}
						</div>
						<div className="flex space-x-4">
							{tool.gitUrl && (
								<a
									href={tool.gitUrl}
									target="_blank"
									rel="noopener noreferrer"
									className="flex items-center text-gray-600 hover:text-gray-800"
								>
									<FaGithub className="mr-2" />
									GitHub
								</a>
							)}
							{tool.websiteUrl && (
								<a
									href={tool.websiteUrl}
									target="_blank"
									rel="noopener noreferrer"
									className="flex items-center text-gray-600 hover:text-gray-800"
								>
									<FaGlobe className="mr-2" />
									Website
								</a>
							)}
						</div>
					</div>
				))}
			</div>
		</section>
	);
}
