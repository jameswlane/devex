import { execFileSync } from "node:child_process";

export function version(command, args, cwd, env = process.env) {
	return execFileSync(command, args, {
		cwd,
		env,
		encoding: "utf8",
		stdio: ["ignore", "pipe", "pipe"],
	}).trim();
}

// Keep the fingerprint and execution guard on the same effective Go settings.
// go env includes persistent GOENV overrides as well as process environment values.
export const goSettings = [
	"GOVERSION",
	"GOOS",
	"GOARCH",
	"CGO_ENABLED",
	"GOFLAGS",
	"GOAMD64",
	"GOARM",
	"GOARM64",
	"GOEXPERIMENT",
	"GOTOOLCHAIN",
	"GOWORK",
	"CC",
	"CXX",
	"CGO_CFLAGS",
	"CGO_CPPFLAGS",
	"CGO_CXXFLAGS",
	"CGO_LDFLAGS",
	"GOENV",
	"GOROOT",
	"GOPATH",
	"GO111MODULE",
];

export function goFingerprint(directory, env = process.env) {
	return version("go", ["env", ...goSettings], directory, env);
}

// Only probe executable modules in the selected graph. Module directives can
// resolve different toolchains, including toolchains unavailable offline.
export function toolchainEnv(
	root,
	{ directories = [], lint },
	env = process.env,
) {
	const result = { ...env };
	// Never accept caller-supplied fingerprints in place of the actual executables.
	delete result.DEVEX_GO_TOOLCHAIN;
	delete result.DEVEX_GOLANGCI_VERSION;
	if (directories.length) {
		const versions = [...new Set(directories)]
			.sort()
			.map((directory) => goFingerprint(directory, env));
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
