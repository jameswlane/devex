import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, test } from "node:test";
import { fileURLToPath } from "node:url";
import { load } from "js-yaml";

const repo = fileURLToPath(new URL("../../", import.meta.url));
const turbo = path.join(repo, "node_modules/.bin/turbo");
const fixtures = [];
afterEach(() => {
	for (const root of fixtures.splice(0))
		fs.rmSync(root, { recursive: true, force: true });
});

function write(root, file, content) {
	const target = path.join(root, file);
	fs.mkdirSync(path.dirname(target), { recursive: true });
	fs.writeFileSync(target, content);
}

function fixture() {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), "devex-turbo-"));
	fixtures.push(root);
	for (const file of [
		"package.json",
		"turbo.json",
		"pnpm-workspace.yaml",
		"pnpm-lock.yaml",
		"biome.json",
		"packages/Taskfile.plugin.yml",
		".mise.toml",
		"scripts/turbo/run.mjs",
		"scripts/turbo/go-task.mjs",
		"scripts/turbo/toolchain.mjs",
	]) {
		write(root, file, fs.readFileSync(path.join(repo, file)));
	}
	for (const parent of ["apps", "packages"]) {
		for (const entry of fs.readdirSync(path.join(repo, parent), {
			withFileTypes: true,
		})) {
			if (!entry.isDirectory()) continue;
			for (const name of ["package.json", "turbo.json"]) {
				const file = `${parent}/${entry.name}/${name}`;
				if (fs.existsSync(path.join(repo, file)))
					write(root, file, fs.readFileSync(path.join(repo, file)));
			}
		}
	}
	write(
		root,
		".gitignore",
		"node_modules/\n.turbo/\n**/app/generated/\n**/devex-plugin-*\n!**/devex-plugin-*.go\n",
	);
	for (const file of [
		"go.mod",
		"go.sum",
		"Taskfile.yml",
		".golangci.yml",
		"devex-plugin-apt.go",
	])
		write(root, `packages/package-manager-apt/${file}`, `original ${file}\n`);
	write(root, "packages/plugin-sdk/go.mod", "module example.com/sdk\n");
	write(root, "packages/plugin-sdk/go.sum", "original\n");
	write(root, "packages/plugin-sdk/shared.go", "package sdk\n");
	write(root, "apps/web/scripts/generate-tools.mjs", "// generator\n");
	write(root, "apps/cli/config/applications/test.yaml", "name: test\n");
	execFileSync("git", ["init", "-q"], { cwd: root });
	execFileSync("git", ["add", "."], { cwd: root });
	return root;
}

function run(root, args, env = {}) {
	return execFileSync(turbo, ["run", ...args, "--daemon=false"], {
		cwd: root,
		encoding: "utf8",
		env: { ...process.env, TURBO_TELEMETRY_DISABLED: "1", ...env },
		stdio: ["ignore", "pipe", "pipe"],
	});
}
function dry(root, task, packageName, env = {}) {
	return JSON.parse(
		run(root, [task, `--filter=@devex/${packageName}`, "--dry=json"], env),
	).tasks;
}
function task(root, name, packageName, env = {}) {
	return dry(root, name, packageName, env).find(
		(entry) => entry.taskId === `@devex/${packageName}#${name}`,
	);
}
function change(root, file) {
	fs.appendFileSync(path.join(root, file), "\n# changed\n");
}
function script(root, packagePath, scripts) {
	const file = `${packagePath}/package.json`;
	const manifest = JSON.parse(fs.readFileSync(path.join(root, file)));
	Object.assign(manifest.scripts, scripts);
	write(root, file, JSON.stringify(manifest));
}

test("lint hashes module manifests, local/shared Taskfiles, root Biome, and SDK manifests", () => {
	const root = fixture();
	let previous = task(root, "lint", "package-manager-apt").hash;
	for (const file of [
		"packages/package-manager-apt/go.mod",
		"packages/package-manager-apt/go.sum",
		"packages/package-manager-apt/Taskfile.yml",
		"packages/package-manager-apt/.golangci.yml",
		"packages/Taskfile.plugin.yml",
		"packages/plugin-sdk/go.mod",
		"packages/plugin-sdk/go.sum",
		"packages/plugin-sdk/shared.go",
	]) {
		change(root, file);
		const next = task(root, "lint", "package-manager-apt").hash;
		assert.notEqual(next, previous, file);
		previous = next;
	}
	const manifestHash = task(root, "lint", "package-manager-apt").hash;
	script(root, "packages/package-manager-apt", { lint: "echo changed lint" });
	assert.notEqual(task(root, "lint", "package-manager-apt").hash, manifestHash);
	const registryHash = task(root, "lint", "registry").hash;
	const biome = JSON.parse(fs.readFileSync(path.join(root, "biome.json")));
	biome.formatter.lineWidth = 100;
	write(root, "biome.json", JSON.stringify(biome));
	assert.notEqual(task(root, "lint", "registry").hash, registryHash);
});

test("real web build depends on generation; catalog and generator edits invalidate both hashes", () => {
	const root = fixture();
	const build = task(root, "build", "web");
	assert.ok(build.dependencies.includes("@devex/web#generate"));
	let generation = task(root, "generate", "web");
	assert.ok(generation.outputs.includes("app/generated/**"));
	let buildHash = build.hash;
	for (const file of [
		"apps/cli/config/applications/test.yaml",
		"apps/web/scripts/generate-tools.mjs",
	]) {
		change(root, file);
		const next = task(root, "generate", "web");
		assert.notEqual(next.hash, generation.hash, file);
		const nextBuild = task(root, "build", "web").hash;
		assert.notEqual(nextBuild, buildHash, file);
		generation = next;
		buildHash = nextBuild;
	}
});

test("registry build, dev and typecheck depend on same-package Prisma generation", () => {
	const root = fixture();
	for (const name of ["build", "dev", "typecheck", "test", "db:seed"]) {
		const entry = task(root, name, "registry");
		assert.ok(entry.dependencies.includes("@devex/registry#db:generate"), name);
		assert.notEqual(entry.command, "<NONEXISTENT>");
	}
	for (const name of ["build", "dev"]) {
		assert.ok(
			!task(root, name, "web").dependencies.some((dependency) =>
				dependency.endsWith("#db:generate"),
			),
		);
	}
});

test("configured task names have real workspace scripts, and CLI wrappers target real tasks", () => {
	const config = JSON.parse(fs.readFileSync(path.join(repo, "turbo.json")));
	const root = fixture();
	const scripts = new Set();
	for (const parent of ["apps", "packages"]) {
		for (const entry of fs.readdirSync(path.join(root, parent))) {
			const file = path.join(root, parent, entry, "package.json");
			if (fs.existsSync(file))
				for (const name of Object.keys(
					JSON.parse(fs.readFileSync(file)).scripts || {},
				))
					scripts.add(name);
		}
	}
	for (const name of Object.keys(config.tasks))
		assert.ok(scripts.has(name), name);
	const cliTasks = fs.readFileSync(
		path.join(repo, "apps/cli/Taskfile.yml"),
		"utf8",
	);
	for (const name of ["build", "test", "lint"]) {
		assert.ok(
			cliTasks.includes(`pnpm turbo:run run ${name} --filter=@devex/cli`),
		);
		assert.notEqual(task(root, name, "cli").command, "<NONEXISTENT>");
	}
	const manifest = JSON.parse(fs.readFileSync(path.join(repo, "package.json")));
	assert.equal(
		manifest.scripts.typecheck,
		"node scripts/turbo/run.mjs run typecheck",
	);
});

test("cache restoration preserves plugin sources while restoring the existing binary path", () => {
	const root = fixture();
	const plugin = "packages/package-manager-apt";
	script(root, plugin, {
		build:
			"node -e \"require('fs').writeFileSync('devex-plugin-package-manager-apt','binary')\"",
		download: 'node -e ""',
	});
	script(root, "packages/plugin-sdk", {
		build: 'node -e ""',
		download: 'node -e ""',
	});
	run(root, ["build", "--filter=@devex/package-manager-apt"]);
	const source = `${plugin}/devex-plugin-apt.go`;
	const entry = task(root, "build", "package-manager-apt");
	assert.ok(Object.hasOwn(entry.inputs, "devex-plugin-apt.go"));
	const hash = entry.hash;
	const cached = task(root, "build", "package-manager-apt");
	assert.equal(cached.cache.local, true);
	fs.unlinkSync(path.join(root, plugin, "devex-plugin-package-manager-apt"));
	const result = run(root, ["build", "--filter=@devex/package-manager-apt"]);
	assert.match(result, /cache hit/);
	assert.equal(
		fs.readFileSync(
			path.join(root, plugin, "devex-plugin-package-manager-apt"),
			"utf8",
		),
		"binary",
	);
	assert.equal(
		fs.readFileSync(path.join(root, source), "utf8"),
		"original devex-plugin-apt.go\n",
	);
	change(root, source);
	assert.notEqual(task(root, "build", "package-manager-apt").hash, hash);
	// Inspect the actual cache manifest: source files must never be restored.
	const archive = path.join(root, ".turbo/cache", `${hash}.tar.zst`);
	const listing = execFileSync("tar", ["--zstd", "-tf", archive], {
		encoding: "utf8",
	});
	assert.match(listing, /devex-plugin-package-manager-apt/);
	assert.doesNotMatch(listing, /devex-plugin-apt\.go/);
});

test("generation failures stop the web build before it executes", () => {
	const root = fixture();
	script(root, "apps/web", {
		generate: 'node -e "process.exit(7)"',
		build: "node -e \"require('fs').writeFileSync('build-ran','yes')\"",
	});
	const result = spawnSync(
		turbo,
		["run", "build", "--filter=@devex/web", "--daemon=false"],
		{ cwd: root, encoding: "utf8" },
	);
	assert.notEqual(result.status, 0);
	assert.match(result.stdout + result.stderr, /generate/);
	assert.equal(fs.existsSync(path.join(root, "apps/web/build-ran")), false);
});

test("a generation cache hit restores the catalog without rerunning its script", () => {
	const root = fixture();
	script(root, "apps/web", {
		generate:
			"node -e \"const fs=require('fs');fs.mkdirSync('app/generated',{recursive:true});fs.writeFileSync('app/generated/tools.json','catalog');console.log('generated catalog')\"",
	});
	run(root, ["generate", "--filter=@devex/web"]);
	fs.rmSync(path.join(root, "apps/web/app/generated"), { recursive: true });
	const result = run(root, ["generate", "--filter=@devex/web"]);
	assert.match(result, /cache hit/);
	assert.equal(
		fs.readFileSync(
			path.join(root, "apps/web/app/generated/tools.json"),
			"utf8",
		),
		"catalog",
	);
});

test("Prisma generation failures prevent registry client consumers", () => {
	const root = fixture();
	script(root, "apps/registry", {
		"db:generate": 'node -e "process.exit(9)"',
		"db:seed": "node -e \"require('fs').writeFileSync('db:seed-ran','yes')\"",
		build: "node -e \"require('fs').writeFileSync('build-ran','yes')\"",
		dev: "node -e \"require('fs').writeFileSync('dev-ran','yes')\"",
		typecheck: "node -e \"require('fs').writeFileSync('typecheck-ran','yes')\"",
	});
	for (const name of ["build", "dev", "typecheck", "test", "db:seed"]) {
		const result = spawnSync(
			turbo,
			["run", name, "--filter=@devex/registry", "--daemon=false"],
			{ cwd: root, encoding: "utf8" },
		);
		assert.notEqual(result.status, 0);
		assert.match(result.stdout + result.stderr, /db:generate/);
		assert.equal(
			fs.existsSync(path.join(root, `apps/registry/${name}-ran`)),
			false,
		);
	}
});

test("Go tests compile independently and hash SDK, fixtures, defaults and shared configuration", () => {
	const root = fixture();
	const module = "packages/package-manager-apt";
	for (const file of ["testdata/example.json", "config/default.yaml"])
		write(root, `${module}/${file}`, "original\n");
	for (const name of ["cli", "plugin-sdk", "package-manager-apt", "web"]) {
		const entry = task(root, "test", name);
		assert.deepEqual(entry.dependencies, [], name);
	}
	let previous = task(root, "test", "package-manager-apt").hash;
	for (const file of [
		"packages/plugin-sdk/shared.go",
		"packages/plugin-sdk/go.mod",
		"packages/plugin-sdk/go.sum",
		`${module}/testdata/example.json`,
		`${module}/config/default.yaml`,
		`${module}/Taskfile.yml`,
		".mise.toml",
		"scripts/turbo/go-task.mjs",
	]) {
		change(root, file);
		const next = task(root, "test", "package-manager-apt").hash;
		assert.notEqual(next, previous, file);
		previous = next;
	}
	const config = JSON.parse(
		fs.readFileSync(path.join(root, "packages/turbo-go/turbo.json")),
	);
	config.tasks.test.env.push("EXAMPLE_TOOL_FLAG");
	write(root, "packages/turbo-go/turbo.json", JSON.stringify(config));
	assert.notEqual(task(root, "test", "package-manager-apt").hash, previous);
	assert.equal((task(root, "build", "plugin-sdk").outputs || []).length, 0);
});

test("database/Redis variables and Next env files never affect Go task hashes", () => {
	const root = fixture();
	const variables = {
		DATABASE_URL: "postgres://example/database",
		DIRECT_DATABASE_URL: "postgres://example/direct",
		PRISMA_DATABASE_URL: "postgres://example/prisma",
		POSTGRES_URL: "postgres://example/postgres",
		REDIS_URL: "redis://example",
		UPSTASH_REDIS_REST_TOKEN: "example-token",
	};
	for (const name of ["build", "test", "lint"]) {
		const original = task(root, name, "package-manager-apt").hash;
		assert.equal(
			task(root, name, "package-manager-apt", variables).hash,
			original,
		);
	}
	const goHash = task(root, "test", "package-manager-apt").hash;
	const webHash = task(root, "test", "web").hash;
	const registryHash = task(root, "test", "registry").hash;
	write(root, "apps/registry/.env.test.local", "DATABASE_URL=example\n");
	assert.notEqual(task(root, "test", "registry").hash, registryHash);
	assert.equal(task(root, "test", "web").hash, webHash);
	write(root, "apps/web/.env.test.local", "NEXT_PUBLIC_EXAMPLE=example\n");
	assert.notEqual(task(root, "test", "web").hash, webHash);
	assert.equal(task(root, "test", "package-manager-apt").hash, goHash);
	assert.notEqual(
		task(root, "test", "registry", variables).hash,
		task(root, "test", "registry").hash,
	);
});

test("strict environment passes registry runtime settings and web test/dev settings", () => {
	const root = fixture();
	const variables = {
		PRISMA_DATABASE_URL: "example-prisma",
		DIRECT_DATABASE_URL: "example-direct",
		REDIS_PASSWORD: "example-redis",
		UPSTASH_REDIS_REST_TOKEN: "example-upstash",
		REGISTRY_ADMIN_API_KEY: "example-admin",
		REGISTRY_RELEASE_WEBHOOK_SECRET: "example-release",
		HEALTH_CHECK_TIMEOUT_MS: "1234",
		NEXT_PUBLIC_EXAMPLE: "example-public",
		NODE_ENV: "test",
	};
	const check =
		"node -e \"const assert=require('assert/strict');for(const [key,value] of Object.entries(" +
		JSON.stringify(variables).replaceAll('"', '\\"') +
		'))assert.equal(process.env[key],value,key)"';
	script(root, "apps/registry", {
		test: check,
		dev: check,
		"db:generate": 'node -e ""',
	});
	for (const name of ["test", "dev"])
		run(root, [name, "--filter=@devex/registry"], variables);
	const webCheck =
		"node -e \"require('assert/strict').equal(process.env.NEXT_PUBLIC_EXAMPLE,'example-public');require('assert/strict').equal(process.env.NODE_ENV,'test')\"";
	script(root, "apps/web", { test: webCheck, dev: webCheck });
	for (const name of ["test", "dev"])
		run(root, [name, "--filter=@devex/web"], variables);
});

test("vulnerability scans execute on every invocation and propagate scanner failure", () => {
	const root = fixture();
	// Keep the actual package command/runner: only replace the Go executable.
	const bin = path.join(root, "fake-bin");
	fs.appendFileSync(path.join(root, ".gitignore"), "scan-count\nfail-scan\n");
	write(
		root,
		"fake-bin/go",
		`#!/usr/bin/env node
const fs = require('fs');
if (process.argv[2] === 'env') { console.log('go-test'); process.exit(0); }
require('assert/strict').deepEqual(process.argv.slice(2), ['run','golang.org/x/vuln/cmd/govulncheck@v1.9.0','./...']);
fs.appendFileSync('scan-count', 'scan\\n');
if (fs.existsSync('fail-scan')) process.exit(17);
`,
	);
	fs.chmodSync(path.join(bin, "go"), 0o755);
	const env = {
		PATH: `${bin}${path.delimiter}${process.env.PATH}`,
		DEVEX_GO_TOOLCHAIN: '["go-test"]',
	};
	const firstHash = task(root, "vulncheck", "package-manager-apt", env).hash;
	for (let attempt = 0; attempt < 2; attempt++) {
		run(root, ["vulncheck", "--filter=@devex/package-manager-apt"], env);
		assert.equal(
			task(root, "vulncheck", "package-manager-apt", env).hash,
			firstHash,
		);
	}
	assert.equal(
		fs.readFileSync(
			path.join(root, "packages/package-manager-apt/scan-count"),
			"utf8",
		),
		"scan\nscan\n",
	);
	assert.equal(
		task(root, "vulncheck", "package-manager-apt").resolvedTaskDefinition.cache,
		false,
	);
	write(root, "packages/package-manager-apt/fail-scan", "fail");
	assert.throws(() =>
		run(root, ["vulncheck", "--filter=@devex/package-manager-apt"], env),
	);
	assert.equal(
		fs.readFileSync(
			path.join(root, "packages/package-manager-apt/scan-count"),
			"utf8",
		),
		"scan\nscan\nscan\n",
	);
});

test("active Go and lint tool versions change only relevant task hashes", () => {
	const root = fixture();
	for (const name of ["build", "test", "lint"]) {
		assert.notEqual(
			task(root, name, "package-manager-apt", {
				DEVEX_GO_TOOLCHAIN: '["go1.26.2"]',
			}).hash,
			task(root, name, "package-manager-apt", {
				DEVEX_GO_TOOLCHAIN: '["go1.26.3"]',
			}).hash,
		);
	}
	for (const name of ["build", "test"])
		assert.equal(
			task(root, name, "package-manager-apt", {
				DEVEX_GOLANGCI_VERSION: "2.13",
			}).hash,
			task(root, name, "package-manager-apt", {
				DEVEX_GOLANGCI_VERSION: "2.14",
			}).hash,
		);
	assert.notEqual(
		task(root, "lint", "package-manager-apt", {
			DEVEX_GOLANGCI_VERSION: "2.13",
		}).hash,
		task(root, "lint", "package-manager-apt", {
			DEVEX_GOLANGCI_VERSION: "2.14",
		}).hash,
	);
	assert.equal(
		task(root, "test", "web", { DEVEX_GO_TOOLCHAIN: '["go-other"]' }).hash,
		task(root, "test", "web").hash,
	);
});

test("root runner fingerprints real tools, respects overrides, and fails when tools cannot resolve", () => {
	const root = fixture();
	fs.symlinkSync(
		path.join(repo, "node_modules"),
		path.join(root, "node_modules"),
		"dir",
	);
	write(
		root,
		"fake-bin/go",
		`#!/usr/bin/env node
if(process.env.FAIL_TOOL)process.exit(19);
require('assert/strict').equal(process.argv[2], 'env');
for (const key of ['GOVERSION','GOOS','GOARCH','CGO_ENABLED','GOFLAGS']) require('assert/strict').ok(process.argv.includes(key));
console.log(process.env.GOTOOLCHAIN || 'go-fixture-1');
`,
	);
	write(
		root,
		"fake-bin/golangci-lint",
		`#!/usr/bin/env node
console.log(require('fs').readFileSync(${JSON.stringify(path.join(root, "lint-version"))},'utf8').trim());
`,
	);
	for (const command of ["go", "golangci-lint"])
		fs.chmodSync(path.join(root, "fake-bin", command), 0o755);
	write(root, "lint-version", "golangci-fixture-1");
	const env = {
		...process.env,
		PATH: `${root}/fake-bin${path.delimiter}${process.env.PATH}`,
		DEVEX_GO_TOOLCHAIN: "forged",
		DEVEX_GOLANGCI_VERSION: "forged",
	};
	const invoke = (name, extra = {}) =>
		spawnSync(
			process.execPath,
			[
				path.join(root, "scripts/turbo/run.mjs"),
				"run",
				name,
				"--filter=@devex/package-manager-apt",
				"--dry=json",
			],
			{ encoding: "utf8", env: { ...env, ...extra } },
		);
	const get = (name, extra) => {
		const result = invoke(name, extra);
		assert.equal(result.status, 0, result.stderr);
		return JSON.parse(result.stdout).tasks.find(
			(entry) => entry.taskId === `@devex/package-manager-apt#${name}`,
		);
	};
	const first = get("test");
	assert.ok(
		first.environmentVariables.configured.some((entry) =>
			entry.startsWith("DEVEX_GO_TOOLCHAIN="),
		),
	);
	assert.notEqual(
		get("test", { GOTOOLCHAIN: "go-fixture-2" }).hash,
		first.hash,
	);
	const lint = get("lint");
	write(root, "lint-version", "golangci-fixture-2");
	assert.notEqual(get("lint").hash, lint.hash);
	assert.equal(get("test").hash, first.hash);
	assert.notEqual(invoke("test", { FAIL_TOOL: "1" }).status, 0);
	// Next-only commands must work even if Go and lint are unavailable.
	const next = spawnSync(
		process.execPath,
		[
			path.join(root, "scripts/turbo/run.mjs"),
			"run",
			"test",
			"--filter=@devex/web",
			"--dry=json",
		],
		{ encoding: "utf8", env: { ...env, FAIL_TOOL: "1" } },
	);
	assert.equal(next.status, 0, next.stderr);
	// Full Go graphs exceed spawnSync's default output buffer in real checkouts.
	const full = spawnSync(
		process.execPath,
		[
			path.join(root, "scripts/turbo/run.mjs"),
			"run",
			"build",
			"lint",
			"test",
			"vulncheck",
			"--filter=@devex/cli",
			"--filter=./packages/*",
			"--dry=json",
		],
		{ encoding: "utf8", env, maxBuffer: 64 * 1024 * 1024 },
	);
	assert.equal(full.status, 0, full.stderr);
	assert.ok(JSON.parse(full.stdout).tasks.length > 190);
});

test("raw Turbo cannot populate Go cache entries without measured tool fingerprints", () => {
	const root = fixture();
	write(
		root,
		"fake-bin/go",
		"#!/usr/bin/env node\nconsole.log('go-fixture');\n",
	);
	fs.chmodSync(path.join(root, "fake-bin/go"), 0o755);
	const result = spawnSync(
		turbo,
		["run", "test", "--filter=@devex/package-manager-apt", "--daemon=false"],
		{
			cwd: root,
			encoding: "utf8",
			env: {
				...process.env,
				DEVEX_GO_TOOLCHAIN: "",
				PATH: `${root}/fake-bin${path.delimiter}${process.env.PATH}`,
			},
		},
	);
	assert.notEqual(result.status, 0);
	assert.match(result.stdout + result.stderr, /Use pnpm turbo:run/);
	assert.equal(task(root, "test", "package-manager-apt").cache.local, false);
});

for (const name of ["Lint CLI", "Build CLI", "Test CLI"]) {
	test(`checked-in CI command executes ${name} through measured tool guards`, () => {
		const workflow = load(
			fs.readFileSync(path.join(repo, ".github/workflows/ci.yml"), "utf8"),
		);
		const command = workflow.jobs["test-cli"].steps.find(
			(step) => step.name === name,
		).run;
		const root = fixture();
		fs.symlinkSync(
			path.join(repo, "node_modules"),
			path.join(root, "node_modules"),
			"dir",
		);
		write(root, "apps/cli/go.mod", "module example.com/cli\n");
		for (const tool of ["go", "golangci-lint"]) {
			write(
				root,
				`fake-bin/${tool}`,
				`#!/usr/bin/env node
const fs = require('fs');
const args = process.argv.slice(2);
if (args[0] === 'env') console.log('go-ci-fixture');
else if (args.join(' ') === 'version') console.log('golangci-ci-fixture');
else fs.appendFileSync(${JSON.stringify(path.join(root, "executed.jsonl"))}, JSON.stringify({tool:${JSON.stringify(tool)},args,hash:process.env.TURBO_HASH})+'\\n');
`,
			);
			fs.chmodSync(path.join(root, `fake-bin/${tool}`), 0o755);
		}
		const env = {
			...process.env,
			PATH: `${root}/fake-bin${path.delimiter}${process.env.PATH}`,
			TURBO_TELEMETRY_DISABLED: "1",
			DEVEX_GO_TOOLCHAIN: "",
			DEVEX_GOLANGCI_VERSION: "",
		};
		// Execute the checked-in shell command verbatim. Actual package scripts and
		// go-task guard stay intact; only external Go/lint executables are faked.
		const result = spawnSync("sh", ["-c", command], {
			cwd: root,
			env,
			encoding: "utf8",
		});
		assert.equal(result.status, 0, result.stdout + result.stderr);
		const calls = fs
			.readFileSync(path.join(root, "executed.jsonl"), "utf8")
			.trim()
			.split("\n")
			.map((line) => JSON.parse(line));
		const expected =
			name === "Lint CLI" ? "run" : name === "Build CLI" ? "build" : "test";
		assert.ok(
			calls.some((call) => call.args[0] === expected && call.hash),
			JSON.stringify(calls),
		);
	});
}

test("shared configuration changes route to relevant existing CI jobs", () => {
	const workflow = load(
		fs.readFileSync(path.join(repo, ".github/workflows/ci.yml"), "utf8"),
	);
	const filters = load(
		workflow.jobs["detect-changes"].steps.find((step) => step.id === "changes")
			.with.filters,
	);
	const matches = (patterns, file) =>
		patterns.some(
			(pattern) =>
				!pattern.startsWith("!") &&
				(pattern.endsWith("/**")
					? file.startsWith(pattern.slice(0, -2))
					: file === pattern),
		);
	const common = [
		"turbo.json",
		"package.json",
		"pnpm-lock.yaml",
		"pnpm-workspace.yaml",
		".mise.toml",
		"biome.json",
		"scripts/turbo/run.mjs",
	];
	for (const [file, jobs] of [
		...common.map((file) => [file, ["cli", "sdk", "web", "registry", "docs"]]),
		["packages/turbo-go/turbo.json", ["cli", "sdk"]],
		["packages/Taskfile.plugin.yml", ["cli", "sdk"]],
		["packages/turbo-next/turbo.json", ["web", "registry", "docs"]],
	]) {
		for (const event of ["push", "pull_request"])
			assert.ok(matches(workflow.on[event].paths, file), `${event}: ${file}`);
		for (const job of jobs)
			assert.ok(matches(filters[job], file), `${file} must select ${job}`);
	}
	const regressionWorkflow = load(
		fs.readFileSync(path.join(repo, ".github/workflows/turbo.yml"), "utf8"),
	);
	for (const event of ["push", "pull_request"])
		assert.ok(
			matches(regressionWorkflow.on[event].paths, ".github/workflows/ci.yml"),
		);
});

test("registry seeding waits for the generated client before executing", () => {
	const root = fixture();
	script(root, "apps/registry", {
		"db:generate":
			"node -e \"require('fs').writeFileSync('client-ready','yes')\"",
		"db:seed":
			"node -e \"require('assert/strict').equal(require('fs').readFileSync('client-ready','utf8'),'yes');require('fs').writeFileSync('seed-ran','yes')\"",
	});
	run(root, ["db:seed", "--filter=@devex/registry"]);
	assert.equal(
		fs.readFileSync(path.join(root, "apps/registry/seed-ran"), "utf8"),
		"yes",
	);
});

test("runner probes only deduplicated executable Go graph directories and propagates selected failures", () => {
	const root = fixture();
	fs.symlinkSync(
		path.join(repo, "node_modules"),
		path.join(root, "node_modules"),
		"dir",
	);
	write(root, "apps/cli/go.mod", "module example.com/cli\n");
	// Give the filtered CLI a graph dependency to exercise transitive selection.
	const cli = JSON.parse(
		fs.readFileSync(path.join(root, "apps/cli/package.json")),
	);
	cli.devDependencies = {
		...cli.devDependencies,
		"@devex/plugin-sdk": "workspace:*",
	};
	write(root, "apps/cli/package.json", JSON.stringify(cli));
	const calls = path.join(root, "probes.jsonl");
	write(
		root,
		"fake-bin/go",
		`#!/usr/bin/env node
const fs = require('fs');
fs.appendFileSync(${JSON.stringify(calls)}, JSON.stringify(process.cwd())+'\\n');
if(process.cwd().endsWith(process.env.FAIL_MODULE))process.exit(19);
console.log('go-fixture');
`,
	);
	fs.chmodSync(path.join(root, "fake-bin/go"), 0o755);
	const invoke = (tasks, fail) =>
		spawnSync(
			process.execPath,
			[
				path.join(root, "scripts/turbo/run.mjs"),
				"run",
				...tasks,
				"--filter=@devex/cli",
				"--dry=json",
			],
			{
				encoding: "utf8",
				env: {
					...process.env,
					PATH: `${root}/fake-bin${path.delimiter}${process.env.PATH}`,
					FAIL_MODULE: fail,
				},
			},
		);
	const result = invoke(["build", "test"], "package-manager-apt");
	assert.equal(result.status, 0, result.stderr);
	assert.deepEqual(
		fs.readFileSync(calls, "utf8").trim().split("\n").map(JSON.parse).sort(),
		["apps/cli", "packages/plugin-sdk"]
			.map((directory) => path.join(root, directory))
			.sort(),
	);
	for (const module of ["cli", "plugin-sdk"]) {
		const failure = invoke(["build"], module);
		assert.notEqual(failure.status, 0);
		assert.match(failure.stderr, /Turbo toolchain setup failed/);
	}
	// A graph node without a script cannot execute and must not require a toolchain.
	const sdk = JSON.parse(
		fs.readFileSync(path.join(root, "packages/plugin-sdk/package.json")),
	);
	delete sdk.scripts.build;
	delete sdk.scripts.download;
	write(root, "packages/plugin-sdk/package.json", JSON.stringify(sdk));
	fs.unlinkSync(calls);
	const transit = invoke(["build"], "plugin-sdk");
	assert.equal(transit.status, 0, transit.stderr);
	assert.deepEqual(
		fs.readFileSync(calls, "utf8").trim().split("\n").map(JSON.parse),
		[path.join(root, "apps/cli")],
	);
});

test("persistent GOENV edits change hashes and the execution guard rejects stale settings", async () => {
	const root = fixture();
	fs.symlinkSync(
		path.join(repo, "node_modules"),
		path.join(root, "node_modules"),
		"dir",
	);
	write(root, "apps/cli/go.mod", "module example.com/cli\ngo 1.26\n");
	const goenv = path.join(root, "isolated-goenv");
	const env = { ...process.env, GOENV: goenv };
	for (const key of ["GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS"])
		delete env[key];
	const invoke = () => {
		const result = spawnSync(
			process.execPath,
			[
				path.join(root, "scripts/turbo/run.mjs"),
				"run",
				"test",
				"--filter=@devex/cli",
				"--dry=json",
			],
			{ env, encoding: "utf8" },
		);
		assert.equal(result.status, 0, result.stderr);
		return JSON.parse(result.stdout).tasks.find(
			(task) => task.taskId === "@devex/cli#test",
		).hash;
	};
	let previous = invoke();
	for (const setting of [
		"GOOS=freebsd",
		"GOARCH=arm64",
		"CGO_ENABLED=1",
		"CGO_ENABLED=0",
		"GOFLAGS=-trimpath",
	]) {
		execFileSync("go", ["env", "-w", setting], {
			cwd: path.join(root, "apps/cli"),
			env,
		});
		const next = invoke();
		assert.notEqual(next, previous, setting);
		previous = next;
	}
	// Capture a valid fingerprint, mutate only the persistent file, then run guard.
	const { goFingerprint } = await import("./toolchain.mjs");
	const fingerprint = goFingerprint(path.join(root, "apps/cli"), env);
	execFileSync("go", ["env", "-w", "GOFLAGS=-tags=changed"], {
		cwd: path.join(root, "apps/cli"),
		env,
	});
	const guard = spawnSync(
		process.execPath,
		[
			path.join(root, "scripts/turbo/go-task.mjs"),
			process.execPath,
			"-e",
			"process.exit(0)",
		],
		{
			cwd: path.join(root, "apps/cli"),
			encoding: "utf8",
			env: {
				...env,
				TURBO_HASH: "stale",
				DEVEX_GO_TOOLCHAIN: JSON.stringify([fingerprint]),
			},
		},
	);
	assert.notEqual(guard.status, 0);
	assert.match(guard.stderr, /Use pnpm turbo:run/);
});

test("per-deployment VERCEL_URL does not bust Next.js build cache hashes", () => {
	const root = fixture();
	for (const app of ["registry", "web", "docs"]) {
		const first = task(root, "build", app, {
			VERCEL: "1",
			VERCEL_URL: "devex-preview-aaa.vercel.app",
			NEXT_PUBLIC_VERCEL_URL: "devex-preview-aaa.vercel.app",
		}).hash;
		const second = task(root, "build", app, {
			VERCEL: "1",
			VERCEL_URL: "devex-preview-bbb.vercel.app",
			NEXT_PUBLIC_VERCEL_URL: "devex-preview-bbb.vercel.app",
		}).hash;
		assert.equal(second, first, app);
	}
	assert.equal(
		task(root, "build", "registry", {
			POSTGRES_URL: "postgres://a",
			KV_REST_API_READ_ONLY_TOKEN: "tok-a",
		}).hash,
		task(root, "build", "registry", {
			POSTGRES_URL: "postgres://b",
			KV_REST_API_READ_ONLY_TOKEN: "tok-b",
		}).hash,
	);
});

test("go-task dev-build keeps -race on windows/amd64 and omits it on windows/arm64", async () => {
	const { isDirectExecution, resolveGoTask } = await import("./go-task.mjs");
	assert.deepEqual(
		resolveGoTask("dev-build", ["devex-plugin-package-manager-winget", "."], {
			env: {},
			platform: "win32",
			arch: "x64",
		}),
		{
			command: "go",
			args: [
				"build",
				"-race",
				"-o",
				"devex-plugin-package-manager-winget",
				".",
			],
		},
	);
	assert.deepEqual(
		resolveGoTask("dev-build", ["devex-plugin-package-manager-winget", "."], {
			env: {},
			platform: "win32",
			arch: "arm64",
		}),
		{
			command: "go",
			args: ["build", "-o", "devex-plugin-package-manager-winget", "."],
		},
	);
	assert.deepEqual(
		resolveGoTask("dev-build", ["devex-plugin-package-manager-winget", "."], {
			env: { GOOS: "windows", GOARCH: "arm64" },
			platform: "win32",
			arch: "x64",
		}),
		{
			command: "go",
			args: ["build", "-o", "devex-plugin-package-manager-winget", "."],
		},
	);
	const linkDir = fs.mkdtempSync(path.join(os.tmpdir(), "devex-go-task-link-"));
	fixtures.push(linkDir);
	const symlink = path.join(linkDir, "go-task-link.mjs");
	const goTaskPath = path.join(repo, "scripts/turbo/go-task.mjs");
	fs.symlinkSync(goTaskPath, symlink);
	assert.equal(isDirectExecution(symlink), true);
	assert.equal(
		isDirectExecution(path.join(repo, "scripts/turbo/run.mjs")),
		false,
	);
	assert.equal(isDirectExecution(undefined), false);
});
