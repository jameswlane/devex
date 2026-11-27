export function AIAgents() {
	const highlights = [
		{
			title: "Guided & Template Setup",
			description:
				"Pick from 8 terminal coding agents in devex setup or install the ai-assisted lifecycle template with automatic Linux sandbox prerequisites.",
		},
		{
			title: "Declarative agents.yaml",
			description:
				"Share instructions, model defaults, conservative permission presets, and secret-free MCP servers across Claude Code, Codex CLI, Gemini CLI, and OpenCode.",
		},
		{
			title: "Safe Diagnostics & Login",
			description:
				"Verify binaries, PATH shadowing, sandboxes, and MCP configs with devex agent doctor, then authenticate directly via devex agent login.",
		},
	];

	return (
		<section id="ai-agents" className="py-12">
			<div className="flex flex-col md:flex-row md:items-end md:justify-between mb-6 gap-2">
				<div>
					<h2 className="text-3xl font-bold text-gray-800">AI Coding Agents</h2>
					<p className="text-gray-600 mt-1">
						Opt-in installation, sandboxing, and cross-harness configuration
						without storing credentials in DevEx config.
					</p>
				</div>
				<a
					href="https://docs.devex.sh/guides/ai-coding-agents"
					target="_blank"
					rel="noopener noreferrer"
					className="text-sm font-medium text-blue-600 hover:text-blue-800"
				>
					Read the guide →
				</a>
			</div>
			<div className="grid grid-cols-1 md:grid-cols-3 gap-6">
				{highlights.map((item) => (
					<div
						key={item.title}
						className="bg-white p-6 rounded-lg shadow-md flex flex-col justify-between"
					>
						<div>
							<h3 className="text-lg font-semibold text-gray-800 mb-2">
								{item.title}
							</h3>
							<p className="text-sm text-gray-600 leading-relaxed">
								{item.description}
							</p>
						</div>
					</div>
				))}
			</div>
		</section>
	);
}
