import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { toolchainEnv } from "./toolchain.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
const turbo = path.join(root, "node_modules/turbo/bin/turbo");
const args = process.argv.slice(2);
const invoke = (args, options = {}) =>
	spawnSync(process.execPath, [turbo, ...args], {
		cwd: root,
		stdio: "inherit",
		maxBuffer: 64 * 1024 * 1024,
		...options,
	});

try {
	// Inspect the actual graph before probing tools: Next-only work needs no Go
	// installation, and Go build/test needs no golangci-lint installation.
	const planArgs = args.filter((arg) => !/^--dry(?:-run)?(?:=|$)/.test(arg));
	const separator = planArgs.indexOf("--");
	planArgs.splice(separator < 0 ? planArgs.length : separator, 0, "--dry=json");
	const plan = invoke(planArgs, { encoding: "utf8", stdio: "pipe" });
	if (plan.error) throw plan.error;
	if (plan.status !== 0) {
		process.stdout.write(plan.stdout || "");
		process.stderr.write(plan.stderr || plan.error?.message || "");
		process.exit(plan.status || 1);
	}
	const tasks = JSON.parse(plan.stdout).tasks;
	const goTasks = tasks.filter(
		(task) =>
			task.command !== "<NONEXISTENT>" &&
			existsSync(path.join(root, task.directory, "go.mod")),
	);
	const env = toolchainEnv(root, {
		directories: goTasks.map((task) => path.join(root, task.directory)),
		lint: goTasks.some((task) => task.task === "lint"),
	});
	const result = invoke(args, { env });
	if (result.error) throw result.error;
	process.exit(result.status || (result.signal ? 1 : 0));
} catch (error) {
	console.error(`Turbo toolchain setup failed: ${error.message}`);
	process.exitCode = 1;
}
