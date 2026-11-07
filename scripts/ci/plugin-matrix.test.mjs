// Tests for CI plugin matrix selection. Run: pnpm ci:test
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { after, before, describe, it } from "node:test";
import { fileURLToPath } from "node:url";
import {
	recorderAdopters,
	SDKTEST_IMPORT,
	selectPlugins,
} from "./plugin-matrix.mjs";

const repo = path.resolve(
	path.dirname(fileURLToPath(import.meta.url)),
	"../..",
);
const script = path.join(repo, "scripts/ci/plugin-matrix.mjs");
const adopterTest = `package main\n\nimport (\n\t${SDKTEST_IMPORT}\n)\n`;

let root;
/**
 * Writes a fixture file beneath the temporary repository root.
 * @param {string} file Path relative to the fixture root.
 * @param {string} [content]
 */
function write(file, content = "") {
	const target = path.join(root, file);
	fs.mkdirSync(path.dirname(target), { recursive: true });
	fs.writeFileSync(target, content);
}

before(() => {
	root = fs.mkdtempSync(path.join(os.tmpdir(), "plugin-matrix-"));
	write("packages/plugin-sdk/sdktest/recorder_test.go", adopterTest);
	write("packages/plugin-sdk/main.go");
	write("packages/desktop-adopter/main.go");
	write("packages/desktop-adopter/command_contract_test.go", adopterTest);
	write("packages/nested-adopter/main.go");
	write("packages/nested-adopter/internal/fake_test.go", adopterTest);
	write("packages/plain-plugin/main.go");
	write("packages/plain-plugin/main_test.go", "package main\n");
	write("packages/node-plugin/package.json", "{}");
	write("packages/vendored/main.go");
	write("packages/vendored/node_modules/x/a_test.go", adopterTest);
	write("packages/vendored/notes.go", adopterTest);
	write("packages/not-a-plugin/README.md");
	write("packages/not-a-plugin/a_test.go", adopterTest);
});

after(() => fs.rmSync(root, { recursive: true, force: true }));

describe("recorderAdopters", () => {
	it("finds plugins whose tests import sdktest, at any depth", () => {
		assert.deepEqual(recorderAdopters(root), [
			"desktop-adopter",
			"nested-adopter",
		]);
	});
});

describe("selectPlugins", () => {
	it("selects only changed plugins for a plugin-only change", () => {
		assert.deepEqual(
			selectPlugins({
				root,
				sdkChanged: false,
				changedFiles: [
					"packages/plain-plugin/main.go",
					"packages/node-plugin/index.js",
					"packages/plain-plugin/README.md",
				],
			}),
			["node-plugin", "plain-plugin"],
		);
	});

	it("selects every adopter for an SDK-only change", () => {
		assert.deepEqual(
			selectPlugins({
				root,
				sdkChanged: true,
				changedFiles: ["packages/plugin-sdk/execution.go"],
			}),
			["desktop-adopter", "nested-adopter"],
		);
	});

	it("unions adopters with changed plugins without duplicates", () => {
		assert.deepEqual(
			selectPlugins({
				root,
				sdkChanged: true,
				changedFiles: [
					"packages/plugin-sdk/sdk.go",
					"packages/desktop-adopter/main.go",
					"packages/plain-plugin/main.go",
				],
			}),
			["desktop-adopter", "nested-adopter", "plain-plugin"],
		);
	});

	it("ignores the SDK, non-plugin directories, and files outside packages", () => {
		assert.deepEqual(
			selectPlugins({
				root,
				sdkChanged: false,
				changedFiles: [
					"packages/plugin-sdk/sdk.go",
					"packages/not-a-plugin/a_test.go",
					"apps/cli/main.go",
					"packages/README.md",
					"",
				],
			}),
			[],
		);
	});
});

describe("command line", () => {
	it("prints a JSON matrix for the repository", () => {
		const output = execFileSync(process.execPath, [script, "--sdk", "false"], {
			input: "packages/desktop-gnome/main.go\napps/cli/main.go\n",
		});
		assert.deepEqual(JSON.parse(output), ["desktop-gnome"]);
	});

	it("selects the repository's recorder adopters for SDK changes", () => {
		const selected = JSON.parse(
			execFileSync(process.execPath, [script, "--sdk", "true"], {
				input: "packages/plugin-sdk/execution.go\n",
			}),
		);
		for (const plugin of [
			"desktop-budgie",
			"desktop-gnome",
			"desktop-kde",
			"package-manager-apt",
			"package-manager-brew",
			"package-manager-dnf",
			"package-manager-snap",
			"package-manager-yay",
		]) {
			assert.ok(selected.includes(plugin), `${plugin} is selected`);
		}
		assert.ok(!selected.includes("plugin-sdk"));
	});

	it("rejects a missing or invalid --sdk value", () => {
		for (const args of [[], ["--sdk"], ["--sdk", "yes"]]) {
			assert.throws(
				() =>
					execFileSync(process.execPath, [script, ...args], {
						input: "",
						stdio: "pipe",
					}),
				(error) => error.status === 2,
			);
		}
	});
});
