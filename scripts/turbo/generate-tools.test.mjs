import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, test } from "node:test";
import { fileURLToPath } from "node:url";
import { generateTools } from "../../apps/web/scripts/generate-tools.mjs";

const fixtures = [];
afterEach(async () => {
	await Promise.all(
		fixtures
			.splice(0)
			.map((root) => rm(root, { recursive: true, force: true })),
	);
});
async function fixture() {
	const root = await mkdtemp(path.join(os.tmpdir(), "devex-catalog-"));
	fixtures.push(root);
	const inputDirectory = path.join(root, "input");
	const outputDirectory = path.join(root, "output");
	await mkdir(inputDirectory);
	return { inputDirectory, outputDirectory };
}
const application = `name: Example
description: An example application
category: Development
linux:
  install_method: apt
  install_command: example
`;

test("catalog generation is recursive, deterministic, and preserves platform metadata", async () => {
	const options = await fixture();
	await mkdir(path.join(options.inputDirectory, "nested"));
	await writeFile(
		path.join(options.inputDirectory, "nested/app.yaml"),
		`${application}  official_support: true\n  alternatives:\n    - install_method: dnf\n      install_command: example\ndefault: true\ndesktop_environments: [gnome]\ntags: [development]\n`,
	);
	await writeFile(path.join(options.inputDirectory, "README.md"), "Ignored");
	const catalog = await generateTools(options);
	assert.equal(catalog.tools.length, 1);
	assert.deepEqual(catalog.tools[0], {
		name: "Example",
		description: "An example application",
		category: "Development",
		type: "application",
		official: false,
		default: true,
		tags: ["development"],
		desktopEnvironments: ["gnome"],
		platforms: {
			linux: {
				installMethod: "apt",
				installCommand: "example",
				officialSupport: true,
				alternatives: [{ install_method: "dnf", install_command: "example" }],
			},
			macos: null,
			windows: null,
		},
	});
	assert.deepEqual(catalog.categories, ["Development"]);
	assert.deepEqual(catalog.stats, {
		total: 1,
		applications: 1,
		plugins: 0,
		platforms: { linux: 1, macos: 0, windows: 0, total: 1 },
	});
	const output = path.join(options.outputDirectory, "tools.json");
	const first = await readFile(output, "utf8");
	await generateTools(options);
	assert.equal(await readFile(output, "utf8"), first);
});

test("all_platforms applies to each OS and download methods can omit install_command", async () => {
	const options = await fixture();
	await writeFile(
		path.join(options.inputDirectory, "app.yaml"),
		`name: Download
description: Downloaded tool
category: Tools
all_platforms:
  install_method: curlpipe
  download_url: https://example.com/tool
`,
	);
	const catalog = await generateTools(options);
	for (const platform of ["linux", "macos", "windows"]) {
		assert.equal(
			catalog.tools[0].platforms[platform].installMethod,
			"curlpipe",
		);
		assert.equal(
			catalog.tools[0].platforms[platform].installCommand,
			"https://example.com/tool",
		);
	}
	assert.equal(catalog.stats.platforms.total, 3);
});

for (const [name, invalid, expected] of [
	["malformed YAML", "name: [", /app.yaml/],
	["empty YAML", "", /empty|nonempty name/],
	["missing required metadata", "name: Example\n", /description/],
	[
		"missing platforms",
		"name: Example\ndescription: Example\ncategory: Tools\n",
		/installation platform/,
	],
	[
		"invalid platform",
		application.replace("install_method: apt", "install_method: 42"),
		/install_method/,
	],
]) {
	test(`${name} fails without replacing the existing catalog`, async () => {
		const options = await fixture();
		const input = path.join(options.inputDirectory, "app.yaml");
		await writeFile(input, application);
		await generateTools(options);
		const output = path.join(options.outputDirectory, "tools.json");
		const previous = await readFile(output, "utf8");
		await writeFile(input, invalid);
		await assert.rejects(generateTools(options), expected);
		assert.equal(await readFile(output, "utf8"), previous);
	});
}

test("duplicate names fail rather than silently dropping a source", async () => {
	const options = await fixture();
	await writeFile(path.join(options.inputDirectory, "first.yaml"), application);
	await writeFile(
		path.join(options.inputDirectory, "second.yaml"),
		application,
	);
	await assert.rejects(
		generateTools(options),
		/Duplicate application: Example/,
	);
});

test("missing or empty source directories fail rather than caching an empty catalog", async () => {
	const options = await fixture();
	await assert.rejects(generateTools(options), /No application YAML/);
	await rm(options.inputDirectory, { recursive: true });
	await assert.rejects(generateTools(options), /ENOENT/);
});

test("CLI generation resolves its paths independently of the working directory", async () => {
	// Run a copy with the same relative layout, avoiding writes to the shared checkout.
	const options = await fixture();
	const root = options.inputDirectory;
	const web = path.join(root, "apps/web");
	await mkdir(path.join(web, "scripts"), { recursive: true });
	await mkdir(path.join(root, "apps/cli/config/applications"), {
		recursive: true,
	});
	const original = fileURLToPath(
		new URL("../../apps/web/scripts/generate-tools.mjs", import.meta.url),
	);
	// ESM resolves js-yaml from the real script's dependencies via an absolute URL.
	const yaml = import.meta.resolve("js-yaml");
	await writeFile(
		path.join(web, "scripts/generate-tools.mjs"),
		(await readFile(original, "utf8")).replace(
			'"js-yaml"',
			JSON.stringify(yaml),
		),
	);
	await writeFile(
		path.join(root, "apps/cli/config/applications/app.yaml"),
		application,
	);
	const script = path.join(web, "scripts/generate-tools.mjs");
	const success = spawnSync(process.execPath, [script], {
		cwd: os.tmpdir(),
		encoding: "utf8",
	});
	assert.equal(success.status, 0, success.stderr);
	assert.equal(
		JSON.parse(
			await readFile(path.join(web, "app/generated/tools.json"), "utf8"),
		).tools.length,
		1,
	);
	await writeFile(
		path.join(root, "apps/cli/config/applications/app.yaml"),
		"name: [",
	);
	const failure = spawnSync(process.execPath, [script], {
		cwd: os.tmpdir(),
		encoding: "utf8",
	});
	assert.equal(failure.status, 1);
	assert.match(failure.stderr, /app.yaml/);
});
