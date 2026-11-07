// Selects the plugin modules CI tests. A changed plugin is always selected; a
// shared SDK change also selects every recorder adopter, discovered from test
// imports so new contract suites join the matrix without editing the workflow.
//
// Usage: git diff --name-only BASE..HEAD | node scripts/ci/plugin-matrix.mjs --sdk true
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const SDKTEST_IMPORT =
	'"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"';
const SDK_DIR = "plugin-sdk";
const SKIPPED_DIRS = new Set(["node_modules", "dist", "vendor", "testdata"]);

/**
 * Reports whether packages/<name> is a plugin module CI tests: not the SDK,
 * and it has a main.go or package.json.
 * @param {string} root Repository root.
 * @param {string} name Directory name under packages/.
 * @returns {boolean}
 */
function isPlugin(root, name) {
	const dir = path.join(root, "packages", name);
	return (
		name !== SDK_DIR &&
		(fs.existsSync(path.join(dir, "main.go")) ||
			fs.existsSync(path.join(dir, "package.json")))
	);
}

/**
 * Reports whether any Go test file beneath dir imports sdktest. Hidden
 * directories and node_modules, dist, vendor, and testdata are skipped.
 * @param {string} dir Directory to search recursively.
 * @returns {boolean}
 */
function importsSdktest(dir) {
	for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
		if (entry.name.startsWith(".") || SKIPPED_DIRS.has(entry.name)) continue;
		const target = path.join(dir, entry.name);
		if (entry.isDirectory()) {
			if (importsSdktest(target)) return true;
		} else if (
			entry.isFile() &&
			entry.name.endsWith("_test.go") &&
			fs.readFileSync(target, "utf8").includes(SDKTEST_IMPORT)
		) {
			return true;
		}
	}
	return false;
}

/**
 * Lists plugin directories whose Go tests import sdktest, sorted by name.
 * @param {string} root Repository root.
 * @returns {string[]}
 */
export function recorderAdopters(root) {
	const packages = path.join(root, "packages");
	return fs
		.readdirSync(packages, { withFileTypes: true })
		.filter((entry) => entry.isDirectory() && isPlugin(root, entry.name))
		.map((entry) => entry.name)
		.filter((name) => importsSdktest(path.join(packages, name)))
		.sort();
}

/**
 * Returns the sorted, unique plugin directories to test: every changed plugin,
 * plus every recorder adopter when the SDK changed.
 * @param {{root: string, changedFiles: string[], sdkChanged: boolean}} options
 * @returns {string[]}
 */
export function selectPlugins({ root, changedFiles, sdkChanged }) {
	const selected = new Set(sdkChanged ? recorderAdopters(root) : []);
	for (const file of changedFiles) {
		const match = /^packages\/([^/]+)\//.exec(file.trim());
		if (match && isPlugin(root, match[1])) selected.add(match[1]);
	}
	return [...selected].sort();
}

/**
 * Reads changed paths from stdin and prints the JSON plugin matrix. Exits 2
 * unless --sdk is given exactly "true" or "false".
 */
function main() {
	const args = process.argv.slice(2);
	const flag = args.indexOf("--sdk");
	const value = flag === -1 ? undefined : args[flag + 1];
	if (value !== "true" && value !== "false") {
		console.error("usage: plugin-matrix.mjs --sdk true|false < changed-files");
		process.exit(2);
	}
	const root = path.resolve(
		path.dirname(fileURLToPath(import.meta.url)),
		"..",
		"..",
	);
	const changedFiles = fs
		.readFileSync(0, "utf8")
		.split("\n")
		.filter((line) => line.trim() !== "");
	const plugins = selectPlugins({
		root,
		changedFiles,
		sdkChanged: value === "true",
	});
	process.stdout.write(`${JSON.stringify(plugins)}\n`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
	main();
}
