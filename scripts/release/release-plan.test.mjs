// Tests for the pure versioning rules in release-plan.mjs that the
// release-components workflow relies on. Run: node --test scripts/release/
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { bumpFor, nextVersion } from "./release-plan.mjs";

describe("bumpFor", () => {
	it("returns null for no commits", () => {
		assert.equal(bumpFor([]), null);
	});

	it("bumps patch for fix and other non-feat commits", () => {
		assert.equal(
			bumpFor([{ subject: "fix: correct apt key path", body: "" }]),
			"patch",
		);
		assert.equal(bumpFor([{ subject: "chore: tidy up", body: "" }]), "patch");
		assert.equal(
			bumpFor([{ subject: "docs(readme): typo", body: "" }]),
			"patch",
		);
	});

	it("bumps minor for feat commits", () => {
		assert.equal(
			bumpFor([{ subject: "feat: add dnf backend", body: "" }]),
			"minor",
		);
		assert.equal(
			bumpFor([{ subject: "feat(registry): scoped feature", body: "" }]),
			"minor",
		);
	});

	it("bumps major for breaking changes via ! or BREAKING CHANGE footer", () => {
		assert.equal(
			bumpFor([{ subject: "feat!: drop v1 config", body: "" }]),
			"major",
		);
		assert.equal(
			bumpFor([{ subject: "fix(api)!: change return type", body: "" }]),
			"major",
		);
		assert.equal(
			bumpFor([
				{ subject: "fix: x", body: "text\nBREAKING CHANGE: new format" },
			]),
			"major",
		);
		assert.equal(
			bumpFor([{ subject: "fix: x", body: "BREAKING-CHANGE: new format" }]),
			"major",
		);
	});

	it("takes the highest bump across commits", () => {
		const commits = [
			{ subject: "fix: one", body: "" },
			{ subject: "feat: two", body: "" },
			{ subject: "fix: three", body: "" },
		];
		assert.equal(bumpFor(commits), "minor");
		assert.equal(
			bumpFor([...commits, { subject: "feat!: four", body: "" }]),
			"major",
		);
	});
});

describe("nextVersion", () => {
	it("starts at 0.1.0 when there is no previous tag", () => {
		assert.equal(nextVersion(null, "major"), "0.1.0");
		assert.equal(nextVersion(null, "patch"), "0.1.0");
	});

	it("applies patch/minor/major bumps", () => {
		assert.equal(nextVersion([1, 2, 3], "patch"), "1.2.4");
		assert.equal(nextVersion([1, 2, 3], "minor"), "1.3.0");
		assert.equal(nextVersion([2, 0, 0], "major"), "3.0.0");
	});

	it("treats breaking changes as minor while the component is 0.x", () => {
		assert.equal(nextVersion([0, 4, 2], "major"), "0.5.0");
	});
});
