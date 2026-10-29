#!/usr/bin/env node
// Works out which components need a release and what their next version is.
//
// A component is the CLI (apps/cli, tags vX.Y.Z), the plugin SDK
// (packages/plugin-sdk, tags packages/plugin-sdk/vX.Y.Z) or a plugin
// (packages/<name>, tags packages/<name>/vX.Y.Z). For each one, commits that touch
// its directory since its last tag decide the bump, using conventional commits:
//   feat -> minor, fix/perf/anything else -> patch, "!" or BREAKING CHANGE -> major
//   (while a component is 0.x, a breaking change bumps minor instead of major).
//
// Usage:
//   node scripts/release/release-plan.mjs                 # print the plan
//   node scripts/release/release-plan.mjs --json          # machine-readable plan
//   node scripts/release/release-plan.mjs --only cli,package-manager-apt
//   node scripts/release/release-plan.mjs --tag           # create annotated tags locally
import { execFileSync } from "node:child_process";
import { existsSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(
	path.dirname(fileURLToPath(import.meta.url)),
	"../..",
);
const SEMVER = /^(\d+)\.(\d+)\.(\d+)$/;
// Paths that never warrant a release on their own
const IGNORED = [/\.md$/, /(^|\/)node_modules\//, /(^|\/)package\.json$/];

function git(...args) {
	return execFileSync("git", args, { cwd: repoRoot, encoding: "utf8" }).trim();
}

export function components() {
	const list = [{ name: "cli", dir: "apps/cli", prefix: "v" }];
	for (const entry of readdirSync(path.join(repoRoot, "packages"), {
		withFileTypes: true,
	})) {
		if (!entry.isDirectory()) continue;
		const dir = `packages/${entry.name}`;
		if (existsSync(path.join(repoRoot, dir, ".goreleaser.yml"))) {
			list.push({ name: entry.name, dir, prefix: `${dir}/v` });
		}
	}
	return list.sort((a, b) =>
		a.name === "cli" ? -1 : b.name === "cli" ? 1 : a.name.localeCompare(b.name),
	);
}

function parseVersion(version) {
	const m = SEMVER.exec(version);
	return m ? m.slice(1).map(Number) : null;
}

function latestTag(prefix) {
	const tags = git("tag", "--list", `${prefix}*`)
		.split("\n")
		.filter(Boolean)
		.map((tag) => ({ tag, version: parseVersion(tag.slice(prefix.length)) }))
		.filter((t) => t.version);
	tags.sort(
		(a, b) =>
			a.version[0] - b.version[0] ||
			a.version[1] - b.version[1] ||
			a.version[2] - b.version[2],
	);
	return tags.at(-1) ?? null;
}

function commitsSince(tag, dir) {
	const range = tag ? `${tag}..HEAD` : "HEAD";
	const out = git("log", range, "--format=%H%x1f%s%x1f%b%x1e", "--", dir);
	return out
		.split("\x1e")
		.map((record) => record.trim())
		.filter(Boolean)
		.map((record) => {
			const [sha, subject, body = ""] = record.split("\x1f");
			return { sha, subject, body };
		})
		.filter(({ sha }) => {
			const files = git(
				"diff-tree",
				"--no-commit-id",
				"--name-only",
				"-r",
				sha,
				"--",
				dir,
			)
				.split("\n")
				.filter(Boolean);
			return files.some((file) => !IGNORED.some((re) => re.test(file)));
		});
}

export function bumpFor(commits) {
	let bump = null;
	const rank = { patch: 1, minor: 2, major: 3 };
	for (const { subject, body } of commits) {
		let level = "patch";
		if (
			/^\w+(\([^)]*\))?!:/.test(subject) ||
			/^BREAKING[ -]CHANGE:/m.test(body)
		)
			level = "major";
		else if (/^feat(\([^)]*\))?:/.test(subject)) level = "minor";
		if (!bump || rank[level] > rank[bump]) bump = level;
	}
	return bump;
}

export function nextVersion(current, bump) {
	const [major, minor, patch] = current ?? [0, 0, 0];
	if (!current) return "0.1.0";
	if (bump === "major")
		return major === 0 ? `0.${minor + 1}.0` : `${major + 1}.0.0`;
	if (bump === "minor") return `${major}.${minor + 1}.0`;
	return `${major}.${minor}.${patch + 1}`;
}

export function plan(only) {
	return components()
		.filter((c) => !only || only.includes(c.name))
		.map((c) => {
			const last = latestTag(c.prefix);
			const commits = commitsSince(last?.tag, c.dir);
			const bump = bumpFor(commits);
			const version = bump ? nextVersion(last?.version, bump) : null;
			return {
				name: c.name,
				dir: c.dir,
				previousTag: last?.tag ?? null,
				commits: commits.length,
				bump,
				tag: version ? `${c.prefix}${version}` : null,
			};
		});
}

function main() {
	const args = process.argv.slice(2);
	const onlyIndex = args.indexOf("--only");
	const only =
		onlyIndex >= 0 ? args[onlyIndex + 1]?.split(",").filter(Boolean) : null;
	const result = plan(only);
	const releases = result.filter((r) => r.tag);

	if (args.includes("--json")) {
		console.log(JSON.stringify(releases));
	} else {
		for (const r of result) {
			const status = r.tag ? `${r.bump.padEnd(5)} -> ${r.tag}` : "no changes";
			console.log(
				`${r.name.padEnd(28)} ${String(r.commits).padStart(3)} commit(s)  ${status}`,
			);
		}
	}

	if (args.includes("--tag")) {
		const head = git("rev-parse", "HEAD");
		for (const r of releases) {
			git("tag", "-a", r.tag, "-m", `Release ${r.tag}`, head);
			console.error(`tagged ${r.tag} at ${head.slice(0, 12)}`);
		}
		if (releases.length)
			console.error(
				"Push with: git push origin " + releases.map((r) => r.tag).join(" "),
			);
	}
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
	main();
}
