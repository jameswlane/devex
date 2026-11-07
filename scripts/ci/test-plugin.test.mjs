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
function write(file, content = "", mode = 0o644) {
	const target = path.join(root, file);
	fs.mkdirSync(path.dirname(target), { recursive: true });
	fs.writeFileSync(target, content, { mode });
}

// fake records "<tool> <argv...>" and exits with $FAKE_<TOOL>_<SUBCOMMAND>_EXIT.
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

describe("CI workflow", () => {
	it("runs each selected plugin through test-plugin.sh without a root test-file gate", () => {
		const workflow = fs.readFileSync(
			path.join(repo, ".github/workflows/ci.yml"),
			"utf8",
		);
		const job = workflow.slice(
			workflow.indexOf("  test-plugins:"),
			workflow.indexOf("  test-js:"),
		);
		assert.match(
			job,
			/run: bash scripts\/ci\/test-plugin\.sh "\$\{\{ matrix\.plugin \}\}"/,
		);
		assert.doesNotMatch(job, /_test\.go/);
		assert.doesNotMatch(job, /\|\| echo "Linting failed/);
	});
});
