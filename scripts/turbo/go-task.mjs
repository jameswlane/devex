import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { goFingerprint, version } from "./toolchain.mjs";

const ARCH_MAP = { x64: "amd64", ia32: "386" };

export function resolveGoTask(
	command,
	args,
	{ env = process.env, platform = process.platform, arch = process.arch } = {},
) {
	if (command === "vulncheck") {
		// Pin the scanner, but always fetch the current vulnerability database.
		return {
			command: "go",
			args: ["run", "golang.org/x/vuln/cmd/govulncheck@v1.9.0", "./..."],
		};
	}
	if (command === "dev-build") {
		const [output, pkg = "."] = args;
		if (!output) {
			throw new Error("dev-build requires an output binary path");
		}
		const goos = env.GOOS || (platform === "win32" ? "windows" : platform);
		const goarch = env.GOARCH || ARCH_MAP[arch] || arch;
		// Go supports -race on windows/amd64, not windows/arm64.
		const race = goos === "windows" && goarch === "arm64" ? [] : ["-race"];
		return { command: "go", args: ["build", ...race, "-o", output, pkg] };
	}
	return { command, args };
}

function main() {
	try {
		const [rawCommand, ...rawArgs] = process.argv.slice(2);
		// biome-ignore lint/suspicious/noUndeclaredEnvVars: Turbo injects its task hash at execution time.
		if (process.env.TURBO_HASH) {
			const actualGo = goFingerprint(process.cwd());
			// biome-ignore lint/suspicious/noUndeclaredEnvVars: Declared by the shared @devex/turbo-go configuration.
			const expectedGo = JSON.parse(process.env.DEVEX_GO_TOOLCHAIN || "[]");
			if (!expectedGo.includes(actualGo))
				throw new Error(
					"Use pnpm turbo:run (or root build/test/lint scripts) to hash the active Go toolchain before running Turbo tasks",
				);
			if (rawCommand === "golangci-lint") {
				const actualLint = version(rawCommand, ["version"], process.cwd());
				// biome-ignore lint/suspicious/noUndeclaredEnvVars: Declared on lint in the shared @devex/turbo-go configuration.
				if (actualLint !== process.env.DEVEX_GOLANGCI_VERSION)
					throw new Error(
						"Use pnpm turbo:run to hash the active golangci-lint version",
					);
			}
		}
		const { command, args } = resolveGoTask(rawCommand, rawArgs);
		const result = spawnSync(command, args, { stdio: "inherit" });
		if (result.error) throw result.error;
		process.exitCode = result.status || (result.signal ? 1 : 0);
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}

if (
	process.argv[1] &&
	path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
	main();
}
