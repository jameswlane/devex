export function Resources() {
	const resources = [
		{ name: "Documentation", url: "https://github.com/jameswlane/devex/wiki" },
		{
			name: "Discussions",
			url: "https://github.com/jameswlane/devex/discussions",
		},
		{ name: "Issues", url: "https://github.com/jameswlane/devex/issues" },
		{ name: "Changelog", url: "https://github.com/jameswlane/devex/releases" },
	];

	return (
		<section id="resources" className="py-12">
			<h2 className="text-3xl font-bold text-gray-800 mb-6">Resources</h2>
			<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
				{resources.map((resource) => (
					<a
						key={resource.name}
						href={resource.url}
						target="_blank"
						rel="noopener noreferrer"
						className="bg-white p-4 rounded-lg shadow text-center hover:bg-gray-50 transition-colors"
					>
						<h3 className="text-xl font-semibold text-gray-800">
							{resource.name}
						</h3>
					</a>
				))}
			</div>
		</section>
	);
}
