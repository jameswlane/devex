import { mkdir, readdir, readFile, rename, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { load } from "js-yaml";

const platforms = ["linux", "macos", "windows"];
const webRoot = fileURLToPath(new URL("../", import.meta.url));

async function yamlFiles(directory) {
	const entries = await readdir(directory, { withFileTypes: true });
	const files = await Promise.all(
		entries.map((entry) => {
			const target = path.join(directory, entry.name);
			if (entry.isDirectory()) return yamlFiles(target);
			return entry.isFile() && entry.name.endsWith(".yaml") ? [target] : [];
		}),
	);
	return files.flat().sort();
}

function platformInfo(config) {
	if (!config) return null;
	if (
		typeof config.install_method !== "string" ||
		(config.install_command != null &&
			typeof config.install_command !== "string")
	) {
		throw new Error(
			"platform requires an install_method string and optional install_command string",
		);
	}
	return {
		installMethod: config.install_method,
		installCommand: config.install_command ?? config.download_url ?? "",
		alternatives: config.alternatives ?? [],
		officialSupport: config.official_support ?? false,
	};
}

/** Generate the application catalog consumed by registry seeding. */
export async function generateTools({
	inputDirectory = path.resolve(webRoot, "../cli/config/applications"),
	outputDirectory = path.join(webRoot, "app/generated"),
} = {}) {
	const files = await yamlFiles(inputDirectory);
	if (files.length === 0) throw new Error("No application YAML files found");
	const tools = [];
	const names = new Set();
	for (const file of files) {
		try {
			const app = load(await readFile(file, "utf8"));
			for (const field of ["name", "description", "category"]) {
				if (typeof app?.[field] !== "string" || !app[field].trim()) {
					throw new Error(`application requires a nonempty ${field} string`);
				}
			}
			if (names.has(app.name))
				throw new Error(`Duplicate application: ${app.name}`);
			names.add(app.name);
			const platformData = Object.fromEntries(
				platforms.map((platform) => [
					platform,
					platformInfo(app.all_platforms ?? app[platform]),
				]),
			);
			if (!Object.values(platformData).some(Boolean)) {
				throw new Error(
					"application requires at least one installation platform",
				);
			}
			tools.push({
				name: app.name,
				description: app.description,
				category: app.category,
				type: "application",
				official: app.official ?? false,
				default: app.default ?? false,
				platforms: platformData,
				tags: app.tags ?? [],
				desktopEnvironments: app.desktop_environments ?? [],
			});
		} catch (error) {
			throw new Error(`${file}: ${error.message}`, { cause: error });
		}
	}
	const counts = Object.fromEntries(
		platforms.map((platform) => [
			platform,
			tools.filter((tool) => tool.platforms[platform]).length,
		]),
	);
	const catalog = {
		tools,
		categories: [...new Set(tools.map((tool) => tool.category))].sort(),
		stats: {
			total: tools.length,
			applications: tools.length,
			plugins: 0,
			platforms: {
				...counts,
				total: Object.values(counts).reduce((a, b) => a + b, 0),
			},
		},
	};
	// Validate every source before replacing the previous catalog. No timestamps:
	// identical inputs must produce identical cached artifacts.
	await mkdir(outputDirectory, { recursive: true });
	const target = path.join(outputDirectory, "tools.json");
	const temporary = `${target}.tmp`;
	await writeFile(temporary, `${JSON.stringify(catalog, null, 2)}\n`);
	await rename(temporary, target);
	return catalog;
}

if (
	process.argv[1] &&
	path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
	generateTools().catch((error) => {
		console.error(error.message);
		process.exitCode = 1;
	});
}
