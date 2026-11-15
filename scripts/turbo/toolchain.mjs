import { execFileSync } from "node:child_process";
import { existsSync, readdirSync } from "node:fs";
import path from "node:path";

export function version(command, args, cwd, env = process.env) {
	return execFileSync(command, args, {
		cwd,
		env,
		encoding: "utf8",
		stdio: ["ignore", "pipe", "pipe"],
	}).trim();
}

export function goModules(root) {
	return ["apps", "packages"].flatMap((parent) =>
		readdirSync(path.join(root, parent), { withFileTypes: true })
			.filter((entry) => entry.isDirectory())
			.map((entry) => path.join(root, parent, entry.name))
			.filter((directory) => existsSync(path.join(directory, "go.mod"))),
	);
}

// Query within every module: GOTOOLCHAIN=auto may resolve a different Go version
// from a module's go/toolchain directives than `go version` at the repository root.
export function toolchainEnv(root, { go, lint }, env = process.env) {
	const result = { ...env };
	// Never accept caller-supplied fingerprints in place of the actual executables.
	delete result.DEVEX_GO_TOOLCHAIN;
	delete result.DEVEX_GOLANGCI_VERSION;
	if (go) {
		const versions = goModules(root).map((directory) =>
			version("go", ["env", "GOVERSION"], directory, env),
		);
		result.DEVEX_GO_TOOLCHAIN = JSON.stringify([...new Set(versions)].sort());
	}
	if (lint)
		result.DEVEX_GOLANGCI_VERSION = version(
			"golangci-lint",
			["version"],
			root,
			env,
		);
	return result;
}
