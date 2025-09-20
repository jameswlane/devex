export function Resources() {
	const resources = [
		{ name: "Documentation", url: "https://docs.devex.sh", icon: "📚" },
		{
			name: "Discord",
			url: "https://discord.gg/6eNy4W6Zfu",
			icon: "💬",
		},
		{
			name: "Discussions",
			url: "https://github.com/jameswlane/devex/discussions",
			icon: "💭",
		},
		{
			name: "Issues",
			url: "https://github.com/jameswlane/devex/issues",
			icon: "🐛",
		},
		{ name: "GitHub", url: "https://github.com/jameswlane/devex", icon: "⭐" },
		{
			name: "Changelog",
			url: "https://github.com/jameswlane/devex/releases",
			icon: "📋",
		},
	];

	return (
		<section id="resources" className="py-12">
			<h2 className="text-3xl font-bold text-gray-800 mb-6">Resources</h2>
			<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
				{resources.map((resource) => (
					<a
						key={resource.name}
						href={resource.url}
						target="_blank"
						rel="noopener noreferrer"
						className="bg-white p-6 rounded-lg shadow text-center hover:bg-gray-50 hover:shadow-md transition-all duration-200"
					>
						<div className="text-3xl mb-2">{resource.icon}</div>
						<h3 className="text-lg font-semibold text-gray-800">
							{resource.name}
						</h3>
					</a>
				))}
			</div>
		</section>
	);
}
