// Tests for the per-plugin CI job script. Run: pnpm ci:test
// The script runs against fixture modules with fake go and golangci-lint
// executables that record their argv, so no Go toolchain work happens.
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, it } from "node:test";
import { fileURLToPath } from "node:url";

const repo = path.resolve(
	path.dirname(fileURLToPath(import.meta.url)),
	"../..",
);
const script = path.join(repo, "scripts/ci/test-plugin.sh");

let root;
let log;
/**
 * Writes a fixture file beneath the temporary repository root.
 * @param {string} file Path relative to the fixture root.
 * @param {string} [content]
 * @param {number} [mode] File permissions.
 */
function write(file, content = "", mode = 0o644) {
	const target = path.join(root, file);
	fs.mkdirSync(path.dirname(target), { recursive: true });
	fs.writeFileSync(target, content, { mode });
}

/**
 * Installs a fake executable that appends "<tool> <argv...>" to $FAKE_LOG and
 * exits with $FAKE_<TOOL>_<SUBCOMMAND>_EXIT (default 0).
 * @param {string} name Executable name, such as "go".
 */
function fake(name) {
	const variable = name.replace(/-/g, "_").toUpperCase();
	write(
		`bin/${name}`,
		`#!/bin/sh
printf '%s\\n' "${name} $*" >> "$FAKE_LOG"
code=$(eval "printf '%s' \\"\\\${FAKE_${variable}_$1_EXIT:-0}\\"")
exit "$code"
`,
		0o755,
	);
}

/**
 * Runs test-plugin.sh for a fixture plugin with only the fakes and system
 * directories on PATH, returning the process result and recorded tool calls.
 * @param {string} plugin Plugin directory name passed to the script.
 * @param {Record<string, string>} [env] Extra environment, such as fake exit codes.
 * @returns {import("node:child_process").SpawnSyncReturns<string> & {calls: string[]}}
 */
function run(plugin, env = {}) {
	const result = spawnSync("bash", [script, plugin], {
		encoding: "utf8",
		env: {
			PATH: `${path.join(root, "bin")}:/usr/bin:/bin`,
			DEVEX_CI_ROOT: root,
			FAKE_LOG: log,
			...env,
		},
	});
	const calls = fs.existsSync(log)
		? fs.readFileSync(log, "utf8").trim().split("\n").filter(Boolean)
		: [];
	return { ...result, calls };
}

beforeEach(() => {
	root = fs.mkdtempSync(path.join(os.tmpdir(), "test-plugin-"));
	log = path.join(root, "calls.log");
	fake("go");
	fake("golangci-lint");
	write("packages/nested-only/go.mod", "module example.com/nested\n");
	write("packages/nested-only/main.go", "package main\n");
	write("packages/nested-only/internal/foo/foo_test.go", "package foo\n");
	write("packages/no-tests/go.mod", "module example.com/none\n");
	write("packages/no-tests/main.go", "package main\n");
});

afterEach(() => fs.rmSync(root, { recursive: true, force: true }));

describe("test-plugin.sh", () => {
	it("runs go test for a module whose tests are only in nested packages", () => {
		const result = run("nested-only");
		assert.equal(result.status, 0, result.stdout + result.stderr);
		assert.deepEqual(result.calls, [
			"go mod tidy",
			"go mod download",
			"golangci-lint run",
			"go build -v -o devex-plugin-nested-only .",
			"go test -v ./...",
		]);
	});

	it("runs go test for every Go module, even without test files", () => {
		const result = run("no-tests");
		assert.equal(result.status, 0, result.stdout + result.stderr);
		assert.ok(result.calls.includes("go test -v ./..."));
	});

	it("fails with an actionable annotation when tests fail", () => {
		const result = run("nested-only", { FAKE_GO_test_EXIT: "1" });
		assert.equal(result.status, 1);
		assert.match(
			result.stdout,
			/::error title=go test failed::packages\/nested-only: run 'go test -v \.\/\.\.\.' in that directory/,
		);
	});

	it("fails on lint errors before building or testing", () => {
		const result = run("nested-only", { FAKE_GOLANGCI_LINT_run_EXIT: "1" });
		assert.equal(result.status, 1);
		assert.match(
			result.stdout,
			/::error title=golangci-lint failed::packages\/nested-only/,
		);
		assert.ok(!result.calls.some((call) => call.startsWith("go test")));
		assert.ok(!result.calls.some((call) => call.startsWith("go build")));
	});

	it("fails with an annotation when the build fails", () => {
		const result = run("nested-only", { FAKE_GO_build_EXIT: "1" });
		assert.equal(result.status, 1);
		assert.match(result.stdout, /::error title=go build failed::/);
		assert.ok(!result.calls.some((call) => call.startsWith("go test")));
	});

	it("rejects names that would leave the packages directory", () => {
		for (const name of ["", ".", "..", "../escape", "a/b"]) {
			const result = run(name);
			assert.equal(result.status, 2, `rejects ${JSON.stringify(name)}`);
			assert.deepEqual(result.calls, []);
		}
	});
});

/**
 * Returns the test-plugins job text from the real CI workflow.
 * @returns {string}
 */
function testPluginsJob() {
	const workflow = fs.readFileSync(
		path.join(repo, ".github/workflows/ci.yml"),
		"utf8",
	);
	return workflow.slice(
		workflow.indexOf("  test-plugins:"),
		workflow.indexOf("  test-js:"),
	);
}

/**
 * Returns the script text of the job's "Test plugin" step run key.
 * @param {string} job
 * @returns {string}
 */
function testPluginRun(job) {
	const match =
		/- name: Test plugin [^\n]*\n(?:\s+(?!run:)[^\n]*\n)*\s+run: ([^\n]+)\n/.exec(
			job,
		);
	assert.ok(match, "Test plugin step has a single-line run key");
	return match[1];
}

describe("CI workflow", () => {
	it("runs each selected plugin through test-plugin.sh without a root test-file gate", () => {
		const job = testPluginsJob();
		assert.equal(
			testPluginRun(job),
			'bash scripts/ci/test-plugin.sh "$PLUGIN_NAME"',
		);
		assert.doesNotMatch(job, /_test\.go/);
		assert.doesNotMatch(job, /\|\| echo "Linting failed/);
	});

	it("passes the matrix value through the environment, not run text", () => {
		const job = testPluginsJob();
		assert.match(
			job,
			/\n\s+env:\n(?:\s+#[^\n]*\n)*\s+PLUGIN_NAME: \$\{\{ matrix\.plugin \}\}\n\s+run: /,
		);
		for (const line of job.split("\n").filter((l) => l.includes("${{"))) {
			assert.match(
				line.trim(),
				/^(- name|name|if|plugin|PLUGIN_NAME|github_token): /,
				`expression only in non-script keys: ${line.trim()}`,
			);
		}
	});

	it("hands shell metacharacters in the plugin name to the script literally", () => {
		const marker = path.join(root, "injected");
		const hostile = `nested-only$(touch ${marker})"; touch ${marker}; "`;
		const result = spawnSync(
			"bash",
			["-e", "-c", testPluginRun(testPluginsJob())],
			{
				cwd: repo,
				encoding: "utf8",
				env: {
					PATH: `${path.join(root, "bin")}:/usr/bin:/bin`,
					DEVEX_CI_ROOT: root,
					FAKE_LOG: log,
					PLUGIN_NAME: hostile,
				},
			},
		);
		assert.notEqual(result.status, 0);
		assert.ok(!fs.existsSync(marker), "no command in the name was executed");
		assert.ok(!fs.existsSync(log), "no tool ran for an unknown plugin");
	});

	it("runs the workflow's command for a selected plugin", () => {
		const result = spawnSync(
			"bash",
			["-e", "-c", testPluginRun(testPluginsJob())],
			{
				cwd: repo,
				encoding: "utf8",
				env: {
					PATH: `${path.join(root, "bin")}:/usr/bin:/bin`,
					DEVEX_CI_ROOT: root,
					FAKE_LOG: log,
					PLUGIN_NAME: "nested-only",
				},
			},
		);
		assert.equal(result.status, 0, result.stdout + result.stderr);
		assert.ok(fs.readFileSync(log, "utf8").includes("go test -v ./..."));
	});
});
