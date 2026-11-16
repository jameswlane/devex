import { describe, expect, it } from "vitest";
import { createPlatformInfo } from "../../scripts/seed";

describe("seed createPlatformInfo", () => {
	it("stores the snake_case platform keys the sync and API use", () => {
		const alternatives = [{ install_method: "snap", install_command: "x" }];
		expect(
			createPlatformInfo({
				installMethod: "apt",
				installCommand: "docker-ce",
				officialSupport: true,
				alternatives,
			}),
		).toEqual({
			install_method: "apt",
			install_command: "docker-ce",
			official_support: true,
			alternatives,
		});
	});

	it("defaults official_support and alternatives", () => {
		expect(
			createPlatformInfo({ installMethod: "brew", installCommand: "git" }),
		).toEqual({
			install_method: "brew",
			install_command: "git",
			official_support: false,
			alternatives: [],
		});
		expect(createPlatformInfo(null)).toBeNull();
	});
});
