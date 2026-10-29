import { describe, expect, it } from "@jest/globals";
import { isGitHubUrl } from "@/lib/github-url";

describe("isGitHubUrl", () => {
	it.each([
		["https://github.com/devex/devex", true],
		["http://github.com/devex/devex", false],
		["https://evil.com/github.com", false],
		["https://github.com.evil.com/devex/devex", false],
		["https://notgithub.com/devex/devex", false],
		["https://www.github.com/devex/devex", false],
		["", false],
		["garbage", false],
	])("returns the expected result for %s", (value, expected) => {
		expect(isGitHubUrl(value)).toBe(expected);
	});
});
