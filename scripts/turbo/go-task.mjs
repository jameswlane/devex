import { spawnSync } from "node:child_process";
import { goFingerprint, version } from "./toolchain.mjs";

try {
	let [command, ...args] = process.argv.slice(2);
	// biome-ignore lint/suspicious/noUndeclaredEnvVars: Turbo injects its task hash at execution time.
	if (process.env.TURBO_HASH) {
		const actualGo = goFingerprint(process.cwd());
		// biome-ignore lint/suspicious/noUndeclaredEnvVars: Declared by the shared @devex/turbo-go configuration.
		const expectedGo = JSON.parse(process.env.DEVEX_GO_TOOLCHAIN || "[]");
		if (!expectedGo.includes(actualGo))
			throw new Error(
				"Use pnpm turbo:run (or root build/test/lint scripts) to hash the active Go toolchain before running Turbo tasks",
			);
		if (command === "golangci-lint") {
			const actualLint = version(command, ["version"], process.cwd());
			// biome-ignore lint/suspicious/noUndeclaredEnvVars: Declared on lint in the shared @devex/turbo-go configuration.
			if (actualLint !== process.env.DEVEX_GOLANGCI_VERSION)
				throw new Error(
					"Use pnpm turbo:run to hash the active golangci-lint version",
				);
		}
	}
	if (command === "vulncheck") {
		// Pin the scanner, but always fetch the current vulnerability database.
		command = "go";
		args = ["run", "golang.org/x/vuln/cmd/govulncheck@v1.9.0", "./..."];
	}
	const result = spawnSync(command, args, { stdio: "inherit" });
	if (result.error) throw result.error;
	process.exitCode = result.status || (result.signal ? 1 : 0);
} catch (error) {
	console.error(error.message);
	process.exitCode = 1;
}
