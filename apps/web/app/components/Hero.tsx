"use client";

import { useEffect, useState } from "react";

type Platform = "windows" | "macos" | "linux";

interface InstallCommand {
	command: string;
	description: string;
}

const installCommands: Record<Platform, InstallCommand> = {
	windows: {
		command: "irm https://get.devex.sh/install.ps1 | iex",
		description: "Run in PowerShell as Administrator",
	},
	macos: {
		command: "curl -fsSL https://get.devex.sh/install | bash",
		description: "Run in Terminal",
	},
	linux: {
		command: "curl -fsSL https://get.devex.sh/install | bash",
		description: "Run in Terminal",
	},
};

function detectOS(): Platform {
	if (typeof window === "undefined") return "linux"; // Default for SSR

	const userAgent = window.navigator.userAgent.toLowerCase();

	if (userAgent.includes("win")) return "windows";
	if (userAgent.includes("mac")) return "macos";
	return "linux";
}

export function Hero() {
	const [platform, setPlatform] = useState<Platform>("linux");
	const [mounted, setMounted] = useState(false);
	const [copyStatus, setCopyStatus] = useState<"idle" | "success" | "error">(
		"idle",
	);

	useEffect(() => {
		setPlatform(detectOS());
		setMounted(true);
	}, []);

	const currentCommand = installCommands[platform];
	const otherPlatforms = Object.entries(installCommands).filter(
		([key]) => key !== platform,
	) as [Platform, InstallCommand][];

	const copyToClipboard = async () => {
		try {
			await navigator.clipboard.writeText(currentCommand.command);
			setCopyStatus("success");
			// Clear success state after 2 seconds
			setTimeout(() => setCopyStatus("idle"), 2000);
		} catch (err) {
			console.error("Failed to copy:", err);
			setCopyStatus("error");
			// Clear error state after 3 seconds
			setTimeout(() => setCopyStatus("idle"), 3000);
		}
	};

	return (
		<section className="text-center py-12">
			<h1 className="text-4xl font-bold text-gray-800 mb-4">
				Setup Your Development Environment with Ease
			</h1>
			<p className="text-xl text-gray-600 mb-8">
				DevEx: The cross-platform CLI for Linux, macOS, and Windows
			</p>

			{/* Platform Selector */}
			<div className="mb-6">
				<div className="inline-flex rounded-lg bg-gray-100 p-1">
					{(["linux", "macos", "windows"] as Platform[]).map((os) => (
						<button
							key={os}
							type="button"
							onClick={() => setPlatform(os)}
							className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
								platform === os
									? "bg-white text-gray-900 shadow-sm"
									: "text-gray-500 hover:text-gray-700"
							}`}
						>
							{os === "macos"
								? "macOS"
								: os.charAt(0).toUpperCase() + os.slice(1)}
						</button>
					))}
				</div>
			</div>

			{/* Install Command */}
			<div className="space-y-4">
				<div className="bg-gray-800 text-white p-4 rounded-lg inline-block max-w-2xl">
					<div className="flex items-center justify-between space-x-4">
						<code className="text-sm flex-1 text-left">
							{mounted ? currentCommand.command : installCommands.linux.command}
						</code>
						<button
							type="button"
							onClick={copyToClipboard}
							className={`text-sm px-3 py-1 rounded transition-colors ${
								copyStatus === "success"
									? "bg-green-600 text-white"
									: copyStatus === "error"
										? "bg-red-600 text-white"
										: "text-gray-300 hover:text-white bg-gray-700 hover:bg-gray-600"
							}`}
							title="Copy to clipboard"
							disabled={copyStatus !== "idle"}
						>
							{copyStatus === "success"
								? "Copied!"
								: copyStatus === "error"
									? "Failed!"
									: "Copy"}
						</button>
					</div>
				</div>
				<p className="text-sm text-gray-500">
					{mounted
						? currentCommand.description
						: installCommands.linux.description}
				</p>
			</div>

			{/* Alternative Platforms */}
			{mounted && (
				<details className="mt-8 text-sm text-gray-600">
					<summary className="cursor-pointer hover:text-gray-800 transition-colors">
						Other platforms
					</summary>
					<div className="mt-4 space-y-2">
						{otherPlatforms.map(([os, cmd]) => (
							<div key={os} className="text-left max-w-2xl mx-auto">
								<strong className="capitalize">
									{os === "macos" ? "macOS" : os}:
								</strong>
								<div className="bg-gray-100 p-2 rounded mt-1 font-mono text-xs">
									{cmd.command}
								</div>
							</div>
						))}
					</div>
				</details>
			)}

			<p className="mt-6 text-sm text-gray-500">
				The installer will automatically detect your platform and install the
				appropriate tools
			</p>
		</section>
	);
}
